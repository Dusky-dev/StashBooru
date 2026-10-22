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
const stats = {
  converted: 0,
  savedBytes: 0,
  averageSavedBytes: 0,
  savedPercent: 0,
  cacheBytes: 0,
  netSavedBytes: 0,
  largerFiles: 0,
};
const config = {
  cacheLimitBytes: 1024 ** 3,
  formatDefaults: {},
  encodingDefaults: {},
  backend: "local",
  savings: { minimumSavedBytes: 0, minimumSavedPercent: 0 },
};
const record = {
  id: "f".repeat(32),
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
      size: 500,
      fingerprints: [],
    },
  },
  result: { encoder: "webp", seconds: 1 },
};
try {
  for (const viewport of [
    { width: 1440, height: 1000 },
    { width: 390, height: 844 },
  ])
    for (const mode of ["conversion", "upscale"]) {
      const page = await browser.newPage({ viewport });
      const errors = [],
        reads = [],
        posts = [];
      const jobs = new Map();
      let nextBatch = 1;
      const feature =
        mode === "conversion" ? "image/converter" : "image/upscale-restore";
      const storageKey = (prefix) =>
        `stashbooru.processingJob:${origin}${prefix}${feature}`;
      const goto = (prefix) =>
        page.goto(
          `${origin}/tests/browser/fixtures/processing-job-resume.html?mode=${mode}&prefix=${encodeURIComponent(prefix)}`
        );
      const footer = () => page.getByRole("dialog").locator(".modal-footer");
      const statistics = () =>
        page
          .getByRole("tab", { name: "Statistics & restore", exact: true })
          .click();
      const start = () =>
        page
          .getByRole("button", {
            name: "Save limit and trim cache",
            exact: true,
          })
          .click();
      page.on("pageerror", (error) => errors.push(error.message));
      await page.addInitScript(() => {
        localStorage.setItem("media-converter-job", "7");
        localStorage.setItem("media-upscaling-restore-job", "7");
      });
      await page.route("**/image/**", async (route) => {
        const url = new URL(route.request().url());
        if (url.searchParams.has("capabilities"))
          return route.fulfill({
            json: { formats: [], upscalers: [], backend: "local" },
          });
        if (route.request().method() === "POST") {
          const body = route.request().postDataJSON();
          posts.push({ path: url.pathname, ...body });
          if (body.action === "preview")
            return route.fulfill({ json: { plans: [] } });
          if (body.action === "cancel") {
            const current = jobs.get(url.pathname);
            assert.equal(body.jobID, current.jobID);
            assert.equal(body.jobBatch, current.batch);
            current.status = "cancelled";
            return route.fulfill({ json: { cancelled: true } });
          }
          assert.equal(body.action, "configure");
          const current = {
            jobID: 7,
            batch: (nextBatch++).toString(16).padStart(32, "0"),
            status: "running",
            action: "configure",
            total: 1,
            items: [],
          };
          jobs.set(url.pathname, current);
          return route.fulfill({
            json: { jobID: current.jobID, batch: current.batch },
          });
        }
        const id = Number(url.searchParams.get("jobID")),
          batch = url.searchParams.get("batch");
        reads.push({ path: url.pathname, id, batch });
        const current = jobs.get(url.pathname),
          matches = id > 0 && id === current?.jobID && batch === current?.batch;
        return route.fulfill({
          json: {
            config,
            stats,
            batchStats: stats,
            latestStats: stats,
            history: [record],
            historyTotal: 1,
            job: matches ? current : undefined,
            jobUnavailable: id > 0 && !matches,
            trials: [],
            trialTotal: 0,
            trialStats: {
              verified: 0,
              skipped: 0,
              failed: 0,
              applied: 0,
              potentialSavedBytes: 0,
              weightedSavedPercent: 0,
              cacheBytes: 0,
            },
          },
        });
      });
      await goto("/alpha/");
      await statistics();
      await page
        .getByRole("link", { name: "original.png", exact: true })
        .waitFor();
      assert.ok(
        reads.filter((r) => r.path.endsWith(feature)).every((r) => r.id === 0),
        "Legacy numeric storage must not resume arbitrary jobs"
      );
      await start();
      await footer().getByText("Running", { exact: true }).waitFor();
      const saved = await page.evaluate(
        (key) => JSON.parse(localStorage.getItem(key)),
        storageKey("/alpha/")
      );
      assert.equal(saved.jobID, 7);
      assert.match(saved.batch, /^[a-f0-9]{32}$/);
      await page.reload();
      await footer().getByText("Running", { exact: true }).waitFor();
      assert.ok(reads.some((r) => r.id === 7 && r.batch === saved.batch));
      // A different server prefix must not pick up the first server's job pointer.
      const before = reads.length;
      await goto("/beta/");
      await statistics();
      await page
        .getByRole("link", { name: "original.png", exact: true })
        .waitFor();
      assert.ok(
        reads
          .slice(before)
          .filter((r) => r.path.endsWith(feature))
          .every((r) => r.id === 0)
      );
      assert.equal(
        await footer().getByText("Running", { exact: true }).count(),
        0
      );
      await goto("/alpha/");
      await footer().getByText("Running", { exact: true }).waitFor();
      // Restart/expiry: the numeric ID is reused for a different batch.
      jobs.set(`/alpha/${feature}`, {
        jobID: 7,
        batch: "e".repeat(32),
        status: "running",
        total: 1,
        items: [],
        error: "Unrelated work",
      });
      await page.reload();
      await statistics();
      await page
        .getByText(
          "Live job details expired or the server restarted. Saved results and available originals remain in their history tabs.",
          { exact: true }
        )
        .waitFor();
      await page
        .getByRole("link", { name: "original.png", exact: true })
        .waitFor();
      assert.equal(
        await page.evaluate(
          (key) => localStorage.getItem(key),
          storageKey("/alpha/")
        ),
        null
      );
      assert.equal(
        await page.getByText("Unrelated work", { exact: true }).count(),
        0
      );
      assert.equal(
        await page
          .getByRole("button", { name: "Cancel remaining work", exact: true })
          .count(),
        0
      );
      assert.equal(posts.filter((p) => p.action === "cancel").length, 0);
      if (process.env.STASH_BROWSER_SCREENSHOTS) {
        fs.mkdirSync(process.env.STASH_BROWSER_SCREENSHOTS, {
          recursive: true,
        });
        await page.screenshot({
          path: path.join(
            process.env.STASH_BROWSER_SCREENSHOTS,
            `resume-${mode}-${viewport.width}.png`
          ),
          animations: "disabled",
        });
      }
      await start();
      await footer().getByText("Running", { exact: true }).waitFor();
      if (mode === "conversion") {
        await page
          .getByRole("button", { name: "Cancel remaining work", exact: true })
          .click();
        await footer().getByText("Cancelled", { exact: true }).waitFor();
        assert.equal(posts.filter((p) => p.action === "cancel").length, 1);
      }
      // Storage denial must not turn a successful submission into a retry/error.
      await page.evaluate(() => {
        localStorage.clear();
        Storage.prototype.setItem = () => {
          throw new DOMException("blocked", "SecurityError");
        };
      });
      await page
        .getByRole("button", { name: "Close", exact: true })
        .last()
        .click();
      await page
        .getByRole("button", { name: "Reopen processing", exact: true })
        .click();
      await statistics();
      const submitted = posts.filter((p) => p.action === "configure").length;
      await start();
      await page
        .getByText(
          "Browser storage is unavailable. Work can continue, but this live job cannot be resumed after refresh.",
          { exact: true }
        )
        .waitFor();
      await footer().getByText("Running", { exact: true }).waitFor();
      assert.equal(
        posts.filter((p) => p.action === "configure").length,
        submitted + 1
      );
      assert.deepEqual(errors, []);
      assert.ok(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= innerWidth + 1
        )
      );
      await page.close();
      console.log(
        `${mode} ${viewport.width}px: legacy pointers, refresh resume, prefix isolation, reused IDs, retained history, cancellation identity and blocked storage passed`
      );
    }
} finally {
  await browser.close();
  await server.close();
}
