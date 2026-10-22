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
try {
  for (const viewport of [
    { width: 1440, height: 1000 },
    { width: 390, height: 844 },
  ]) {
    const page = await browser.newPage({ viewport });
    const errors = [];
    page.on("pageerror", (error) => errors.push(error.message));
    page.on("console", (message) => {
      if (
        message.type() === "error" &&
        /unmounted component|Maximum update/.test(message.text())
      )
        errors.push(message.text());
    });
    await page.addInitScript(() => {
      window.copiedJobError = "";
      navigator.clipboard.writeText = async (text) => {
        window.copiedJobError = text;
      };
    });
    await page.goto(
      `${origin}/tests/browser/fixtures/job-feedback.html?q=retained&page=2`
    );
    const scope = () =>
      page.getByRole("region", { name: "Conversion job feedback" });
    await scope().getByText("Completed with errors", { exact: true }).waitFor();
    assert.equal(
      await scope()
        .getByRole("link", { name: "Image #1", exact: true })
        .getAttribute("href"),
      "/images/1"
    );
    assert.equal(
      await scope()
        .getByRole("link", { name: "Video #1", exact: true })
        .getAttribute("href"),
      "/scenes/1"
    );
    assert.equal(
      await scope().getByRole("link", { name: /Trial/ }).count(),
      0,
      "A trial without a media target must not fabricate a native link"
    );
    assert.equal(
      await scope()
        .getByRole("link", { name: "View job queue (new tab)", exact: true })
        .getAttribute("href"),
      "/settings?tab=tasks#job-queue"
    );
    assert.equal(
      await scope()
        .getByRole("link", { name: "View job queue (new tab)", exact: true })
        .getAttribute("target"),
      "_blank"
    );
    await scope()
      .getByRole("button", { name: "Copy error", exact: true })
      .click();
    assert.equal(
      await page.evaluate(() => window.copiedJobError),
      "Image #1: Image decode failed\nDecoder detail\nVideo #1: Video decode failed\nTrial trial-a: Missing trial"
    );
    await scope().getByRole("link", { name: "Video #1", exact: true }).click();
    await page
      .getByLabel("Fixture location")
      .getByText("/scenes/1", { exact: true })
      .waitFor();
    assert.equal(
      await scope().count(),
      0,
      "Leaving through an affected item closes the caller"
    );
    await page.goBack();
    assert.match(page.url(), /q=retained&page=2$/);
    await page
      .getByRole("button", { name: "Reopen feedback", exact: true })
      .click();
    await page
      .getByRole("button", { name: "Many errors", exact: true })
      .click();
    await scope()
      .getByText(
        "Showing 50 of 53 item errors. Copy error includes all returned errors.",
        { exact: true }
      )
      .waitFor();
    assert.equal(
      await scope()
        .getByRole("link", { name: /Image #/ })
        .count(),
      50
    );
    await scope()
      .getByRole("button", { name: "Copy error", exact: true })
      .click();
    assert.match(
      await page.evaluate(() => window.copiedJobError),
      /Image #53: Failure 53\nFull detail 53$/
    );
    if (process.env.STASH_BROWSER_SCREENSHOTS) {
      fs.mkdirSync(process.env.STASH_BROWSER_SCREENSHOTS, { recursive: true });
      await page.screenshot({
        path: path.join(
          process.env.STASH_BROWSER_SCREENSHOTS,
          `job-feedback-${viewport.width}.png`
        ),
        fullPage: true,
      });
    }
    await page
      .getByRole("button", { name: "Clear errors", exact: true })
      .click();
    for (const [value, label] of [
      ["ready", "Queued"],
      ["queued", "Queued"],
      ["running", "Running"],
      ["stopping", "Stopping"],
      ["finished", "Completed"],
      ["complete", "Completed"],
      ["cancelled", "Cancelled"],
      ["failed", "Failed"],
      ["future-state", "future-state"],
    ]) {
      await page.getByLabel("Job state", { exact: true }).selectOption(value);
      await scope().getByText(label, { exact: true }).waitFor();
      assert.equal(
        await scope()
          .getByRole("button", { name: "Cancel remaining work", exact: true })
          .count(),
        ["queued", "running"].includes(value) ? 1 : 0
      );
    }
    await page.getByLabel("Job state", { exact: true }).selectOption("running");
    await scope()
      .getByRole("button", { name: "Cancel remaining work", exact: true })
      .click();
    assert.equal(
      await scope()
        .getByRole("button", { name: "Requesting cancellation…", exact: true })
        .isDisabled(),
      true
    );
    await page
      .getByText("Selected items: 2 · Cancellation requests: 1", {
        exact: true,
      })
      .waitFor();
    await page.evaluate(() => window.releaseJobCancel());
    await scope().getByText("Cancelled", { exact: true }).waitFor();
    assert.equal(
      await scope()
        .getByRole("button", { name: "Cancel remaining work", exact: true })
        .count(),
      0
    );
    assert.ok(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth + 1
      )
    );
    assert.deepEqual(errors, []);
    await page.close();
    console.log(
      `${viewport.width}px feedback: partial failures, full multiline copy, typed item links, Back context, queue link, status labels, cancellation and overflow passed`
    );
  }
} finally {
  await browser.close();
  await server.close();
}
