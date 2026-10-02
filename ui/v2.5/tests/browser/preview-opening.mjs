import assert from "node:assert/strict";

// Observe the actual native player from before the click, including its first
// render and mount. Footer-only observations missed the original cycling.
// All stream metadata and playback-activity substitutions are browser-local.
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

try {
  for (const fixture of [
    { path: "/media", cards: ".media-list .scene-card", width: 1440 },
    { path: "/scenes", cards: ".scene-card", width: 390 },
    {
      path: `/performers/${characterID}`,
      cards: ".unified-media-inline-pane .scene-card",
      width: 1440,
      scoped: true,
    },
  ]) {
    const page = await browser.newPage({
      viewport: { width: 1440, height: 1000 },
    });
    const errors = [];
    let cancelledThumbnails = 0;
    page.on("pageerror", (error) => errors.push(error.stack ?? error.message));
    page.on("requestfailed", (request) => {
      if (/\/scene\/.*thumbs/.test(request.url())) cancelledThumbnails += 1;
    });
    await page.route("**/graphql", async (route) => {
      const request = route.request().postDataJSON();
      if (request?.operationName === "SceneSaveActivity") {
        await route.fulfill({ json: { data: { sceneSaveActivity: true } } });
        return;
      }
      if (request?.operationName === "SceneAddPlay") {
        await route.fulfill({
          json: { data: { sceneAddPlay: { count: 1, history: [] } } },
        });
        return;
      }
      let response;
      let payload;
      try {
        response = await route.fetch();
        payload = await response.json();
      } catch (error) {
        if (/closed|disposed|aborted/i.test(error.message)) return;
        throw error;
      }
      function largeVideo(value) {
        if (!value || typeof value !== "object") return;
        if (value.__typename === "VideoFile") {
          value.width = 1920;
          value.height = 1080;
        }
        for (const child of Object.values(value)) largeVideo(child);
      }
      largeVideo(payload);
      try {
        await route.fulfill({ response, json: payload });
      } catch (error) {
        if (!/closed|aborted|already handled/i.test(error.message)) throw error;
      }
    });
    await page.route("**/scene/**/*thumbs*", async (route) => {
      await new Promise((resolve) => setTimeout(resolve, 2000));
      try {
        await route.fulfill({ contentType: "text/vtt", body: "WEBVTT\n\n" });
      } catch (error) {
        if (
          !/closed|aborted|already handled|Invalid InterceptionId/i.test(
            error.message
          )
        )
          throw error;
      }
    });
    await page.goto(`${baseURL}${fixture.path}`);
    await page.waitForFunction(() => Boolean(window.PluginApi));
    await page.evaluate(() => {
      const api = window.PluginApi;
      window.openingPlayerEvents = [];
      function PlayerProbe({ id, children }) {
        api.React.useEffect(() => {
          window.openingPlayerEvents.push({ kind: "mount", id });
          return () => window.openingPlayerEvents.push({ kind: "unmount", id });
        }, [id]);
        return children;
      }
      api.patch.after("ScenePlayer", (...args) => {
        const id = String(args[0].scene.id);
        window.openingPlayerEvents.push({ kind: "render", id });
        return api.React.createElement(PlayerProbe, { id }, args.at(-1));
      });
    });
    await page.setViewportSize({
      width: fixture.width,
      height: fixture.width < 500 ? 844 : 1000,
    });
    if (fixture.scoped) {
      await page
        .locator('.performer-tabs > .nav-tabs [data-unified-media-all="tab"]')
        .click();
    }
    const cards = page.locator(fixture.cards);
    await cards.nth(1).waitFor();
    const count = await cards.count();
    for (const [attempt, index] of [
      count - 1,
      0,
      Math.floor(count / 2),
      count - 1,
    ].entries()) {
      const card = cards.nth(index);
      const href = await card.locator("a.scene-card-link").getAttribute("href");
      const path = new URL(href, baseURL).pathname;
      const id = path.split("/").at(-1);
      await card.hover();
      await page.evaluate(() => {
        window.openingPlayerEvents = [];
        window.openingFrames = [];
        function sample() {
          const player = document.querySelector(
            ".unified-media-native-scene-player .video-js"
          );
          const bounds = player?.getBoundingClientRect();
          window.openingFrames.push({
            href: document
              .querySelector(".Lightbox-footer-center .image-link")
              ?.getAttribute("href"),
            players: document.querySelectorAll(
              ".unified-media-native-scene-player"
            ).length,
            videos: document.querySelectorAll(
              ".unified-media-native-scene-player video"
            ).length,
            bounds: bounds && {
              left: bounds.left,
              top: bounds.top,
              width: bounds.width,
              height: bounds.height,
            },
          });
          window.openingFrame = requestAnimationFrame(sample);
        }
        window.openingFrame = requestAnimationFrame(sample);
      });
      await card.locator(".unified-media-video-preview-button button").click();
      await page
        .locator(".unified-media-native-scene-player .video-js")
        .waitFor();
      await page.waitForTimeout(1100);
      const observations = await page.evaluate(() => {
        cancelAnimationFrame(window.openingFrame);
        return {
          events: window.openingPlayerEvents,
          frames: window.openingFrames,
        };
      });
      assert.ok(observations.events.length > 0);
      assert.deepEqual(
        [...new Set(observations.events.map((event) => event.id))],
        [id],
        `${path}: another Video rendered during opening`
      );
      assert.equal(
        observations.events.filter((event) => event.kind === "mount").length,
        1,
        `${path}: opening remounted the player`
      );
      assert.ok(
        observations.frames.every(
          (frame) => frame.players <= 1 && frame.videos <= 1
        )
      );
      assert.ok(
        observations.frames.every(
          (frame) => !frame.href || frame.href === path
        ),
        `${path}: transient incorrect footer`
      );
      const frames = observations.frames.filter(
        (frame) => frame.bounds?.width > 0
      );
      assert.ok(frames.length > 0);
      for (const frame of frames) {
        for (const property of ["left", "top", "width", "height"]) {
          assert.ok(
            Math.abs(frame.bounds[property] - frames[0].bounds[property]) < 1,
            `${path}: transient player geometry ${JSON.stringify(frame.bounds)}`
          );
        }
      }
      const controls = await page
        .locator(".unified-media-native-scene-player .vjs-control-bar")
        .evaluate((node) => ({
          actual: node.getBoundingClientRect().height,
          css: Number.parseFloat(getComputedStyle(node).height),
        }));
      assert.ok(
        Math.abs(controls.actual - controls.css) < 1,
        `Fitted Video controls were scaled: ${JSON.stringify(controls)}`
      );
      console.log(
        "Opening",
        JSON.stringify({
          fixture: fixture.path,
          attempt,
          id,
          mounts: 1,
          bounds: frames[0].bounds,
        })
      );
      if (attempt === 2) {
        await page
          .locator(
            ".unified-media-native-scene-player .vjs-vtt-thumbnail-display"
          )
          .waitFor({ state: "attached" });
        await page
          .locator(".unified-media-native-scene-player .video-js")
          .hover();
        await page.mouse.wheel(0, -100);
        await page.getByTitle("Reset zoom", { exact: true }).click();
        assert.equal(
          await page
            .locator(".unified-media-native-scene-player")
            .getAttribute("data-scene-id"),
          id
        );
        assert.equal(
          await page.evaluate(
            () =>
              window.openingPlayerEvents.filter(
                (event) => event.kind === "mount"
              ).length
          ),
          1
        );
      }
      if (index === count - 1 && !fixture.scoped) {
        const nextHref = await cards
          .first()
          .locator("a.scene-card-link")
          .getAttribute("href");
        const nextPath = new URL(nextHref, baseURL).pathname;
        const nextID = nextPath.split("/").at(-1);
        await page.evaluate(() => {
          window.openingPlayerEvents = [];
        });
        await page.getByTitle("Close Lightbox", { exact: true }).focus();
        await page.keyboard.press("ArrowRight");
        await page.waitForFunction(
          (target) =>
            document
              .querySelector(".Lightbox-footer-center .image-link")
              ?.getAttribute("href") === target,
          nextPath
        );
        await page
          .locator(".unified-media-native-scene-player .video-js")
          .waitFor();
        assert.deepEqual(
          await page.evaluate(() => [
            ...new Set(
              window.openingPlayerEvents
                .filter((event) => event.kind === "render")
                .map((event) => event.id)
            ),
          ]),
          [nextID]
        );
        await page.getByTitle("Close Lightbox", { exact: true }).focus();
        await page.keyboard.press("ArrowLeft");
        await page.waitForFunction(
          (target) =>
            document
              .querySelector(".Lightbox-footer-center .image-link")
              ?.getAttribute("href") === target,
          path
        );
      }
      if (attempt === 3) {
        await page.locator(".Lightbox-footer-center .image-link").click();
        await page.waitForURL(`${baseURL}${path}`);
      } else await page.keyboard.press("Escape");
      await page.locator(".Lightbox").waitFor({ state: "hidden" });
      assert.equal(
        await page.locator(".unified-media-native-scene-player").count(),
        0
      );
    }
    assert.ok(
      cancelledThumbnails > 0,
      "Pending thumbnail requests survived player disposal"
    );
    assert.deepEqual(errors, []);
    await page.unrouteAll({ behavior: "ignoreErrors" });
    await page.close();
  }
  console.log(
    "PASS: cold/warm first, middle and last previews render only the selected Video, fit before painting, retain full-size controls and dispose cleanly"
  );
} finally {
  await browser.close();
}
