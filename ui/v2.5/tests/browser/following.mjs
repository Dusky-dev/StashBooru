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
      name: "following-fixture",
      configureServer(app) {
        app.middlewares.use((req, _res, next) => {
          const pathname = req.url.split("?")[0];
          if (
            pathname === "/following" ||
            pathname === "/stash/following" ||
            pathname === "/other" ||
            pathname === "/stash/other"
          ) {
            req.url = `/tests/browser/fixtures/following${pathname.startsWith("/stash/") ? "-prefix" : ""}.html`;
          }
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
    page.on("console", (e) => {
      if (
        e.type() === "error" &&
        /unmounted|Maximum update depth|unhandled/i.test(e.text())
      )
        errors.push(e.text());
    });
    const follows = [];
    let items = [];
    const seen = new Map();
    let sequence = 0;
    let syncs = 0;
    let failNext = false;
    let holdNext = false;
    let releaseHeld;
    let heldReady;
    let discardHeld = false;
    const source = {
      id: "danbooru",
      instance: "fixture-danbooru",
      engine: "danbooru",
      name: "Danbooru",
      url: "https://danbooru.example.org",
    };
    function discover(id, follow) {
      let item = items.find((i) => i.postID === String(id));
      if (!item) {
        item = {
          sequence: String(++sequence),
          postID: String(id),
          retrievedAt: "2026-10-20T10:00:00Z",
          ignoredAt: "",
          downloadedAt: "",
          matchesTruncated: false,
          matches: [],
          content: {
            title: `Post ${id}`,
            url: `https://danbooru.example.org/posts/${id}`,
            thumbnail: `booru-browser/media?id=${id}`,
            browseURL: `/booru?source=danbooru&q=id%3A${id}&inspect=danbooru%3A${id}`,
            rating: "sensitive",
            tags: [
              { name: "artist_a", category: "artist" },
              { name: "series_s", category: "copyright" },
            ],
          },
        };
        items.push(item);
      }
      if (
        !item.matches.some(
          (m) => m.followID === follow.id && m.revision === follow.queryRevision
        )
      )
        item.matches.push({
          followID: follow.id,
          name: follow.name,
          revision: follow.queryRevision,
          target: follow.target,
        });
    }
    await page.route("**/booru-browser/**", async (route) => {
      const url = new URL(route.request().url());
      if (url.pathname.endsWith("/sources"))
        return route.fulfill({ json: [source] });
      if (url.searchParams.get("id") === "429")
        return route.fulfill({ status: 404, body: "" });
      return route.fulfill({
        contentType: "image/svg+xml",
        body: '<svg xmlns="http://www.w3.org/2000/svg" width="640" height="480"><rect width="640" height="480" fill="#243242"/><circle cx="320" cy="240" r="160" fill="#7197aa"/></svg>',
      });
    });
    await page.route("**/following/**", async (route) => {
      const request = route.request();
      const url = new URL(request.url());
      assert.ok(
        url.pathname.startsWith(`${prefix}/following/`),
        "API requests lost the base prefix"
      );
      const body = request.method() === "POST" ? request.postDataJSON() : {};
      if (url.pathname.endsWith("/follows")) {
        if (request.method() === "GET") return route.fulfill({ json: follows });
        if (body.action === "create") {
          const follow = {
            ...body,
            id: String.fromCharCode(65 + follows.length).repeat(26),
            sourceURL: source.url,
            status: "available",
            syncSupported: true,
            active: true,
            version: 1,
            queryRevision: 1,
            feed: { hasMore: false, checkedAt: "", lastError: "" },
          };
          follows.push(follow);
          return route.fulfill({ json: follow });
        }
        const follow = follows.find((f) => f.id === body.id);
        if (body.version !== follow.version)
          return route.fulfill({
            status: 409,
            json: { error: "Follow changed. Reload and retry." },
          });
        if (body.action === "update") {
          if (body.target !== follow.target || body.kind !== follow.kind) {
            follow.queryRevision++;
            follow.feed = { hasMore: false, checkedAt: "", lastError: "" };
          }
          Object.assign(follow, {
            name: body.name,
            target: body.target,
            kind: body.kind,
          });
        } else follow.active = body.action === "refollow";
        follow.version++;
        return route.fulfill({ json: follow });
      }
      if (url.pathname.endsWith("/sync")) {
        syncs++;
        const follow = follows.find((f) => f.id === body.id);
        if (holdNext) {
          holdNext = false;
          await new Promise((resolve) => {
            releaseHeld = resolve;
            heldReady?.();
          });
          if (discardHeld) {
            discardHeld = false;
            return route.abort().catch(() => {});
          }
        }
        if (failNext) {
          failNext = false;
          follow.feed.lastError =
            "Provider temporarily unavailable. Retry to resume.";
          return route.fulfill({
            status: 429,
            headers: { "Retry-After": "3" },
            json: { error: follow.feed.lastError },
          });
        }
        const pending = follow.feed.hasMore;
        if (follows[0] === follow && !pending) discover(9, follow);
        discover(10, follow);
        if (follows[0] === follow && pending) discover(11, follow);
        follow.feed = {
          hasMore: follows[0] === follow && !follow.feed.checkedAt,
          checkedAt: "2026-10-20T10:00:00Z",
          lastError: "",
        };
        return route.fulfill({
          json: {
            checkpoint: follow.feed,
            matched: pending ? 1 : 2,
            notice: "Saved one page. Seen decisions remain explicit.",
          },
        });
      }
      if (url.pathname.endsWith("/seen")) {
        const item = items.find((i) => i.sequence === body.sequence);
        const previous = seen.get(item.postID) ?? { version: 0, at: "" };
        assert.equal(body.version, previous.version);
        seen.set(item.postID, {
          version: previous.version + 1,
          at: body.seen ? "2026-10-20T10:01:00Z" : "",
        });
        return route.fulfill({ json: { version: previous.version + 1 } });
      }
      if (url.pathname.endsWith("/prune")) {
        const before = items.length;
        items = items.filter((i) => !seen.get(i.postID)?.at);
        return route.fulfill({ json: { removed: before - items.length } });
      }
      if (url.pathname.endsWith("/feed")) {
        const p = url.searchParams;
        const selected = items
          .map((item) => {
            const decision = seen.get(item.postID) ?? { version: 0, at: "" };
            return {
              ...item,
              seenAt: decision.at,
              seenVersion: decision.version,
              matches: item.matches.map((m) => {
                const f = follows.find((f) => f.id === m.followID);
                return {
                  ...m,
                  name: f.name,
                  current: f.active && f.queryRevision === m.revision,
                };
              }),
            };
          })
          .filter(
            (item) =>
              item.matches.some(
                (m) =>
                  (!p.get("follow") || p.get("follow") === m.followID) &&
                  (p.get("history") === "1" || m.current)
              ) &&
              (p.get("unseen") !== "1" || !item.seenAt) &&
              (!p.get("before") ||
                Number(item.sequence) < Number(p.get("before")))
          )
          .sort((a, b) => Number(b.sequence) - Number(a.sequence));
        const slice = selected.slice(0, 25);
        return route.fulfill({
          json: {
            items: slice,
            next: selected.length > 25 ? slice.at(-1).sequence : "",
          },
        });
      }
      throw new Error(`Unexpected API path ${url.pathname}`);
    });
    await page.goto(`${origin}${prefix}/following`);
    await page.getByLabel("Source", { exact: true }).selectOption("danbooru");
    await page.getByLabel("Follow name", { exact: true }).fill("Artist A");
    await page
      .getByLabel("Exact provider tag or query", { exact: true })
      .fill("artist_a");
    await page.getByRole("button", { name: "Follow", exact: true }).click();
    const artist = page.getByRole("region", {
      name: "Follow Artist A",
      exact: true,
    });
    await artist.getByRole("button", { name: "Check new posts" }).click();
    await page.waitForFunction(
      () => document.querySelectorAll(".following-post").length === 2
    );
    assert.equal(
      await page
        .getByRole("link", { name: "Browse / Download" })
        .first()
        .getAttribute("href"),
      `${prefix}/booru?source=danbooru&q=id%3A10&inspect=danbooru%3A10`
    );
    failNext = true;
    await artist.getByRole("button", { name: "Continue check" }).click();
    await page.getByText(/Retry after 3 seconds/).waitFor();
    await page.reload();
    await page.getByText(/Last check failed/).waitFor();
    assert.equal(syncs, 2, "reload must not fetch the provider automatically");
    await artist.getByRole("button", { name: "Continue check" }).click();
    await page.waitForFunction(
      () => document.querySelectorAll(".following-post").length === 3
    );
    await page.getByLabel("Follow name", { exact: true }).fill("Series S");
    await page.getByLabel("Target type").selectOption("copyright");
    await page
      .getByLabel("Exact provider tag or query", { exact: true })
      .fill("series_s");
    await page.getByRole("button", { name: "Follow", exact: true }).click();
    const series = page.getByRole("region", {
      name: "Follow Series S",
      exact: true,
    });
    await series.getByRole("button", { name: "Check new posts" }).click();
    await page.waitForFunction(
      () =>
        document.querySelectorAll(".following-post").length === 3 &&
        document.querySelectorAll(".following-post .following-matches button")
          .length === 4
    );
    await page.evaluate(() => window.scrollTo(0, 0));
    if (process.env.STASH_BROWSER_SCREENSHOTS) {
      fs.mkdirSync(process.env.STASH_BROWSER_SCREENSHOTS, { recursive: true });
      await page.screenshot({
        path: path.join(
          process.env.STASH_BROWSER_SCREENSHOTS,
          `following-${viewport.width}.png`
        ),
        fullPage: true,
      });
    }
    const post10 = page.getByRole("article", { name: "Post 10", exact: true });
    await post10
      .getByRole("button", { name: "Mark seen", exact: true })
      .click();
    await post10
      .getByRole("button", { name: "Mark unseen", exact: true })
      .waitFor();
    await page.getByText("Unseen only", { exact: true }).click();
    await page.waitForFunction(
      () => document.querySelectorAll(".following-post").length === 2
    );
    assert.match(page.url(), /unseen=1/);
    await page.reload();
    await page.waitForFunction(
      () => document.querySelectorAll(".following-post").length === 2
    );
    await page.goBack();
    await page.waitForFunction(
      () => document.querySelectorAll(".following-post").length === 3
    );
    await artist.getByRole("button", { name: "Unfollow", exact: true }).click();
    await page.waitForFunction(
      () => document.querySelectorAll(".following-post").length === 1
    );
    await page.getByText("Include History", { exact: true }).click();
    await page.waitForFunction(
      () => document.querySelectorAll(".following-post").length === 3
    );
    for (let id = 400; id < 430; id++) discover(id, follows[1]);
    await page.goto(`${origin}${prefix}/following`);
    await page.waitForFunction(
      () => document.querySelectorAll(".following-post").length === 25
    );
    await page.getByText("Preview unavailable", { exact: true }).waitFor();
    const firstPage = await page
      .locator(".following-post h3")
      .allTextContents();
    await page.getByRole("link", { name: "Older saved", exact: true }).click();
    await page.waitForFunction(
      () => document.querySelectorAll(".following-post").length === 6
    );
    assert.ok(
      (await page.locator(".following-post h3").allTextContents()).every(
        (name) => !firstPage.includes(name)
      )
    );
    assert.match(page.url(), /before=/);
    await page.reload();
    await page.waitForFunction(
      () => document.querySelectorAll(".following-post").length === 6
    );
    await page.goBack();
    await page.waitForFunction(
      () => document.querySelectorAll(".following-post").length === 25
    );
    await series.getByRole("button", { name: "Edit", exact: true }).click();
    follows[1].version++;
    follows[1].queryRevision++;
    follows[1].target = "series_remote";
    await page
      .getByLabel("Exact provider tag or query", { exact: true })
      .fill("series_draft");
    await page
      .getByRole("button", { name: "Save changes", exact: true })
      .click();
    const reloadEdit = page.getByRole("button", {
      name: "Reload latest follow",
      exact: true,
    });
    await reloadEdit.waitFor();
    assert.equal(
      await page
        .getByLabel("Exact provider tag or query", { exact: true })
        .inputValue(),
      "series_draft",
      "conflict recovery must keep the draft until explicitly reloaded"
    );
    assert.equal(
      await page
        .getByRole("button", { name: "Save changes", exact: true })
        .isDisabled(),
      true
    );
    await reloadEdit.click();
    assert.equal(
      await page
        .getByLabel("Exact provider tag or query", { exact: true })
        .inputValue(),
      "series_remote"
    );
    assert.equal(
      await page.getByLabel("Source", { exact: true }).isDisabled(),
      true
    );
    await page
      .getByLabel("Exact provider tag or query", { exact: true })
      .fill("series_t");
    await page
      .getByRole("button", { name: "Save changes", exact: true })
      .click();
    await page.getByText(/No saved posts match/).waitFor();
    await series.getByRole("button", { name: "Check new posts" }).click();
    await post10
      .getByRole("button", { name: "Mark unseen", exact: true })
      .waitFor();
    await page.getByRole("button", { name: "Remove seen snapshots" }).click();
    await page.getByText(/No saved posts match/).waitFor();
    await page.getByLabel("Follow name", { exact: true }).fill("Artist C");
    await page
      .getByLabel("Exact provider tag or query", { exact: true })
      .fill("artist_c");
    await page.getByRole("button", { name: "Follow", exact: true }).click();
    const third = page.getByRole("region", {
      name: "Follow Artist C",
      exact: true,
    });
    holdNext = true;
    const ready = new Promise((resolve) => {
      heldReady = resolve;
    });
    await third.getByRole("button", { name: "Check new posts" }).click();
    await ready;
    await page
      .getByRole("button", { name: "Cancel request", exact: true })
      .click();
    await page.getByText(/Request cancelled/).waitFor();
    discardHeld = true;
    releaseHeld();
    await third.getByRole("button", { name: "Check new posts" }).click();
    await post10
      .getByRole("button", { name: "Mark unseen", exact: true })
      .waitFor();
    holdNext = true;
    const leavingReady = new Promise((resolve) => {
      heldReady = resolve;
    });
    await third.getByRole("button", { name: "Check new posts" }).click();
    await leavingReady;
    await page
      .getByRole("link", { name: "Leave Following fixture", exact: true })
      .click();
    await page.getByText("Other fixture page", { exact: true }).waitFor();
    discardHeld = true;
    releaseHeld();
    await page
      .getByRole("link", { name: "Return to Following", exact: true })
      .click();
    await post10.waitFor();
    assert.equal(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth
      ),
      true,
      "horizontal overflow"
    );
    assert.deepEqual(errors, []);
    console.log(
      `Following mounted checks passed ${viewport.width}×${viewport.height}, base=${prefix || "/"}`
    );
    await page.close();
  }
} finally {
  await browser.close();
  await server.close();
}
