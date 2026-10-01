import assert from "node:assert/strict";

// Read-only browser fixtures for a Character linked to at least one Image,
// Video, Copyright and direct Variant. The injected relation data stays local
// to this browser; the suite never writes library metadata.
const { chromium } = await import(
  process.env.PLAYWRIGHT_MODULE ?? "playwright"
);
const baseURL = process.env.STASH_BROWSER_URL ?? "http://127.0.0.1:9999";
const characterID = process.env.STASH_BROWSER_CHARACTER_ID;
assert.ok(characterID, "Set STASH_BROWSER_CHARACTER_ID to an isolated fixture");
const browser = await chromium.launch({
  headless: true,
  executablePath: process.env.CHROMIUM_EXECUTABLE,
  args: ["--no-sandbox", "--disable-dev-shm-usage"],
});
const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
const errors = [];
page.on("pageerror", (error) => errors.push(error.message));
let many = false;

await page.route("**/browser-relations/*.svg", async (route) => {
  const shape = route.request().url().includes("portrait")
    ? [400, 1600]
    : [1600, 400];
  await route.fulfill({
    contentType: "image/svg+xml",
    body: `<svg xmlns="http://www.w3.org/2000/svg" width="${shape[0]}" height="${shape[1]}"><rect width="100%" height="100%" fill="#597a96"/><rect x="8" y="8" width="${shape[0] - 16}" height="${shape[1] - 16}" fill="none" stroke="white" stroke-width="8"/></svg>`,
  });
});
await page.route("**/graphql", async (route) => {
  const request = route.request().postDataJSON();
  if (
    !["FindPerformer", "FindPerformerVariants"].includes(request?.operationName)
  )
    return route.continue();
  const response = await route.fetch();
  const payload = await response.json();
  const isCopyright = request.operationName === "FindPerformer";
  const result = isCopyright
    ? payload.data?.findPerformer
    : payload.data?.findPerformers;
  const items = isCopyright ? result?.copyrights : result?.performers;
  if (items?.length) {
    const fixtures = Array.from({ length: many ? 18 : 1 }, (_, index) => ({
      ...items[0],
      id: `browser-${isCopyright ? "copyright" : "variant"}-${index}`,
      name: `${isCopyright ? "Copyright" : "Variant"} ${index + 1}`,
      image_path: `/browser-relations/${isCopyright ? "copyright" : "variant"}-${(index + Number(isCopyright)) % 2 ? "portrait" : "landscape"}.svg`,
    }));
    if (isCopyright) result.copyrights = fixtures;
    else {
      result.performers = fixtures;
      result.count = fixtures.length;
    }
  }
  await route.fulfill({ response, json: payload });
});

try {
  await page.goto(`${baseURL}/performers/${characterID}`);
  await page
    .locator(".performer-relations .performer-variant-grid img")
    .waitFor();
  await page.waitForFunction(() =>
    [...document.querySelectorAll(".performer-relations img")].every(
      (image) => image.complete && image.naturalWidth
    )
  );
  const measure = () =>
    page.locator(".performer-relations").evaluate((node) => {
      const copyright = node.querySelector(".detail-item.copyrights");
      const variants = node.querySelector(".detail-item.character-variants");
      const c = copyright.getBoundingClientRect();
      const v = variants.getBoundingClientRect();
      return {
        sameRow: Math.abs(c.y - v.y) <= 1,
        gap: v.x - c.right,
        heights: [...node.querySelectorAll("img")].map(
          (image) => image.getBoundingClientRect().height
        ),
        cards: [...node.querySelectorAll(".booru-entity-card")].map((card) => {
          const frame = card
            .querySelector(".booru-entity-card-image-link")
            .getBoundingClientRect();
          return {
            copyright: card.classList.contains("booru-entity-card-copyright"),
            ratio: frame.width / frame.height,
            fit: getComputedStyle(card.querySelector("img")).objectFit,
          };
        }),
        panels: [...node.querySelectorAll(".detail-item-value")].map(
          (panel) => ({
            height: panel.clientHeight,
            scrollHeight: panel.scrollHeight,
            width: panel.clientWidth,
            scrollWidth: panel.scrollWidth,
            overflow: getComputedStyle(panel).overflowY,
          })
        ),
        viewport: innerHeight,
      };
    });
  const single = await measure();
  assert.ok(
    single.sameRow && single.gap >= 0 && single.gap <= 17,
    JSON.stringify(single)
  );
  function checkShapes(measurement) {
    for (const card of measurement.cards) {
      assert.ok(
        Math.abs(card.ratio - (card.copyright ? 16 / 9 : 3 / 4)) < 0.01,
        JSON.stringify(card)
      );
      assert.equal(card.fit, "contain");
    }
  }
  checkShapes(single);
  assert.ok(
    single.heights.every(
      (height) => height <= Math.min(256, single.viewport * 0.4) + 1
    )
  );
  console.log(
    "PASS: adjacent Copyrights use landscape cards; Variants use portrait cards without cropping or stretching"
  );
  if (process.env.STASH_BROWSER_SCREENSHOT_PREFIX)
    await page.screenshot({
      path: `${process.env.STASH_BROWSER_SCREENSHOT_PREFIX}-desktop.png`,
    });
  await page.setViewportSize({ width: 1440, height: 400 });
  const short = await measure();
  checkShapes(short);
  assert.ok(
    short.heights.every((height) => height <= short.viewport * 0.4 + 1)
  );
  await page.setViewportSize({ width: 1440, height: 1000 });

  const allTab = page.locator(
    '.performer-tabs > .nav-tabs [data-unified-media-all="tab"]'
  );
  await allTab.click();
  await page
    .locator(".unified-media-inline-pane .scene-card")
    .first()
    .waitFor();
  await page
    .locator(".unified-media-inline-pane .image-card")
    .first()
    .waitFor();
  const video = page.locator(".unified-media-inline-pane .scene-card").first();
  await video.hover();
  await video.locator(".unified-media-video-preview-button button").click();
  await page.locator(".unified-media-native-scene-player .video-js").waitFor();
  await page.getByTitle("Close Lightbox", { exact: true }).click();
  await page.locator(".Lightbox").waitFor({ state: "hidden" });
  console.log("PASS: Character All uses the shared Video preview player");
  const edit = page.locator(".details-edit button.edit");
  await edit.click();
  await page.locator(".detail-header.edit").waitFor();
  await page.waitForFunction(
    () =>
      !document.querySelector(
        ".performer-tabs .unified-media-inline-pane:not([hidden])"
      ) &&
      !document.querySelector(".performer-tabs .unified-media-controller") &&
      !document.querySelector(".performer-tabs .nav-tabs")
  );
  await page
    .getByRole("button", { name: "Cancel", exact: true })
    .first()
    .click();
  await allTab.waitFor();
  assert.equal(
    await page.locator(".performer-tabs.unified-media-all-active").count(),
    0
  );
  console.log(
    "PASS: active Character All contents unmount on Edit; Cancel restores native tabs"
  );

  many = true;
  await page.reload();
  await page.locator(".performer-relations img").nth(35).waitFor();
  await page.waitForFunction(() =>
    [...document.querySelectorAll(".performer-relations img")].every(
      (image) => image.complete && image.naturalWidth
    )
  );
  const large = await measure();
  checkShapes(large);
  assert.ok(large.sameRow, JSON.stringify(large));
  for (const panel of large.panels) {
    assert.ok(
      panel.height <= Math.min(384, large.viewport * 0.5) + 1,
      JSON.stringify(panel)
    );
    assert.ok(
      panel.scrollHeight > panel.height && panel.overflow === "auto",
      JSON.stringify(panel)
    );
    assert.ok(panel.scrollWidth <= panel.width + 1, JSON.stringify(panel));
  }
  for (const panel of await page
    .locator(".performer-relations .detail-item-value")
    .all()) {
    await panel.evaluate((node) => {
      node.scrollTop = node.scrollHeight;
    });
    assert.ok(await panel.locator("article").last().isVisible());
  }
  await page.setViewportSize({ width: 390, height: 844 });
  await page.waitForTimeout(100);
  const mobile = await page.evaluate(() => ({
    scroll: document.documentElement.scrollWidth,
    width: innerWidth,
  }));
  assert.ok(mobile.scroll <= mobile.width + 1, JSON.stringify(mobile));
  console.log(
    "PASS: large relation lists scroll within their height cap; mobile wraps without page overflow"
  );
  if (process.env.STASH_BROWSER_SCREENSHOT_PREFIX)
    await page.screenshot({
      path: `${process.env.STASH_BROWSER_SCREENSHOT_PREFIX}-mobile.png`,
    });
  assert.deepEqual(errors, []);
} finally {
  await browser.close();
}
