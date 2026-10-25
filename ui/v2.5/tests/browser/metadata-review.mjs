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
    const errors = [],
      reads = [],
      writes = [],
      optionalReads = [];
    let failApply = true;
    page.on("pageerror", (error) => errors.push(error.message));
    page.on("console", (message) => {
      if (
        message.type() === "error" &&
        /unmounted component|Maximum update|React error/.test(message.text())
      )
        errors.push(message.text());
    });
    async function handle(route) {
      const url = new URL(route.request().url());
      const [, kind, id, endpoint] = url.pathname.split("/");
      const reply = (json, status = 200) =>
        status >= 400
          ? route.fulfill({ status, body: json.error })
          : route.fulfill({ status, json });
      const tag = { name: "fixture_tag", category: "general", score: 1 };
      if (url.pathname === "/image/visual-similarity/camie/config")
        return reply({
          threshold: 0.492,
          eva02Threshold: 0.35,
          limit: 50,
          filenameEnabled: true,
          filenameLayout: "%artist%",
        });
      if (endpoint === "thumbnail" || endpoint === "image") {
        return route.fulfill({
          status: 200,
          contentType: "image/svg+xml",
          body: '<svg xmlns="http://www.w3.org/2000/svg" width="160" height="90"><rect width="160" height="90" fill="#777"/></svg>',
        });
      }
      if (endpoint === "local-metadata") {
        reads.push(`${kind}:${id}`);
        return reply({ backend: "local", model: "local", tags: [tag] });
      }
      if (endpoint === "knowledge-tags") {
        const data = route.request().postDataJSON();
        assert.equal(data.tags.length, 1);
        assert.equal(data.tags[0].name, "fixture_tag");
        if (url.searchParams.has("preview"))
          return reply({
            items: [
              { prediction: tag, action: "reuse", relationshipMode: "add" },
            ],
            inheritedTags: [],
            canApply: true,
            suppressedCount: 0,
            reviewCount: 0,
          });
        if (failApply) {
          failApply = false;
          return reply({ error: "Fixture save failure" }, 500);
        }
        writes.push(`${kind}:${id}`);
        return reply({
          imageID: Number(id),
          sceneID: Number(id),
          tags: [{ id: 9, ...tag, created: false }],
        });
      }
      optionalReads.push(`${kind}:${id}:${endpoint}`);
      if (endpoint === "booru-metadata")
        return reply({ error: "Provider unavailable in this fixture" }, 503);
      if (["camie-tags", "eva02-tags", "frame-metadata"].includes(endpoint))
        return reply({ error: "Model is not installed" }, 503);
      throw new Error(`Unexpected metadata request ${url.pathname}`);
    }
    await page.route("**/image/**", handle);
    await page.route("**/scene/**", handle);
    await page.goto(
      `${origin}/tests/browser/fixtures/metadata-review.html?q=keep&page=2`
    );
    const retainedURL = page.url();
    const more = () => page.locator("#more-menu");
    const main = () =>
      page
        .getByRole("dialog")
        .filter({ has: page.locator(".metadata-review-info") });
    async function close() {
      await main()
        .getByRole("button", { name: "Close", exact: true })
        .last()
        .click();
    }
    async function open() {
      await more().click();
      await page.getByText("Review metadata…", { exact: true }).click();
    }
    async function apply() {
      await main()
        .getByRole("button", { name: "Apply selected metadata", exact: true })
        .click();
      await page
        .getByRole("button", { name: "Apply this plan", exact: true })
        .click();
    }
    await more().click();
    assert.equal(
      await page.getByText("Review metadata…", { exact: true }).count(),
      0,
      "Empty selection has no review action"
    );
    await page.keyboard.press("Escape");
    await page.locator("label[for=fixture-selected]").click();
    await open();
    await main().getByText("Image #1", { exact: true }).waitFor();
    assert.equal(
      await main()
        .getByRole("img", { name: "Image #1 receiving metadata" })
        .getAttribute("src"),
      "image/1/image"
    );
    const previewDimensions = await main().evaluate((element) => {
      const preview = element.querySelector(
        ".image-tagging-review-preview img"
      );
      const body = element.querySelector(".modal-body");
      if (!preview || !body) return null;
      return {
        width: preview.getBoundingClientRect().width,
        height: preview.getBoundingClientRect().height,
        bodyHeight: body.getBoundingClientRect().height,
        objectFit: getComputedStyle(preview).objectFit,
      };
    });
    assert.ok(previewDimensions, "Image preview is mounted");
    assert.equal(previewDimensions.objectFit, "contain");
    assert.ok(
      viewport.width >= 768
        ? previewDimensions.width > 210 &&
            previewDimensions.height >= previewDimensions.bodyHeight - 40
        : previewDimensions.width > 250 && previewDimensions.height > 200,
      "Preview fills the available review pane without overflowing"
    );
    await main()
      .getByRole("button", { name: "Apply selected metadata", exact: true })
      .click({ trial: true });
    assert.deepEqual(reads, ["image:1"]);
    assert.deepEqual(
      optionalReads,
      [],
      "Opening never starts provider/model analysis"
    );
    assert.equal(
      await main()
        .getByRole("link", { name: "Tagging setup (new tab)" })
        .getAttribute("target"),
      "_blank"
    );
    await main().getByText("Item 1 of 3", { exact: true }).waitFor();
    await page.evaluate(() =>
      window.dispatchEvent(new Event("fixture-review-input-change"))
    );
    await apply();
    await main().getByText("Fixture save failure", { exact: true }).waitFor();
    await main().getByText("Image #1", { exact: true }).waitFor();
    assert.deepEqual(writes, []);
    await apply();
    await main().getByText("Video #1", { exact: true }).waitFor();
    await main().getByText("Item 2 of 3", { exact: true }).waitFor();
    assert.deepEqual(
      writes,
      ["image:1"],
      "Same numeric Video ID must not be the Image write target"
    );
    await main()
      .getByRole("button", { name: "Skip item", exact: true })
      .click();
    await main().getByText("Image #2", { exact: true }).waitFor();
    assert.equal(
      await main()
        .getByRole("img", { name: "Image #2 receiving metadata" })
        .getAttribute("src"),
      "image/2/image"
    );
    assert.deepEqual(writes, ["image:1"], "Skip writes nothing");
    await close();
    assert.equal(page.url(), retainedURL);
    await page
      .getByText("Selected items: 3. Refreshes: 1", { exact: true })
      .waitFor();
    await page
      .getByRole("button", { name: "Generate shortcut", exact: true })
      .click();
    const advisory = page
      .getByRole("dialog")
      .filter({ hasText: "Bulk tagging uses individual review" });
    await advisory.getByRole("button", { name: "OK", exact: true }).waitFor();
    assert.equal(
      await advisory
        .getByRole("button", { name: "Cancel", exact: true })
        .count(),
      1
    );
    await advisory.getByRole("button", { name: "Cancel", exact: true }).click();
    await page.getByRole("dialog").waitFor({ state: "detached" });
    await page
      .getByRole("button", { name: "Generate shortcut", exact: true })
      .click();
    await advisory.getByRole("button", { name: "Don't show again" }).click();
    assert.equal(
      await page.evaluate(() =>
        localStorage.getItem(
          "stashbooru:manual-bulk-tagging-warning-dismissed:v1"
        )
      ),
      "1"
    );
    await main().getByText("Image #1", { exact: true }).waitFor();
    await main()
      .getByRole("button", { name: "Skip item", exact: true })
      .click();
    await main().getByText("Image #2", { exact: true }).waitFor();
    await close();
    for (const view of ["Images", "Videos"]) {
      await page.getByLabel("View", { exact: true }).selectOption(view);
      await open();
      await main()
        .getByText(view === "Images" ? "Image #1" : "Video #1", { exact: true })
        .waitFor();
      if (view === "Videos") {
        await apply();
        await page.getByRole("dialog").waitFor({ state: "detached" });
        assert.deepEqual(writes, ["image:1", "scene:1"]);
      } else {
        assert.equal(await page.locator(".image-tagging-dialog").count(), 1);
        const toastClose = page.locator(".toast-container .close");
        if (await toastClose.isVisible()) {
          // Toast controls can be clipped outside the viewport by layout/animations.
          await toastClose.evaluate((element) => element.click());
        }
        await page.evaluate(async () => {
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
        assert.equal(
          await main().evaluate((element) => getComputedStyle(element).opacity),
          "1"
        );
        assert.ok(
          await page.evaluate(
            () => document.documentElement.scrollWidth <= innerWidth + 1
          ),
          "Open dialog fits viewport"
        );
        if (process.env.STASH_BROWSER_SCREENSHOTS) {
          fs.mkdirSync(process.env.STASH_BROWSER_SCREENSHOTS, {
            recursive: true,
          });
          await page.screenshot({
            path: path.join(
              process.env.STASH_BROWSER_SCREENSHOTS,
              `metadata-review-${viewport.width}.png`
            ),
          });
        }
        await main()
          .getByRole("button", { name: "Look up by MD5", exact: true })
          .click();
        await main()
          .getByText("Provider unavailable in this fixture", { exact: true })
          .waitFor();
        await main().getByText("Image #1", { exact: true }).waitFor();
        await close();
      }
    }
    for (const [button, title, identity] of [
      ["Single image", "Review image metadata", "Image #1"],
      ["Single video", "Review video metadata", "Video #1"],
    ]) {
      await page.getByRole("button", { name: button, exact: true }).click();
      await main().getByText(identity, { exact: true }).waitFor();
      assert.equal(await main().getByText(title, { exact: true }).count(), 1);
      assert.equal(
        await main()
          .getByRole("button", { name: "Skip item", exact: true })
          .count(),
        0
      );
      await close();
    }
    assert.equal(page.url(), retainedURL);
    assert.deepEqual(errors, []);
    assert.ok(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth + 1
      )
    );
    await page.close();
    console.log(
      `${viewport.width}px metadata: native actions, typed snapshot, review/apply/failure/skip/close, legacy shortcut, provider errors and retained selection/URL passed`
    );
  }
} finally {
  await browser.close();
  await server.close();
}
