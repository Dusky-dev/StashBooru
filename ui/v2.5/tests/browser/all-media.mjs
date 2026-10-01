import assert from "node:assert/strict";

// Optional mounted-browser regression suite. Use an isolated instance with
// both Images and Videos; see docs/p07-global-media.md for invocation.
const { chromium } = await import(
  process.env.PLAYWRIGHT_MODULE ?? "playwright"
);
const baseURL = process.env.STASH_BROWSER_URL ?? "http://127.0.0.1:9999";
const browser = await chromium.launch({
  headless: true,
  executablePath: process.env.CHROMIUM_EXECUTABLE,
  args: ["--no-sandbox", "--disable-dev-shm-usage"],
});

async function checkVideoPreview(page, selector) {
  const cards = page.locator(selector);
  const first = cards.first();
  const link = await first.locator("a.scene-card-link").getAttribute("href");
  const originalURL = page.url();
  await first.hover();
  const preview = first.locator(".unified-media-video-preview-button button");
  assert.equal(
    await preview
      .locator('svg[data-icon="magnifying-glass"], svg[data-icon="search"]')
      .count(),
    1
  );
  await preview.click();
  await page.locator(".unified-media-native-scene-player .video-js").waitFor();
  assert.equal(page.url(), originalURL, "Preview navigated away from the list");
  const scenePath = new URL(link, baseURL).pathname;
  await page.waitForFunction(
    (path) =>
      document
        .querySelector(".Lightbox-footer-center .image-link")
        ?.getAttribute("href") === path,
    scenePath
  );
  if ((await cards.count()) > 1) {
    const nextLink = await cards
      .nth(1)
      .locator("a.scene-card-link")
      .getAttribute("href");
    await page
      .locator(".Lightbox-display > .Lightbox-navbutton")
      .last()
      .click();
    await page.waitForFunction(
      (path) =>
        document
          .querySelector(".Lightbox-footer-center .image-link")
          ?.getAttribute("href") === path,
      new URL(nextLink, baseURL).pathname
    );
  }
  await page.getByTitle("Close Lightbox", { exact: true }).click();
  await page.locator(".Lightbox").waitFor({ state: "hidden" });
  await page
    .locator(".unified-media-native-scene-player")
    .waitFor({ state: "hidden" });
}

async function check(mode) {
  const page = await browser.newPage({
    viewport: { width: 1440, height: 1000 },
  });
  const errors = [];
  page.on("pageerror", (error) => errors.push(error.stack ?? error.message));
  page.on("console", (message) => {
    if (
      message.type() === "error" &&
      /React error|Maximum update|Invalid filter mode/.test(message.text())
    )
      errors.push(message.text());
  });
  let completed = false;
  await page.route("**/graphql", async (route) => {
    const request = route.request().postDataJSON();
    if (request?.operationName !== "FindMedia") return route.continue();
    await new Promise((resolve) => setTimeout(resolve, 700));
    if (mode === "error") {
      await route.fulfill({
        json: {
          data: null,
          errors: [{ message: "Browser regression query error" }],
        },
      });
    } else if (mode === "empty") {
      await route.fulfill({
        json: {
          data: {
            findMedia: {
              __typename: "FindMediaResultType",
              count: 0,
              image_count: 0,
              video_count: 0,
              items: [],
            },
          },
        },
      });
    } else {
      const response = await route.fetch();
      const payload = await response.json();
      if (mode === "videos") {
        const result = payload.data.findMedia;
        result.items = result.items.filter((item) => item.scene);
        result.count = result.items.length;
        result.image_count = 0;
        result.video_count = result.count;
      }
      await route.fulfill({ response, json: payload });
    }
    completed = true;
  });
  try {
    await page.goto(`${baseURL}/media`, { waitUntil: "domcontentloaded" });
    await page.locator(".media-list").waitFor();
    for (let i = 0; !completed && i < 100; i++) await page.waitForTimeout(100);
    assert.ok(completed, "FindMedia response did not complete");
    await page.waitForTimeout(300);
    assert.ok(
      !(await page.locator("body").innerText()).includes(
        "Something went wrong"
      ),
      "All hit the app error boundary"
    );
    assert.equal(
      await page.locator(".main .nav-tabs").count(),
      0,
      "All renders redundant media-type tabs"
    );
    if (mode === "error") {
      assert.ok(
        (await page.locator(".media-list").innerText()).includes(
          "Browser regression query error"
        )
      );
    } else if (mode === "empty") {
      assert.equal(
        await page
          .locator(".media-list .image-card, .media-list .scene-card")
          .count(),
        0
      );
    } else {
      await page.locator(".media-list .scene-card").first().waitFor();
      await checkVideoPreview(page, ".media-list .scene-card");
      if (mode === "loaded") {
        const image = page.locator(".media-list .image-card").first();
        const imageLink = await image
          .locator("a.image-card-link")
          .getAttribute("href");
        await image.hover();
        await image.locator(".preview-button button").click();
        await page.locator(".Lightbox").waitFor();
        await page.waitForFunction(
          (path) =>
            document
              .querySelector(".Lightbox-footer-center .image-link")
              ?.getAttribute("href") === path,
          new URL(imageLink, baseURL).pathname
        );
        assert.equal(
          await page.locator(".unified-media-native-scene-player").count(),
          0,
          "Image reopened the previous Video player"
        );
        await page.getByTitle("Close Lightbox", { exact: true }).click();
        await page.locator(".Lightbox").waitFor({ state: "hidden" });
        const imageCheck = page
          .locator(".media-list .image-card input.card-check")
          .first();
        const videoCheck = page
          .locator(".media-list .scene-card input.card-check")
          .first();
        await imageCheck.check();
        await videoCheck.check();
        assert.equal(await page.locator(".selected-count").innerText(), "2");
        assert.equal(
          await page
            .locator(".media-list .unified-media-video-preview-button")
            .count(),
          0,
          "Preview buttons remain active while selecting"
        );
        await imageCheck.uncheck();
        assert.ok(
          await videoCheck.isChecked(),
          "Image deselection changed Video selection"
        );
        await videoCheck.uncheck();
      }
      await page
        .locator(".media-list button")
        .filter({ has: page.locator("svg[data-icon='square']") })
        .click();
      await page
        .locator(".media-wall .wall-item")
        .first()
        .waitFor({ timeout: 10000 });
      assert.equal(await page.locator(".main .nav-tabs").count(), 0);
      const videoLink = page
        .locator(".media-wall .wall-item a[href^='/scenes/']")
        .first();
      await videoLink.click({ timeout: 10000 });
      await page.waitForURL(/\/scenes\/\d+/);
      await page.goBack();
      await page.locator(".media-list").waitFor();
    }
    assert.deepEqual(errors, [], `Browser errors in ${mode}`);
    console.log(
      `PASS: All ${mode}, delayed query, native list controls and no duplicate tabs`
    );
  } finally {
    await page.close();
  }
}

try {
  for (const mode of ["loaded", "empty", "videos", "error"]) await check(mode);
  const page = await browser.newPage();
  try {
    await page.goto(`${baseURL}/scenes`);
    await page.locator(".scene-card").first().waitFor();
    await checkVideoPreview(page, ".scene-card");
    console.log(
      "PASS: magnifying-glass previews and Video carousel work on the native Videos page"
    );
    for (const [legacy, native] of [
      ["/media/images", "/images"],
      ["/media/videos", "/scenes"],
    ]) {
      await page.goto(`${baseURL}${legacy}?q=browser-regression`, {
        waitUntil: "domcontentloaded",
      });
      await page.waitForURL((url) => url.pathname === native);
      assert.equal(
        new URL(page.url()).searchParams.get("q"),
        "browser-regression"
      );
      assert.ok(
        !(await page.locator("body").innerText()).includes(
          "Something went wrong"
        )
      );
    }
    console.log(
      "PASS: legacy type routes use the native Images/Videos pages and preserve search"
    );
  } finally {
    await page.close();
  }
} finally {
  await browser.close();
}
