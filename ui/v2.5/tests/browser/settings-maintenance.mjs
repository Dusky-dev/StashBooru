import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { createServer } from "vite";

const engines = await import(process.env.PLAYWRIGHT_MODULE ?? "playwright");
const root = path.resolve(import.meta.dirname, "../..");
const baseline = process.env.STASH_SETTINGS_BASELINE === "1";
const server = await createServer({
  root,
  define: {
    "import.meta.env.VITE_APP_STASH_VERSION": JSON.stringify(
      "v0.1.0-1348-g516246302 (fixture)"
    ),
    "import.meta.env.VITE_APP_GITHASH": JSON.stringify("fixture-build"),
    "import.meta.env.VITE_APP_DATE": JSON.stringify("2026-10-05 (fixture)"),
  },
  server: { host: "127.0.0.1", port: 0 },
  plugins: [
    {
      name: "settings-fixture-route",
      configureServer(vite) {
        vite.middlewares.use((req, _res, next) => {
          if (req.url?.split("?")[0] === "/settings") {
            req.url = `/tests/browser/fixtures/settings-maintenance.html${req.url.includes("?") ? req.url.slice(req.url.indexOf("?")) : ""}`;
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
const shots = process.env.STASH_BROWSER_SCREENSHOTS;
if (shots) fs.mkdirSync(shots, { recursive: true });
const active = (page) => page.locator(".tab-pane.active");
const uiLink = (page, tab) =>
  page.locator(`#settings-menu-container a[href='/settings?tab=${tab}']`);
const formats = [
  {
    id: "webp",
    label: "WebP",
    family: "image",
    available: true,
    controls: ["quality", "lossless"],
    encoders: [{ id: "webp", label: "WebP", hardware: "cpu" }],
  },
  {
    id: "png",
    label: "PNG",
    family: "image",
    available: true,
    controls: [],
    encoders: [{ id: "png", label: "PNG", hardware: "cpu" }],
  },
  {
    id: "mp4",
    label: "MP4",
    family: "video",
    available: true,
    controls: ["quality"],
    encoders: [{ id: "h264", label: "H.264", hardware: "cpu" }],
  },
];
let currentPage;
try {
  console.log(`Browser: ${browser.version()}`);
  for (const [name, viewport] of [
    ["desktop", { width: 1440, height: 1000 }],
    ["mobile", { width: 390, height: 844 }],
  ]) {
    const page = await browser.newPage({ viewport });
    currentPage = page;
    const errors = [];
    page.on("pageerror", (error) => errors.push(error.message));
    page.on("console", (msg) => {
      if (/unmounted component|Maximum update depth/i.test(msg.text()))
        errors.push(msg.text());
    });
    const requests = [];
    let failLocal = false;
    let holdStatus;
    let holdNextStatus = false;
    let statusStarted;
    let tagging = {
      threshold: 0.492,
      eva02Threshold: 0.35,
      limit: 50,
      filenameEnabled: true,
      filenameLayout: "[%artist%](%copyright%).%character%_%md5%.%ext%",
    };
    let worker = { url: "http://worker.invalid:8000", tokenConfigured: true };
    let local = {
      waifu2xExecutable: "/usr/bin/waifu2x-ncnn-vulkan",
      waifu2xModels: "/models/cunet",
      seedVR2CLI: "",
      seedVR2Models: "",
      seedVR2Model: "",
      seedVR2Python: "",
      seedVR2BlocksToSwap: 0,
    };
    const config = {
      backend: "auto",
      savings: { minimumSavedBytes: 0, minimumSavedPercent: 0 },
      trialCacheLimitBytes: 10 * 1024 ** 3,
      trialTTLHours: 24,
      formatDefaults: { image: "webp", video: "mp4" },
      encodingDefaults: {
        image: { quality: 90, effort: 7 },
        video: { quality: 80, effort: 7 },
      },
    };
    const capability = (backend) => ({
      backend,
      formats,
      upscalers: [
        {
          id: "waifu2x",
          label: "waifu2x",
          available: true,
          cpu: true,
          notice: "Fixture model ready",
        },
        {
          id: "seedvr2",
          label: "SeedVR2",
          available: true,
          cpu: false,
          notice: "Fixture GPU model ready",
        },
      ],
    });
    const status = {
      installed: true,
      loaded: false,
      workerOK: true,
      backend: "remote",
      model: "EVA02",
      revision: "fixture",
      dimensions: 768,
      indexedImages: 42,
      totalImages: 60,
      modelPath: `/models/${"long-path-".repeat(20)}model.onnx`,
    };
    await page.route(/\/(image|metadata)\//, async (route) => {
      const url = new URL(route.request().url());
      const method = route.request().method();
      const body =
        method === "POST" ? route.request().postDataJSON() : undefined;
      requests.push({ path: url.pathname, search: url.search, method, body });
      let data = {};
      if (url.pathname.endsWith("/remote-config")) {
        if (body)
          worker = {
            url: body.url,
            tokenConfigured: body.clearToken
              ? false
              : !!body.token || worker.tokenConfigured,
          };
        data = worker;
      } else if (url.pathname.endsWith("/camie/config")) {
        if (body) tagging = body;
        data = tagging;
      } else if (url.pathname.endsWith("/camie/status"))
        data = {
          ...status,
          model: "Camie",
          modelExists: true,
          metadataExists: true,
          metadataPath: "/models/camie.json",
          tagCount: 1000,
        };
      else if (url.pathname.endsWith("/visual-similarity/status")) {
        if (holdNextStatus) {
          holdNextStatus = false;
          await new Promise((resolve) => {
            holdStatus = resolve;
            statusStarted?.();
          });
        }
        data = status;
      } else if (/\/(index|download)$/.test(url.pathname)) data = { jobID: 71 };
      else if (url.pathname.endsWith("/upscaler-config")) {
        if (failLocal) {
          await route.fulfill({ status: 503, body: "Local setup unavailable" });
          return;
        }
        if (body) local = body;
        data = local;
      } else if (url.pathname.endsWith("/converter")) {
        if (url.searchParams.has("capabilities"))
          data = capability(
            url.searchParams.get("backend") === "local" ? "local" : "remote"
          );
        else if (url.searchParams.has("config"))
          data = {
            config,
            inputFormats: [
              { id: "image", label: "Other images", family: "image" },
              { id: "video", label: "Other videos", family: "video" },
            ],
            outputFormats: formats,
          };
        else if (body?.action === "save-defaults") {
          Object.assign(config, body);
          data = config;
        }
      } else if (url.pathname.includes("association"))
        data = {
          settings: {
            characters: true,
            artists: true,
            copyrights: true,
            tags: true,
          },
        };
      await route.fulfill({
        contentType: "application/json",
        body: JSON.stringify(data),
      });
    });
    await page.goto(
      `${baseURL}/settings?tab=${baseline ? "system" : "library"}`
    );
    if (baseline) {
      await page.locator("#inference-worker input[type=url]:enabled").waitFor();
      if (shots)
        await page.screenshot({ path: `${shots}/before-settings-${name}.png` });
      await page.close();
      continue;
    }
    await page.locator("#inherit-character-ancestors").waitFor();
    assert.equal(
      requests.filter((r) => r.path.startsWith("/image/")).length,
      0,
      "unvisited processing settings must not contact workers"
    );
    await uiLink(page, "processing").click();
    await page
      .getByRole("button", { name: "Save tagging defaults", exact: true })
      .waitFor();
    await page.locator("#inference-worker-url:enabled").waitFor();
    await page.locator("#upscaling-default-scale").waitFor();
    assert.equal(
      await active(page)
        .getByRole("button", { name: "Save tagging defaults", exact: true })
        .count(),
      1
    );
    assert.equal(
      await page.locator("#media-upscaling details[open]").count(),
      0
    );
    await page.waitForFunction(
      () =>
        getComputedStyle(document.querySelector(".tab-pane.active")).opacity ===
        "1"
    );
    await page.locator("#inference-worker").scrollIntoViewIfNeeded();
    if (shots)
      await page.screenshot({
        path: `${shots}/processing-settings-${name}.png`,
      });

    // A status refresh must not discard an unsaved tagging draft.
    await page
      .getByLabel("EVA02 tagging threshold", { exact: true })
      .fill("0.6");
    await page
      .getByLabel("Filename layout", { exact: true })
      .fill("%character%_%md5%.%ext%");
    await page
      .getByRole("button", { name: "Refresh status", exact: true })
      .click();
    await page.locator("#tagging-eva02-threshold:enabled").waitFor();
    assert.equal(
      await page
        .getByLabel("EVA02 tagging threshold", { exact: true })
        .inputValue(),
      "0.6"
    );
    await page
      .getByRole("button", { name: "Save tagging defaults", exact: true })
      .click();
    await page.getByText("Saved tagging defaults.", { exact: true }).waitFor();
    assert.equal(tagging.eva02Threshold, 0.6);
    assert.equal(tagging.filenameLayout, "%character%_%md5%.%ext%");
    await page.getByLabel("Per-category limit", { exact: true }).fill("0");
    assert.equal(
      await page
        .getByRole("button", { name: "Save tagging defaults", exact: true })
        .isDisabled(),
      true
    );
    await page.getByLabel("Per-category limit", { exact: true }).fill("50");

    // Browser defaults and server setup have separate save actions and effects.
    await page.getByLabel("Scale", { exact: true }).selectOption("4");
    await page
      .getByRole("button", { name: "Save upscaling defaults", exact: true })
      .click();
    assert.equal(
      JSON.parse(
        await page.evaluate(() =>
          localStorage.getItem("stashbooru-upscaling-defaults")
        )
      ).scale,
      4
    );
    assert.equal(
      requests.filter(
        (r) => r.path.endsWith("/upscaler-config") && r.method === "POST"
      ).length,
      0
    );
    await page.locator("#media-upscaling summary").click();
    await page
      .getByLabel("Executable path", { exact: true })
      .fill("/custom/waifu2x");
    await page.getByLabel("Scale", { exact: true }).selectOption("2");
    await page
      .getByRole("button", { name: "Save local model setup", exact: true })
      .click();
    await page
      .getByText("Saved local upscaler setup.", { exact: true })
      .waitFor();
    assert.equal(local.waifu2xExecutable, "/custom/waifu2x");
    assert.equal(
      JSON.parse(
        await page.evaluate(() =>
          localStorage.getItem("stashbooru-upscaling-defaults")
        )
      ).scale,
      4,
      "saving paths must not save a browser draft"
    );
    await page.locator("#upscaling-default-scale").waitFor();
    await page
      .locator(".toast-container.hidden")
      .waitFor({ state: "attached" });
    await page.locator("#media-upscaling").scrollIntoViewIfNeeded();
    if (shots)
      await page.screenshot({
        path: `${shots}/upscaling-settings-${name}.png`,
      });

    // Repeated navigation preserves local drafts; returning from a queued job re-enables controls.
    await page
      .getByLabel("EVA02 tagging threshold", { exact: true })
      .fill("0.7");
    await uiLink(page, "system").click();
    assert.equal(await active(page).locator("#inference-worker").count(), 0);
    await uiLink(page, "processing").click();
    assert.equal(
      await page
        .getByLabel("EVA02 tagging threshold", { exact: true })
        .inputValue(),
      "0.7"
    );
    await page
      .getByRole("button", { name: "Generate visual embeddings", exact: true })
      .click();
    await page.waitForURL(/tab=tasks/);
    await uiLink(page, "processing").click();
    assert.equal(
      await page
        .getByRole("button", {
          name: "Generate visual embeddings",
          exact: true,
        })
        .isEnabled(),
      true
    );

    // Direct old bookmarks, unrelated parameters, hash positioning and browser refresh.
    await page.goto(
      `${baseURL}/settings?tab=system&return=images%2F42#media-upscaling`
    );
    await page.waitForURL(/tab=processing.*#media-upscaling$/);
    await page.locator("#upscaling-default-scale").waitFor();
    assert.equal(new URL(page.url()).searchParams.get("return"), "images/42");
    await page.waitForFunction(() => {
      const y = document
        .querySelector("#media-upscaling h1")
        ?.getBoundingClientRect().y;
      const atBottom =
        window.scrollY + window.innerHeight >=
        document.documentElement.scrollHeight - 2;
      return y >= 0 && (y < 180 || (atBottom && y < window.innerHeight));
    });
    assert.equal(
      await page.getByLabel("Scale", { exact: true }).inputValue(),
      "4"
    );
    await page.goto(
      `${baseURL}/settings?tab=system#association-inheritance-settings`
    );
    await page.waitForURL(/tab=library/);
    await page.locator("#inherit-character-ancestors").waitFor();

    failLocal = true;
    await page.goto(`${baseURL}/settings?tab=processing#media-upscaling`);
    await page
      .getByRole("button", { name: "Save upscaling defaults", exact: true })
      .waitFor();
    await page.getByLabel("Scale", { exact: true }).selectOption("2");
    await page
      .getByRole("button", { name: "Save upscaling defaults", exact: true })
      .click();
    assert.equal(
      JSON.parse(
        await page.evaluate(() =>
          localStorage.getItem("stashbooru-upscaling-defaults")
        )
      ).scale,
      2
    );
    await page.locator("#media-upscaling summary").click();
    assert.equal(
      await page
        .getByRole("button", { name: "Save local model setup", exact: true })
        .isDisabled(),
      true
    );

    const overflow = await page.evaluate(
      () => document.documentElement.scrollWidth > window.innerWidth + 1
    );
    assert.equal(overflow, false, `${name} page must fit the viewport`);
    // The mounted About panel separates version tracks and can retry a failed check.
    await uiLink(page, "about").click();
    await page
      .getByRole("heading", { name: "StashBooru 1.0.0", exact: true })
      .waitFor();
    await page
      .getByText("You are running the latest stable StashBooru release.")
      .waitFor();
    assert.equal(
      await page.getByRole("button", { name: "Check for updates" }).isEnabled(),
      true
    );
    await page
      .getByRole("heading", { name: "Upstream Stash version", exact: true })
      .waitFor();
    await page.waitForFunction(
      () =>
        getComputedStyle(document.querySelector(".tab-pane.active")).opacity ===
        "1"
    );
    await page
      .locator(".toast-container.hidden")
      .waitFor({ state: "attached" });
    if (shots) {
      await page
        .getByRole("heading", { name: "StashBooru 1.0.0", exact: true })
        .evaluate((element) => element.scrollIntoView({ block: "start" }));
      await page.screenshot({
        path: `${shots}/about-${name}.png`,
        fullPage: name === "mobile",
      });
    }
    for (const [version, expected] of [
      ["1.10.0", "StashBooru 1.10.0 is available."],
      ["0.9.0", "This build is newer than the latest stable release (0.9.0)."],
      ["v0.30.0", "Could not compare release versions."],
    ]) {
      await page.evaluate((value) => {
        window.settingsFixture.release.version = value;
      }, version);
      await page.getByRole("button", { name: "Check for updates" }).click();
      await page.getByText(expected, { exact: true }).waitFor();
    }
    await page.evaluate(() => {
      window.settingsFixture.release.error = "GitHub unavailable";
    });
    await page.getByRole("button", { name: "Check for updates" }).click();
    await page
      .getByText("Could not check for updates: GitHub unavailable")
      .waitFor();
    assert.equal(
      await page
        .getByRole("link", { name: "Download release", exact: true })
        .count(),
      0
    );
    await page.evaluate(() => {
      window.settingsFixture.release.error = "";
      window.settingsFixture.release.version = "1.0.0";
    });
    await page.getByRole("button", { name: "Check for updates" }).click();
    await page
      .getByText("You are running the latest stable StashBooru release.")
      .waitFor();
    await uiLink(page, "processing").click();
    // Late status responses after leaving Settings must not update an unmounted view.
    holdNextStatus = true;
    const started = new Promise((resolve) => {
      statusStarted = resolve;
    });
    await page
      .getByRole("button", { name: "Refresh status", exact: true })
      .click();
    await started;
    await page.evaluate(() => window.settingsFixture.dispose());
    holdStatus();
    await page.waitForTimeout(100);
    await page.close();
    assert.deepEqual(errors, [], errors.join("\n"));
    console.log(
      `${name}: settings navigation, save scopes, draft retention, errors and layout passed`
    );
  }
} catch (error) {
  if (currentPage && !currentPage.isClosed()) {
    console.error(
      await currentPage.evaluate(() => ({
        url: location.href,
        panes: [...document.querySelectorAll(".tab-pane")].map((p) => ({
          id: p.id,
          active: p.className,
        })),
        anchor: document
          .querySelector("#media-upscaling")
          ?.getBoundingClientRect().y,
        scrollY,
      }))
    );
    if (shots)
      await currentPage.screenshot({ path: `${shots}/settings-failure.png` });
  }
  throw error;
} finally {
  await browser.close();
  await server.close();
}
