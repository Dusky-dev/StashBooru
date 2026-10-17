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
      name: "booru-fixture-route",
      configureServer(app) {
        app.middlewares.use((req, _res, next) => {
          if (req.url.startsWith("/booru?") || req.url === "/booru") {
            req.url =
              "/tests/browser/fixtures/booru-browser.html" +
              (req.url.includes("?")
                ? req.url.slice(req.url.indexOf("?"))
                : "");
          }
          next();
        });
      },
    },
  ],
});
await server.listen();
const baseURL = `http://127.0.0.1:${server.httpServer.address().port}`;
const browser = await engines.chromium.launch({
  headless: true,
  executablePath: process.env.CHROMIUM_EXECUTABLE,
  args: ["--no-sandbox", "--disable-dev-shm-usage", "--disable-gpu"],
});
const sources = ["danbooru", "gelbooru", "philomena"].map((id) => ({
  id,
  name: id[0].toUpperCase() + id.slice(1),
  engine: id,
  url: `https://${id}.example.org`,
  queryHelp:
    id === "philomena"
      ? "Native Philomena syntax: commas or AND."
      : "Space-separated tags, native provider syntax.",
  authentication: "Optional API credentials on the server.",
  credentialsConfigured: id === "gelbooru",
  categorizedTags: id === "danbooru",
}));

function result(source, query, page) {
  const id = query === "fast" ? 900 : query === "slow" ? 800 : page * 100 + 1;
  return {
    sourceID: source,
    page,
    limit: 24,
    fetchedAt: "2026-10-17T16:00:00Z",
    hasMore: page === 1,
    posts: [
      {
        id: String(id),
        sourceID: source,
        postURL: `https://${source}.example.org/posts/${id}`,
        previewURL: `https://cdn.example.org/${query === "broken" ? "missing.jpg" : "preview.svg"}`,
        sampleURL: `https://cdn.example.org/${query === "broken" ? "missing.jpg" : "preview.svg"}`,
        width: 640,
        height: 480,
        bytes: 4096,
        format: "png",
        rating: "safe",
        md5: "0123456789abcdef0123456789abcdef",
        tags: [
          { name: "artist_a", category: "artist" },
          { name: "series_a", category: "copyright" },
          { name: "<img src=x onerror=alert(1)>", category: "unknown" },
          { name: "long_tag_".repeat(30), category: "meta" },
        ],
      },
    ],
  };
}

try {
  for (const viewport of [
    { width: 1440, height: 1000 },
    { width: 390, height: 844 },
  ]) {
    const page = await browser.newPage({ viewport });
    const errors = [];
    page.on("pageerror", (error) => errors.push(error.message));
    page.on("console", (entry) => {
      if (
        entry.type() === "error" &&
        /unmounted component|Maximum update depth|unhandled/i.test(entry.text())
      )
        errors.push(entry.text());
    });
    const requests = [];
    let denied = true;
    let sourcesDenied = false;
    const delayed = [];
    await page.route("**/booru-browser/**", async (route) => {
      const url = new URL(route.request().url());
      if (url.pathname.endsWith("/sources")) {
        await route.fulfill({
          status: sourcesDenied ? 500 : 200,
          json: sourcesDenied
            ? { error: "Configuration unavailable" }
            : sources,
        });
        return;
      }
      const source = url.searchParams.get("source");
      const query = url.searchParams.get("q");
      const number = Number(url.searchParams.get("page"));
      requests.push({ source, query, page: number });
      if (query === "slow") {
        delayed.push({ route, source, query, number });
        return;
      }
      if (source === "gelbooru" && denied) {
        await route.fulfill({
          status: 429,
          json: { error: "Fixture rate limit", retryAfter: 60 },
        });
        return;
      }
      await route.fulfill({ json: result(source, query, number) });
    });
    await page.route("https://cdn.example.org/**", (route) =>
      route.request().url().endsWith("missing.jpg")
        ? route.fulfill({ status: 404, body: "" })
        : route.fulfill({
            contentType: "image/svg+xml",
            body: '<svg xmlns="http://www.w3.org/2000/svg" width="640" height="480"><rect width="640" height="480" fill="#20334d"/><circle cx="320" cy="240" r="130" fill="#d8b26e"/></svg>',
          })
    );
    await page.goto(`${baseURL}/booru`);
    await page.getByLabel("Gelbooru · gelbooru", { exact: true }).waitFor();
    assert.equal(
      requests.length,
      0,
      "opening the browser searched automatically"
    );
    await page.getByLabel("Gelbooru · gelbooru", { exact: true }).check();
    await page
      .getByLabel("Search tags (native source syntax)", { exact: true })
      .fill("landscape");
    assert.equal(
      requests.length,
      0,
      "editing the query searched automatically"
    );
    await page
      .getByRole("button", { name: "Search selected sources", exact: true })
      .click();
    const dan = page.getByRole("region", { name: "Danbooru", exact: true });
    const gel = page.getByRole("region", { name: "Gelbooru", exact: true });
    await dan
      .getByRole("button", { name: "Inspect Danbooru post 101", exact: true })
      .waitFor();
    await gel.getByRole("alert").filter({ hasText: "60 seconds" }).waitFor();
    assert.equal(requests.length, 2);

    await page
      .getByLabel("Search tags (native source syntax)", { exact: true })
      .fill("draft query");
    await dan
      .getByRole("button", { name: "Inspect Danbooru post 101", exact: true })
      .click();
    const dialog = page.getByRole("dialog");
    await dialog.waitFor();
    await dialog.getByText("artist_a", { exact: true }).waitFor();
    await dialog.getByText("series_a", { exact: true }).waitFor();
    await dialog
      .getByText("Tags — category not supplied", { exact: true })
      .waitFor();
    assert.equal(await dialog.locator("script").count(), 0);
    assert.ok(page.url().includes("inspect=danbooru%3A101"));
    assert.equal(requests.length, 2, "inspection refetched searches");
    await page.keyboard.press("Escape");
    await dialog.waitFor({ state: "hidden" });
    assert.equal(
      await page
        .getByLabel("Search tags (native source syntax)", { exact: true })
        .inputValue(),
      "draft query"
    );

    await dan.getByRole("button", { name: "Next", exact: true }).click();
    await dan
      .getByRole("button", { name: "Inspect Danbooru post 201", exact: true })
      .waitFor();
    assert.ok(page.url().includes("p.danbooru=2"));
    assert.equal(
      requests.filter((r) => r.source === "gelbooru").length,
      1,
      "paging one source refetched another"
    );
    await page.goBack();
    await dan
      .getByRole("button", { name: "Inspect Danbooru post 101", exact: true })
      .waitFor();
    denied = false;
    await gel
      .getByRole("button", { name: "Retry source", exact: true })
      .click();
    await gel
      .getByRole("button", { name: "Inspect Gelbooru post 101", exact: true })
      .waitFor();
    const searches = requests.length;
    await gel
      .getByRole("button", { name: "Inspect Gelbooru post 101", exact: true })
      .click();
    await page.getByRole("dialog").waitFor();
    await page.reload();
    await page.getByRole("dialog").waitFor();
    assert.ok(
      requests.length > searches,
      "refresh did not reconstruct URL state"
    );
    await page
      .getByRole("dialog")
      .getByRole("button", { name: "Close", exact: true })
      .last()
      .click();

    const input = page.getByLabel("Search tags (native source syntax)", {
      exact: true,
    });
    await input.fill("slow");
    await page
      .getByRole("button", { name: "Search selected sources", exact: true })
      .click();
    await page.waitForFunction(
      () => document.querySelectorAll('[aria-busy="true"]').length === 2
    );
    await input.fill("fast");
    await page
      .getByRole("button", { name: "Search selected sources", exact: true })
      .click();
    await dan
      .getByRole("button", { name: "Inspect Danbooru post 900", exact: true })
      .waitFor();
    for (const item of delayed.splice(0)) {
      await item.route
        .fulfill({ json: result(item.source, item.query, item.number) })
        .catch(() => {});
    }
    assert.equal(
      await page.getByRole("button", { name: /post 800$/ }).count(),
      0,
      "late response replaced current results"
    );
    const overflow = await page.evaluate(
      () => document.documentElement.scrollWidth > window.innerWidth + 1
    );
    assert.equal(overflow, false, "page overflow");

    if (process.env.STASH_BROWSER_SCREENSHOTS) {
      fs.mkdirSync(process.env.STASH_BROWSER_SCREENSHOTS, { recursive: true });
      await page.screenshot({
        path: path.join(
          process.env.STASH_BROWSER_SCREENSHOTS,
          `booru-${viewport.width}.png`
        ),
        fullPage: true,
      });
    }
    await input.fill("broken");
    await page
      .getByRole("button", { name: "Search selected sources", exact: true })
      .click();
    await dan
      .getByText("Remote preview could not be loaded. Use Open remote post.", {
        exact: true,
      })
      .waitFor();
    await dan
      .getByRole("button", { name: "Inspect Danbooru post 101", exact: true })
      .click();
    await page
      .getByRole("dialog")
      .getByText("Remote preview could not be loaded. Use Open remote post.", {
        exact: true,
      })
      .waitFor();
    assert.equal(
      await page
        .getByRole("dialog")
        .evaluate((element) => element.scrollWidth > window.innerWidth + 1),
      false,
      "inspection overflow"
    );
    await page.keyboard.press("Escape");
    await page.goto(`${baseURL}/booru?source=disabled&q=cat`);
    await page
      .getByRole("alert")
      .filter({ hasText: "no longer enabled" })
      .waitFor();
    const beforeInvalid = requests.length;
    await page.goto(`${baseURL}/booru?source=danbooru&q=cat&p.danbooru=-1`);
    await page.getByRole("alert").filter({ hasText: "Invalid page" }).waitFor();
    assert.equal(requests.length, beforeInvalid, "invalid URL searched");
    sourcesDenied = true;
    await page.goto(`${baseURL}/booru`);
    await page
      .getByRole("alert")
      .filter({ hasText: "Configuration unavailable" })
      .waitFor();
    sourcesDenied = false;
    await page
      .getByRole("button", { name: "Retry source", exact: true })
      .click();
    await page.getByLabel("Danbooru · danbooru", { exact: true }).waitFor();
    await page
      .getByLabel("Search tags (native source syntax)", { exact: true })
      .fill("slow");
    await page
      .getByRole("button", { name: "Search selected sources", exact: true })
      .click();
    await page.getByRole("button", { name: "Cancel", exact: true }).click();
    await page
      .getByRole("alert")
      .filter({ hasText: "Search cancelled" })
      .waitFor();
    await page
      .getByRole("link", { name: "Leave browser fixture", exact: true })
      .click();
    for (const item of delayed.splice(0))
      await item.route
        .fulfill({ json: result(item.source, item.query, item.number) })
        .catch(() => {});
    assert.deepEqual(errors, []);
    await page.close();
    console.log(
      `Booru browser ${viewport.width}×${viewport.height}: search/paging/inspection/refresh/back, partial failures/retry, missing previews, races/cancel/cleanup and overflow passed.`
    );
  }
} finally {
  await browser.close();
  await server.close();
}
