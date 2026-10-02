import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

const engines = await import(process.env.PLAYWRIGHT_MODULE ?? "playwright");
const engineName = process.env.STASH_BROWSER_ENGINE ?? "chromium";
const baseURL = process.env.STASH_BROWSER_URL ?? "http://127.0.0.1:9999";
const clip = readFileSync(process.env.STASH_BROWSER_VIDEO_FILE);
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
const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
await page.addInitScript(() => {
  window.previewMediaHandlers = {};
  if (!navigator.mediaSession) return;
  const setActionHandler = navigator.mediaSession.setActionHandler.bind(
    navigator.mediaSession
  );
  navigator.mediaSession.setActionHandler = (action, handler) => {
    window.previewMediaHandlers[action] = handler;
    return setActionHandler(action, handler);
  };
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
    if (value.__typename === "Scene" && value.sceneStreams)
      value.sceneStreams = [
        {
          __typename: "SceneStreamEndpoint",
          url: `${baseURL}/browser-preview/stream-${value.id}.webm`,
          mime_type: "video/webm",
          label: "Browser VP8 fixture",
        },
      ];
    for (const child of Object.values(value)) visit(child);
  }
  visit(payload);
  await route.fulfill({ response, json: payload });
});
try {
  await page.goto(`${baseURL}/scenes/1`);
  await page.locator(".VideoPlayer .video-js").waitFor();
  await page.evaluate(async () => {
    window.detailPlayerElement = document.querySelector(
      ".VideoPlayer .video-js"
    );
    const player = window.detailPlayerElement.player;
    player.muted(true);
    player.loop(true);
    await player.play();
  });
  await page.waitForFunction(
    () => window.detailPlayerElement.player.currentTime() > 0
  );
  await page.evaluate(() =>
    window.dispatchEvent(
      new CustomEvent("unified-media:preview-scene", { detail: { id: "2" } })
    )
  );
  await page.locator(".unified-media-native-scene-player .video-js").waitFor();
  const ids = await page
    .locator(".VideoPlayer .video-js")
    .evaluateAll((nodes) => nodes.map((node) => node.id));
  console.log(`${engineName} simultaneous player IDs`, ids);
  assert.equal(ids.length, 2);
  assert.equal(
    new Set(ids).size,
    2,
    "Preview overwrote the detail player's VideoJS ID"
  );
  assert.equal(
    await page.evaluate(() => window.detailPlayerElement.player.paused()),
    true,
    "Underlying detail playback continued behind the viewer"
  );
  await page.getByTitle("Close Lightbox", { exact: true }).click();
  await page.locator(".Lightbox").waitFor({ state: "hidden" });
  assert.equal(
    await page.evaluate(
      () =>
        window.detailPlayerElement.isConnected &&
        !window.detailPlayerElement.player.isDisposed()
    ),
    true
  );
  await page.waitForFunction(() => !window.detailPlayerElement.player.paused());
  await page.evaluate(() => window.previewMediaHandlers.pause());
  assert.equal(
    await page.evaluate(() => window.detailPlayerElement.player.paused()),
    true
  );
  await page.evaluate(() => window.previewMediaHandlers.play());
  await page.waitForFunction(() => !window.detailPlayerElement.player.paused());
  assert.equal(await page.locator(".VideoPlayer .video-js").count(), 1);
  assert.deepEqual(errors, []);
  console.log(
    `PASS (${engineName}): a preview has its own VideoJS identity and yields/resumes the existing detail player without disposing it`
  );
} finally {
  await page.unrouteAll({ behavior: "ignoreErrors" });
  await browser.close();
}
