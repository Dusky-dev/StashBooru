import assert from "node:assert/strict";

// Read-only fixtures: source image shapes are changed only in this browser.
// Use an isolated instance containing Characters, Artists and Copyrights.
const { chromium } = await import(
  process.env.PLAYWRIGHT_MODULE ?? "playwright"
);
const baseURL = process.env.STASH_BROWSER_URL ?? "http://127.0.0.1:9999";
const browser = await chromium.launch({
  headless: true,
  executablePath: process.env.CHROMIUM_EXECUTABLE,
  args: ["--no-sandbox", "--disable-dev-shm-usage"],
});
const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
const errors = [];
page.on("pageerror", (error) => errors.push(error.stack ?? error.message));
const lists = {
  FindPerformers: ["findPerformers", "performers", "landscape"],
  FindStudios: ["findStudios", "studios", "portrait"],
  FindCopyrights: ["findCopyrights", "copyrights", "portrait"],
};
await page.route("**/browser-card-shapes/*.svg", async (route) => {
  const [width, height] = route.request().url().includes("landscape")
    ? [1600, 400]
    : [400, 1600];
  await route.fulfill({
    contentType: "image/svg+xml",
    body: `<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}"><rect width="100%" height="100%" fill="#597a96"/></svg>`,
  });
});
await page.route("**/graphql", async (route) => {
  const fixture = lists[route.request().postDataJSON()?.operationName];
  if (!fixture) return route.continue();
  const response = await route.fetch();
  const payload = await response.json();
  const [resultKey, itemKey, shape] = fixture;
  for (const item of payload.data?.[resultKey]?.[itemKey] ?? []) {
    item.image_path = `/browser-card-shapes/${shape}.svg`;
  }
  await route.fulfill({ response, json: payload });
});

try {
  await page.goto(`${baseURL}/media`);
  const menu = page.locator(
    '.navbar-collapse .nav-link[data-rb-event-key^="/"]'
  );
  const expected = [
    "/media",
    "/images",
    "/scenes",
    "/performers",
    "/studios",
    "/copyrights",
    "/tags",
    "/galleries",
    "/groups",
  ];
  await page.waitForFunction(
    () =>
      document.querySelectorAll(
        '.navbar-collapse .nav-link[data-rb-event-key^="/"]'
      ).length >= 9
  );
  async function checkMenu() {
    const items = await menu.evaluateAll((nodes) =>
      nodes
        .filter((node) => getComputedStyle(node).display !== "none")
        .map((node) => ({
          path: node.getAttribute("data-rb-event-key"),
          label: node.textContent.trim(),
          x: node.getBoundingClientRect().x,
        }))
    );
    assert.deepEqual(
      items.map((item) => item.path),
      expected
    );
    assert.equal(items[0].label, "All");
    return items;
  }
  const desktop = await checkMenu();
  assert.ok(
    desktop.every((item, index) => index === 0 || item.x > desktop[index - 1].x)
  );
  await page.setViewportSize({ width: 390, height: 844 });
  await page.locator(".nav-menu-toggle").click();
  await page.locator(".navbar-collapse.show").waitFor();
  await checkMenu();
  console.log(
    "PASS: All is capitalized and first; Copyrights sit between Artists and Tags on desktop and mobile"
  );

  for (const [path, selector, ratio, fit] of [
    ["/performers", ".performer-card-image", 2 / 3, "cover"],
    ["/studios", ".studio-card-image", 1, "contain"],
    ["/copyrights", ".copyright-card-image", 16 / 9, "contain"],
  ]) {
    for (const viewport of [
      { width: 1440, height: 1000 },
      { width: 390, height: 844 },
    ]) {
      await page.setViewportSize(viewport);
      await page.goto(`${baseURL}${path}`);
      const image = page.locator(selector).first();
      await image.waitFor();
      await image.evaluate((node) => node.scrollIntoView());
      await page.waitForFunction((selector) => {
        const image = document.querySelector(selector);
        return image?.complete && image.naturalWidth;
      }, selector);
      const shape = await image.evaluate((node) => {
        const bounds = node.getBoundingClientRect();
        return {
          ratio: bounds.width / bounds.height,
          fit: getComputedStyle(node).objectFit,
          width: bounds.width,
          height: bounds.height,
          maxHeight: getComputedStyle(node).maxHeight,
          cssRatio: getComputedStyle(node).aspectRatio,
        };
      });
      assert.ok(
        Math.abs(shape.ratio - ratio) < 0.01,
        `${path}: ${JSON.stringify(shape)}`
      );
      assert.equal(shape.fit, fit);
      if (
        process.env.STASH_BROWSER_SCREENSHOT_PREFIX &&
        viewport.width === 1440
      ) {
        await page.screenshot({
          path: `${process.env.STASH_BROWSER_SCREENSHOT_PREFIX}-${path.slice(1)}-cards.png`,
        });
      }
    }
  }
  console.log(
    "PASS: Character portrait, Artist square and Copyright landscape card images survive opposite-shape sources on desktop and mobile"
  );
  assert.deepEqual(errors, []);
} finally {
  await browser.close();
}
