import assert from "node:assert/strict";

// No server writes: simulate a second lightbox caller's live query refresh
// and native Apollo resume updates while the Video caller owns the preview.
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
page.on("pageerror", (e) => errors.push(e.stack ?? e.message));
page.on("console", (m) => {
  if (m.type() === "error" && /React error|Maximum update/.test(m.text()))
    errors.push(m.text());
});
try {
  await page.goto(`${baseURL}/media`);
  const card = page.locator(".media-list .scene-card").first();
  await card.waitFor();
  await page.evaluate(() => {
    const api = window.PluginApi;
    const image = {
      id: "browser-background-image",
      visual_files: [{ width: 300, height: 300 }],
      paths: {
        image:
          'data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" width="300" height="300"></svg>',
        thumbnail: "",
        preview: "",
      },
    };
    function CacheProbe() {
      window.previewProbeClient = api.libraries.Apollo.useApolloClient();
      const [version, setVersion] = api.React.useState(0);
      window.previewBackgroundVersion = version;
      window.previewRefreshBackground = () => setVersion((value) => value + 1);
      const images = api.React.useMemo(
        () => [{ ...image, title: `Background ${version}` }],
        [version]
      );
      // Simulate the native Images list receiving fresh data while the
      // separately mounted Video preview caller owns the shared viewer.
      api.hooks.useLightbox({
        images,
        page: 1,
        pages: 1,
        pageSize: 1,
        totalCount: 1,
      });
      return null;
    }
    api.patch.after("MainNavBar.MenuItems", (...args) =>
      api.React.createElement(
        api.React.Fragment,
        null,
        args.at(-1),
        api.React.createElement(CacheProbe)
      )
    );
  });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.locator(".nav-menu-toggle").click();
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.waitForFunction(() => Boolean(window.previewProbeClient));
  await card.hover();
  await card.locator(".unified-media-video-preview-button button").click();
  await page.locator(".unified-media-native-scene-player .video-js").waitFor();
  await page.evaluate(() => {
    window.previewProbe = [];
    window.previewProbeFrame = setInterval(
      () =>
        window.previewProbe.push({
          href: document
            .querySelector(".Lightbox-footer-center .image-link")
            ?.getAttribute("href"),
          lightboxes: document.querySelectorAll(".Lightbox").length,
          players: document.querySelectorAll(
            ".unified-media-native-scene-player"
          ).length,
          slides: document.querySelectorAll(
            ".Lightbox-carousel > .Lightbox-carousel-image"
          ).length,
          imageCount: document.querySelectorAll(
            ".Lightbox-carousel-image-wrapper img"
          ).length,
          playerID: document
            .querySelector(".unified-media-native-scene-player")
            ?.getAttribute("data-scene-id"),
        }),
      100
    );
  });
  const link = await card.locator("a.scene-card-link").getAttribute("href");
  const id = new URL(link, baseURL).pathname.split("/").at(-1);
  await page.evaluate(() => window.previewRefreshBackground());
  await page.waitForFunction(() => window.previewBackgroundVersion === 1);
  for (let value = 1; value <= 4; value++) {
    await page.evaluate(
      ({ id, value }) => {
        const api = window.PluginApi;
        window.previewProbeClient.writeFragment({
          id: `Scene:${id}`,
          fragment: api.libraries.Apollo
            .gql`fragment PreviewResume on Scene { id resume_time }`,
          data: { __typename: "Scene", id, resume_time: value },
        });
      },
      { id, value }
    );
    await page.waitForTimeout(200);
  }
  await page.waitForTimeout(6500);
  const samples = await page.evaluate(() => {
    clearInterval(window.previewProbeFrame);
    return window.previewProbe;
  });
  console.log(
    "Preview idle observations",
    JSON.stringify({
      samples: samples.length,
      hrefs: [...new Set(samples.map((s) => s.href))],
      lightboxes: [...new Set(samples.map((s) => s.lightboxes))],
      players: [...new Set(samples.map((s) => s.players))],
      playerIDs: [...new Set(samples.map((s) => s.playerID))],
      imageCount: [...new Set(samples.map((s) => s.imageCount))],
      slides: [...new Set(samples.map((s) => s.slides))],
      errors,
    })
  );
  assert.ok(
    samples.every((s) => s.lightboxes === 1 && s.players === 1),
    "Preview disappeared while idle"
  );
  assert.equal(
    new Set(samples.map((s) => s.href)).size,
    1,
    "Preview advanced without input"
  );
  assert.ok(
    samples.every(
      (s) => s.imageCount === 0 && s.playerID === id && s.slides > 1
    ),
    "Background cache refresh replaced the active Video preview with Image slides"
  );
  await page
    .locator(".unified-media-native-scene-player video")
    .dispatchEvent("ended");
  await page.waitForTimeout(300);
  assert.equal(
    await page
      .locator(".Lightbox-footer-center .image-link")
      .getAttribute("href"),
    new URL(link, baseURL).pathname,
    "Video completion advanced the preview without input"
  );
  assert.deepEqual(errors, []);
  await page.getByTitle("Close Lightbox", { exact: true }).click();
  await page.locator(".Lightbox").waitFor({ state: "hidden" });
  await page.waitForTimeout(800);
  assert.equal(
    await page.locator(".Lightbox").count(),
    0,
    "Closed preview reopened"
  );
  const cards = page.locator(".media-list .scene-card");
  assert.ok((await cards.count()) > 1, "Use a fixture with multiple Videos");
  for (const [index, close] of [
    [1, "escape"],
    [0, "back"],
    [1, "button"],
  ]) {
    const selected = cards.nth(index);
    const href = await selected
      .locator("a.scene-card-link")
      .getAttribute("href");
    await selected.hover();
    // A double click must be one open request, not a queued second viewer.
    await selected
      .locator(".unified-media-video-preview-button button")
      .evaluate((button) => {
        button.click();
        button.click();
      });
    await page
      .locator(".unified-media-native-scene-player .video-js")
      .waitFor();
    await page.waitForFunction(
      (path) =>
        document
          .querySelector(".Lightbox-footer-center .image-link")
          ?.getAttribute("href") === path,
      new URL(href, baseURL).pathname
    );
    await page.waitForTimeout(500);
    assert.equal(await page.locator(".Lightbox").count(), 1);
    assert.equal(
      await page.locator(".unified-media-native-scene-player").count(),
      1
    );
    if (close === "escape") await page.keyboard.press("Escape");
    else if (close === "back") await page.goBack();
    else await page.getByTitle("Close Lightbox", { exact: true }).click();
    await page.locator(".Lightbox").waitFor({ state: "hidden" });
    await page.waitForTimeout(500);
    assert.equal(
      await page.locator(".Lightbox").count(),
      0,
      `${close}: preview reopened`
    );
    assert.equal(
      await page.locator(".unified-media-native-scene-player").count(),
      0,
      `${close}: player survived dismissal`
    );
  }
  console.log(
    "PASS: repeated/double-clicked Video previews close cleanly with Escape, browser Back and the close button"
  );
  assert.deepEqual(errors, []);
  console.log(
    "PASS: Video preview stays on its selected item until navigation or close"
  );
} finally {
  await browser.close();
}
