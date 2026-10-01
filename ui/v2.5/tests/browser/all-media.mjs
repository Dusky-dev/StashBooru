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

async function check(mode) {
  const page = await browser.newPage({
    viewport: { width: 1440, height: 1000 },
  });
  const errors = [];
  page.on("pageerror", (error) => errors.push(error.message));
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
      if (mode === "loaded") {
        const imageCheck = page
          .locator(".media-list .image-card input.card-check")
          .first();
        const videoCheck = page
          .locator(".media-list .scene-card input.card-check")
          .first();
        await imageCheck.check();
        await videoCheck.check();
        assert.equal(await page.locator(".selected-count").innerText(), "2");
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
