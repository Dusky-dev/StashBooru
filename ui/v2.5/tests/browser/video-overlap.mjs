import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { createServer } from "vite";

// Mounted native React/Bootstrap components, mocked API, actual synthetic H.264
// streams. Native SQLite/job behavior is covered by the backend integration test.
const fixtures = process.env.STASH_VIDEO_OVERLAP_FIXTURES;
assert.ok(
  fixtures,
  "Generate fixtures with scripts/tests/video_overlap_fixtures.py --output PATH"
);
const streams = ["base.mp4", "reordered.mp4"].map((name) =>
  fs.readFileSync(path.join(fixtures, name))
);
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
const media = {
  duration: 24,
  width: 128,
  height: 96,
  codec: "h264",
  bitRate: 120000,
  rotation: 0,
  audioTracks: 1,
  subtitleTracks: 0,
  audioCodecs: ["aac"],
};
const interval = (a, b, offset) => ({
  a: { start: a, end: a + 8 },
  b: { start: b, end: b + 8 },
  offset,
  samples: 8,
  distinctFrames: 8,
  meanDistance: 1,
  meanDHashDistance: 2,
  support: 1,
  gapsA: [],
  gapsB: [],
});
function row(id) {
  return {
    a: {
      id: 1,
      title: "Synthetic original",
      basename: "base.mp4",
      metadataFields: 6,
      media,
    },
    b: {
      id,
      title: `Reordered compilation #${id}`,
      basename: "reordered.mp4",
      metadataFields: 4,
      media,
    },
    match: {
      a: 1,
      b: id,
      class: "compilation-segments",
      intervals: [interval(0, 12, 12), interval(14, 2, -12)],
      coverageA: 2 / 3,
      coverageB: 2 / 3,
      unmatchedA: [
        { start: 8, end: 14 },
        { start: 22, end: 24 },
      ],
      unmatchedB: [
        { start: 0, end: 2 },
        { start: 10, end: 12 },
        { start: 20, end: 24 },
      ],
      tolerance: 1.25,
      audio:
        "Whole-track audio differs; matched audio intervals have not been verified.",
      evidence:
        "Two monotonic mappings with distinct offsets. Actual frame samples support visual overlap.",
      confidence: "Strong sampled evidence",
      limited: false,
    },
  };
}
try {
  for (const viewport of [
    { width: 1440, height: 1000 },
    { width: 390, height: 844 },
  ]) {
    const context = await browser.newContext({ viewport });
    const page = await context.newPage(),
      errors = [],
      requests = [];
    page.on("pageerror", (error) => errors.push(error.message));
    let config = { backend: "local", sampleSeconds: 1, audioDigest: false },
      job;
    let stale = false,
      empty = false,
      limited = false,
      reject = false;
    const interrupted = {
      id: "checkpoint",
      jobID: 9,
      action: "index",
      status: "interrupted",
      backend: "local",
      total: 11,
      processed: 3,
      indexed: 2,
      skipped: 0,
      failed: 1,
      items: [{ id: 4, status: "failed", error: "Invalid fixture video" }],
    };
    await page.route("**/scene/overlap**", async (route) => {
      const req = route.request(),
        url = new URL(req.url());
      if (req.method() === "POST") {
        const body = req.postDataJSON();
        requests.push(body);
        if (reject) {
          await route.fulfill({
            status: 409,
            body: "Configure the remote worker first",
          });
          return;
        }
        if (body.action === "configure") config = body.config;
        if (body.action === "index" || body.action === "resume")
          job = { ...interrupted, id: "index", jobID: 10, status: "running" };
        if (body.action === "cancel") job = { ...job, status: "cancelled" };
        if (body.action === "search")
          job = {
            ...interrupted,
            id: "search",
            action: "search",
            status: "complete",
            failed: 0,
            items: [],
          };
        await route.fulfill({ json: { id: job?.id, jobID: job?.jobID } });
        return;
      }
      const pageNumber = Number(url.searchParams.get("page") || 1);
      const allRows = Array.from({ length: 11 }, (_, i) => row(i + 2));
      if (stale)
        allRows[0].stale =
          "Primary file changed since this review; index/search again.";
      await route.fulfill({
        json: {
          config,
          indexed: 12,
          totalVideos: 12,
          algorithm: "fixture",
          job,
          jobs: [interrupted, ...(job ? [job] : [])],
          review: {
            id: "search",
            reference: 1,
            status: "complete",
            total: empty ? 0 : allRows.length,
            rows: empty
              ? []
              : allRows.slice((pageNumber - 1) * 10, pageNumber * 10),
            skipped: 1,
            errors: ["Video 99: index is stale"],
            alignmentLimited: limited ? 1 : 0,
            candidates: {
              ids: Array.from({ length: 11 }, (_, i) => i + 2),
              total: limited ? 20 : 11,
              limited,
              omittedCommonBands: limited ? 4 : 0,
            },
          },
        },
      });
    });
    await page.route("**/scene/*/stream.mp4", async (route) => {
      const id = Number(
        route
          .request()
          .url()
          .match(/scene\/(\d+)\//)[1]
      );
      const data = streams[id === 1 ? 0 : 1],
        range = route
          .request()
          .headers()
          .range?.match(/bytes=(\d+)-(\d*)/);
      if (range) {
        const start = Number(range[1]),
          end = Math.min(Number(range[2] || data.length - 1), data.length - 1);
        await route.fulfill({
          status: 206,
          headers: {
            "Content-Type": "video/mp4",
            "Accept-Ranges": "bytes",
            "Content-Range": `bytes ${start}-${end}/${data.length}`,
          },
          body: data.subarray(start, end + 1),
        });
      } else await route.fulfill({ contentType: "video/mp4", body: data });
    });
    await page.goto(`${baseURL}/tests/browser/fixtures/video-overlap.html`);
    await page.getByText("12 stored signatures", { exact: false }).waitFor();
    await page.locator("#video-overlap-backend").selectOption("remote");
    await page
      .getByRole("button", { name: "Index reference", exact: true })
      .click();
    await page.getByRole("button", { name: "Cancel job" }).waitFor();
    assert.equal(
      requests.at(-1).config.backend,
      "remote",
      "unsaved explicit remote-only choice reaches the job"
    );
    await page.getByRole("button", { name: "Cancel job" }).click();
    await page
      .getByText("index: cancelled", { exact: false })
      .first()
      .waitFor();
    await page
      .getByText("Recent jobs / saved checkpoints", { exact: true })
      .click();
    await page
      .getByRole("button", { name: "Resume checkpoint" })
      .first()
      .click();
    await page.getByRole("button", { name: "Cancel job" }).click();
    await page
      .getByText("index: cancelled", { exact: false })
      .first()
      .waitFor();
    await page.locator("#video-overlap-codecs").fill("av1, hevc, h264");
    assert.equal(
      await page.locator("#video-overlap-codecs").inputValue(),
      "av1, hevc, h264"
    );
    await page
      .getByRole("button", { name: "Find overlapping segments", exact: true })
      .click();
    await page
      .getByText("search: complete", { exact: false })
      .first()
      .waitFor();
    const first = page.locator('[data-overlap-id="2"]');
    await first.waitFor();
    assert.equal(await first.locator('a[href="/scenes/2"]').count(), 1);
    assert.match(await first.innerText(), /Preserve both/);
    if (shots && viewport.width > 1000) {
      await first.scrollIntoViewIfNeeded();
      await page.screenshot({
        path: path.join(shots, "p10-review-desktop.png"),
      });
    }
    assert.equal(await page.locator("video").count(), 0);
    for (let open = 0; open < 2; open++) {
      await first
        .getByRole("button", { name: "Compare matched segments" })
        .click();
      const dialog = page.locator(".video-overlap-comparison"),
        videos = dialog.locator("video");
      await dialog
        .getByRole("button", { name: "Play matched segment" })
        .waitFor();
      await page.waitForFunction(() =>
        [...document.querySelectorAll(".video-overlap-comparison video")].every(
          (v) => v.readyState >= 1
        )
      );
      assert.deepEqual(
        await videos.evaluateAll((vs) => vs.map((v) => v.paused)),
        [true, true],
        "no autoplay on opening"
      );
      assert.ok((await videos.nth(1).evaluate((v) => v.currentTime)) >= 11.9);
      assert.equal(
        await page.locator(".video-overlap-review").isVisible(),
        false
      );
      await dialog
        .getByRole("button", { name: "Play matched segment" })
        .click();
      await page.waitForFunction(() =>
        [...document.querySelectorAll(".video-overlap-comparison video")].every(
          (v) => !v.paused && v.readyState >= 2
        )
      );
      await page.waitForFunction(
        () =>
          document.querySelector(".video-overlap-comparison video")
            .currentTime > 0.4
      );
      const times = await videos.evaluateAll((vs) =>
        vs.map((v) => v.currentTime)
      );
      assert.ok(
        Math.abs(times[1] - times[0] - 12) < 0.5,
        `offset drift: ${times}`
      );
      await dialog.getByRole("button", { name: "Pause both" }).click();
      assert.deepEqual(
        await videos.evaluateAll((vs) => vs.map((v) => v.paused)),
        [true, true]
      );
      await dialog.locator("#overlap-segment").selectOption("1");
      await page.waitForFunction(
        () =>
          Math.abs(
            document.querySelector(".video-overlap-comparison video")
              .currentTime - 14
          ) < 0.1
      );
      await dialog.getByLabel("Listen to audio").selectOption("b");
      assert.deepEqual(
        await videos.evaluateAll((vs) => vs.map((v) => v.muted)),
        [true, false]
      );
      if (shots && viewport.width < 500 && open === 0) {
        await dialog.locator(".modal-body").evaluate((body) => {
          body.scrollTop = 0;
        });
        await page.screenshot({
          path: path.join(shots, "p10-comparison-mobile.png"),
        });
      }
      await videos.nth(0).evaluate((v) => {
        v.currentTime = 22;
        v.dispatchEvent(new Event("timeupdate"));
      });
      assert.deepEqual(
        await videos.evaluateAll((vs) => vs.map((v) => v.paused)),
        [true, true]
      );
      await videos.evaluateAll((vs) => {
        window.closedOverlapVideos = vs;
      });
      await dialog.getByRole("button", { name: "Close comparison" }).click();
      await first.waitFor();
      assert.ok(
        await page.evaluate(() =>
          window.closedOverlapVideos.every(
            (v) => v.paused && !v.getAttribute("src")
          )
        )
      );
    }
    await page.getByRole("button", { name: "Next pairs" }).click();
    await page.locator('[data-overlap-id="12"]').waitFor();
    await page.getByRole("button", { name: "Previous pairs" }).click();
    await first.waitFor();
    stale = true;
    limited = true;
    await first.getByText("Primary file changed", { exact: false }).waitFor();
    assert.equal(
      await first
        .getByRole("button", { name: "Compare matched segments" })
        .isDisabled(),
      true
    );
    await page
      .getByText("Candidate/posting or alignment limits", { exact: false })
      .waitFor();
    assert.ok(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth + 1
      )
    );
    empty = true;
    await page
      .getByText("No supported intervals on this page", { exact: false })
      .waitFor();
    reject = true;
    await page
      .getByRole("button", { name: "Index reference", exact: true })
      .click();
    await page
      .getByText("Configure the remote worker first", { exact: true })
      .waitFor();
    reject = false;
    await page.reload();
    await page.locator("#video-overlap-codecs").waitFor();
    assert.equal(
      await page.locator("#video-overlap-codecs").inputValue(),
      "av1, hevc, h264"
    );
    await page.locator(".video-overlap-review .close").click();
    await page
      .getByRole("button", { name: "Open video overlap review", exact: true })
      .click();
    await page.locator("#video-overlap-reference").waitFor();
    assert.equal(
      await page.locator("#video-overlap-reference").inputValue(),
      ""
    );
    assert.equal(errors.length, 0, errors.join("\n"));
    console.log(
      `PASS ${viewport.width}x${viewport.height}: jobs, settings, pagination, real playback, offsets, pause, cleanup, stale, empty, errors, preferences`
    );
    await context.close();
  }
} finally {
  await browser.close();
  await server.close();
}
