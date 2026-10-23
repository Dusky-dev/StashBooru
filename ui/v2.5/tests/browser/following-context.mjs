import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { createServer } from "vite";
const { chromium } = await import(
  process.env.PLAYWRIGHT_MODULE ?? "playwright"
);
const server = await createServer({
  root: path.resolve(import.meta.dirname, "../.."),
  server: { host: "127.0.0.1", port: 0 },
  plugins: [
    {
      name: "following-context-fixture",
      configureServer(app) {
        app.middlewares.use((req, _res, next) => {
          const pathname = req.url.split("?")[0];
          if (/^\/(?:stash\/)?(?:following|profile|other)$/.test(pathname)) {
            const query = req.url.includes("?")
              ? req.url.slice(req.url.indexOf("?"))
              : "";
            req.url = `/tests/browser/fixtures/following${pathname.startsWith("/stash/") ? "-prefix" : ""}.html${query}`;
          }
          next();
        });
      },
    },
  ],
});
await server.listen();
const origin = `http://127.0.0.1:${server.httpServer.address().port}`;
const browser = await chromium.launch({
  headless: true,
  executablePath: process.env.CHROMIUM_EXECUTABLE,
  args: ["--no-sandbox", "--disable-dev-shm-usage", "--disable-gpu"],
});
const shots = process.env.STASH_BROWSER_SCREENSHOTS;
if (shots) fs.mkdirSync(shots, { recursive: true });
try {
  for (const [layout, viewport, prefix] of [
    ["desktop", { width: 1440, height: 1000 }, ""],
    ["mobile", { width: 390, height: 844 }, "/stash"],
  ]) {
    const context = await browser.newContext({ viewport });
    const errors = [];
    context.on("page", (page) => {
      page.on("pageerror", (error) => errors.push(error.message));
      page.on("console", (event) => {
        if (/unmounted component|Maximum update depth/i.test(event.text()))
          errors.push(event.text());
      });
    });
    const base = `${origin}${prefix}`;
    const requests = [];
    const wrongSource = {
      id: "original",
      instance: "other-instance",
      name: "Different site",
      engine: "danbooru",
      url: "https://other.example.org",
    };
    const matchingSource = {
      id: "renamed",
      instance: "saved-instance",
      name: "Renamed Danbooru",
      engine: "danbooru",
      url: "https://danbooru.example.org",
    };
    let sources = [wrongSource, matchingSource];
    let failIdentity = false;
    let holdIdentity = false;
    let releaseIdentity;
    let identityStarted;
    const follows = [];
    const identities = [
      ["artist", "waligner_(artist)", "Waligner"],
      ["character", "character_タグ_(a&b)", "Character A"],
      ["copyright", "series_(2026)", "Series A"],
    ].map(([kind, target, entityName], i) => ({
      id: String(i + 1),
      kind,
      target,
      entityName,
      entityID: 7,
      engine: "danbooru",
      form: "tag",
      sourceID: "original",
      instance: "saved-instance",
      sourceName: "Original Danbooru",
      sourceURL: matchingSource.url,
      label: target,
      version: 1,
    }));
    identities.push(
      {
        ...identities[0],
        id: "4",
        engine: "gelbooru",
        instance: "gel",
        sourceName: "Gelbooru",
      },
      {
        ...identities[0],
        id: "5",
        engine: "pawchive",
        form: "account",
        instance: "paw",
        sourceName: "Pawchive",
        target: "patreon_49481727",
      }
    );
    await context.route(
      /\/(?:identities\/list|booru-browser\/sources|following\/(?:follows|feed|sync))/,
      async (route) => {
        const request = route.request();
        const url = new URL(request.url());
        const body =
          request.method() === "POST" ? request.postDataJSON() : undefined;
        requests.push({
          path: url.pathname,
          query: url.searchParams,
          method: request.method(),
          body,
        });
        assert.ok(
          url.pathname.startsWith(`${prefix}/`),
          "API path lost the server prefix"
        );
        if (url.pathname.endsWith("/identities/list")) {
          const id = url.searchParams.get("identity");
          if (id && holdIdentity) {
            holdIdentity = false;
            await new Promise((resolve) => {
              releaseIdentity = resolve;
              identityStarted?.();
            });
          }
          if (failIdentity)
            return route.fulfill({
              status: 503,
              json: { error: "Connections unavailable" },
            });
          return route
            .fulfill({
              json: {
                items: identities.filter((row) =>
                  id ? row.id === id : row.kind === url.searchParams.get("kind")
                ),
                next: "",
              },
            })
            .catch(() => {});
        }
        if (url.pathname.endsWith("/sources"))
          return route.fulfill({ json: sources });
        if (url.pathname.endsWith("/feed"))
          return route.fulfill({ json: { items: [], next: "" } });
        if (url.pathname.endsWith("/follows") && !body)
          return route.fulfill({ json: follows });
        assert.equal(
          body?.action,
          "create",
          "context navigation must never sync or mutate an existing follow"
        );
        const follow = {
          ...body,
          id: String.fromCharCode(65 + follows.length).repeat(26),
          version: 1,
          queryRevision: 1,
          sourceURL: matchingSource.url,
          status: "available",
          syncSupported: true,
          active: true,
        };
        follows.push(follow);
        return route.fulfill({ json: follow });
      }
    );
    const writes = () =>
      requests.filter((request) => request.method === "POST");
    const profile = await context.newPage();
    for (const identity of identities.slice(0, 3)) {
      await profile.goto(`${base}/profile?kind=${identity.kind}`);
      await profile
        .getByLabel("Retained fixture input")
        .fill("Unsaved original profile draft");
      const disclosure = profile.getByText("Sources & aliases", {
        exact: true,
      });
      await disclosure.focus();
      await disclosure.press("Enter");
      const followLink = profile.getByRole("link", {
        name: `Follow ${identity.target} on ${identity.sourceName}`,
        exact: true,
      });
      await followLink.waitFor();
      assert.equal(
        await followLink.getAttribute("href"),
        `${prefix}/following?fromIdentity=${identity.id}#following-manager`
      );
      const before = writes().length;
      const [draft] = await Promise.all([
        profile.waitForEvent("popup"),
        followLink.click(),
      ]);
      await draft.locator("#following-target:enabled").waitFor();
      assert.equal(
        await draft.getByLabel("Follow name", { exact: true }).inputValue(),
        identity.entityName
      );
      assert.equal(
        await draft.getByLabel("Target type").inputValue(),
        identity.kind
      );
      assert.equal(
        await draft.getByLabel("Exact provider tag or query").inputValue(),
        identity.target
      );
      assert.equal(
        await draft.getByLabel("Source", { exact: true }).inputValue(),
        matchingSource.id,
        "reused settings ID must not select another instance"
      );
      assert.equal(
        await draft.getByLabel("Source", { exact: true }).isDisabled(),
        true
      );
      assert.equal(
        writes().length,
        before,
        "opening a draft must only read local data"
      );
      assert.equal(
        await draft.evaluate(
          () => document.documentElement.scrollWidth > innerWidth + 1
        ),
        false
      );

      for (const field of ["#following-name", "#following-source"]) {
        assert.notEqual(
          await draft
            .locator(field)
            .evaluate((element) => getComputedStyle(element).backgroundColor),
          "rgb(255, 255, 255)"
        );
      }
      await draft
        .getByLabel("Exact provider tag or query")
        .fill(`${identity.target}_draft`);
      const reloaded = draft.waitForResponse((response) =>
        response.url().endsWith("/booru-browser/sources")
      );
      await draft
        .getByRole("button", { name: "Reload saved feed", exact: true })
        .click();
      await reloaded;
      assert.equal(
        await draft.getByLabel("Exact provider tag or query").inputValue(),
        `${identity.target}_draft`
      );
      await draft.reload();
      await draft.locator("#following-target:enabled").waitFor();
      assert.equal(
        await draft.getByLabel("Exact provider tag or query").inputValue(),
        identity.target,
        "refresh reconstructs only the explicit context"
      );
      if (shots && identity.kind === "artist") {
        await draft
          .locator("#following-manager")
          .evaluate((element) => element.scrollIntoView({ block: "start" }));
        await draft.screenshot({
          path: `${shots}/follow-context-${layout}.png`,
        });
      }
      await draft.getByRole("button", { name: "Follow", exact: true }).click();
      await draft
        .getByRole("region", {
          name: `Follow ${identity.entityName}`,
          exact: true,
        })
        .waitFor();
      assert.deepEqual(writes().at(-1).body, {
        action: "create",
        source: matchingSource.id,
        instance: matchingSource.instance,
        name: identity.entityName,
        kind: identity.kind,
        target: identity.target,
      });
      assert.equal(
        new URL(draft.url()).searchParams.has("fromIdentity"),
        false
      );
      assert.equal(
        await draft.getByLabel("Follow name", { exact: true }).inputValue(),
        ""
      );
      assert.equal(
        await profile.getByLabel("Retained fixture input").inputValue(),
        "Unsaved original profile draft"
      );
      assert.equal(
        new URL(profile.url()).searchParams.get("kind"),
        identity.kind
      );
      assert.equal(await profile.locator("details[open]").count(), 1);
      await draft.close();
    }
    await profile.goto(`${base}/profile?kind=artist`);
    await profile.getByText("Sources & aliases", { exact: true }).click();
    await profile.getByRole("link", { name: /Follow waligner/ }).waitFor();
    assert.equal(
      await profile.getByRole("link", { name: /^Follow / }).count(),
      1
    );
    assert.equal(
      await profile
        .getByText("Feed checks are not supported for this source yet.", {
          exact: true,
        })
        .count(),
      2
    );
    if (shots)
      await profile
        .locator(".identity-sources")
        .screenshot({ path: `${shots}/follow-profile-links-${layout}.png` });
    const duplicate = await context.newPage();
    await duplicate.goto(`${base}/following?fromIdentity=1#following-manager`);
    await duplicate
      .getByText(/This exact target already has a saved follow/)
      .waitFor();
    assert.equal(
      await duplicate
        .getByRole("button", { name: "Follow", exact: true })
        .isDisabled(),
      true
    );
    await duplicate
      .getByRole("link", { name: "Manage saved follow", exact: true })
      .click();
    assert.match(new URL(duplicate.url()).hash, /^#saved-follow-/);
    assert.equal(writes().length, 3);
    await duplicate.close();
    sources = [wrongSource];
    const unavailable = await context.newPage();
    await unavailable.goto(`${base}/following?fromIdentity=1`);
    await unavailable.getByText(/no substitute was selected/).waitFor();
    assert.equal(
      await unavailable.getByLabel("Source", { exact: true }).inputValue(),
      ""
    );
    assert.equal(
      await unavailable
        .getByRole("button", { name: "Follow", exact: true })
        .isDisabled(),
      true
    );
    await unavailable
      .getByRole("button", { name: "Start a blank follow", exact: true })
      .click();
    assert.equal(
      new URL(unavailable.url()).searchParams.has("fromIdentity"),
      false
    );
    assert.equal(
      await unavailable.getByLabel("Source", { exact: true }).inputValue(),
      wrongSource.id
    );
    await unavailable.close();
    sources = [matchingSource];
    for (const [id, message] of [
      ["4", /Account providers and other engines are not supported yet/],
      ["0", /connection link is invalid/],
      ["999", /connection no longer exists/],
    ]) {
      const invalid = await context.newPage();
      await invalid.goto(`${base}/following?fromIdentity=${id}`);
      await invalid.getByText(message).waitFor();
      assert.equal(
        await invalid
          .getByRole("button", { name: "Follow", exact: true })
          .isDisabled(),
        true
      );
      await invalid.close();
    }
    failIdentity = true;
    const failed = await context.newPage();
    await failed.goto(`${base}/following?fromIdentity=1`);
    await failed.getByText("Connections unavailable").waitFor();
    assert.equal(
      await failed
        .getByRole("button", { name: "Follow", exact: true })
        .isDisabled(),
      true
    );
    await failed.close();
    failIdentity = false;
    sources = [];
    const empty = await context.newPage();
    await empty.goto(`${base}/following`);
    await empty.getByText(/Enable a Danbooru-compatible source/).waitFor();
    assert.equal(
      await empty
        .getByRole("button", { name: "Follow", exact: true })
        .isDisabled(),
      true
    );
    await empty.close();
    sources = [matchingSource];
    holdIdentity = true;
    const started = new Promise((resolve) => {
      identityStarted = resolve;
    });
    const late = await context.newPage();
    await late.goto(`${base}/following?fromIdentity=1`);
    await started;
    assert.equal(
      await late.getByLabel("Follow name", { exact: true }).isDisabled(),
      true
    );
    const aborted = late.waitForEvent("requestfailed", {
      predicate: (request) =>
        request.url().includes("/identities/list?identity=1"),
    });
    await late.getByRole("link", { name: "Leave Following fixture" }).click();
    await aborted;
    releaseIdentity();
    await late.getByText("Other fixture page", { exact: true }).waitFor();
    assert.equal(
      writes().length,
      3,
      "only the three explicit creates may write"
    );
    assert.deepEqual(errors, []);
    assert.equal(
      await profile.evaluate(
        () => document.documentElement.scrollWidth > innerWidth + 1
      ),
      false
    );
    await context.close();
    console.log(
      `${layout}: contextual follow drafts, exact source binding, explicit saves, capability limits and cleanup passed`
    );
  }
} finally {
  await browser.close();
  await server.close();
}
