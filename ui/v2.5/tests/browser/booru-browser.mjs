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
  instance: `fixture-${id}`,
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
  const response = {
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
        mediaKey: (query === "broken" ? "b" : "a").repeat(48),
        downloadAvailable: true,
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
  if (query === "gallery")
    response.posts = Array.from({ length: 12 }, (_, index) => ({
      ...response.posts[0],
      id: String(id + index),
      width: index % 2 ? 480 : 640,
      height: index % 2 ? 640 : 480,
      tags: response.posts[0].tags.slice(0, 2),
    }));
  return response;
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
    let savedSources = sources.map((source) => ({
      ...source,
      mediaHosts: [],
      login: "",
      userID: source.id === "gelbooru" ? "42" : "",
      disabled: false,
      preset: true,
      apiKeyConfigured: source.credentialsConfigured,
    }));
    let revision = 1;
    let conflict = false;
    let defaultJXL = false;
    let tagDatabase = { count: 0, revision: "" };
    let downloadState;
    const downloadRequests = [];
    let failNextDownload = false;
    const writes = [];
    const review = new Map();
    let reviewConflict = false;
    const reviewState = (source, instance, id) =>
      review.get(`${source}:${instance}:${id}`) ?? {
        id,
        version: 0,
        ignoredAt: "",
        downloadedAt: "",
        updatedAt: "",
      };
    const settingsView = () => ({
      revision: String(revision),
      sources: savedSources,
    });
    const publicSources = () =>
      savedSources
        .filter((source) => !source.disabled)
        .map((source) => ({
          ...source,
          credentialsConfigured: source.apiKeyConfigured,
        }));
    await page.route("**/booru-browser/**", async (route) => {
      const url = new URL(route.request().url());
      if (url.pathname.endsWith("/download-settings")) {
        if (route.request().method() === "POST")
          defaultJXL = route.request().postDataJSON().jpegToJXL;
        await route.fulfill({ json: { jpegToJXL: defaultJXL } });
        return;
      }
      if (url.pathname.endsWith("/tags")) {
        if (
          url.searchParams.get("source") === "gelbooru" &&
          url.searchParams.get("q")?.startsWith("az")
        ) {
          await route.fulfill({
            status: 500,
            json: { error: "Fixture tag database unavailable" },
          });
          return;
        }
        if (route.request().method() === "POST") {
          assert.equal(
            route.request().headers()["x-booru-revision"] ?? "",
            tagDatabase.revision
          );
          assert.ok(route.request().postData().includes("azure_hair"));
          tagDatabase = { count: 2, revision: "uploaded" };
        }
        await route.fulfill({
          json: {
            ...tagDatabase,
            suggestions:
              url.searchParams.get("q")?.startsWith("az") && tagDatabase.count
                ? [
                    {
                      name: "blue_hair",
                      category: "general",
                      count: 42,
                      alias: "azure_hair",
                    },
                  ]
                : [],
          },
        });
        return;
      }
      if (url.pathname.endsWith("/media")) {
        if (url.searchParams.get("key") === "b".repeat(48))
          await route.fulfill({ status: 502, body: "Unavailable" });
        else
          await route.fulfill({
            contentType: "image/png",
            body: Buffer.from(
              "iVBORw0KGgoAAAANSUhEUgAAAKAAAAB4CAIAAAD6wG44AAACV0lEQVR4nO3dwU3DQBBG4Q2iEKiBJrhQE6IruqEVDpYsK5Ht4Gyy/zzed46UyTytoxwcn17ePpq4nkYPoPsyMJyB4QwMZ2A4A8MZGM7AcAaGMzCcgeEMDGdgOAPDGRjOwHAGhjMwnIHhDAxnYDgDwxkYzsBwBoYzMNzz6AHu7vvzdfsF718/j5lkiBPy3qTdqGt4sVGBD3e9hCkNCdwx7RIgc/nAd0q7VDpz4cAPSLtUNHPVn0kPrjvkHbsoGXjUris2LnaJDllxoct1pRMcUrclTbKrTOC0nabNs6ZG4MxtZk51pkZgHVYgcPJBSZ5tkh44f4PhE0YHDt/dLHnO6MC6XW7g5GNxKXba3MDqIjRw7IHYkDlzaGD1khg48yhcI3DyxMDqyMBwcYEDr3J/kjZ/XGD1ZWA4A8NlBU77Ajsm6lNkBVZ3BoYzMJyB4QwMZ2A4A8MZGM7AcAaGywpc6LbMDVGfIiuwujMwnIHh4gJHfYEdkDZ/XGD1ZWC4xMBpV7nrBU6eGFgdhQYOPAq7MmcODaxecgNnHog1sdPmBlYX0YFjj8WZ5DmjA7fs3U3CJ0wP3LI3mDzbpEBg3aJG4MyDkjnVmRqBW9420+ZZUyZwS9ppziS7iv2l/2Tg/ZmF0k4qneDZqC2Xq9uKBm4jdl2xbit6iV7yyWfbygee+OzCNZDAE58+egkVeObzg2fMwEs+ARwe+J+r+jNJVzIwnIHhDAxnYDgDwxkYzsBwBoYzMJyB4QwMZ2A4A8MZGM7AcAaGMzCcgeEMDGdgOAPDGRjOwHAGhvsFSzt+pR7bQwoAAAAASUVORK5CYII=",
              "base64"
            ),
          });
        return;
      }
      if (url.pathname.endsWith("/download")) {
        if (route.request().method() === "POST") {
          const payload = route.request().postDataJSON();
          if (payload.cancel)
            downloadState = {
              jobID: 71,
              token: "c".repeat(32),
              status: "cancelled",
              converted: false,
            };
          else {
            downloadRequests.push(payload);
            downloadState = failNextDownload
              ? {
                  jobID: 71,
                  token: "c".repeat(32),
                  status: "failed",
                  error: "Fixture checksum mismatch",
                  converted: false,
                }
              : {
                  jobID: 71,
                  token: "c".repeat(32),
                  status: "complete",
                  converted: payload.convert,
                  url: "downloads/abc123/danbooru-101.jxl",
                  filename: "danbooru-101.jxl",
                };
            failNextDownload = false;
          }
          await route.fulfill({ json: { jobID: 71, token: "c".repeat(32) } });
        } else await route.fulfill({ json: downloadState });
        return;
      }
      if (url.pathname.endsWith("/review/read")) {
        const payload = route.request().postDataJSON();
        await route.fulfill({
          json: payload.posts.map((post) => ({
            ...reviewState(payload.source, payload.instance, post.id),
            checkedAt: "2026-10-17T23:30:00Z",
            matchStatus: "no-indexed-match",
            matches:
              payload.source === "danbooru" && post.id === "101"
                ? [
                    {
                      kind: "image",
                      id: 42,
                      fileID: "1",
                      evidence: "current-file-md5",
                    },
                    {
                      kind: "scene",
                      id: 42,
                      fileID: "2",
                      evidence: "current-file-md5",
                    },
                  ]
                : [],
          })),
        });
        return;
      }
      if (url.pathname.endsWith("/review/update")) {
        const payload = route.request().postDataJSON();
        if (
          reviewConflict ||
          payload.changes.some(
            (change) =>
              reviewState(payload.source, payload.instance, change.id)
                .version !== change.version
          )
        ) {
          reviewConflict = false;
          await route.fulfill({
            status: 409,
            json: {
              error:
                "Review state changed. Nothing was changed; reload review status.",
            },
          });
          return;
        }
        const states = payload.changes.map((change) => {
          const state = {
            ...reviewState(payload.source, payload.instance, change.id),
            version: change.version + 1,
            updatedAt: "2026-10-17T23:30:00Z",
          };
          if (change.ignored !== undefined)
            state.ignoredAt = change.ignored ? "2026-10-17T23:30:00Z" : "";
          if (change.downloaded !== undefined)
            state.downloadedAt = change.downloaded
              ? "2026-10-17T23:30:00Z"
              : "";
          review.set(
            `${payload.source}:${payload.instance}:${change.id}`,
            state
          );
          return state;
        });
        await route.fulfill({ json: states });
        return;
      }
      if (url.pathname.endsWith("/settings")) {
        if (route.request().method() === "GET") {
          await route.fulfill({ json: settingsView() });
          return;
        }
        const payload = route.request().postDataJSON();
        writes.push(payload);
        if (conflict || payload.revision !== String(revision)) {
          conflict = false;
          await route.fulfill({
            status: 409,
            json: {
              error:
                "Source settings changed. Reload saved sources before saving again.",
            },
          });
          return;
        }
        const old = savedSources.find(
          (source) => source.id === payload.source.id
        );
        if (payload.action === "remove")
          savedSources = savedSources.filter(
            (source) => source.id !== payload.source.id
          );
        else {
          const { apiKey, ...source } = payload.source;
          const next = {
            ...sources[0],
            ...old,
            ...source,
            preset: old?.preset ?? false,
            apiKeyConfigured: payload.clearAPIKey
              ? false
              : !!apiKey || !!old?.apiKeyConfigured,
          };
          savedSources = savedSources
            .filter((source) => source.id !== next.id)
            .concat(next);
        }
        revision++;
        await route.fulfill({ json: settingsView() });
        return;
      }
      if (url.pathname.endsWith("/sources")) {
        await route.fulfill({
          status: sourcesDenied ? 500 : 200,
          json: sourcesDenied
            ? { error: "Configuration unavailable" }
            : publicSources(),
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
    await page
      .getByRole("button", { name: "Source settings", exact: true })
      .click();
    await page
      .getByLabel("Convert JPEG downloads to JPEG XL by default")
      .check();
    await page
      .getByRole("button", { name: "Save download preference", exact: true })
      .click();
    await page
      .getByText("Download preference saved.", { exact: true })
      .waitFor();
    await page.getByLabel("Tag database CSV", { exact: true }).setInputFiles({
      name: "danbooru.csv",
      mimeType: "text/csv",
      buffer: Buffer.from("blue_hair,0,42,azure_hair\nartist_a,1,5,\n"),
    });
    await page
      .getByRole("button", { name: "Upload tag database", exact: true })
      .click();
    await page
      .getByText("Imported 2 tags for Danbooru.", { exact: true })
      .waitFor();
    await page
      .getByRole("button", { name: "Edit Danbooru", exact: true })
      .click();
    const editor = page.getByRole("dialog");
    await editor.getByLabel("Username", { exact: true }).fill("fixture-user");
    await editor.getByLabel("API key", { exact: true }).fill("fixture-secret");
    await editor
      .getByRole("button", { name: "Save source", exact: true })
      .click();
    await editor.waitFor({ state: "hidden" });
    assert.equal(writes.at(-1).source.apiKey, "fixture-secret");
    await page
      .getByRole("button", { name: "Edit Danbooru", exact: true })
      .click();
    assert.equal(
      await editor.getByLabel("API key", { exact: true }).inputValue(),
      "",
      "saved key returned to input"
    );
    await editor
      .getByRole("button", { name: "Save source", exact: true })
      .click();
    await editor.waitFor({ state: "hidden" });
    assert.equal(writes.at(-1).source.apiKey, "");
    assert.equal(writes.at(-1).clearAPIKey, false);
    await page
      .getByRole("button", { name: "Edit Danbooru", exact: true })
      .click();
    await editor
      .getByLabel("Remove saved credentials on save", { exact: true })
      .check();
    await editor
      .getByRole("button", { name: "Save source", exact: true })
      .click();
    await editor.waitFor({ state: "hidden" });
    assert.equal(writes.at(-1).clearAPIKey, true);
    assert.equal(writes.at(-1).source.login, "");
    await page
      .getByRole("button", { name: "Edit Danbooru", exact: true })
      .click();
    await editor.getByLabel("API key", { exact: true }).fill("unsaved-secret");
    await editor.getByLabel("Username", { exact: true }).fill("fixture-user");
    conflict = true;
    await editor
      .getByRole("button", { name: "Save source", exact: true })
      .click();
    await editor
      .getByRole("alert")
      .filter({ hasText: "settings changed" })
      .waitFor();
    assert.equal(
      await editor.getByLabel("API key", { exact: true }).inputValue(),
      "unsaved-secret",
      "failed save discarded draft"
    );
    await editor
      .getByRole("button", { name: "Reload saved sources", exact: true })
      .click();
    await editor.waitFor({ state: "hidden" });
    await page
      .getByRole("button", { name: "Add custom booru", exact: true })
      .click();
    await editor.getByLabel("Name", { exact: true }).fill("Fixture custom");
    await editor
      .getByLabel("Source ID", { exact: true })
      .fill("fixture-custom");
    await editor
      .getByLabel("API engine", { exact: true })
      .selectOption("gelbooru");
    await editor
      .getByLabel("Site URL", { exact: true })
      .fill("https://custom.example.org");
    await editor
      .getByLabel("Additional image hosts", { exact: true })
      .fill("cdn.example.org");
    await editor.getByLabel("User ID", { exact: true }).fill("42");
    await editor
      .getByLabel("API key", { exact: true })
      .fill("fixture-custom-secret");
    const writesBeforeDuplicate = writes.length;
    await editor.getByLabel("Source ID", { exact: true }).fill("danbooru");
    await editor
      .getByRole("button", { name: "Save source", exact: true })
      .click();
    await editor
      .getByRole("alert")
      .filter({ hasText: "already in use" })
      .waitFor();
    assert.equal(
      writes.length,
      writesBeforeDuplicate,
      "duplicate custom ID overwrote a source"
    );
    await editor
      .getByLabel("Source ID", { exact: true })
      .fill("fixture-custom");
    assert.equal(
      await editor.evaluate(
        (element) => element.scrollWidth > window.innerWidth + 1
      ),
      false,
      "settings editor overflow"
    );
    await page.waitForFunction(() => {
      const dialog = document.querySelector(".modal.show .modal-dialog");
      return dialog && getComputedStyle(dialog).transform === "none";
    });
    await editor.evaluate((element) => {
      element.scrollTop = 0;
    });
    if (process.env.STASH_BROWSER_SCREENSHOTS) {
      fs.mkdirSync(process.env.STASH_BROWSER_SCREENSHOTS, { recursive: true });
      await page.screenshot({
        path: path.join(
          process.env.STASH_BROWSER_SCREENSHOTS,
          `booru-settings-${viewport.width}.png`
        ),
        fullPage: false,
      });
    }
    await editor
      .getByRole("button", { name: "Save source", exact: true })
      .click();
    await editor.waitFor({ state: "hidden" });
    assert.equal(writes.at(-1).source.engine, "gelbooru");
    assert.deepEqual(writes.at(-1).source.mediaHosts, ["cdn.example.org"]);
    await page
      .getByRole("button", { name: "Back to posts", exact: true })
      .click();
    await page
      .getByLabel("Fixture custom · gelbooru", { exact: true })
      .waitFor();
    await page.reload();
    await page
      .getByLabel("Fixture custom · gelbooru", { exact: true })
      .waitFor();
    await page
      .getByRole("button", { name: "Source settings", exact: true })
      .click();
    await page
      .getByRole("button", { name: "Edit Fixture custom", exact: true })
      .click();
    assert.equal(
      await editor.getByLabel("API key", { exact: true }).inputValue(),
      ""
    );
    await editor.getByLabel("Enable this source", { exact: true }).uncheck();
    await editor
      .getByRole("button", { name: "Save source", exact: true })
      .click();
    await editor.waitFor({ state: "hidden" });
    await page
      .getByRole("button", { name: "Back to posts", exact: true })
      .click();
    await page
      .getByLabel("Fixture custom · gelbooru", { exact: true })
      .waitFor({ state: "hidden" });
    await page
      .getByRole("button", { name: "Source settings", exact: true })
      .click();
    await page
      .getByRole("button", { name: "Edit Fixture custom", exact: true })
      .click();
    await editor
      .getByRole("button", { name: "Remove custom source", exact: true })
      .click();
    await editor.waitFor({ state: "hidden" });
    assert.equal(
      await page
        .getByRole("button", { name: "Edit Fixture custom", exact: true })
        .count(),
      0
    );
    await page
      .getByRole("button", { name: "Back to posts", exact: true })
      .click();
    assert.equal(requests.length, 0, "editing settings issued searches");
    await page.getByLabel("Gelbooru · gelbooru", { exact: true }).check();
    const queryInput = page.getByLabel("Search tags (native source syntax)", {
      exact: true,
    });
    await queryInput.fill("cat -az fox");
    await queryInput.evaluate((input) => input.setSelectionRange(7, 7));
    await queryInput.press("ArrowLeft");
    await queryInput.press("ArrowRight");
    await page.getByRole("option", { name: /blue_hair/ }).waitFor();
    if (process.env.STASH_BROWSER_SCREENSHOTS)
      await page.screenshot({
        path: path.join(
          process.env.STASH_BROWSER_SCREENSHOTS,
          `booru-autocomplete-${viewport.width}.png`
        ),
      });
    await queryInput.press("Enter");
    assert.equal(await queryInput.inputValue(), "cat -blue_hair fox");
    assert.equal(
      requests.length,
      0,
      "autocomplete submitted a search despite a partial database failure"
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

    const tagPanel = page.getByRole("region", {
      name: "Tags on this page",
      exact: true,
    });
    if (viewport.width < 768)
      await tagPanel
        .getByRole("button", { name: "Show page tags", exact: true })
        .click();
    await tagPanel
      .getByRole("button", { name: "artist a", exact: true })
      .click();
    assert.equal(
      await page
        .getByLabel("Search tags (native source syntax)", { exact: true })
        .inputValue(),
      "artist_a"
    );
    assert.equal(
      requests.length,
      2,
      "using a sidebar tag searched automatically"
    );
    if (viewport.width < 768)
      await tagPanel
        .getByRole("button", { name: "Hide page tags", exact: true })
        .click();
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
    assert.ok(
      await dialog
        .getByLabel("Convert JPEG to JPEG XL before downloading")
        .isChecked()
    );
    await dialog
      .getByRole("button", { name: "Prepare download", exact: true })
      .click();
    await dialog
      .getByRole("link", { name: "Save JPEG XL", exact: true })
      .waitFor();
    if (process.env.STASH_BROWSER_SCREENSHOTS) {
      await dialog
        .getByRole("region", { name: "Download this post" })
        .scrollIntoViewIfNeeded();
      await page.screenshot({
        path: path.join(
          process.env.STASH_BROWSER_SCREENSHOTS,
          `booru-download-${viewport.width}.png`
        ),
      });
    }
    assert.equal(downloadRequests.at(-1).convert, true);
    assert.equal(downloadRequests.at(-1).key, "a".repeat(48));
    failNextDownload = true;
    await dialog
      .getByRole("button", { name: "Prepare again", exact: true })
      .click();
    await dialog
      .getByText("Fixture checksum mismatch", { exact: true })
      .waitFor();
    assert.equal(
      await dialog
        .getByRole("link", { name: "Save JPEG XL", exact: true })
        .count(),
      0,
      "failed download retained a save link"
    );

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

    await input.fill("gallery");
    await page
      .getByRole("button", { name: "Search selected sources", exact: true })
      .click();
    await dan
      .getByRole("button", { name: "Inspect Danbooru post 112", exact: true })
      .waitFor();
    await dan
      .getByRole("button", { name: "Select visible", exact: true })
      .click({ trial: true });
    await gel
      .getByRole("button", { name: "Select visible", exact: true })
      .click({ trial: true });
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
    // B02 states are independent, persisted and filtered only within this provider page.
    await dan
      .getByRole("button", { name: "Mark downloaded", exact: true })
      .click();
    await dan
      .getByText("Marked downloaded · manual", { exact: true })
      .waitFor();
    assert.equal(
      await gel
        .getByText("Marked downloaded · manual", { exact: true })
        .count(),
      0,
      "state leaked to the same ID on another provider"
    );
    await dan.getByRole("button", { name: "Ignore", exact: true }).click();
    await dan
      .getByText("0 shown / 1 provider posts · 1 ignored hidden", {
        exact: true,
      })
      .waitFor();
    assert.equal(
      await dan
        .getByRole("button", { name: "Inspect Danbooru post 101", exact: true })
        .count(),
      0
    );
    assert.equal(
      await dan.getByRole("button", { name: "Next", exact: true }).isEnabled(),
      true,
      "local filtering changed provider paging"
    );
    await dan
      .getByRole("button", { name: "Undo last review change", exact: true })
      .click();
    await dan
      .getByText("Marked downloaded · manual", { exact: true })
      .waitFor();
    await dan.getByRole("button", { name: "Ignore", exact: true }).click();
    await dan
      .getByText("0 shown / 1 provider posts · 1 ignored hidden", {
        exact: true,
      })
      .waitFor();
    await page.getByLabel("Show ignored posts", { exact: true }).check();
    await dan.getByText("Ignored", { exact: true }).waitFor();
    assert.ok(page.url().includes("ignored=1"));
    await page.reload();
    await dan.getByText("Ignored", { exact: true }).waitFor();
    await dan
      .getByText("Marked downloaded · manual", { exact: true })
      .waitFor();
    await dan.getByRole("button", { name: "Unignore", exact: true }).click();
    await dan.getByRole("button", { name: "Ignore", exact: true }).waitFor();
    await dan
      .getByRole("button", { name: "Clear manual mark", exact: true })
      .click();
    await dan
      .getByRole("button", { name: "Mark downloaded", exact: true })
      .waitFor();
    assert.equal(
      await dan
        .getByRole("link", { name: "In library · Image #42", exact: true })
        .getAttribute("href"),
      "/images/42"
    );
    assert.equal(
      await dan
        .getByRole("link", { name: "In library · Video #42", exact: true })
        .getAttribute("href"),
      "/scenes/42"
    );
    await gel
      .getByRole("button", { name: "Mark downloaded", exact: true })
      .click();
    await gel
      .getByText("Marked downloaded · manual", { exact: true })
      .waitFor();
    assert.equal(
      await gel.getByRole("link", { name: /In library/ }).count(),
      0,
      "manual marking fabricated a local link"
    );
    await dan
      .getByRole("button", { name: "Select visible", exact: true })
      .click();
    await dan
      .getByRole("button", { name: "Ignore selected", exact: true })
      .click();
    await dan.getByText("Ignored", { exact: true }).waitFor();
    await dan
      .getByRole("button", { name: "Select visible", exact: true })
      .click();
    await page.getByLabel("Show ignored posts", { exact: true }).uncheck();
    await page.getByLabel("Show ignored posts", { exact: true }).check();
    assert.equal(
      await dan
        .getByLabel("Select Danbooru post 101", { exact: true })
        .isChecked(),
      false,
      "hidden selections were resurrected by the filter"
    );
    await dan
      .getByRole("button", { name: "Select visible", exact: true })
      .click();
    await dan
      .getByRole("button", { name: "Mark selected downloaded", exact: true })
      .click();
    await dan
      .getByText("Marked downloaded · manual", { exact: true })
      .waitFor();
    await dan
      .getByRole("button", { name: "Undo last review change", exact: true })
      .click();
    await dan
      .getByRole("button", { name: "Mark downloaded", exact: true })
      .waitFor();
    await dan.getByText("Ignored", { exact: true }).waitFor();
    await dan
      .getByRole("button", { name: "Select visible", exact: true })
      .click();
    reviewConflict = true;
    await dan
      .getByRole("button", { name: "Unignore selected", exact: true })
      .click();
    await dan
      .getByRole("alert")
      .filter({ hasText: "Nothing was changed" })
      .waitFor();
    await dan.getByText("Ignored", { exact: true }).waitFor();
    assert.equal(
      await dan
        .getByRole("button", { name: "Unignore selected", exact: true })
        .isDisabled(),
      true,
      "stale review writes remained enabled"
    );
    await dan
      .getByRole("button", { name: "Reload review status", exact: true })
      .click();
    await dan.getByRole("button", { name: "Unignore", exact: true }).click();
    await dan.getByRole("button", { name: "Ignore", exact: true }).waitFor();
    await page.getByLabel("Show ignored posts", { exact: true }).uncheck();
    const searchesBeforeFilter = requests.length;
    await page.getByLabel("Show ignored posts", { exact: true }).check();
    await page.getByLabel("Show ignored posts", { exact: true }).uncheck();
    assert.equal(
      requests.length,
      searchesBeforeFilter,
      "changing review filters refetched provider searches"
    );
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
      `Booru browser ${viewport.width}×${viewport.height}: same-origin PNG thumbnails, CSV upload/alias keyboard autocomplete, JPEG XL preference/download/failure; durable review/bulk/undo/filters/conflicts/typed links; settings create/credentials/clear/conflict/reload/disable/remove; tag sidebar/search/paging/inspection/refresh/back, partial failures/retry, missing previews, races/cancel/cleanup and overflow passed.`
    );
  }
} finally {
  await browser.close();
  await server.close();
}
