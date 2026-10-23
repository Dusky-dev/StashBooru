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
const stats = {
  converted: 0,
  savedBytes: 0,
  averageSavedBytes: 0,
  savedPercent: 0,
  cacheBytes: 0,
  netSavedBytes: 0,
  largerFiles: 0,
};
try {
  for (const [layout, viewport] of [
    ["desktop", { width: 1440, height: 1000 }],
    ["mobile", { width: 390, height: 844 }],
  ]) {
    for (const count of [1, 2]) {
      const page = await browser.newPage({ viewport });
      const errors = [];
      page.on("pageerror", (error) => errors.push(error.message));
      const posts = [];
      let available = true;
      let failCheck = false;
      let holdCheck = false;
      let releaseCheck;
      let checkStarted;
      await page.route("**/image/**", async (route) => {
        const request = route.request();
        const url = new URL(request.url());
        if (request.method() === "POST") {
          posts.push({ path: url.pathname, body: request.postDataJSON() });
          return route.fulfill({ json: { jobID: 17, batch: "a".repeat(32) } });
        }
        if (url.searchParams.has("capabilities")) {
          if (holdCheck) {
            holdCheck = false;
            await new Promise((resolve) => {
              releaseCheck = resolve;
              checkStarted?.();
            });
          }
          if (failCheck)
            return route.fulfill({
              status: 503,
              body: "Worker check unavailable",
            });
          return route.fulfill({
            json: {
              backend: "local",
              formats: [
                {
                  id: "png",
                  label: "PNG",
                  family: "image",
                  available: true,
                  controls: [],
                },
              ],
              upscalers: [
                {
                  id: "waifu2x",
                  label: "waifu2x",
                  available,
                  cpu: true,
                  notice: "Fixture model",
                },
                { id: "seedvr2", label: "SeedVR2", available, cpu: false },
              ],
            },
          });
        }
        assert.ok(url.pathname.endsWith("/upscale-restore"));
        return route.fulfill({
          json: {
            config: { cacheLimitBytes: 20 * 1024 ** 3 },
            stats,
            batchStats: stats,
            latestStats: stats,
            history: [],
            historyTotal: 0,
          },
        });
      });
      await page.context().route("**/settings?tab=processing", (route) =>
        route.fulfill({
          contentType: "text/html",
          body: "<h1>Processing settings fixture</h1>",
        })
      );
      await page.goto(
        `${origin}/tests/browser/fixtures/processing-job-resume.html?mode=upscale${count === 2 ? "&many=1" : ""}`
      );
      const modal = () =>
        page.locator(".modal.show").filter({ hasText: "Upscale images" });
      const copyName =
        count === 1
          ? "Create upscaled image"
          : `Create ${count} upscaled images`;
      const replaceName =
        count === 1
          ? "Upscale and replace image…"
          : `Upscale and replace ${count} images…`;
      const details = () =>
        modal()
          .locator("details")
          .filter({ hasText: "Worker and processing options" });
      const create = () =>
        modal().getByRole("button", { name: copyName, exact: true });
      await modal().locator("#upscaler-model:enabled").waitFor();
      assert.equal(
        await modal().locator("#upscaler-create-copy").isChecked(),
        true
      );
      assert.equal(await details().getAttribute("open"), null);
      assert.equal(
        await modal().getByLabel("Processor", { exact: true }).isVisible(),
        false
      );
      await modal()
        .getByText(/New files are saved beside their source files/)
        .waitFor();
      assert.equal(
        await modal().evaluate(
          (element) =>
            !!(
              element
                .querySelector("#upscaler-create-copy")
                .compareDocumentPosition(
                  element.querySelector("#upscaler-model")
                ) & Node.DOCUMENT_POSITION_FOLLOWING
            )
        ),
        true,
        "save behavior must precede processing controls"
      );
      if (shots && count === 1)
        await page.screenshot({
          path: `${shots}/upscaling-output-${layout}.png`,
        });
      await modal().getByLabel("Scale", { exact: true }).selectOption("4");
      await modal()
        .getByLabel("Output format", { exact: true })
        .selectOption("png");
      await details().locator("summary").focus();
      await details().locator("summary").press("Enter");
      await modal()
        .getByLabel("Processor", { exact: true })
        .selectOption("cpu");
      await modal()
        .getByLabel("Upscaler", { exact: true })
        .selectOption("seedvr2");
      assert.equal(await create().isDisabled(), true);
      await details().locator("summary").click();
      await modal()
        .getByText(/This model requires a GPU/)
        .waitFor();
      assert.match(
        await details().locator("summary").textContent(),
        /CPU only.*setup needed/
      );
      await details().locator("summary").click();
      await modal()
        .getByLabel("Processor", { exact: true })
        .selectOption("gpu");
      assert.equal(await create().isEnabled(), true);
      await modal()
        .getByLabel("Upscaler", { exact: true })
        .selectOption("waifu2x");
      await modal()
        .getByLabel("Processor", { exact: true })
        .selectOption("cpu");
      const settings = modal().getByRole("link", {
        name: /Edit upscaler defaults.*new tab/,
      });
      assert.equal(await settings.getAttribute("target"), "_blank");
      const originalURL = page.url();
      const [popup] = await Promise.all([
        page.waitForEvent("popup"),
        settings.click(),
      ]);
      await popup
        .getByRole("heading", { name: "Processing settings fixture" })
        .waitFor();
      assert.match(popup.url(), /\/settings\?tab=processing#media-upscaling$/);
      await popup.close();
      assert.equal(page.url(), originalURL);
      assert.equal(
        await modal().getByLabel("Scale", { exact: true }).inputValue(),
        "4"
      );
      assert.equal(
        await modal().getByLabel("Output format", { exact: true }).inputValue(),
        "png"
      );
      assert.equal(
        await modal().getByLabel("Processor", { exact: true }).inputValue(),
        "cpu"
      );
      assert.equal(posts.length, 0);

      // Keep the advanced disclosure and draft mounted while rechecking readiness.
      holdCheck = true;
      failCheck = true;
      const started = new Promise((resolve) => {
        checkStarted = resolve;
      });
      await modal()
        .getByRole("button", { name: "Recheck worker", exact: true })
        .click();
      await started;
      await modal().getByRole("status").waitFor();
      assert.equal(await create().isDisabled(), true);
      assert.notEqual(await details().getAttribute("open"), null);
      assert.equal(
        await modal().getByLabel("Scale", { exact: true }).isDisabled(),
        true
      );
      releaseCheck();
      await modal()
        .getByText("Worker check unavailable", { exact: true })
        .waitFor();
      assert.equal(
        await create().isDisabled(),
        true,
        "a failed check must not reuse stale ready capabilities"
      );
      failCheck = false;
      await modal()
        .getByRole("button", { name: "Recheck worker", exact: true })
        .click();
      await modal()
        .locator("button:enabled")
        .filter({ hasText: copyName })
        .waitFor();
      assert.notEqual(await details().getAttribute("open"), null);
      assert.equal(
        await modal().getByLabel("Scale", { exact: true }).inputValue(),
        "4"
      );
      assert.equal(
        await modal().getByLabel("Output format", { exact: true }).inputValue(),
        "png"
      );
      assert.equal(
        await modal().getByLabel("Processor", { exact: true }).inputValue(),
        "cpu"
      );
      await details().locator("summary").click();
      await modal().locator("#upscaler-replace-original").focus();
      await modal().locator("#upscaler-replace-original").press("Space");
      await modal()
        .getByText(/may later be evicted/)
        .waitFor();
      await modal()
        .getByRole("button", { name: replaceName, exact: true })
        .click();
      await page
        .getByText(
          count === 1 ? "Replace current image?" : "Replace selected images?",
          { exact: true }
        )
        .waitFor();
      assert.equal(
        posts.length,
        0,
        "replacement requires its separate confirmation"
      );
      await page.getByRole("button", { name: "Cancel", exact: true }).click();
      await modal().locator("#upscaler-model:enabled").waitFor();
      assert.equal(posts.length, 0);
      assert.equal(
        await modal().getByLabel("Scale", { exact: true }).inputValue(),
        "4"
      );
      await modal().locator("#upscaler-create-copy").check();
      assert.equal(
        await modal().locator("#upscaler-create-copy").isChecked(),
        true
      );
      await create().click();
      await modal()
        .getByText(/Non-destructive upscaling job #17 submitted/)
        .waitFor();
      assert.equal(posts.length, 1);
      assert.equal(posts[0].path, "/image/upscale");
      assert.equal(posts[0].body.action, undefined);
      assert.equal(posts[0].body.useEncodingDefaults, true);
      assert.deepEqual(
        posts[0].body.targets,
        Array.from({ length: count }, (_, i) => ({ kind: "image", id: i + 1 }))
      );
      assert.equal(posts[0].body.options.upscaleScale, 4);
      assert.equal(posts[0].body.options.format, "png");
      assert.equal(posts[0].body.options.hardware, "cpu");
      assert.equal(await create().isDisabled(), true);
      await modal()
        .getByRole("button", { name: "Close", exact: true })
        .last()
        .click();
      await page
        .getByRole("button", { name: "Reopen processing", exact: true })
        .click();
      await modal().locator("#upscaler-model:enabled").waitFor();
      assert.equal(
        await modal().locator("#upscaler-create-copy").isChecked(),
        true,
        "a new dialog must keep the non-destructive default"
      );
      await modal().locator("#upscaler-replace-original").focus();
      await modal().locator("#upscaler-replace-original").press("Space");
      await modal()
        .getByRole("button", { name: replaceName, exact: true })
        .click();
      assert.equal(posts.length, 1);
      await page
        .getByRole("button", { name: "Replace and upscale", exact: true })
        .click();
      await modal()
        .getByText(/Replacement upscaling job #17 submitted/)
        .waitFor();
      assert.equal(posts.length, 2);
      assert.equal(posts[1].path, "/image/upscale-restore");
      assert.equal(posts[1].body.action, "replace");
      assert.equal(posts[1].body.useEncodingDefaults, true);
      assert.deepEqual(posts[1].body.targets, posts[0].body.targets);
      assert.equal(
        await modal()
          .getByRole("button", { name: replaceName, exact: true })
          .isDisabled(),
        true
      );
      await modal()
        .getByRole("button", { name: "Close", exact: true })
        .last()
        .click();
      available = false;
      await page
        .getByRole("button", { name: "Reopen processing", exact: true })
        .click();
      await modal()
        .locator("summary")
        .getByText(/setup needed/)
        .waitFor();
      assert.equal(await create().isDisabled(), true);
      await details().locator("summary").click();
      await modal()
        .getByText(/Configure the local executable/)
        .waitFor();
      assert.equal(posts.length, 2);
      assert.equal(
        await page
          .locator(".modal.show .modal-content")
          .evaluate((element) => element.scrollWidth > element.clientWidth + 1),
        false
      );
      assert.deepEqual(errors, []);
      await page.close();
      console.log(
        `${layout}, ${count} Image(s): output-first controls, settings retention, readiness recovery and copy/confirmed-replacement requests passed`
      );
    }
  }
} finally {
  await browser.close();
  await server.close();
}
