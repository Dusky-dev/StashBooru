import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { createServer } from "vite";

// Real native components/styles with browser-local API and image fixtures.
// This suite never connects to a user's library or performs real file writes.
const engines = await import(process.env.PLAYWRIGHT_MODULE ?? "playwright");
const root = path.resolve(import.meta.dirname, "../..");
const server = await createServer({
  root,
  server: { host: "127.0.0.1", port: 0 },
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
const formats = [
  {
    id: "webp",
    label: "WebP",
    extension: "webp",
    family: "image",
    cpu: ["libwebp"],
    gpu: [],
    controls: ["quality", "effort", "lossless"],
    available: true,
  },
  {
    id: "av1-mp4",
    label: "AV1 MP4",
    extension: "mp4",
    family: "video",
    cpu: ["libaom-av1"],
    gpu: [],
    controls: ["quality", "effort"],
    available: true,
  },
];
const config = {
  cacheLimitBytes: 20 * 1024 ** 3,
  trialCacheLimitBytes: 10 * 1024 ** 3,
  trialTTLHours: 24,
  formatDefaults: { png: "webp", mp4: "av1-mp4" },
  encodingDefaults: {},
  backend: "local",
  savings: { minimumSavedBytes: 0, minimumSavedPercent: 0 },
};
const emptyStats = {
  converted: 0,
  savedBytes: 0,
  averageSavedBytes: 0,
  savedPercent: 0,
  cacheBytes: 0,
  netSavedBytes: 0,
  largerFiles: 0,
  skipped: 0,
  failed: 0,
};
function makeTrial(id, kind, status, before, after, policy) {
  return {
    id: id.repeat(32),
    target: { kind, id: 1 },
    batch: "fixture",
    createdAt: new Date().toISOString(),
    expiresAt: new Date(Date.now() + 86400000).toISOString(),
    status,
    before: {
      [kind === "image" ? "image" : "video"]: {
        path: `/fixture/${kind}.png`,
        basename: `${kind}-original.png`,
        size: before,
        width: 128,
        height: 128,
      },
    },
    options: {
      quality: 90,
      effort: 7,
      hardware: "cpu",
      lossless: false,
      dropAudio: false,
      allowAlphaLoss: false,
    },
    format: formats[kind === "image" ? 0 : 1],
    savings: policy,
    result: {
      width: 128,
      height: 128,
      frames: kind === "image" ? 1 : 24,
      size: after,
      seconds: 1.2,
      encoder: kind === "image" ? "libwebp" : "libaom-av1",
    },
    backend: "local",
    verified: true,
    retained: true,
    error:
      status === "skipped" ? "output was not smaller; source kept" : undefined,
    notices: [
      "Decoding verification does not establish pixel equality or all embedded profile metadata.",
    ],
  };
}
try {
  for (const viewport of [
    { width: 1440, height: 1000 },
    { width: 390, height: 844 },
  ]) {
    const context = await browser.newContext({ viewport });
    const page = await context.newPage();
    const errors = [];
    page.on("pageerror", (error) => errors.push(error.message));
    const requests = [];
    let trials = [],
      job,
      nextJob = 1;
    const policy = { minimumSavedBytes: 100, minimumSavedPercent: 10 };
    await page.route("**/image/converter**", async (route) => {
      const request = route.request(),
        url = new URL(request.url());
      if (url.searchParams.has("trialID")) {
        const png = await page.evaluate((changed) => {
          const canvas = document.createElement("canvas");
          canvas.width = canvas.height = 128;
          const ctx = canvas.getContext("2d");
          ctx.fillStyle = "#3388aa";
          ctx.fillRect(0, 0, 128, 128);
          ctx.fillStyle = changed ? "#ee9933" : "#eeeeee";
          ctx.fillRect(32, 32, 32, 32);
          return canvas.toDataURL().split(",")[1];
        }, url.searchParams.get("file") === "output");
        await route.fulfill({
          status: 200,
          contentType: "image/png",
          body: Buffer.from(png, "base64"),
        });
        return;
      }
      let payload;
      if (request.method() === "POST") {
        const body = request.postDataJSON();
        requests.push(body);
        if (body.action === "preview")
          payload = {
            plans: [
              {
                input: "png",
                output: "webp",
                count: 1,
                quality: 90,
                effort: 7,
                decodingSpeed: 0,
              },
              {
                input: "mp4",
                output: "av1-mp4",
                count: 1,
                quality: 80,
                effort: 7,
                decodingSpeed: 0,
              },
            ],
          };
        else if (body.action === "save-defaults") payload = config;
        else {
          job = {
            jobID: nextJob++,
            action: body.action,
            status: "complete",
            total: 2,
            items: [],
          };
          if (body.action === "estimate")
            job.estimate = {
              total: 200,
              sampled: 24,
              covered: false,
              estimatedSavedBytes: 0,
              observedLowBytes: 0,
              observedHighBytes: 0,
              strata: [
                {
                  name: "mp4 / animated=true / medium (1–16 MiB)",
                  count: 100,
                  sampled: 0,
                  failed: 1,
                },
              ],
            };
          if (body.action === "trial") {
            trials = [
              makeTrial("1", "image", "verified", 1000, 600, body.savings),
              makeTrial("2", "scene", "skipped", 1000, 1100, body.savings),
            ];
            job.items = trials.map((t) => ({
              target: t.target,
              trialID: t.id,
              status: t.status,
            }));
          }
          if (body.action === "apply-trials") {
            trials = trials.map((t) =>
              body.trialIDs.includes(t.id)
                ? { ...t, status: "applied", retained: false }
                : t
            );
            job.items = [
              { target: { kind: "image", id: 1 }, status: "complete" },
            ];
          }
          if (body.action === "discard-trials")
            trials = trials.filter((t) => !body.trialIDs.includes(t.id));
          payload = { jobID: job.jobID, batch: "fixture" };
        }
      } else if (url.searchParams.has("capabilities"))
        payload = { formats, backend: "local", notice: "" };
      else if (url.searchParams.has("config"))
        payload = {
          config,
          inputFormats: [
            { id: "png", label: "PNG", family: "image" },
            { id: "mp4", label: "MP4", family: "video" },
          ],
          outputFormats: formats,
        };
      else
        payload = {
          config,
          stats: emptyStats,
          batchStats: emptyStats,
          latestStats: emptyStats,
          history: [],
          historyTotal: 0,
          job,
          trials,
          trialTotal: trials.length,
          trialStats: {
            verified: trials.filter((t) => t.status === "verified").length,
            skipped: trials.filter((t) => t.status === "skipped").length,
            failed: 0,
            applied: trials.filter((t) => t.status === "applied").length,
            potentialSavedBytes: trials.some((t) => t.status === "verified")
              ? 400
              : 0,
            weightedSavedPercent: trials.some((t) => t.status === "verified")
              ? 40
              : 0,
            cacheBytes: trials
              .filter((t) => t.retained)
              .reduce((n, t) => n + t.result.size, 0),
          },
        };
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(payload),
      });
    });
    await page.goto(`${baseURL}/tests/browser/fixtures/conversion-review.html`);
    const dialog = page.locator(".modal.show").filter({
      has: page.locator(".modal-title").filter({ hasText: "Media converter" }),
    });
    try {
      await dialog
        .getByRole("button", {
          name: "Verified trial for 2 files",
          exact: true,
        })
        .waitFor({ timeout: 10000 });
    } catch (error) {
      console.error(errors, await page.locator("body").innerText());
      if (shots)
        await page.screenshot({
          path: path.join(shots, "fixture-failure.png"),
        });
      throw error;
    }
    await dialog
      .locator("#converter-job-saved-bytes")
      .fill(String(policy.minimumSavedBytes));
    await dialog
      .locator("#converter-job-saved-percent")
      .fill(String(policy.minimumSavedPercent));
    await dialog
      .getByRole("button", { name: "Estimate savings", exact: true })
      .click();
    await dialog
      .getByText("Some strata/targets failed or were not covered;", {
        exact: false,
      })
      .waitFor();
    assert.equal(
      trials.length,
      0,
      "estimate must not appear as retained trials"
    );
    await dialog.getByRole("tab", { name: "Convert", exact: true }).click();
    await dialog
      .getByRole("button", { name: "Verified trial for 2 files", exact: true })
      .click();
    const imageRow = dialog.locator('[data-trial-id="' + "1".repeat(32) + '"]');
    const videoRow = dialog.locator('[data-trial-id="' + "2".repeat(32) + '"]');
    await imageRow.waitFor();
    assert.match(await imageRow.innerText(), /1,000 → 600 bytes/);
    assert.match(
      await videoRow.innerText(),
      /output was not smaller; source kept/
    );
    assert.equal(await imageRow.locator('a[href="/images/1"]').count(), 1);
    assert.equal(await videoRow.locator('a[href="/scenes/1"]').count(), 1);
    assert.equal(await videoRow.getByRole("checkbox").isDisabled(), true);
    assert.deepEqual(
      requests.find((r) => r.action === "trial").savings,
      policy
    );
    assert.equal(
      requests.some((r) => r.action === "start"),
      false
    );
    assert.equal(
      await page.evaluate(
        () => document.documentElement.scrollWidth > innerWidth + 1
      ),
      false,
      "mobile page overflow"
    );
    await page.waitForFunction(() => {
      const buttons = [...document.querySelectorAll(".modal.show button")];
      return buttons.some(
        (b) =>
          b.textContent === "Select eligible trials on this page" && !b.disabled
      );
    });
    await page.waitForFunction(() =>
      [...document.querySelectorAll(".modal.show")]
        .filter((n) => getComputedStyle(n).display !== "none")
        .every((n) => getComputedStyle(n).opacity === "1")
    );
    if (shots)
      await page.screenshot({
        path: path.join(shots, `review-${viewport.width}.png`),
      });
    await imageRow
      .getByRole("button", { name: "Compare", exact: true })
      .click();
    const comparison = page.locator(".modal.show").filter({
      has: page
        .locator(".modal-title")
        .filter({ hasText: "Compare verified trial" }),
    });
    await comparison
      .locator(".Lightbox-reference-comparison-media")
      .first()
      .waitFor();
    assert.equal(
      await dialog.isVisible(),
      false,
      "parent converter cannot cover comparison"
    );
    await page.waitForFunction(() =>
      [...document.querySelectorAll(".modal.show")]
        .filter((n) => getComputedStyle(n).display !== "none")
        .every((n) => getComputedStyle(n).opacity === "1")
    );
    for (const mode of ["slider", "blink", "difference", "both"]) {
      await comparison.locator("#trial-comparison-mode").selectOption(mode);
      await comparison.locator(".Lightbox-reference-comparison").waitFor();
      assert.equal(
        await comparison
          .locator(".Lightbox-reference-comparison")
          .evaluate((n) => n.getBoundingClientRect().width > 0),
        true
      );
    }
    if (shots)
      await page.screenshot({
        path: path.join(shots, `comparison-${viewport.width}.png`),
      });
    await comparison
      .getByRole("button", { name: "Close", exact: true })
      .click();
    await comparison.waitFor({ state: "hidden" });
    await dialog
      .getByRole("button", {
        name: "Select eligible trials on this page",
        exact: true,
      })
      .click();
    await dialog
      .getByRole("button", { name: "Apply 1 selected trials", exact: true })
      .click();
    await imageRow.getByText("Verified · applied", { exact: false }).waitFor();
    assert.deepEqual(
      requests.find((r) => r.action === "apply-trials").trialIDs,
      ["1".repeat(32)]
    );
    await videoRow
      .getByRole("button", { name: "Discard", exact: true })
      .click();
    await videoRow.waitFor({ state: "hidden" });
    await dialog
      .getByRole("button", { name: "Close", exact: true })
      .last()
      .click();
    await dialog.waitFor({ state: "hidden" });
    await page.locator("#converter-global-saved-bytes").fill("200");
    await page.locator("#converter-global-saved-percent").fill("20");
    await page.locator("#converter-trial-cache").fill("1.5");
    await page.locator("#converter-trial-expiry").fill("12");
    await page
      .getByRole("button", { name: "Save format defaults", exact: true })
      .click();
    await page.waitForFunction(() =>
      [...document.querySelectorAll("button")].some(
        (b) => b.textContent === "Save format defaults" && !b.disabled
      )
    );
    const saved = requests.find((r) => r.action === "save-defaults");
    assert.deepEqual(saved.savings, {
      minimumSavedBytes: 200,
      minimumSavedPercent: 20,
    });
    assert.equal(saved.trialCacheLimitBytes, 1.5 * 1024 ** 3);
    assert.equal(saved.trialTTLHours, 12);
    await page
      .getByRole("button", { name: "Open converter", exact: true })
      .click();
    await dialog
      .getByRole("tab", {
        name: "Estimate / trial review / apply",
        exact: true,
      })
      .click();
    await dialog.locator('[data-trial-id="' + "1".repeat(32) + '"]').waitFor();
    assert.deepEqual(errors, [], "mounted component errors");
    await context.close();
    console.log(
      `Conversion review desktop/mobile workflow passed at ${viewport.width}px`
    );
  }
} finally {
  await browser.close();
  await server.close();
}
