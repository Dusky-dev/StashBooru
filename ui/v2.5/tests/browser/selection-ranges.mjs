import assert from "node:assert/strict";
import fs from "node:fs";

// Read-only catalogue check. Use an isolated fixture with four Images sorting
// before two Videos, including an Image and Video with equal numeric IDs.
const { chromium } = await import(
  process.env.PLAYWRIGHT_MODULE ?? "playwright"
);
const origin = process.env.STASH_BROWSER_URL ?? "http://127.0.0.1:9999";
const shots = process.env.STASH_BROWSER_SCREENSHOTS;
if (shots) fs.mkdirSync(shots, { recursive: true });
const browser = await chromium.launch({
  executablePath: process.env.CHROMIUM_EXECUTABLE,
  args: ["--no-sandbox", "--disable-dev-shm-usage"],
});
const selector = ".image-card, .scene-card";
async function keys(cards) {
  return cards.evaluateAll((nodes) =>
    nodes.map((card) => {
      const link = card.querySelector("a.image-card-link, a.scene-card-link");
      const [, kind, id] = new URL(link.href).pathname.split("/");
      return `${kind === "images" ? "image" : "scene"}:${id}`;
    })
  );
}
async function count(page, expected) {
  await page.waitForFunction(
    (value) =>
      document.querySelector(".selected-count")?.textContent === String(value),
    expected
  );
}
async function changePage(page, number) {
  await page
    .locator(".pagination")
    .first()
    .getByTitle(number === 1 ? "First" : "Next", { exact: true })
    .click();
  await page.waitForURL(
    (url) => Number(url.searchParams.get("p") ?? 1) === number
  );
  await page
    .locator(".paginationIndex")
    .first()
    .filter({ hasText: number === 1 ? "1-2" : "3-4" })
    .waitFor();
}
async function capture(page, name) {
  if (shots)
    await page.screenshot({ path: `${shots}/${name}.png`, fullPage: true });
}
try {
  for (const [name, viewport] of [
    ["desktop", { width: 1440, height: 1000 }],
    ["mobile", { width: 390, height: 844 }],
  ]) {
    const page = await browser.newPage({ viewport });
    const errors = [];
    page.on("pageerror", (e) => errors.push(e.message));
    try {
      for (const path of ["media", "images"]) {
        await page.goto(
          `${origin}/${path}?perPage=2&sortby=title&sortdir=asc&disp=0`,
          { waitUntil: "networkidle" }
        );
        const cards = page.locator(selector);
        assert.equal(await cards.count(), 2, "Seed at least four Images");
        const firstPage = await keys(cards);
        await cards.first().locator("input.card-check").check();
        await count(page, 1);
        await changePage(page, 2);
        assert.equal(await cards.count(), 2);
        assert.ok(!(await keys(cards)).includes(firstPage[0]));
        await cards
          .nth(1)
          .locator("input.card-check")
          .click({ modifiers: ["Shift"] });
        await count(page, 3);
        assert.equal(
          await cards.locator("input.card-check:checked").count(),
          2,
          "Range skipped the first visible item after paging"
        );
        await changePage(page, 1);
        assert.equal(
          await cards.first().locator("input.card-check").isChecked(),
          true
        );
        assert.equal(
          await cards.nth(1).locator("input.card-check").isChecked(),
          false
        );
        await capture(page, `selection-range-${path}-${name}`);
        const url = page.url();
        await page.reload({ waitUntil: "networkidle" });
        assert.equal(page.url(), url);
        assert.equal(
          await cards.locator("input.card-check:checked").count(),
          0
        );
        assert.equal(await page.locator(".selected-count").count(), 0);
        const image = cards
          .filter({ has: page.locator(".image-card-link") })
          .first();
        for (const close of [
          () => page.goBack(),
          () => page.keyboard.press("Escape"),
        ]) {
          await image.hover();
          await image.locator(".preview-button button").click();
          await page.locator(".Lightbox").waitFor();
          await close();
          await page.locator(".Lightbox").waitFor({ state: "hidden" });
          assert.equal(page.url(), url);
        }
      }
      await page.goto(`${origin}/media?perPage=40&sortby=title&disp=0`, {
        waitUntil: "networkidle",
      });
      const cards = page.locator(selector);
      const allKeys = await keys(cards);
      const imageIndex = allKeys.findIndex(
        (key) =>
          key.startsWith("image:") &&
          allKeys.includes(`scene:${key.split(":")[1]}`)
      );
      assert.ok(
        imageIndex >= 0,
        "Seed an Image and Video with equal numeric IDs"
      );
      const sceneIndex = allKeys.indexOf(
        `scene:${allKeys[imageIndex].split(":")[1]}`
      );
      const image = cards.nth(imageIndex);
      const scene = cards.nth(sceneIndex);
      // Starting selection with a Video must work before selecting any Image.
      await scene.locator("input.card-check").check();
      await count(page, 1);
      await scene.locator("input.card-check").uncheck();
      await scene.hover();
      const preview = scene.locator(".unified-media-video-preview-button");
      const bounds = await preview.boundingBox();
      const cardBounds = await scene.boundingBox();
      assert.ok(
        bounds.width < cardBounds.width / 2 &&
          bounds.height < cardBounds.height / 2,
        "Video preview overlay covers the card"
      );
      await capture(page, `video-selection-${name}`);
      await preview.locator("button").click();
      await page
        .locator(".unified-media-native-scene-player .video-js")
        .waitFor();
      await page.keyboard.press("Escape");
      await page.locator(".Lightbox").waitFor({ state: "hidden" });
      await image.locator("input.card-check").check();
      await scene.locator("input.card-check").check();
      await count(page, 2);
      await image.locator("input.card-check").uncheck();
      assert.equal(await scene.locator("input.card-check").isChecked(), true);
      await count(page, 1);
      await image.locator("a.image-card-link").first().click();
      await count(page, 2);
      assert.equal(await page.locator(".Lightbox").count(), 0);
      await page.goto(`${origin}/scenes?perPage=40&sortby=title&disp=0`, {
        waitUntil: "networkidle",
      });
      await page.locator(".scene-card input.card-check").first().check();
      await count(page, 1);
      assert.equal(await page.locator(".Lightbox").count(), 0);
      assert.deepEqual(errors, []);
      console.log(
        `${name}: All/native ranges, typed selection, reload, Video-first selection and preview Back/Escape passed`
      );
    } catch (error) {
      await capture(page, `selection-${name}-failure`);
      throw error;
    } finally {
      await page.close();
    }
  }
} finally {
  await browser.close();
}
