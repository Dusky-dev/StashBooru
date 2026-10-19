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
const animatedGIF = Buffer.from(
  "R0lGODlhCAAIAIEAAP8AAAAAAAAAAAAAACH/C05FVFNDQVBFMi4wAwEAAAAh+QQAFAAAACwAAAAACAAIAAAIDwABCBxIsKDBgwgTKkwYEAAh+QQBFAABACwAAAAACAAIAIEAAP8AAAAAAAAAAAAIDwABCBxIsKDBgwgTKkwYEAA7",
  "base64"
);
const previewVideo = Buffer.from(
  "GkXfo59ChoEBQveBAULygQRC84EIQoKEd2VibUKHgQJChYECGFOAZwEAAAAAAAJjEU2bdLpNu4tTq4QVSalmU6yBoU27i1OrhBZUrmtTrIHYTbuMU6uEElTDZ1OsggEeTbuMU6uEHFO7a1OsggJN7AEAAAAAAABZAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAVSalmsirXsYMPQkBNgI1MYXZmNjAuMTYuMTAwV0GNTGF2ZjYwLjE2LjEwMESJiECPQAAAAAAAFlSua8GuAQAAAAAAADjXgQFzxYjrxSoRqelINZyBACK1nIN1bmSIgQCGhVZfVlA4g4EBI+ODhAvrwgDgibCBQLqBQJqBAhJUw2f8c3OgY8CAZ8iaRaOHRU5DT0RFUkSHjUxhdmY2MC4xNi4xMDBzc9ZjwItjxYjrxSoRqelINWfIoUWjh0VOQ09ERVJEh5RMYXZjNjAuMzEuMTAyIGxpYnZweGfIoUWjiERVUkFUSU9ORIeTMDA6MDA6MDEuMDAwMDAwMDAwAB9DtnVAqOeBAKPDgQAAgJADAJ0BKkAAQAAARwiFhYiFhIgCAgJ1qgP4AgboQVwx0hMAVVgA/v9NEv/8WFfxYV/FhX/xYV/8/M7txfzmAKOWgQDIANEBAAEQEAAYABhYL/QACI6AAKOWgQGQANEBAAEQEAAYABhYL/QACI6AAKOWgQJYANEBAAEQEAAYABhYL/QACI6AAKOWgQMgANEBAAEQEAAYABhYL/QACI6AABxTu2uRu4+zgQC3iveBAfGCAZ/wgQM=",
  "base64"
);
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
        mediaKey: (query === "broken"
          ? "b"
          : query === "proxy-fail"
            ? "d"
            : "a"
        ).repeat(48),
        directPreviewURL:
          query === "proxy-fail"
            ? "https://cdn.example.org/preview.svg"
            : undefined,
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
      mediaKey: String(id + index).padStart(48, "a"),
      format: index % 3 === 0 ? "webm" : index === 1 ? "gif" : "png",
      width: index % 2 ? 480 : 640,
      height: index % 2 ? 640 : 480,
      tags: [
        ...response.posts[0].tags.slice(0, 2),
        { name: "character_a", category: "character" },
        { name: "character_b", category: "character" },
      ],
    }));
  if (query === "attachments") {
    response.posts[0].attachments = [
      {
        ...response.posts[0],
        id: "p" + "e".repeat(32),
        mediaKey: "e".repeat(48),
        format: "png",
        md5: undefined,
        attachments: undefined,
      },
    ];
    response.posts.push({
      ...response.posts[0],
      id: "102",
      mediaKey: "f".repeat(48),
      attachments: [
        {
          ...response.posts[0].attachments[0],
          id: "p" + "9".repeat(32),
          mediaKey: "9".repeat(48),
        },
      ],
    });
  }
  return response;
}

try {
  for (const viewport of [
    { width: 1440, height: 1000 },
    { width: 390, height: 844 },
  ]) {
    const page = await browser.newPage({ viewport });
    page.setDefaultNavigationTimeout(60000);
    const errors = [];
    page.on("console", (message) => {
      if (
        /Encountered two children|Maximum update depth|Rendered .* hooks/.test(
          message.text()
        )
      )
        errors.push(message.text());
    });
    async function setChecked(input, checked) {
      if ((await input.isChecked()) === checked) return;
      const inputID = await input.getAttribute("id");
      if (inputID?.startsWith("booru-source-") && !(await input.isVisible()))
        await page.getByRole("button", { name: /^Sources \(/ }).click();
      if (
        ["booru-thumbnails", "booru-show-ignored"].includes(inputID) &&
        !(await input.isVisible())
      )
        await page
          .getByRole("button", { name: "Display options", exact: true })
          .click();
      const label = input.locator("xpath=following-sibling::label");
      if (await label.count()) await label.click();
      else await input.setChecked(checked);
      assert.equal(await input.isChecked(), checked);
      if (
        inputID?.startsWith("booru-source-") ||
        ["booru-thumbnails", "booru-show-ignored"].includes(inputID)
      )
        await page.locator(".booru-heading h2").click();
    }
    async function pickFilter(label, value) {
      const choices = {
        "Sort by": {
          "": "Provider default",
          newest: "Newest",
          popular: "Popular",
          updated: "Updated",
        },
        Rating: {
          "": "All ratings",
          safe: "Safe / General",
          questionable: "Questionable",
          explicit: "Explicit",
        },
        "Media on this page": {
          "": "All media",
          images: "Images",
          videos: "Videos",
        },
        Autocomplete: {
          live: "Live, with CSV fallback",
          database: "Installed CSV only",
        },
      };
      await page.getByRole("button", { name: label, exact: true }).click();
      await page
        .getByRole("menuitemradio", {
          name: choices[label][value],
          exact: true,
        })
        .click();
    }
    page.on("pageerror", (error) => errors.push(error.message));
    page.on("console", (entry) => {
      if (
        entry.type() === "error" &&
        /unmounted component|Maximum update depth|unhandled/i.test(entry.text())
      )
        errors.push(entry.text());
    });
    const bookmarks = { favorites: [], searches: [] };
    const requests = [];
    const autocompleteRequests = [];
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
    let defaultDirectory = "/library";
    let filenameTemplate = "";
    let tagDatabase = { count: 0, revision: "" };
    let downloadState;
    let motionKind = "webm";
    const motionRequests = [];
    let holdNextPreferences = false;
    let heldPreferences;
    let onHeldEditorPreferences;
    const downloadRequests = [];
    let failNextDownload = false;
    let holdNextDownload = false;
    let delayNextDownload = false;
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
      if (url.pathname.endsWith("/bookmarks")) {
        if (route.request().method() === "POST") {
          const value = route.request().postDataJSON();
          if (value.action === "favorite")
            bookmarks.favorites = value.favorite
              ? [
                  {
                    instance: "fixture-danbooru",
                    id: value.key.replace(/^a+/, "") || "101",
                  },
                ]
              : [];
          else {
            bookmarks.searches = bookmarks.searches.filter(
              (item) => item.name !== value.name
            );
            if (value.action === "search") {
              const params = new URLSearchParams(value.search);
              if (!params.get("q")?.trim() && params.get("favorites") !== "1")
                params.set("q", "*");
              bookmarks.searches.push({
                name: value.name,
                search: `?${params}`,
              });
            }
          }
        }
        await route.fulfill({ json: bookmarks });
        return;
      }
      if (url.pathname.endsWith("/favorites")) {
        const value = result(url.searchParams.get("source"), "favorite", 1);
        value.posts = bookmarks.favorites.length
          ? [{ ...value.posts[0], id: bookmarks.favorites[0].id }]
          : [];
        value.total = value.posts.length;
        value.hasMore = false;
        await route.fulfill({ json: value });
        return;
      }
      if (url.pathname.endsWith("/download-settings")) {
        if (route.request().method() === "GET" && holdNextPreferences) {
          holdNextPreferences = false;
          heldPreferences = {
            route,
            json: {
              convert: defaultJXL,
              defaultDirectory,
              filenameTemplate,
              roots: ["/library"],
            },
          };
          return;
        }
        if (route.request().method() === "GET" && onHeldEditorPreferences) {
          const resolve = onHeldEditorPreferences;
          onHeldEditorPreferences = undefined;
          resolve({
            route,
            json: {
              convert: defaultJXL,
              defaultDirectory,
              filenameTemplate,
              roots: ["/library"],
            },
          });
          return;
        }
        if (route.request().method() === "POST") {
          const data = route.request().postDataJSON();
          defaultJXL = data.convert;
          defaultDirectory = data.defaultDirectory;
          filenameTemplate = data.filenameTemplate ?? "";
        }
        await route.fulfill({
          json: {
            convert: defaultJXL,
            defaultDirectory,
            filenameTemplate,
            roots: ["/library"],
          },
        });
        return;
      }
      if (url.pathname.endsWith("/directories")) {
        const current = url.searchParams.get("path");
        await route.fulfill({
          json: {
            directories:
              current === "/library"
                ? ["/library/custom"]
                : current
                  ? []
                  : ["/library"],
            parent: current === "/library/custom" ? "/library" : "",
          },
        });
        return;
      }
      if (url.pathname.endsWith("/tag-path")) {
        const data = route.request().postDataJSON();
        assert.equal(data.path, "/models/tags.csv");
        tagDatabase = { count: 2, revision: "path-installed", path: data.path };
        await route.fulfill({ json: tagDatabase });
        return;
      }
      if (url.pathname.endsWith("/autocomplete")) {
        autocompleteRequests.push({
          source: url.searchParams.get("source"),
          mode: url.searchParams.get("mode"),
          query: url.searchParams.get("q"),
        });
        if (
          url.searchParams.get("source") === "gelbooru" &&
          url.searchParams.get("q")?.startsWith("az")
        ) {
          await route.fulfill({
            status: 502,
            json: { error: "Fixture live suggestions unavailable" },
          });
          return;
        }
        await route.fulfill({
          json: {
            mode:
              url.searchParams.get("mode") === "database" ? "database" : "live",
            suggestions: url.searchParams.get("q")?.startsWith("az")
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
        if (url.searchParams.get("kind") === "motion") {
          motionRequests.push(url.href);
          await route.fulfill({
            contentType: motionKind === "gif" ? "image/gif" : "video/webm",
            body: motionKind === "gif" ? animatedGIF : previewVideo,
          });
          return;
        }
        if (
          ["b".repeat(48), "d".repeat(48)].includes(url.searchParams.get("key"))
        )
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
            downloadState = holdNextDownload
              ? {
                  jobID: 71,
                  token: "c".repeat(32),
                  status: "queued",
                  converted: false,
                }
              : failNextDownload
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
                    library: {
                      kind: "image",
                      id: "42",
                      path: "/library/danbooru-101.jxl",
                      converted: true,
                    },
                    filename: "danbooru-101.jxl",
                  };
            failNextDownload = false;
            holdNextDownload = false;
          }
          if (delayNextDownload) {
            delayNextDownload = false;
            await new Promise((resolve) => setTimeout(resolve, 400));
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
      requests.push({
        source,
        query,
        page: number,
        limit: Number(url.searchParams.get("limit")),
      });
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
    await page
      .getByLabel("Gelbooru · gelbooru", { exact: true })
      .waitFor({ state: "attached" });
    assert.equal(
      requests.length,
      0,
      "opening the browser searched automatically"
    );
    // Live completion works before any tag database has been installed.
    const initialInput = page.getByLabel("Search tags (native source syntax)", {
      exact: true,
    });
    await initialInput.fill("az");
    await page.getByRole("option", { name: /blue_hair/ }).waitFor();
    assert.equal(tagDatabase.count, 0);
    assert.equal(autocompleteRequests.at(-1).mode, "live");
    await initialInput.press("Enter");
    await page
      .getByRole("button", {
        name: "Remove search term blue_hair",
        exact: true,
      })
      .click();
    assert.equal(await initialInput.inputValue(), "");
    assert.equal(requests.length, 0);
    await page.getByRole("button", { name: "Settings", exact: true }).click();
    await setChecked(
      page.getByLabel("Convert new downloads using Media converter defaults"),
      true
    );
    await page
      .getByLabel("Download filename template", { exact: true })
      .fill("%artist%/%md5%.%ext%");
    await page
      .getByRole("button", { name: "Save download settings", exact: true })
      .click();
    await page.getByText("Download settings saved.", { exact: true }).waitFor();
    await page
      .getByLabel("Autocomplete database CSV path on the server", {
        exact: true,
      })
      .fill("/models/tags.csv");
    await page
      .getByRole("button", { name: "Load database from path", exact: true })
      .click();
    await page
      .getByText("Imported 2 tags for Danbooru.", { exact: true })
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
    await setChecked(
      editor.getByLabel("Remove saved credentials on save", { exact: true }),
      true
    );
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
      .waitFor({ state: "attached" });
    await page.reload();
    await page
      .getByLabel("Fixture custom · gelbooru", { exact: true })
      .waitFor({ state: "attached" });
    await page.getByRole("button", { name: "Settings", exact: true }).click();
    await page
      .getByRole("button", { name: "Edit Fixture custom", exact: true })
      .click();
    assert.equal(
      await editor.getByLabel("API key", { exact: true }).inputValue(),
      ""
    );
    await setChecked(
      editor.getByLabel("Enable this source", { exact: true }),
      false
    );
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
    await page.getByRole("button", { name: "Settings", exact: true }).click();
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
    await setChecked(
      page.getByLabel("Gelbooru · gelbooru", { exact: true }),
      true
    );
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

    await setChecked(
      page.getByLabel("Gelbooru · gelbooru", { exact: true }),
      true
    );
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
        .getByLabel("Convert using Media converter defaults")
        .isChecked()
    );
    await dialog
      .getByRole("button", { name: "Download to Stash", exact: true })
      .click();
    await dialog
      .getByRole("link", { name: "Open in Stash", exact: true })
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
    assert.equal(downloadRequests.at(-1).destination, "/library");
    await dialog
      .getByRole("button", { name: "Download to…", exact: true })
      .click();
    const picker = page
      .getByRole("dialog")
      .filter({ hasText: "Choose Stash download folder" });
    assert.equal(
      await picker
        .getByRole("button", { name: "Open directory", exact: true })
        .count(),
      0
    );
    await picker.getByRole("button", { name: "Root", exact: true }).click();
    await picker.getByRole("button", { name: "/library", exact: true }).click();
    await picker
      .getByRole("button", { name: "/library/custom", exact: true })
      .click();
    await picker
      .getByRole("button", { name: "Use this folder", exact: true })
      .click();
    await dialog.getByText("Job #71: complete", { exact: true }).waitFor();
    assert.equal(downloadRequests.at(-1).destination, "/library/custom");

    failNextDownload = true;
    await dialog
      .getByRole("button", { name: "Retry download to Stash", exact: true })
      .click();
    await dialog
      .getByText("Fixture checksum mismatch", { exact: true })
      .waitFor();
    assert.equal(
      await dialog
        .getByRole("link", { name: "Open in Stash", exact: true })
        .count(),
      0,
      "failed download retained a library link"
    );

    holdNextDownload = true;
    await dialog
      .getByRole("button", { name: "Retry download to Stash", exact: true })
      .click();
    await dialog
      .getByRole("button", { name: "Cancel download", exact: true })
      .click();
    await dialog.getByText("Job #71: cancelled", { exact: true }).waitFor();
    holdNextDownload = true;
    await dialog
      .getByRole("button", { name: "Retry download to Stash", exact: true })
      .click();
    await dialog
      .getByRole("button", { name: "Cancel download", exact: true })
      .waitFor();
    await page.keyboard.press("Escape");
    await dialog.waitFor({ state: "hidden" });
    await dan
      .getByRole("button", { name: "Inspect Danbooru post 101", exact: true })
      .click();
    await dialog
      .getByRole("button", { name: "Cancel download", exact: true })
      .waitFor();
    downloadState = {
      jobID: 71,
      token: "c".repeat(32),
      status: "complete",
      converted: true,
      library: {
        kind: "image",
        id: "42",
        path: "/library/danbooru-101.jxl",
        converted: true,
      },
      filename: "danbooru-101.jxl",
    };
    await dialog
      .getByRole("link", { name: "Open in Stash", exact: true })
      .waitFor();
    await page.evaluate(() => {
      for (const key of Object.keys(sessionStorage))
        if (key.startsWith("booru-download:")) sessionStorage.removeItem(key);
    });
    delayNextDownload = true;
    await dialog
      .getByRole("button", { name: "Retry download to Stash", exact: true })
      .click();
    await page.keyboard.press("Escape");
    await dialog.waitFor({ state: "hidden" });
    await page.waitForFunction(() =>
      Object.keys(sessionStorage).some((key) =>
        key.startsWith("booru-download:")
      )
    );
    await dan
      .getByRole("button", { name: "Inspect Danbooru post 101", exact: true })
      .click();
    await dialog
      .getByRole("link", { name: "Open in Stash", exact: true })
      .waitFor();
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
    await dan.getByText(/Remote preview could not be loaded\./).waitFor();
    await dan
      .getByRole("button", { name: "Inspect Danbooru post 101", exact: true })
      .click();
    await page
      .getByRole("dialog")
      .getByText(/Remote preview could not be loaded\./)
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
      .getByLabel("Marked downloaded · manual", { exact: true })
      .waitFor();
    assert.equal(
      await gel
        .getByLabel("Marked downloaded · manual", { exact: true })
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
      .getByLabel("Marked downloaded · manual", { exact: true })
      .waitFor();
    await dan.getByRole("button", { name: "Ignore", exact: true }).click();
    await dan
      .getByText("0 shown / 1 provider posts · 1 ignored hidden", {
        exact: true,
      })
      .waitFor();
    await setChecked(
      page.getByLabel("Show ignored posts", { exact: true }),
      true
    );
    await dan.getByLabel("Ignored", { exact: true }).waitFor();
    assert.ok(page.url().includes("ignored=1"));
    await page.reload();
    await dan.getByLabel("Ignored", { exact: true }).waitFor();
    await dan
      .getByLabel("Marked downloaded · manual", { exact: true })
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
      .getByLabel("Marked downloaded · manual", { exact: true })
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
    await dan.getByLabel("Ignored", { exact: true }).waitFor();
    await dan
      .getByRole("button", { name: "Select visible", exact: true })
      .click();
    await setChecked(
      page.getByLabel("Show ignored posts", { exact: true }),
      false
    );
    await setChecked(
      page.getByLabel("Show ignored posts", { exact: true }),
      true
    );
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
      .getByLabel("Marked downloaded · manual", { exact: true })
      .waitFor();
    await dan
      .getByRole("button", { name: "Undo last review change", exact: true })
      .click();
    await dan
      .getByRole("button", { name: "Mark downloaded", exact: true })
      .waitFor();
    await dan.getByLabel("Ignored", { exact: true }).waitFor();
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
    await dan.getByLabel("Ignored", { exact: true }).waitFor();
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
    await setChecked(
      page.getByLabel("Show ignored posts", { exact: true }),
      false
    );
    const searchesBeforeFilter = requests.length;
    await setChecked(
      page.getByLabel("Show ignored posts", { exact: true }),
      true
    );
    await setChecked(
      page.getByLabel("Show ignored posts", { exact: true }),
      false
    );
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
    // Refresh restores the submitted provider page and separate unfinished form.
    await page.goto(
      `${baseURL}/booru?source=danbooru&q=landscape&p.danbooru=2`
    );
    await dan
      .getByRole("button", { name: "Inspect Danbooru post 201", exact: true })
      .waitFor();
    await page
      .getByLabel("Search tags (native source syntax)", { exact: true })
      .fill("unfinished -blue");
    await setChecked(
      page.getByLabel("Gelbooru · gelbooru", { exact: true }),
      true
    );
    await page.reload();
    await dan
      .getByRole("button", { name: "Inspect Danbooru post 201", exact: true })
      .waitFor();
    assert.equal(
      await page
        .getByLabel("Search tags (native source syntax)", { exact: true })
        .inputValue(),
      "unfinished -blue"
    );
    assert.ok(
      await page.getByLabel("Gelbooru · gelbooru", { exact: true }).isChecked()
    );
    await page.goto(`${baseURL}/booru`);
    await dan
      .getByRole("button", { name: "Inspect Danbooru post 201", exact: true })
      .waitFor();
    assert.ok(page.url().includes("p.danbooru=2"));
    assert.equal(
      await page
        .getByLabel("Search tags (native source syntax)", { exact: true })
        .inputValue(),
      "unfinished -blue"
    );
    await page.goto(`${baseURL}/booru?source=danbooru&q=proxy-fail`);
    await dan.locator("img").first().waitFor();
    await page.waitForFunction(() => {
      const image = document.querySelector(".booru-post img");
      return (
        image?.src.startsWith("https://cdn.example.org/") &&
        image.naturalWidth > 0
      );
    });
    // Sidebar filters, thumbnail hiding, chips and direct card actions remain
    // integrated with native paging, downloads and saved converter preferences.
    await page.goto(`${baseURL}/booru?source=danbooru&q=gallery`);
    await dan
      .getByRole("button", { name: "Inspect Danbooru post 102", exact: true })
      .waitFor();
    assert.equal(
      await page.getByRole("button", { name: "Sort by", exact: true }).count(),
      0
    );
    await dan
      .getByRole("button", { name: "Inspect Danbooru post 102", exact: true })
      .waitFor();
    await pickFilter("Rating", "safe");
    await dan
      .getByRole("button", { name: "Inspect Danbooru post 102", exact: true })
      .waitFor();
    assert.ok(page.url().includes("rating=safe"));
    const searchesBeforeMedia = requests.length;
    await pickFilter("Media on this page", "videos");
    await dan
      .getByText("Showing 4 of 12 posts on this loaded page.", { exact: false })
      .waitFor();
    assert.equal(await dan.locator(".booru-post").count(), 4);
    assert.equal(
      requests.length,
      searchesBeforeMedia,
      "loaded-page media filter refetched provider"
    );
    await setChecked(
      page.getByLabel("Show thumbnails", { exact: true }),
      false
    );
    assert.equal(await dan.locator(".booru-post img").count(), 0);
    await page.reload();
    await dan
      .getByText("Showing 4 of 12 posts on this loaded page.", { exact: false })
      .waitFor();
    assert.equal(
      await page.getByLabel("Show thumbnails", { exact: true }).isChecked(),
      false
    );
    await setChecked(page.getByLabel("Show thumbnails", { exact: true }), true);
    await pickFilter("Media on this page", "");
    await dan
      .getByRole("button", { name: "Download danbooru post 102", exact: true })
      .click();
    const card102 = dan.locator(".booru-post").filter({
      has: page.getByRole("button", {
        name: "Inspect Danbooru post 102",
        exact: true,
      }),
    });
    await card102
      .getByRole("link", { name: "Open in Stash", exact: true })
      .waitFor();
    assert.equal(downloadRequests.at(-1).destination, defaultDirectory);
    assert.equal(downloadRequests.at(-1).convert, defaultJXL);
    await pickFilter("Autocomplete", "database");
    await page
      .getByLabel("Search tags (native source syntax)", { exact: true })
      .fill("az");
    await page.getByRole("option", { name: /blue_hair/ }).waitFor();
    assert.equal(autocompleteRequests.at(-1).mode, "database");
    await page
      .getByLabel("Search tags (native source syntax)", { exact: true })
      .press("Escape");
    await pickFilter("Autocomplete", "live");

    await pickFilter("Rating", "");
    await page.waitForFunction(
      () => document.documentElement.scrollWidth <= window.innerWidth + 1
    );
    if (process.env.STASH_BROWSER_SCREENSHOTS)
      await page.screenshot({
        path: path.join(
          process.env.STASH_BROWSER_SCREENSHOTS,
          `booru-live-${viewport.width}.png`
        ),
        fullPage: true,
      });
    assert.equal(await page.locator(".booru-results-header").count(), 0);
    assert.ok(
      await page.locator(".filtered-list-toolbar .page-size-selector").count()
    );
    assert.ok(await page.locator(".pagination-index-container").count());
    await page.getByRole("button", { name: "Wall view", exact: true }).click();
    await dan.locator(".booru-wall-item").first().waitFor();
    await page.locator(".zoom-slider").fill("2");
    await page.reload();
    await dan.locator(".booru-wall-item").first().waitFor();
    assert.equal(await page.locator(".zoom-slider").inputValue(), "2");
    await page.waitForFunction(() => {
      const img = document.querySelector(".booru-wall-item img");
      const box = img?.getBoundingClientRect();
      return (
        img?.complete &&
        img.naturalWidth > 0 &&
        box?.width > 1 &&
        box?.height > 1
      );
    });
    await dan
      .locator(".booru-wall-item img")
      .first()
      .evaluate((img) => img.decode());
    const wallCheck = dan
      .locator(".booru-wall-item")
      .first()
      .locator(".card-check");
    const wallBounds = await dan
      .locator(".booru-wall-item")
      .first()
      .boundingBox();
    const checkBounds = await wallCheck.boundingBox();
    assert.ok(
      checkBounds.y - wallBounds.y < 30,
      "wall checkbox left the top corner"
    );
    if (process.env.STASH_BROWSER_SCREENSHOTS)
      await page.screenshot({
        path: path.join(
          process.env.STASH_BROWSER_SCREENSHOTS,
          `booru-wall-${viewport.width}.png`
        ),
        fullPage: false,
      });
    await page.getByRole("button", { name: "Grid view", exact: true }).click();
    await page.locator(".zoom-slider").fill("0");
    const firstCard = dan.locator(".booru-post").first();
    await firstCard.getByRole("button", { name: /^Artists for post/ }).click();
    assert.equal(
      await page
        .getByLabel("Search tags (native source syntax)", { exact: true })
        .inputValue(),
      "artist_a"
    );
    await page
      .getByRole("button", { name: "Saved search filters", exact: true })
      .click();
    await page
      .getByLabel("Saved filter name", { exact: true })
      .fill("Artist filter");
    await page
      .getByRole("button", { name: "Save current search", exact: false })
      .click();
    await page
      .getByRole("button", { name: "Saved search filters", exact: true })
      .click();
    await firstCard.getByRole("button", { name: /^Favorite post/ }).click();
    await firstCard
      .getByRole("button", { name: /^Remove favorite post/ })
      .waitFor();
    await firstCard
      .getByRole("button", { name: /^Characters for post/ })
      .click();
    const tagChooser = page
      .getByRole("dialog")
      .filter({ hasText: "Choose Characters" });
    await tagChooser
      .getByRole("button", { name: "character_b", exact: true })
      .click();
    assert.equal(
      await page
        .getByLabel("Search tags (native source syntax)", { exact: true })
        .inputValue(),
      "character_b"
    );
    await page.getByRole("button", { name: "Favorites", exact: true }).click();
    await dan.locator(".booru-post").first().waitFor();
    assert.ok(page.url().includes("favorites=1"));
    await dan
      .locator(".booru-post")
      .first()
      .getByRole("button", { name: /^Remove favorite post/ })
      .click();
    await dan
      .getByText("No posts returned for this page and query.", { exact: true })
      .waitFor();
    await page.reload();
    await dan
      .getByText("No posts returned for this page and query.", { exact: true })
      .waitFor();

    await page
      .getByRole("button", { name: "Saved search filters", exact: true })
      .click();
    await page
      .getByRole("button", { name: "Delete search Artist filter", exact: true })
      .waitFor();
    await page
      .getByRole("button", { name: "Artist filter", exact: true })
      .click();
    await dan.locator(".booru-post").first().waitFor();
    assert.equal(new URL(page.url()).searchParams.get("q"), "artist_a");
    await page
      .getByRole("button", { name: "Saved search filters", exact: true })
      .click();
    await page
      .getByRole("button", { name: "Delete search Artist filter", exact: true })
      .click();
    await page.getByText("No saved filters", { exact: true }).waitFor();
    await page
      .getByRole("button", { name: "Saved search filters", exact: true })
      .click();
    await page.goto(`${baseURL}/booru?source=danbooru`);
    await page
      .getByRole("button", { name: "Saved search filters", exact: true })
      .click();
    await page.getByLabel("Saved filter name", { exact: true }).fill("Recent");
    await page
      .getByRole("button", { name: "Save current search", exact: false })
      .click();
    await page.getByRole("button", { name: "Recent", exact: true }).click();
    await dan.locator(".booru-post").first().waitFor();
    assert.equal(new URL(page.url()).searchParams.get("q"), "*");
    await page.reload();
    await dan.locator(".booru-post").first().waitFor();
    assert.equal(
      await page
        .getByLabel("Search tags (native source syntax)", { exact: true })
        .inputValue(),
      "*"
    );
    await page.goto(`${baseURL}/booru?source=danbooru&q=attachments`);
    await dan
      .getByRole("button", { name: "Inspect Danbooru post 101", exact: true })
      .click();
    const attachmentPreview = page.getByRole("dialog");
    await attachmentPreview
      .getByLabel("Media file", { exact: true })
      .selectOption("1");
    assert.equal(
      await attachmentPreview.getByText("MD5:", { exact: false }).count(),
      0
    );
    await attachmentPreview
      .getByRole("button", { name: "Download to Stash", exact: true })
      .click({ trial: true });
    const attachmentRequest = page.waitForRequest(
      (request) =>
        request.method() === "POST" &&
        new URL(request.url()).pathname.endsWith("/booru-browser/download")
    );
    await attachmentPreview
      .getByRole("button", { name: "Refresh previews", exact: true })
      .focus();
    await page.keyboard.press("ArrowUp");
    assert.equal((await attachmentRequest).postDataJSON().key, "e".repeat(48));
    await attachmentPreview
      .getByRole("button", { name: "Next preview (Right arrow)", exact: true })
      .focus();
    await page.keyboard.press("ArrowRight");
    await attachmentPreview
      .getByText("Danbooru #102", { exact: true })
      .waitFor();
    assert.equal(
      await attachmentPreview
        .getByLabel("Media file", { exact: true })
        .inputValue(),
      "0"
    );
    assert.equal(
      await attachmentPreview.getByText("MD5:", { exact: false }).count(),
      1
    );
    const nextPrimaryRequest = page.waitForRequest(
      (request) =>
        request.method() === "POST" &&
        new URL(request.url()).pathname.endsWith("/booru-browser/download")
    );
    await attachmentPreview
      .getByRole("button", { name: "Refresh previews", exact: true })
      .focus();
    await page.keyboard.press("ArrowUp");
    assert.equal((await nextPrimaryRequest).postDataJSON().key, "f".repeat(48));
    await attachmentPreview
      .getByLabel("Media file", { exact: true })
      .selectOption("1");
    await attachmentPreview
      .getByRole("button", {
        name: "Previous preview (Left arrow)",
        exact: true,
      })
      .click();
    await attachmentPreview
      .getByText("Danbooru #101", { exact: true })
      .waitFor();
    assert.equal(
      await attachmentPreview
        .getByLabel("Media file", { exact: true })
        .inputValue(),
      "0"
    );
    await attachmentPreview
      .locator(".modal-footer")
      .getByRole("button", { name: "Close", exact: true })
      .click();
    await page.goto(`${baseURL}/booru?source=danbooru&q=gallery`);
    await dan.locator(".booru-post").first().waitFor();
    // Native controls and compact cards expose details only in inspection.
    assert.equal(
      await page
        .getByRole("button", { name: "Downloads & autocomplete", exact: true })
        .count(),
      0
    );
    assert.equal(await dan.locator(".booru-card-tags").count(), 0);
    assert.equal(
      await dan
        .getByRole("button", { name: "Ignore selected", exact: true })
        .count(),
      0
    );
    assert.equal(await dan.locator(".card-check").count(), 12);
    assert.equal(
      (await page.locator(".custom-switch input").count()) > 0,
      true
    );
    const inputBackground = await page
      .getByLabel("Search tags (native source syntax)")
      .evaluate((input) => getComputedStyle(input).backgroundColor);
    assert.notEqual(inputBackground, "rgb(255, 255, 255)");
    await setChecked(
      dan.getByLabel("Select Danbooru post 103", { exact: true }),
      true
    );
    await dan
      .locator(".selected-items-info")
      .getByText("1 selected", { exact: true })
      .waitFor();
    await dan
      .getByRole("button", { name: "Clear selection", exact: true })
      .click();
    assert.equal(await dan.locator(".selected-items-info").count(), 0);
    await page.locator(".page-size-selector select").selectOption("40");
    await dan
      .getByRole("button", { name: "Inspect Danbooru post 112", exact: true })
      .waitFor();
    assert.equal(requests.at(-1).limit, 40);
    assert.ok(page.url().includes("limit=40"));
    await page.reload();
    await dan
      .getByRole("button", { name: "Inspect Danbooru post 112", exact: true })
      .waitFor();
    assert.equal(
      await page.locator(".page-size-selector select").inputValue(),
      "40"
    );
    assert.equal(
      motionRequests.length,
      0,
      "browsing loaded animated originals"
    );
    await dan
      .getByRole("button", { name: "Inspect Danbooru post 101", exact: true })
      .click();
    const preview = page.getByRole("dialog");
    await preview
      .getByRole("button", {
        name: "Previous preview (Left arrow)",
        exact: true,
      })
      .waitFor();
    assert.equal(
      await preview
        .getByRole("button", {
          name: "Previous preview (Left arrow)",
          exact: true,
        })
        .isDisabled(),
      true
    );
    await preview
      .getByRole("button", { name: "Play animated preview", exact: true })
      .click();
    await page.waitForFunction(() => {
      const video = document.querySelector(".booru-browser-inspection video");
      return video?.readyState >= 2 && video.videoWidth === 64 && !video.paused;
    });
    assert.equal(motionRequests.length, 1);
    assert.equal(
      new URL(motionRequests[0]).searchParams.get("key"),
      "101".padStart(48, "a")
    );
    // Native player arrows must retain their playback behavior.
    await preview.locator("video").focus();
    await page.keyboard.press("ArrowRight");
    await preview.getByText("Danbooru #101", { exact: true }).waitFor();
    await preview
      .getByRole("button", { name: "Next preview (Right arrow)", exact: true })
      .focus();
    await page.keyboard.press("ArrowRight");
    await preview.getByText("Danbooru #102", { exact: true }).waitFor();
    await preview
      .locator("video")
      .waitFor({ state: "detached", timeout: 3000 });
    motionKind = "gif";
    await preview
      .getByRole("button", { name: "Play animated preview", exact: true })
      .click();
    await page.waitForFunction(() => {
      const image = document.querySelector(".booru-motion-preview img");
      return image?.complete && image.naturalWidth === 8;
    });
    assert.equal(
      new URL(motionRequests.at(-1)).searchParams.get("key"),
      "102".padStart(48, "a")
    );
    await preview
      .getByRole("button", { name: "Stop animated preview", exact: true })
      .click();
    assert.equal(await preview.locator('img[src*="kind=motion"]').count(), 0);
    await preview
      .getByRole("button", { name: "Next preview (Right arrow)", exact: true })
      .focus();
    await page.keyboard.press("ArrowRight");
    await preview.getByText("Danbooru #103", { exact: true }).waitFor();
    const keyboardDownload = page.waitForRequest(
      (request) =>
        request.method() === "POST" &&
        new URL(request.url()).pathname.endsWith("/booru-browser/download")
    );
    await preview
      .getByRole("button", { name: "Download to Stash", exact: true })
      .waitFor({ state: "visible" });
    await preview
      .getByRole("button", { name: "Download to Stash", exact: true })
      .click({ trial: true });
    await preview
      .getByRole("button", { name: "Next preview (Right arrow)", exact: true })
      .focus();
    await page.keyboard.press("ArrowUp");
    const keyboardPayload = (await keyboardDownload).postDataJSON();
    assert.equal(keyboardPayload.destination, defaultDirectory);
    assert.equal(keyboardPayload.key, "103".padStart(48, "a"));
    const keyboardIgnore = page.waitForRequest(
      (request) =>
        request.method() === "POST" &&
        new URL(request.url()).pathname.endsWith("/booru-browser/review/update")
    );
    await preview
      .getByRole("button", { name: "Ignore post (Down arrow)", exact: true })
      .click({ trial: true });
    await preview
      .getByRole("button", { name: "Next preview (Right arrow)", exact: true })
      .focus();
    await page.keyboard.press("ArrowDown");
    const ignored = (await keyboardIgnore).postDataJSON().changes[0];
    assert.equal(ignored.ignored, true);
    assert.equal(ignored.id, "103");
    await preview.getByText("Ignored", { exact: true }).waitFor();
    await preview
      .getByRole("button", { name: "Next preview (Right arrow)", exact: true })
      .focus();
    await page.keyboard.press("ArrowLeft");
    await preview.getByText("Danbooru #102", { exact: true }).waitFor();
    await page.keyboard.press("ArrowRight");
    await preview.getByText("Danbooru #104", { exact: true }).waitFor();
    await page.keyboard.press("Escape");
    await preview.waitFor({ state: "hidden" });
    motionKind = "webm";

    await page.evaluate(() => sessionStorage.clear());
    // Saving preferences invalidates shared requests; a late old response must
    // not restore the old path/conversion flag on mounted download cards.
    holdNextPreferences = true;
    await page.goto(`${baseURL}/booru?source=danbooru&q=gallery`);
    await dan
      .getByRole("button", { name: "Inspect Danbooru post 105", exact: true })
      .waitFor();
    const editorPreferencesRequested = new Promise((resolve) => {
      onHeldEditorPreferences = resolve;
    });
    await page.getByRole("button", { name: "Settings", exact: true }).click();
    const editorPreferences = await editorPreferencesRequested;
    assert.equal(
      await page
        .getByLabel("Default Stash download folder", { exact: true })
        .isDisabled(),
      true
    );
    assert.equal(
      await page
        .getByRole("button", { name: "Choose default folder", exact: true })
        .isDisabled(),
      true
    );
    await editorPreferences.route.fulfill({ json: editorPreferences.json });
    await page
      .getByLabel("Default Stash download folder", { exact: true })
      .fill("/library/custom");
    await setChecked(
      page.getByLabel("Convert new downloads using Media converter defaults"),
      false
    );
    await page
      .getByRole("button", { name: "Save download settings", exact: true })
      .click();
    await page.getByText("Download settings saved.", { exact: true }).waitFor();
    assert.equal(defaultDirectory, "/library/custom");
    assert.equal(filenameTemplate, "%artist%/%md5%.%ext%");
    assert.equal(defaultJXL, false);
    assert.ok(heldPreferences, "card preferences request was not held");
    await heldPreferences.route.fulfill({ json: heldPreferences.json });
    heldPreferences = undefined;
    await page.evaluate(
      () =>
        new Promise((resolve) =>
          requestAnimationFrame(() => requestAnimationFrame(resolve))
        )
    );
    await page
      .getByRole("button", { name: "Back to posts", exact: true })
      .click();
    const cardDownloadRequest = page.waitForRequest(
      (request) =>
        request.method() === "POST" &&
        new URL(request.url()).pathname.endsWith("/booru-browser/download")
    );
    await dan
      .getByRole("button", { name: "Download danbooru post 105", exact: true })
      .click();
    const cardPayload = (await cardDownloadRequest).postDataJSON();
    assert.equal(cardPayload.destination, "/library/custom");
    assert.equal(cardPayload.convert, false);
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
    await page
      .getByLabel("Danbooru · danbooru", { exact: true })
      .waitFor({ state: "attached" });
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
      `Booru browser ${viewport.width}×${viewport.height}: native dark inputs/switches/icon filters/card selection/page size, Left/Right/Up/Down inspection keys, decoded WebM and animated GIF with cleanup, compact actions, same-origin PNG thumbnails, CSV upload/alias keyboard autocomplete, converter defaults/library destination/custom folder/download/failure/cancel/reopen; durable review/bulk/undo/filters/conflicts/typed links; settings create/credentials/clear/conflict/reload/disable/remove; tag sidebar/search/paging/inspection/refresh/back, partial failures/retry, missing previews, races/cancel/cleanup and overflow passed.`
    );
  }
} catch (error) {
  if (
    process.env.STASH_BROWSER_SCREENSHOTS &&
    browser.contexts()[0]?.pages()[0]
  )
    await browser
      .contexts()[0]
      .pages()[0]
      .screenshot({
        path: path.join(
          process.env.STASH_BROWSER_SCREENSHOTS,
          "booru-failure.png"
        ),
        fullPage: true,
      })
      .catch(() => {});
  throw error;
} finally {
  await browser.close();
  await server.close();
}
