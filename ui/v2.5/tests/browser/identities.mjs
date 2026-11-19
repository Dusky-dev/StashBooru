import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { createServer } from "vite";

const engines = await import(process.env.PLAYWRIGHT_MODULE ?? "playwright");
const server = await createServer({
  root: path.resolve(import.meta.dirname, "../.."),
  server: { host: "127.0.0.1", port: 0 },
  plugins: [
    {
      name: "identity-fixture",
      configureServer(app) {
        app.middlewares.use((req, _res, next) => {
          if (/^\/(stash\/)?(identities|booru|studios\/7)(\?|$)/.test(req.url))
            req.url = `/tests/browser/fixtures/identities${req.url.startsWith("/stash/") ? "-prefix" : ""}.html`;
          next();
        });
      },
    },
  ],
});
await server.listen();
const origin = `http://127.0.0.1:${server.httpServer.address().port}`;
const browser = await engines.chromium.launch({
  headless: true,
  executablePath: process.env.CHROMIUM_EXECUTABLE,
  args: ["--no-sandbox", "--disable-dev-shm-usage", "--disable-gpu"],
});
try {
  for (const viewport of [
    { width: 1440, height: 1000 },
    { width: 390, height: 844 },
  ]) {
    const prefix = viewport.width < 500 ? "/stash" : "";
    const page = await browser.newPage({ viewport });
    const errors = [];
    page.on("pageerror", (e) => errors.push(e.message));
    const rows = [
      {
        id: "1",
        sourceID: "r34",
        sourceName: "Rule34",
        instance: "r34",
        target: "waligner",
        label: "waligner",
        form: "tag",
      },
      {
        id: "2",
        sourceID: "gelbooru",
        sourceName: "Gelbooru",
        instance: "gel",
        target: "waligner_(artist)",
        label: "waligner_(artist)",
        form: "tag",
      },
      {
        id: "3",
        sourceID: "pawchive",
        sourceName: "Pawchive",
        instance: "paw",
        target: "creator:patreon:49481727",
        label: "Waligner",
        form: "account",
      },
    ].map((row) => ({
      ...row,
      kind: "artist",
      version: 1,
      entityID: 0,
      entityName: "",
      context: "",
    }));
    let rejected = false,
      failNext = false,
      mutations = [];
    // A separate Artist created from the Gelbooru spelling before connecting.
    let splitMerged = false;
    const merges = [];
    await page.route("**/studio/merge", (route) => {
      merges.push(route.request().postDataJSON());
      splitMerged = true;
      return route.fulfill({
        contentType: "application/json",
        body: JSON.stringify({ destinationID: 7 }),
      });
    });
    let holdObserve = false,
      releaseObserve,
      observed;
    let scan = {
      status: "idle",
      jobID: 0,
      scanned: 0,
      observations: 0,
      skipped: 0,
    };
    const reply = (route, data, status = 200) =>
      route.fulfill({
        status,
        contentType: "application/json",
        body: JSON.stringify(data),
      });
    // Only Rule34 has an icon; the others fall back to the globe.
    const iconRequests = [];
    await page.route("**/booru-browser/icon?*", (route) => {
      const source = new URL(route.request().url()).searchParams.get("source");
      iconRequests.push(source);
      if (source !== "r34")
        return route.fulfill({ status: 404, body: "no icon" });
      return route.fulfill({
        contentType: "image/png",
        body: Buffer.from(
          "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==",
          "base64"
        ),
      });
    });
    // One saved follow: Gelbooru Character A. Other tags offer Follow.
    await page.route("**/following/follows", (route) =>
      route.fulfill({
        contentType: "application/json",
        body: JSON.stringify([
          {
            id: "C".repeat(26),
            instance: "gel",
            kind: "character",
            target: "character_a",
            active: true,
          },
        ]),
      })
    );
    await page.route("**/booru-browser/sources", (route) =>
      reply(route, [
        {
          id: "gelbooru",
          name: "Gelbooru",
          instance: "gel",
          engine: "gelbooru",
        },
        {
          id: "pawchive",
          name: "Pawchive",
          instance: "paw",
          engine: "pawchive",
        },
      ])
    );
    await page.route("**/identities/*", async (route) => {
      const url = new URL(route.request().url());
      const endpoint = url.pathname.split("/").pop();
      const q = url.searchParams;
      if (endpoint === "list") {
        const items = rows.filter(
          (row) =>
            (!q.get("kind") || row.kind === q.get("kind")) &&
            (!q.get("identity") || row.id === q.get("identity")) &&
            (!q.get("entity") || row.entityID === Number(q.get("entity"))) &&
            (!q.get("state") ||
              (q.get("state") === "linked" ? !!row.entityID : !row.entityID)) &&
            (!q.get("q") || row.target.includes(q.get("q")))
        );
        return reply(route, { items, next: "" });
      }
      const canonical =
        q.get("kind") === "copyright"
          ? "Series A"
          : q.get("kind") === "character"
            ? "Character A"
            : "Waligner";
      if (endpoint === "split")
        return reply(route, {
          items:
            q.get("kind") === "artist" &&
            q.get("entity") === "7" &&
            !splitMerged
              ? [
                  {
                    id: 9,
                    name: "waligner (artist)",
                    disambiguation: "",
                    rejected: false,
                  },
                ]
              : [],
        });
      if (endpoint === "candidates")
        return reply(route, {
          items: [
            ...(rejected && !q.has("rejected")
              ? []
              : [{ id: 7, name: canonical, disambiguation: "", rejected }]),
          ],
          truncated: false,
        });
      if (endpoint === "scan") {
        if (route.request().method() === "POST") {
          const data = route.request().postDataJSON();
          scan = {
            ...scan,
            jobID: 8,
            status: data.action === "start" ? "running" : "cancelled",
          };
        }
        return reply(route, scan);
      }
      if (endpoint === "update") {
        const data = route.request().postDataJSON();
        mutations.push(data);
        if (failNext) {
          failNext = false;
          rows[1].version++;
          return reply(
            route,
            {
              error: "An identity or target changed. Reload and review again.",
            },
            409
          );
        }
        if (data.action === "observe") {
          if (holdObserve)
            await new Promise((resolve) => {
              releaseObserve = resolve;
              observed();
            });
          holdObserve = false;
          const row = rows.find(
            (row) => row.target === data.target && row.kind === data.kind
          );
          assert.ok(row, "fixture observes an existing source name");
          return reply(route, row);
        }
        if (data.action === "reject") rejected = true;
        if (data.action === "restore") rejected = false;
        for (const change of data.changes ?? []) {
          const row = rows.find((row) => row.id === change.id);
          assert.equal(row.version, change.version);
          row.version++;
          if (data.action === "link" || data.action === "create") {
            row.entityID = data.action === "create" ? 8 : 7;
            row.entityName =
              data.action === "create"
                ? data.name
                : data.kind === "copyright"
                  ? "Series A"
                  : data.kind === "character"
                    ? "Character A"
                    : "Waligner";
          }
          if (data.action === "unlink") {
            row.entityID = 0;
            row.entityName = "";
          }
        }
        return reply(
          route,
          rows.filter((row) =>
            data.changes.some((change) => change.id === row.id)
          )
        );
      }
      return reply(route, { error: "Unexpected fixture endpoint" }, 500);
    });
    await page.goto(`${origin}${prefix}/identities?kind=artist`);
    // Each row is headed by its own site name, with one action and no
    // duplicate "Select" switch; the empty panel says what to do.
    await page
      .getByRole("heading", { name: "Waligner", exact: true })
      .waitFor();
    assert.equal(
      await page
        .getByRole("heading", { name: "waligner", exact: true })
        .count(),
      2
    );
    assert.equal(
      await page.getByRole("button", { name: "Connect", exact: true }).count(),
      3
    );
    assert.equal(await page.locator(".identity-row .custom-switch").count(), 0);
    await page
      .getByText("Choose Connect on a site name", { exact: false })
      .waitFor();
    // Ticking a second row, then unticking it, must not crash the page.
    await page.locator(".identity-row input[type=checkbox]").nth(0).check();
    await page.locator(".identity-row input[type=checkbox]").nth(1).check();
    await page.locator(".identity-row input[type=checkbox]").nth(1).uncheck();
    await page
      .getByRole("heading", { name: "Connect 1 site name to an Artist" })
      .waitFor();
    await page.getByRole("button", { name: "Clear selection" }).click();
    await page
      .getByRole("button", { name: /^(Connect|Change)$/ })
      .first()
      .click();
    await page
      .getByRole("button", { name: "Also select other sites with this name" })
      .click();
    await page
      .getByRole("heading", { name: "Connect 3 site names to an Artist" })
      .waitFor();
    if (viewport.width < 500) {
      const box = await page.getByRole("complementary").boundingBox();
      assert.ok(
        box.y < viewport.height,
        "Review scrolls mobile resolution into view"
      );
    }
    await page
      .getByRole("button", { name: "Connect to Waligner", exact: true })
      .click();
    await page.getByText("Decision saved.", { exact: true }).waitFor();
    assert.equal(mutations.at(-1).changes.length, 3);
    assert.ok(rows.every((row) => row.entityID === 7));
    await page.reload();
    await page
      .getByRole("link", { name: "Connected to Waligner", exact: true })
      .first()
      .waitFor();
    assert.equal(
      await page.getByRole("button", { name: "Change", exact: true }).count(),
      3,
      "Connected rows offer Change instead of Connect"
    );
    assert.equal(
      await page
        .getByRole("heading", { name: "waligner", exact: true })
        .count(),
      2,
      "Connected rows keep their own site name"
    );
    await page
      .getByRole("link", { name: "Connected to Waligner", exact: true })
      .first()
      .click();
    await page.locator(".identity-sources > summary").click();
    await page.getByText("creator:patreon:49481727", { exact: true }).waitFor();
    // Connected tag identities show their site and open the Booru browser.
    const siteLinks = page.locator(".identity-sources a.identity-site-link");
    // The site's own icon, fetched through this server; a globe otherwise.
    await page
      .locator('.identity-sources img.site-icon[src*="source=r34"]')
      .waitFor();
    await page.waitForFunction(
      () =>
        document.querySelectorAll(".identity-sources .identity-site-link svg")
          .length === 2
    );
    assert.ok(
      iconRequests.every((source) => /^[a-z0-9_-]+$/.test(source)),
      "icons are requested by source ID only"
    );
    assert.deepEqual(
      await siteLinks.evaluateAll((links) =>
        links.map((link) =>
          link.getAttribute("href").replace(/^.*(?=\/booru\?)/, "")
        )
      ),
      [
        "/booru?source=r34&q=waligner",
        "/booru?source=gelbooru&q=waligner_%28artist%29",
        "/booru?source=pawchive&q=creator%3Apatreon%3A49481727",
      ]
    );
    if (process.env.STASH_BROWSER_SCREENSHOTS)
      await page.locator(".identity-sources").screenshot({
        path: path.join(
          process.env.STASH_BROWSER_SCREENSHOTS,
          `identity-sites-${page.viewportSize().width}.png`
        ),
      });
    await siteLinks.first().click();
    await page.waitForURL(/\/booru\?source=r34&q=waligner$/);
    await page.goBack();
    await page.locator(".identity-sources").waitFor();
    if (!(await page.locator(".identity-sources").evaluate((d) => d.open)))
      await page.locator(".identity-sources > summary").click();
    await siteLinks.first().waitFor();
    // Pawchive accounts open the creator's posts in the Booru browser too.
    assert.equal(
      await page.locator(".identity-sources span.identity-site-link").count(),
      0
    );
    // A separate record named after a connected source name can be merged
    // here, only after an explicit confirmation.
    const split = page.locator(".identity-split-records");
    await split
      .getByRole("link", { name: "waligner (artist)", exact: true })
      .waitFor();
    const mergeSplit = split.getByRole("button", {
      name: "Merge into Waligner…",
    });
    await mergeSplit.click();
    const confirmMerge = page.getByRole("dialog");
    await confirmMerge
      .getByText(
        "Waligner is kept. waligner (artist) is removed, and its Images, Videos and source connections move to Waligner."
      )
      .waitFor();
    await confirmMerge.getByRole("button", { name: "Cancel" }).click();
    await confirmMerge.waitFor({ state: "detached" });
    assert.equal(merges.length, 0, "Cancel does not merge");
    await mergeSplit.click();
    await confirmMerge
      .getByRole("button", { name: "Merge", exact: true })
      .click();
    await split.waitFor({ state: "detached" });
    assert.deepEqual(merges, [{ sourceID: 9, destinationID: 7 }]);
    await page.getByText("creator:patreon:49481727", { exact: true }).waitFor();
    const profileURL = page.url();
    await page.getByRole("button", { name: "Manage connections" }).click();
    const connections = page.getByRole("dialog");
    await connections
      .getByText("creator:patreon:49481727", { exact: true })
      .waitFor();
    assert.equal(
      page.url(),
      profileURL,
      "Managing connections keeps the native profile"
    );
    const beforeCancel = mutations.length;
    await connections
      .getByRole("button", { name: "Close", exact: true })
      .last()
      .click();
    assert.equal(mutations.length, beforeCancel, "Closing makes no decisions");
    rows[2].entityID = 0;
    rows[2].entityName = "";
    await page.getByRole("button", { name: "Manage connections" }).click();
    await connections
      .getByRole("button", { name: "Add source name", exact: true })
      .click();
    await connections
      .getByLabel("Site", { exact: true })
      .selectOption("pawchive");
    // Artist account-key sites never match a plain name, so no shortcut there.
    const useName = connections.getByRole("button", {
      name: "Use “Waligner”",
      exact: true,
    });
    assert.equal(await useName.count(), 0);
    // The profile's own name can be added without retyping it on tag sites.
    const sourceName = connections.getByLabel(
      "Exact provider tag or creator expression"
    );
    await connections
      .getByLabel("Site", { exact: true })
      .selectOption("gelbooru");
    await useName.click();
    assert.equal(await sourceName.inputValue(), "Waligner");
    await connections
      .getByLabel("Site", { exact: true })
      .selectOption("pawchive");
    assert.equal(await useName.count(), 0);
    await sourceName.fill("creator:patreon:49481727");
    holdObserve = true;
    const heldObservation = new Promise((resolve) => {
      observed = resolve;
    });
    await connections
      .getByRole("button", { name: "Review connection", exact: true })
      .click();
    await heldObservation;
    assert.equal(
      await connections
        .getByRole("button", { name: "Close", exact: true })
        .last()
        .isDisabled(),
      true
    );
    assert.equal(
      await connections
        .getByRole("button", { name: "Cancel adding source name", exact: true })
        .isDisabled(),
      true
    );
    assert.equal(
      await connections.count(),
      1,
      "Write dialog exists before Escape"
    );
    await page.keyboard.press("Escape");
    assert.equal(
      await connections.count(),
      1,
      "A submitted write cannot be closed mid-flight"
    );
    releaseObserve();
    await connections
      .getByRole("button", { name: "Connect to Waligner", exact: true })
      .waitFor();
    assert.equal(
      rows[2].entityID,
      0,
      "Observation does not connect automatically"
    );
    await connections
      .getByRole("button", { name: "Connect to Waligner", exact: true })
      .click();
    await connections
      .getByText("Connected to Waligner.", { exact: true })
      .waitFor();
    assert.equal(rows[2].entityID, 7);
    assert.equal(page.url(), profileURL);
    await connections
      .getByRole("button", { name: "Close", exact: true })
      .last()
      .click();
    await page.getByRole("link", { name: "Browse fixture" }).click();
    const browseURL = page.url();
    await page.getByRole("button", { name: "Provider tag" }).click();
    // The Booru browser tag menu offers Follow beside Connect; it opens a
    // prefilled follow in a new tab and saves nothing by itself.
    const followItem = page.getByRole("menuitem", {
      name: "Follow waligner_(artist) on Gelbooru",
      exact: true,
    });
    assert.equal(
      await followItem.getAttribute("href"),
      `${prefix}/following?fromIdentity=2#following-manager`
    );
    assert.equal(await followItem.getAttribute("target"), "_blank");
    await page
      .getByRole("menuitem", { name: "Connect Artist", exact: true })
      .click();
    await page
      .getByRole("heading", {
        name: "Connect 1 site name to an Artist",
        exact: true,
      })
      .waitFor();
    assert.equal(
      page.url(),
      browseURL,
      "Preview connection does not navigate away"
    );
    await page
      .getByRole("dialog")
      .getByRole("button", { name: "Remove connection" })
      .click();
    await page.getByText("Connection removed.", { exact: true }).waitFor();
    assert.equal(rows[1].entityID, 0);
    await page.getByRole("link", { name: "Advanced reconciliation" }).click();
    await page
      .getByRole("button", { name: "Not this Artist", exact: true })
      .click();
    await page.getByText("Decision saved.", { exact: true }).waitFor();
    await page.getByRole("button", { name: /^(Connect|Change)$/ }).click();
    await page.getByText("No matching records.", { exact: false }).waitFor();
    await page.getByText("Show rejected matches", { exact: true }).click();
    await page.getByRole("button", { name: "Allow suggestion again" }).click();
    await page.getByText("Decision saved.", { exact: true }).waitFor();
    await page.getByRole("button", { name: /^(Connect|Change)$/ }).click();
    await page.getByText("Create a new Artist", { exact: true }).click();
    await page
      .getByLabel("Display name", { exact: true })
      .fill("Draft kept on conflict");
    failNext = true;
    await page
      .getByRole("button", { name: "Connect to Waligner", exact: true })
      .click();
    await page.getByRole("alert").filter({ hasText: "changed" }).waitFor();
    assert.equal(
      await page.getByLabel("Display name", { exact: true }).inputValue(),
      "Draft kept on conflict"
    );
    assert.equal(
      await page
        .getByRole("button", { name: "Connect to Waligner", exact: true })
        .isDisabled(),
      true
    );
    await page
      .getByRole("button", { name: "Reload latest connections", exact: true })
      .click();
    await page
      .getByText("Latest connections loaded.", { exact: false })
      .waitFor();
    assert.equal(
      await page.getByLabel("Display name", { exact: true }).inputValue(),
      "Draft kept on conflict"
    );
    await page
      .getByLabel("Display name", { exact: true })
      .fill("Curated display");
    await page.getByRole("button", { name: "Create and connect 1" }).click();
    await page
      .getByRole("link", { name: "Connected to Curated display", exact: true })
      .waitFor();
    await page
      .getByRole("button", { name: "Scan saved downloads", exact: true })
      .click();
    await page
      .getByRole("button", { name: "Cancel scan", exact: true })
      .click();
    await page.getByText(/Scan cancelled after/).waitFor();
    await page
      .getByRole("button", { name: "Scan saved downloads", exact: true })
      .click();
    scan = {
      ...scan,
      status: "completed",
      scanned: 10,
      observations: 8,
      skipped: 2,
    };
    await page
      .getByText(
        /Last scan finished: 10 records read, 8 site names found, 2 skipped/
      )
      .waitFor();
    await page
      .getByRole("button", { name: "Show all Artists", exact: true })
      .click();
    await page
      .getByRole("button", { name: /^(Connect|Change)$/ })
      .first()
      .waitFor();
    await page
      .getByRole("button", { name: /^(Connect|Change)$/ })
      .first()
      .click();
    await page
      .getByRole("button", { name: "Connect to Waligner", exact: true })
      .waitFor();
    assert.ok(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth + 1
      ),
      "No horizontal overflow"
    );
    if (process.env.STASH_BROWSER_SCREENSHOTS) {
      fs.mkdirSync(process.env.STASH_BROWSER_SCREENSHOTS, { recursive: true });
      await page.screenshot({
        path: path.join(
          process.env.STASH_BROWSER_SCREENSHOTS,
          `identities-${viewport.width}.png`
        ),
        fullPage: true,
      });
    }
    await page.getByRole("link", { name: "Browse fixture" }).click();
    await page.goBack();
    await page
      .getByRole("button", { name: /^(Connect|Change)$/ })
      .first()
      .waitFor();
    for (const [kind, label, id, target] of [
      ["character", "Character", "4", "character_a"],
      ["copyright", "Copyright", "5", "series_a"],
    ]) {
      rows.push({
        ...rows[0],
        id,
        kind,
        target,
        label: target,
        instance: "gel",
        sourceID: "gelbooru",
        sourceName: "Gelbooru",
        version: 1,
        entityID: 0,
        entityName: "",
      });
      await page.goto(
        `${origin}${prefix}/booru?kind=${kind}&identity=${id}&page=3&observe=1`
      );
      const retainedURL = page.url();
      await page
        .getByRole("button", { name: "Provider tag", exact: true })
        .click();
      if (kind === "character")
        assert.equal(
          new URL(
            await page
              .getByRole("menuitem", {
                name: "Following character_a on Gelbooru: show new posts",
              })
              .getAttribute("href"),
            origin
          ).searchParams.get("follow"),
          "C".repeat(26),
          "a followed tag links to its feed"
        );
      await page
        .getByRole("menuitem", { name: `Connect ${label}`, exact: true })
        .click();
      const dialog = page.getByRole("dialog");
      if (kind === "character") {
        await dialog
          .getByText("Create a new Character", { exact: true })
          .click();
        await dialog
          .getByLabel("Display name", { exact: true })
          .fill("Character A");
        await dialog.getByLabel("Disambiguation (optional)").fill("Series A");
        await dialog
          .getByRole("button", { name: "Create and connect 1", exact: true })
          .click();
        assert.equal(mutations.at(-1).disambiguation, "Series A");
      } else {
        await dialog
          .getByRole("button", { name: "Connect to Series A", exact: true })
          .click();
      }
      await dialog
        .getByText(
          `Connected to ${kind === "character" ? "Character A" : "Series A"}.`,
          { exact: true }
        )
        .waitFor();
      assert.equal(
        new URL(
          await dialog
            .getByRole("link", { name: "Advanced reconciliation", exact: true })
            .getAttribute("href"),
          page.url()
        ).searchParams.get("identity"),
        id,
        "Advanced reconciliation retains an observed identity after the decision"
      );
      assert.equal(page.url(), retainedURL);
      assert.equal(mutations.at(-1).kind, kind);
      assert.equal(mutations.at(-1).changes[0].id, id);
      await page.keyboard.press("Escape");
      await dialog.waitFor({ state: "detached" });
      await page.reload();
      assert.equal(page.url(), retainedURL);
    }
    assert.deepEqual(errors, []);
    await page.close();
    console.log(
      `${viewport.width}px identities, source prefix, bulk/rejection/conflict/scan and cleanup passed`
    );
  }
} finally {
  await browser.close();
  await server.close();
}
