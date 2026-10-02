import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

// Read-only HTTP fixture: native controls must not also drive the Image
// carousel, and decoded card/wall clips must yield to the foreground viewer.
const engines = await import(process.env.PLAYWRIGHT_MODULE ?? "playwright");
const engineName = process.env.STASH_BROWSER_ENGINE ?? "chromium";
const baseURL = process.env.STASH_BROWSER_URL ?? "http://127.0.0.1:9999";
const characterID = process.env.STASH_BROWSER_CHARACTER_ID;
const clip = readFileSync(process.env.STASH_BROWSER_VIDEO_FILE);
assert.ok(characterID, "Set STASH_BROWSER_CHARACTER_ID to an isolated fixture");
const browser = await engines[engineName].launch({
  headless: true,
  executablePath:
    engineName === "chromium"
      ? process.env.CHROMIUM_EXECUTABLE
      : process.env.FIREFOX_EXECUTABLE,
  args:
    engineName === "chromium"
      ? ["--no-sandbox", "--disable-dev-shm-usage"]
      : [],
  firefoxUserPrefs: process.env.STASH_BROWSER_FIREFOX_SINGLE_PROCESS
    ? {
        "fission.autostart": false,
        "browser.tabs.remote.autostart": false,
        "security.sandbox.content.level": 0,
      }
    : undefined,
});

async function fixturePage(path, mode = "PAN_Y", width = 1440) {
  const page = await browser.newPage({
    viewport: { width, height: width < 500 ? 844 : 1000 },
  });
  const errors = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.route("**/browser-preview/*.webm", (route) =>
    route.fulfill({ contentType: "video/webm", body: clip })
  );
  await page.route("**/graphql", async (route) => {
    const operation = route.request().postDataJSON()?.operationName;
    if (operation === "SceneSaveActivity")
      return route.fulfill({ json: { data: { sceneSaveActivity: true } } });
    if (operation === "SceneAddPlay")
      return route.fulfill({
        json: { data: { sceneAddPlay: { count: 1, history: [] } } },
      });
    const response = await route.fetch();
    const payload = await response.json();
    function visit(value) {
      if (!value || typeof value !== "object") return;
      if (value.imageLightbox) value.imageLightbox.scrollMode = mode;
      if (value.__typename === "Scene" && value.paths) {
        value.paths.preview = `${baseURL}/browser-preview/preview-${value.id}.webm`;
        value.paths.vtt = null;
      }
      if (value.__typename === "Scene" && value.sceneStreams) {
        value.sceneStreams = [
          {
            __typename: "SceneStreamEndpoint",
            url: `${baseURL}/browser-preview/stream-${value.id}.webm`,
            mime_type: "video/webm",
            label: "Browser VP8 fixture",
          },
        ];
      }
      for (const child of Object.values(value)) visit(child);
    }
    visit(payload);
    await route.fulfill({ response, json: payload });
  });
  await page.goto(`${baseURL}${path}`);
  await page.waitForFunction(() => Boolean(window.PluginApi));
  await page.evaluate(() => {
    const api = window.PluginApi;
    window.previewInputEvents = [];
    function Probe({ id, children }) {
      window.previewInputClient = api.libraries.Apollo.useApolloClient();
      api.React.useEffect(() => {
        window.previewInputEvents.push({ kind: "mount", id });
        return () => window.previewInputEvents.push({ kind: "unmount", id });
      }, [id]);
      return children;
    }
    api.patch.after("ScenePlayer", (...args) =>
      api.React.createElement(
        Probe,
        { id: String(args[0].scene.id) },
        args.at(-1)
      )
    );
  });
  if (path.startsWith("/performers/"))
    await page
      .locator('.performer-tabs > .nav-tabs [data-unified-media-all="tab"]')
      .click();
  return { page, errors };
}

async function selectedID(page) {
  return page
    .locator(".unified-media-native-scene-player")
    .getAttribute("data-scene-id");
}

async function assertOnlySelected(page, id) {
  assert.equal(await selectedID(page), id);
  assert.deepEqual(
    await page.evaluate(() => [
      ...new Set(window.previewInputEvents.map((event) => event.id)),
    ]),
    [id]
  );
  assert.equal(
    await page.evaluate(
      () =>
        window.previewInputEvents.filter((event) => event.kind === "mount")
          .length
    ),
    1
  );
}

try {
  for (const [path, selector, width] of [
    ["/media", ".media-list .scene-card", 1440],
    ["/scenes", ".scene-card", 390],
    [
      `/performers/${characterID}`,
      ".unified-media-inline-pane .scene-card",
      1440,
    ],
  ]) {
    const { page, errors } = await fixturePage(path, "PAN_Y", width);
    const cards = page.locator(selector);
    await cards.nth(1).waitFor();
    const first = cards.first();
    const link = await first.locator("a.scene-card-link").getAttribute("href");
    const id = new URL(link, baseURL).pathname.split("/").at(-1);
    await first.hover();
    await page.waitForFunction(
      (scope) =>
        document.querySelector(`${scope} .scene-card-preview-video`)
          ?.readyState >= 2,
      selector
    );
    await first.locator(".unified-media-video-preview-button button").click();
    await page
      .locator(".unified-media-native-scene-player .video-js")
      .waitFor();
    await page.locator(".unified-media-native-scene-player .video-js").hover();
    for (let event = 0; event < 12; event++) {
      await page.mouse.wheel(0, event < 6 ? 120 : -120);
      await page.waitForTimeout(25);
    }
    await assertOnlySelected(page, id);
    // The empty area surrounding a fitted player is also an Image scroll
    // target. A continuing page-scroll gesture must not change the Video.
    await page.mouse.move(5, 80);
    await page.mouse.wheel(0, 120);
    await assertOnlySelected(page, id);
    const player = page.locator(".unified-media-native-scene-player .video-js");
    await player.focus();
    await page.keyboard.press("ArrowRight");
    await page.keyboard.down("ArrowLeft");
    for (let event = 0; event < 6; event++)
      await page.keyboard.down("ArrowLeft");
    await page.keyboard.up("ArrowLeft");
    await page.waitForTimeout(150);
    await assertOnlySelected(page, id);
    assert.equal(
      await page
        .locator(`${selector} .scene-card-preview-video`)
        .evaluateAll((videos) => videos.every((video) => video.paused)),
      true,
      "Grid clips kept playing behind the viewer"
    );
    await page.evaluate(async () => {
      await window.previewInputClient.refetchQueries({
        include: [
          window.PluginApi.GQL.FindScenesDocument,
          window.PluginApi.GQL.FindSceneDocument,
        ],
      });
    });
    await assertOnlySelected(page, id);
    // Explicit native next/previous controls still change the selected Video.
    await page
      .locator(".unified-media-native-scene-player .vjs-icon-next-item")
      .click();
    await page.waitForFunction(
      (previous) =>
        document.querySelector(".unified-media-native-scene-player")?.dataset
          .sceneId !== previous,
      id
    );
    await page
      .locator(".unified-media-native-scene-player .vjs-icon-previous-item")
      .click();
    await page.waitForFunction(
      (previous) =>
        document.querySelector(".unified-media-native-scene-player")?.dataset
          .sceneId === previous,
      id
    );
    await page.getByTitle("Close Lightbox", { exact: true }).click();
    await page.locator(".Lightbox").waitFor({ state: "hidden" });
    assert.deepEqual(errors, []);
    console.log(
      `PASS (${engineName}): ${path} keeps one Video through wheel bursts, native seek keys and network refetches; explicit next/previous works`
    );
    await page.unrouteAll({ behavior: "ignoreErrors" });
    await page.close();
  }

  // In zoom mode, wheel events on native controls should not scale the player.
  {
    const { page, errors } = await fixturePage("/media", "ZOOM");
    const card = page.locator(".media-list .scene-card").first();
    await card.hover();
    await card.locator(".unified-media-video-preview-button button").click();
    const player = page.locator(".unified-media-native-scene-player .video-js");
    await player.waitFor();
    await player.hover();
    const before = await player.boundingBox();
    await page
      .locator(".unified-media-native-scene-player .vjs-play-control")
      .hover();
    await page.mouse.wheel(0, -120);
    await page.waitForTimeout(100);
    assert.deepEqual(
      await player.boundingBox(),
      before,
      "A control-bar wheel event resized the player"
    );
    assert.equal(
      await page.getByTitle("Reset zoom", { exact: true }).count(),
      0
    );
    await page.getByTitle("Close Lightbox", { exact: true }).click();
    await page.locator(".Lightbox").waitFor({ state: "hidden" });
    assert.deepEqual(errors, []);
    console.log(
      `PASS (${engineName}): native controls keep their size in Image zoom mode`
    );
    await page.unrouteAll({ behavior: "ignoreErrors" });
    await page.close();
  }

  for (const path of ["/media", "/scenes"]) {
    const { page, errors } = await fixturePage(path);
    await page
      .locator(".main button")
      .filter({ has: page.locator("svg[data-icon='square']") })
      .click();
    const videos = page.locator(
      path === "/media" ? ".media-wall video" : ".scene-wall video"
    );
    await videos.first().waitFor();
    await page.waitForFunction(
      (selector) =>
        [...document.querySelectorAll(selector)].some(
          (video) => !video.paused && video.currentTime > 0
        ),
      path === "/media" ? ".media-wall video" : ".scene-wall video"
    );
    await videos.first().click();
    await page
      .locator(".unified-media-native-scene-player .video-js")
      .waitFor();
    for (let sample = 0; sample < 8; sample++) {
      assert.equal(
        await videos.evaluateAll((items) =>
          items.every((video) => video.paused)
        ),
        true,
        `${path}: wall clips played underneath the foreground viewer`
      );
      await page.waitForTimeout(100);
    }
    await page.getByTitle("Close Lightbox", { exact: true }).click();
    await page.locator(".Lightbox").waitFor({ state: "hidden" });
    await page.waitForFunction(
      (selector) =>
        [...document.querySelectorAll(selector)].some((video) => !video.paused),
      path === "/media" ? ".media-wall video" : ".scene-wall video"
    );
    assert.deepEqual(errors, []);
    console.log(
      `PASS (${engineName}): ${path} wall previews pause while covered and resume after dismissal`
    );
    await page.unrouteAll({ behavior: "ignoreErrors" });
    await page.close();
  }

  // A slow or broken grid preview keeps the screenshot instead of presenting
  // an empty player frame. Playback stops on leave and during selection.
  {
    const { page, errors } = await fixturePage("/scenes");
    const cards = page.locator(".scene-card");
    await cards.nth(1).waitFor();
    await page.route("**/browser-preview/preview-*.webm", async (route) => {
      await new Promise((resolve) => setTimeout(resolve, 700));
      if (route.request().url().includes("preview-1.webm"))
        await route.fulfill({ contentType: "video/webm", body: clip });
      else await route.fulfill({ status: 404 });
    });
    const first = cards.first();
    await first.hover();
    assert.equal(await first.locator(".scene-card-preview-ready").count(), 0);
    await first.locator(".scene-card-preview-ready").waitFor();
    await page.mouse.move(0, 0);
    await page.waitForFunction(() =>
      [...document.querySelectorAll(".scene-card-preview-video")].every(
        (video) => video.paused
      )
    );
    await cards.nth(1).hover();
    await page.waitForTimeout(900);
    assert.equal(
      await cards.nth(1).locator(".scene-card-preview-ready").count(),
      0,
      "Failed preview replaced the screenshot"
    );
    assert.deepEqual(errors, []);
    console.log(
      `PASS (${engineName}): delayed/failed hover clips retain screenshots and stop on pointer leave`
    );
    await page.unrouteAll({ behavior: "ignoreErrors" });
    await page.close();
  }
} finally {
  await browser.close();
}
