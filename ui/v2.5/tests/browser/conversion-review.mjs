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
function makeTrial(id, kind, status, before, after, policy, options) {
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
      ...options,
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
    await page.addInitScript(() => {
      window.copiedConversionError = "";
      navigator.clipboard.writeText = async (text) => {
        window.copiedConversionError = text;
      };
    });
    const errors = [];
    page.on("pageerror", (error) => errors.push(error.message));
    const requests = [];
    let trials = [],
      job,
      nextJob = 1;
    let failMaintenance = false,
      pendingFailure;
    let failCapabilities = false;
    const restoreRecord = {
      id: "a".repeat(32),
      status: "complete",
      cached: true,
      createdAt: "2026-10-22T12:00:00Z",
      backend: "local",
      before: {
        image: {
          id: 1,
          basename: "original.png",
          path: "/fixture/original.png",
          size: 1000,
          fingerprints: [],
        },
      },
      after: {
        image: {
          id: 1,
          basename: "converted.webp",
          path: "/fixture/converted.webp",
          size: 600,
          fingerprints: [],
        },
      },
      result: { encoder: "webp", seconds: 1 },
    };
    const policy = { minimumSavedBytes: 100, minimumSavedPercent: 10 };
    await page.route("**/image/converter**", async (route) => {
      const request = route.request(),
        url = new URL(request.url());
      if (url.searchParams.has("capabilities") && failCapabilities) {
        await route.fulfill({
          status: 503,
          contentType: "text/plain",
          body: "Fixture worker unavailable",
        });
        return;
      }
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
            batch: nextJob.toString(16).padStart(32, "0"),
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
              makeTrial(
                "1",
                "image",
                "verified",
                1000,
                600,
                body.savings,
                body.options
              ),
              makeTrial(
                "2",
                "scene",
                "skipped",
                1000,
                1100,
                body.savings,
                body.options
              ),
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
          if (
            failMaintenance &&
            ["configure", "restore", "recover"].includes(body.action)
          ) {
            job.status = "running";
            pendingFailure = {
              ...job,
              status: "failed",
              error: `${body.action} failed asynchronously\nComplete worker detail`,
            };
          }
          payload = { jobID: job.jobID, batch: job.batch };
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
      else {
        if (pendingFailure) {
          job = pendingFailure;
          pendingFailure = undefined;
        }
        payload = {
          config,
          stats: emptyStats,
          batchStats: emptyStats,
          latestStats: emptyStats,
          history: failMaintenance ? [restoreRecord] : [],
          historyTotal: failMaintenance ? 1 : 0,
          job:
            Number(url.searchParams.get("jobID")) === job?.jobID &&
            url.searchParams.get("batch") === job?.batch
              ? job
              : undefined,
          jobUnavailable:
            Number(url.searchParams.get("jobID")) > 0 &&
            (Number(url.searchParams.get("jobID")) !== job?.jobID ||
              url.searchParams.get("batch") !== job?.batch),
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
      }
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
    const advanced = dialog.locator("details").filter({
      has: page.locator("#converter-hardware"),
    });
    const summary = advanced.locator("summary");
    const trialButton = dialog.getByRole("button", {
      name: "Verified trial for 2 files",
      exact: true,
    });
    await page.waitForFunction(() =>
      [...document.querySelectorAll(".modal.show button")].some(
        (b) => b.textContent === "Verified trial for 2 files" && !b.disabled
      )
    );
    await dialog.getByText("1 file: PNG → WebP", { exact: false }).waitFor();
    await dialog.getByText("1 file: MP4 → AV1 MP4", { exact: false }).waitFor();
    assert.equal(await advanced.getAttribute("open"), null);
    assert.equal(
      await dialog.locator("#converter-hardware").isVisible(),
      false
    );
    assert.equal(
      await dialog.locator("#converter-saved-encoding").isChecked(),
      true
    );
    assert.equal(
      await dialog
        .locator("#converter-job-saved-percent")
        .evaluate((field) =>
          Boolean(
            field.compareDocumentPosition(
              [...document.querySelectorAll("button")].find(
                (button) => button.textContent === "Verified trial for 2 files"
              )
            ) & Node.DOCUMENT_POSITION_FOLLOWING
          )
        ),
      true,
      "Savings limits precede the actions they govern"
    );
    await dialog
      .getByText("Cache eviction can remove them;", { exact: false })
      .waitFor();
    if (shots) {
      await dialog.locator(".tab-pane.active").evaluate(async () => {
        await Promise.all(
          document
            .getAnimations()
            .filter(
              (animation) =>
                animation.effect?.getTiming().iterations !== Infinity
            )
            .map((animation) => animation.finished.catch(() => {}))
        );
      });
      await page.screenshot({
        path: path.join(shots, `converter-options-${viewport.width}.png`),
      });
    }
    await summary.focus();
    await page.keyboard.press("Enter");
    await dialog.locator("#converter-hardware").selectOption("cpu");
    await dialog.locator("#converter-saved-encoding").uncheck();
    await dialog.locator("#converter-quality").fill("87");
    await dialog.locator("#converter-effort").fill("6");
    // Failed worker checks remain actionable without exposing the advanced form.
    failCapabilities = true;
    await dialog
      .getByRole("button", { name: "Recheck worker", exact: true })
      .click();
    await dialog
      .getByText("Fixture worker unavailable", { exact: true })
      .waitFor();
    await summary.click();
    assert.equal(
      await dialog.locator("#converter-hardware").isVisible(),
      false
    );
    assert.equal(await trialButton.isDisabled(), true);
    await dialog
      .getByText("Fixture worker unavailable", { exact: true })
      .waitFor();
    assert.match(await summary.innerText(), /CPU only.*custom overrides/);
    failCapabilities = false;
    await summary.click();
    await dialog
      .getByRole("button", { name: "Recheck worker", exact: true })
      .click();
    await page.waitForFunction(() =>
      [...document.querySelectorAll(".modal.show button")].some(
        (b) => b.textContent === "Verified trial for 2 files" && !b.disabled
      )
    );
    assert.equal(await dialog.locator("#converter-quality").inputValue(), "87");
    assert.equal(await dialog.locator("#converter-effort").inputValue(), "6");
    assert.equal(
      await dialog.locator("#converter-hardware").inputValue(),
      "cpu"
    );
    // A separate settings tab must preserve the dialog, its draft and caller URL.
    const callerURL = page.url();
    const settingsLink = dialog.getByRole("link", {
      name: "Edit format defaults in Processing settings",
      exact: true,
    });
    assert.equal(
      await settingsLink.getAttribute("href"),
      "/settings?tab=processing#media-converter"
    );
    assert.equal(await settingsLink.getAttribute("target"), "_blank");
    await context.route("**/settings?tab=processing", (route) =>
      route.fulfill({
        contentType: "text/html",
        body: "<title>Settings destination fixture</title>",
      })
    );
    const [settingsPage] = await Promise.all([
      context.waitForEvent("page"),
      settingsLink.click(),
    ]);
    await settingsPage.waitForURL("**/settings?tab=processing#media-converter");
    await settingsPage.close();
    assert.equal(page.url(), callerURL);
    assert.equal(await dialog.isVisible(), true);
    assert.equal(await dialog.locator("#converter-quality").inputValue(), "87");
    assert.equal(
      requests.some((r) =>
        ["trial", "estimate", "apply-trials", "start"].includes(r.action)
      ),
      false
    );
    await summary.click();
    await dialog.locator("#converter-job-saved-percent").fill("101");
    assert.equal(await trialButton.isDisabled(), true);
    assert.equal(
      await dialog
        .getByRole("button", { name: "Estimate savings", exact: true })
        .isDisabled(),
      true
    );
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
    assert.equal(
      await dialog
        .getByRole("button", { name: "Apply 0 selected trials", exact: true })
        .isDisabled(),
      true
    );
    assert.equal(
      requests.some((r) => r.action === "apply-trials"),
      false
    );
    await dialog.getByRole("tab", { name: "Convert", exact: true }).click();
    assert.equal(await advanced.getAttribute("open"), null);
    await summary.click();
    assert.equal(await dialog.locator("#converter-quality").inputValue(), "87");
    await summary.click();
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
    const trialRequest = requests.find((r) => r.action === "trial");
    assert.equal(trialRequest.options.hardware, "cpu");
    assert.equal(trialRequest.options.quality, 87);
    assert.equal(trialRequest.options.effort, 6);
    assert.equal(trialRequest.useEncodingDefaults, false);
    assert.equal(trialRequest.useDecodingSpeedDefaults, true);
    assert.deepEqual(trialRequest.targets, [
      { kind: "image", id: 1 },
      { kind: "scene", id: 1 },
    ]);
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
    await dialog.locator(".tab-pane.active").evaluate(async () => {
      await Promise.all(
        document
          .getAnimations()
          .filter(
            (animation) => animation.effect?.getTiming().iterations !== Infinity
          )
          .map((animation) => animation.finished.catch(() => {}))
      );
    });
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
      .locator(".modal-footer")
      .getByText("Completed", { exact: true })
      .waitFor();
    await dialog
      .getByRole("button", { name: "Close", exact: true })
      .last()
      .click();
    await dialog.waitFor({ state: "hidden" });
    await page.locator("#converter-global-saved-bytes").fill("200");
    await page.locator("#converter-global-saved-percent").fill("20");
    await page
      .getByText("Saved trial storage and expiry", { exact: true })
      .click();
    await page.locator("#converter-trial-cache").fill("1.5");
    await page.locator("#converter-trial-expiry").fill("12");
    await page
      .getByRole("button", { name: "Save conversion defaults", exact: true })
      .click();
    await page.waitForFunction(() =>
      [...document.querySelectorAll("button")].some(
        (b) => b.textContent === "Save conversion defaults" && !b.disabled
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
    // Maintenance jobs return an accepted ID first and fail only on a later poll.
    // Keep the launching tab selected: the error must remain visible and copyable.
    failMaintenance = true;
    const statistics = dialog.getByRole("tab", {
      name: "Statistics & restore",
      exact: true,
    });
    await statistics.click();
    for (const [action, button] of [
      ["configure", "Save limit and trim cache"],
      ["restore", "Restore"],
      ["recover", "Recover interrupted operations"],
    ]) {
      await dialog.getByRole("button", { name: button, exact: true }).click();
      const feedback = dialog.getByRole("region", {
        name: "Conversion job feedback",
        exact: true,
      });
      const failure = `${action} failed asynchronously\nComplete worker detail`;
      await feedback.getByText(failure, { exact: true }).waitFor();
      assert.equal(await statistics.getAttribute("aria-selected"), "true");
      await feedback
        .getByRole("button", { name: "Copy error", exact: true })
        .click();
      assert.equal(
        await page.evaluate(() => window.copiedConversionError),
        failure
      );
      assert.equal(
        await feedback.count(),
        1,
        "One feedback panel across all tabs"
      );
      if (action === "restore")
        assert.equal(
          requests.find((r) => r.action === action).recordID,
          restoreRecord.id
        );
    }
    for (const name of [
      "Convert",
      "Estimate / trial review / apply",
      "Statistics & restore",
    ]) {
      await dialog.getByRole("tab", { name, exact: true }).click();
      await dialog
        .getByRole("region", { name: "Conversion job feedback", exact: true })
        .getByText("recover failed asynchronously\nComplete worker detail", {
          exact: true,
        })
        .waitFor();
    }
    const toastClose = page.locator(".toast-container .close");
    if (await toastClose.isVisible()) await toastClose.click();
    await page.evaluate(async () => {
      await Promise.all(
        document
          .getAnimations()
          .filter(
            (animation) => animation.effect?.getTiming().iterations !== Infinity
          )
          .map((animation) => animation.finished.catch(() => {}))
      );
    });
    assert.equal(
      await dialog
        .locator(".tab-pane.active")
        .evaluate((element) => getComputedStyle(element).opacity),
      "1"
    );
    if (shots)
      await page.screenshot({
        path: path.join(shots, `statistics-error-${viewport.width}.png`),
      });
    assert.deepEqual(errors, [], "mounted component errors");
    await context.close();
    console.log(
      `Conversion workflow and asynchronous configure/restore/recover errors passed at ${viewport.width}px`
    );
  }
} finally {
  await browser.close();
  await server.close();
}
