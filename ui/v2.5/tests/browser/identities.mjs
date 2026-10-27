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
    await page.route("**/booru-browser/sources", (route) =>
      reply(route, [
        { id: "gelbooru", name: "Gelbooru", instance: "gel" },
        { id: "pawchive", name: "Pawchive", instance: "paw" },
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
      if (endpoint === "candidates")
        return reply(route, {
          items:
            rejected && !q.has("rejected")
              ? []
              : [{ id: 7, name: canonical, disambiguation: "", rejected }],
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
    await page
      .getByRole("button", { name: "Review Artist", exact: true })
      .first()
      .click();
    await page
      .getByRole("button", { name: "Select matching labels on this page" })
      .click();
    await page
      .getByRole("heading", { name: "Connect 3 Artist identifiers" })
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
    await page
      .getByRole("link", { name: "Connected to Waligner", exact: true })
      .first()
      .click();
    await page.getByText("Sources & aliases", { exact: true }).click();
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
    await connections
      .getByLabel("Exact provider tag or creator expression")
      .fill("creator:patreon:49481727");
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
    await page
      .getByRole("menuitem", { name: "Connect Artist", exact: true })
      .click();
    await page
      .getByRole("heading", {
        name: "Connect 1 Artist identifier",
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
      .getByRole("button", { name: "Different Artist", exact: true })
      .click();
    await page.getByText("Decision saved.", { exact: true }).waitFor();
    await page
      .getByRole("button", { name: "Review Artist", exact: true })
      .click();
    await page.getByText("No matching records.", { exact: false }).waitFor();
    await page.getByText("Show rejected matches", { exact: true }).click();
    await page.getByRole("button", { name: "Allow suggestion again" }).click();
    await page.getByText("Decision saved.", { exact: true }).waitFor();
    await page
      .getByRole("button", { name: "Review Artist", exact: true })
      .click();
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
      .getByRole("button", { name: "Scan saved metadata", exact: true })
      .click();
    await page
      .getByRole("button", { name: "Cancel scan", exact: true })
      .click();
    await page.getByText(/Scan cancelled:/).waitFor();
    await page
      .getByRole("button", { name: "Scan saved metadata", exact: true })
      .click();
    scan = {
      ...scan,
      status: "completed",
      scanned: 10,
      observations: 8,
      skipped: 2,
    };
    await page
      .getByText(/Scan completed: 10 records, 8 observations, 2 skipped/)
      .waitFor();
    await page
      .getByRole("button", { name: "Show all Artists", exact: true })
      .click();
    await page
      .getByRole("button", { name: "Review Artist", exact: true })
      .first()
      .waitFor();
    await page
      .getByRole("button", { name: "Review Artist", exact: true })
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
      .getByRole("button", { name: "Review Artist", exact: true })
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
