import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

// Read-only HTTP fixture: wheel gestures must not change Videos, preview
// arrows navigate once without seeking, and background clips yield.
const engines = await import(process.env.PLAYWRIGHT_MODULE ?? "playwright");
const engineName = process.env.STASH_BROWSER_ENGINE ?? "chromium";
const baseURL = process.env.STASH_BROWSER_URL ?? "http://127.0.0.1:9999";
const characterID = process.env.STASH_BROWSER_CHARACTER_ID;
const clip = readFileSync(process.env.STASH_BROWSER_VIDEO_FILE);
const recorder = readFileSync(
  new URL("../../scripts/record-preview-debug.js", import.meta.url),
  "utf8"
);
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
    await page.evaluate(recorder);
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
    const nextLink = await cards
      .nth(1)
      .locator("a.scene-card-link")
      .getAttribute("href");
    const nextID = new URL(nextLink, baseURL).pathname.split("/").at(-1);
    await page.evaluate(() => {
      window.previewArrowSeeks = 0;
      const video = document.querySelector(
        ".unified-media-native-scene-player video"
      );
      video.pause();
      video.addEventListener("seeking", () => window.previewArrowSeeks++);
    });
    await page.locator(".unified-media-native-scene-player .video-js").focus();
    await page.keyboard.press("ArrowRight");
    await page.waitForFunction(
      (expected) =>
        document.querySelector(".unified-media-native-scene-player")?.dataset
          .sceneId === expected,
      nextID
    );
    assert.equal(
      await page.evaluate(() => window.previewArrowSeeks),
      0,
      "Preview arrow also sought within the outgoing Video"
    );
    await page
      .locator(".unified-media-native-scene-player .vjs-play-control")
      .focus();
    await page.keyboard.down("ArrowLeft");
    for (let event = 0; event < 6; event++)
      await page.keyboard.down("ArrowLeft");
    await page.keyboard.up("ArrowLeft");
    await page.waitForFunction(
      (expected) =>
        document.querySelector(".unified-media-native-scene-player")?.dataset
          .sceneId === expected,
      id
    );
    await page
      .locator(".unified-media-native-scene-player .video-js")
      .waitFor();
    assert.deepEqual(
      await page.evaluate(() =>
        window.previewInputEvents
          .filter((event) => event.kind === "mount")
          .map((event) => event.id)
      ),
      [id, nextID, id],
      "A held arrow rapidly cycled Videos"
    );
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
    // The production-console recorder observes actual mounts/input without
    // relying on the test's ScenePlayer patch or changing playback behavior.
    const trace = await page.evaluate(() => window.stashPreviewDebug.stop());
    assert.equal(trace.format, "stashbooru-preview-v1");
    assert.equal(trace.page, new URL(page.url()).pathname);
    assert.ok(
      trace.events.some(
        (event) => event.kind === "viewer" && event.scenes.includes(id)
      )
    );
    assert.ok(
      trace.events.some(
        (event) => event.kind === "viewer" && event.scenes.includes(nextID)
      )
    );
    assert.ok(
      trace.events.some(
        (event) => event.kind === "video-added" && event.area === "lightbox"
      )
    );
    assert.ok(
      trace.events.some(
        (event) => event.kind.startsWith("visual-") && event.area === "poster"
      ),
      "Trace did not capture the native player's poster"
    );
    assert.ok(
      trace.events.some(
        (event) => event.kind === "input" && event.key === "ArrowRight"
      )
    );
    assert.equal(trace.events.at(-1).kind, "stop");
    assert.equal(trace.dropped, 0);
    assert.ok(
      !JSON.stringify(trace).includes(baseURL),
      "Trace exposed the server address"
    );
    await page.evaluate(() => {
      const video = document.createElement("video");
      video.preload = "none";
      video.src =
        "https://example.invalid/scene/123/preview?signature=private-signature&api_key=private-key";
      video.title = "Private title";
      document.body.append(video);
      window.stashPreviewDebugProbe = video;
      const host = document.createElement("div");
      host.className = "VideoPlayer";
      const poster = document.createElement("div");
      poster.className = "vjs-poster";
      poster.style.backgroundImage =
        'url("https://example.invalid/scene/123/screenshot?signature=private-signature&api_key=private-key")';
      poster.title = "Private title";
      host.append(poster);
      document.body.append(host);
      window.stashPreviewDebugPoster = host;
    });
    await page.evaluate(recorder);
    await page.evaluate(() => {
      window.stashPreviewDebugProbe.remove();
      window.stashPreviewDebugPoster.remove();
    });
    await page.waitForTimeout(30);
    const sanitized = await page.evaluate(() =>
      window.stashPreviewDebug.stop()
    );
    const serialized = JSON.stringify(sanitized);
    assert.ok(serialized.includes("/scene/123/preview"));
    assert.ok(serialized.includes("/scene/123/screenshot"));
    for (const privateValue of [
      "private-signature",
      "private-key",
      "Private title",
      "example.invalid",
    ])
      assert.ok(!serialized.includes(privateValue));
    const downloading = page.waitForEvent("download");
    await page.evaluate(() => window.stashPreviewDebug.download());
    const download = await downloading;
    assert.equal(download.suggestedFilename(), "stashbooru-preview-debug.json");
    assert.deepEqual(
      JSON.parse(readFileSync(await download.path(), "utf8")),
      sanitized
    );
    await page.keyboard.press("ArrowRight");
    assert.deepEqual(
      await page.evaluate(() => window.stashPreviewDebug.stop()),
      sanitized,
      "Stopped recorder continued capturing events"
    );
    assert.deepEqual(errors, []);
    console.log(
      `PASS (${engineName}): ${path} retains selection through wheel/refetch; focused preview arrows navigate without seeking or repeated cycling; explicit buttons work`
    );
    await page.unrouteAll({ behavior: "ignoreErrors" });
    await page.close();
  }

  // A held arrow must also stop after landing on an Image in a mixed queue.
  {
    const { page, errors } = await fixturePage(`/performers/${characterID}`);
    const scope = ".unified-media-inline-pane";
    await page.locator(`${scope} .image-card`).first().waitFor();
    const order = await page
      .locator(`${scope} a.scene-card-link, ${scope} a.image-card-link`)
      .evaluateAll((links) =>
        links.map((link) => ({
          href: link.getAttribute("href"),
          video: link.matches(".scene-card-link"),
        }))
      );
    const edge = order.findIndex(
      (entry, index) =>
        entry.video &&
        order[(index + 1) % order.length] &&
        !order[(index + 1) % order.length].video
    );
    assert.ok(edge >= 0, "Fixture needs a Video followed by an Image");
    const scene = order[edge];
    const image = order[(edge + 1) % order.length];
    const card = page
      .locator(`${scope} .scene-card`)
      .filter({ has: page.locator(`a.scene-card-link[href='${scene.href}']`) });
    await card.hover();
    await card.locator(".unified-media-video-preview-button button").click();
    await page.locator(".unified-media-native-scene-player .video-js").focus();
    await page.keyboard.down("ArrowRight");
    for (let event = 0; event < 8; event++)
      await page.keyboard.down("ArrowRight");
    await page.waitForFunction(
      (expected) =>
        document
          .querySelector(".Lightbox .image-link")
          ?.getAttribute("href") === expected,
      image.href
    );
    assert.equal(
      await page.locator(".unified-media-native-scene-player").count(),
      0
    );
    await page.keyboard.up("ArrowRight");
    await page.keyboard.press("ArrowLeft");
    await page.waitForFunction(
      (expected) =>
        document
          .querySelector(".Lightbox .image-link")
          ?.getAttribute("href") === expected,
      scene.href
    );
    await page
      .locator(".unified-media-native-scene-player .video-js")
      .waitFor();
    await page.getByTitle("Close Lightbox", { exact: true }).click();
    await page.locator(".Lightbox").waitFor({ state: "hidden" });
    assert.deepEqual(errors, []);
    console.log(
      `PASS (${engineName}): a held preview arrow lands once on an Image; release/previous returns to its Video`
    );
    await page.unrouteAll({ behavior: "ignoreErrors" });
    await page.close();
  }

  // The hidden viewer must leave the full Video detail player's seek shortcut.
  {
    const { page, errors } = await fixturePage("/scenes/1");
    await page.locator(".VideoPlayer .video-js").waitFor();
    await page.evaluate(async () => {
      const player = document.querySelector(".VideoPlayer .video-js").player;
      player.muted(true);
      player.loop(true);
      await player.play();
    });
    await page.waitForFunction(
      () => document.querySelector(".VideoPlayer video")?.readyState >= 2
    );
    await page.waitForFunction(
      () =>
        document
          .querySelector(".VideoPlayer .video-js")
          ?.player?.currentTime() > 0.25
    );
    assert.equal(await page.locator(".Lightbox").count(), 0);
    await page.evaluate(() =>
      document.querySelector(".VideoPlayer .video-js").player.pause()
    );
    await page.locator(".VideoPlayer .video-js").focus();
    await page.keyboard.press("ArrowLeft");
    await page.waitForFunction(
      () =>
        document
          .querySelector(".VideoPlayer .video-js")
          ?.player?.currentTime() < 0.1
    );
    assert.equal(await page.locator(".Lightbox").count(), 0);
    assert.deepEqual(errors, []);
    console.log(
      `PASS (${engineName}): the full Video detail player still seeks with arrow keys`
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
