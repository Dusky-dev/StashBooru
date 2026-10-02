import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

// Read-only browser cache updates reproduce card hydration and background
// metadata refreshes. They must never navigate an already open preview.
const { chromium } = await import(
  process.env.PLAYWRIGHT_MODULE ?? "playwright"
);
const baseURL = process.env.STASH_BROWSER_URL ?? "http://127.0.0.1:9999";
const characterID = process.env.STASH_BROWSER_CHARACTER_ID;
const videoFixture = process.env.STASH_BROWSER_VIDEO_FILE
  ? readFileSync(process.env.STASH_BROWSER_VIDEO_FILE)
  : null;
assert.ok(characterID, "Set STASH_BROWSER_CHARACTER_ID to an isolated fixture");
const browser = await chromium.launch({
  headless: true,
  executablePath: process.env.CHROMIUM_EXECUTABLE,
  args: ["--no-sandbox", "--disable-dev-shm-usage"],
});
try {
  for (const [path, selector, scoped, width] of [
    ["/media", ".media-list .scene-card", false, 1440],
    ["/scenes", ".scene-card", false, 390],
    [
      `/performers/${characterID}`,
      ".unified-media-inline-pane .scene-card",
      true,
      1440,
    ],
  ]) {
    const page = await browser.newPage({
      viewport: { width: 1440, height: 1000 },
    });
    const errors = [];
    page.on("pageerror", (error) => errors.push(error.message));
    if (videoFixture) {
      await page.route("**/browser-preview/scenes/*/stream", (route) =>
        route.fulfill({ contentType: "video/webm", body: videoFixture })
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
        function replaceStreams(value) {
          if (!value || typeof value !== "object") return;
          if (value.__typename === "Scene" && "sceneStreams" in value) {
            value.sceneStreams = [
              {
                __typename: "SceneStreamEndpoint",
                url: `${baseURL}/browser-preview/scenes/${value.id}/stream`,
                mime_type: "video/webm",
                label: "Browser VP8 fixture",
              },
            ];
          }
          if (value.__typename === "Scene" && "files" in value) {
            value.files = [
              {
                __typename: "VideoFile",
                id: `browser-video-${value.id}`,
                path: `/browser-preview/${value.id}.webm`,
                size: videoFixture.length,
                frame_count: 48,
                mod_time: "2026-01-01T00:00:00Z",
                duration: 4,
                video_codec: "vp8",
                audio_codec: "",
                width: 320,
                height: 180,
                frame_rate: 12,
                bit_rate: 160000,
                fingerprints: [],
              },
            ];
          }
          for (const child of Object.values(value)) replaceStreams(child);
        }
        replaceStreams(payload.data);
        await route.fulfill({ response, json: payload });
      });
    }
    await page.goto(`${baseURL}${path}`);
    await page.waitForFunction(() => Boolean(window.PluginApi));
    await page.evaluate(() => {
      const api = window.PluginApi;
      function CacheProbe() {
        window.previewQueueClient = api.libraries.Apollo.useApolloClient();
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
      api.patch.after("ScenePlayer", (...args) => {
        window.previewQueuePlayerID = String(args[0].scene.id);
        return args.at(-1);
      });
      window.previewQueueEvents = [];
      window.addEventListener("unified-media:scene-registry", (event) => {
        window.previewQueueEvents.push({
          action: event.detail.action,
          id: event.detail.scene?.id,
          token: event.detail.token,
        });
      });
    });
    await page.setViewportSize({ width: 390, height: 844 });
    await page.locator(".nav-menu-toggle").click();
    await page.setViewportSize({ width, height: width === 390 ? 844 : 1000 });
    if (width === 390) await page.locator(".nav-menu-toggle").click();
    await page.waitForFunction(() => Boolean(window.previewQueueClient));
    if (scoped)
      await page
        .locator('.performer-tabs [data-unified-media-all="tab"]')
        .click();
    const cards = page.locator(selector);
    await cards.nth(1).waitFor();
    if (scoped) {
      await page
        .locator(".unified-media-controller .sort-by-select .dropdown-toggle")
        .click();
      await page
        .locator(
          '.unified-media-controller .sort-by-option[data-value="title"]'
        )
        .click();
      await cards.nth(1).waitFor();
    }
    if (!scoped && width === 1440) {
      const initialPaths = await cards
        .locator("a.scene-card-link")
        .evaluateAll((links) =>
          links.map((link) => new URL(link.href).pathname)
        );
      const sort = page.locator(".main .sort-by-select");
      await sort.locator(".dropdown-toggle").click();
      await sort.locator('.sort-by-option[data-value="title"]').click();
      await sort.locator(":scope > button").click();
      await page.waitForFunction(
        ({ selector, initialPaths }) => {
          const paths = [
            ...document.querySelectorAll(`${selector} a.scene-card-link`),
          ].map((link) => new URL(link.href).pathname);
          return (
            paths.length > 1 &&
            JSON.stringify(paths) !== JSON.stringify(initialPaths)
          );
        },
        { selector, initialPaths }
      );
    }
    const paths = await cards
      .locator("a.scene-card-link")
      .evaluateAll((links) => links.map((link) => new URL(link.href).pathname));
    const selected = cards.first();
    const selectedPath = paths[0];
    await selected.hover();
    await selected
      .locator(".unified-media-video-preview-button button")
      .click();
    await page
      .locator(".unified-media-native-scene-player .video-js")
      .waitFor();
    await page.waitForFunction(
      (path) =>
        document
          .querySelector(".Lightbox-footer-center .image-link")
          ?.getAttribute("href") === path,
      selectedPath
    );
    if (videoFixture) {
      await page.waitForTimeout(1000);
      console.log(
        "Playback fixture state",
        path,
        await page
          .locator(".unified-media-native-scene-player video")
          .evaluate((video) => ({
            src: video.currentSrc,
            ready: video.readyState,
            paused: video.paused,
            time: video.currentTime,
            error: video.error?.message,
          }))
      );
      await page.waitForFunction(() => {
        const video = document.querySelector(
          ".unified-media-native-scene-player video"
        );
        return (
          video?.readyState >= 2 && video.currentTime > 0.1 && !video.paused
        );
      });
    }
    await page.evaluate((selector) => {
      window.previewQueueSamples = [];
      window.previewQueueTimer = setInterval(() => {
        window.previewQueueSamples.push({
          href: document
            .querySelector(".Lightbox-footer-center .image-link")
            ?.getAttribute("href"),
          playerID: window.previewQueuePlayerID,
          cardPaths: [
            ...document.querySelectorAll(`${selector} a.scene-card-link`),
          ].map((link) => new URL(link.href).pathname),
          players: document.querySelectorAll(
            ".unified-media-native-scene-player"
          ).length,
        });
      }, 25);
    }, selector);
    for (let revision = 0; revision < 12; revision++) {
      await page.evaluate(
        ({ id, revision }) => {
          const api = window.PluginApi;
          window.previewQueueClient.writeFragment({
            id: `Scene:${id}`,
            fragment: api.libraries.Apollo
              .gql`fragment PreviewMetadata on Scene { id title date updated_at resume_time }`,
            data: {
              __typename: "Scene",
              id,
              title: `${revision % 2 ? "A" : "Z"} refreshed Video ${revision}`,
              date: `${2050 + revision}-01-01`,
              updated_at: new Date(
                1700000000000 + revision * 1000
              ).toISOString(),
              resume_time: revision,
            },
          });
        },
        { id: paths[revision % paths.length].split("/").at(-1), revision }
      );
      await page.waitForTimeout(100);
    }
    await page.waitForTimeout(6500);
    const observations = await page.evaluate(() => {
      clearInterval(window.previewQueueTimer);
      return {
        samples: window.previewQueueSamples,
        registrations: window.previewQueueEvents,
      };
    });
    console.log(
      "Metadata refresh observations",
      path,
      JSON.stringify({
        hrefs: [...new Set(observations.samples.map((sample) => sample.href))],
        playerIDs: [
          ...new Set(observations.samples.map((sample) => sample.playerID)),
        ],
        players: [
          ...new Set(observations.samples.map((sample) => sample.players)),
        ],
        samples: observations.samples.length,
        registrations: observations.registrations.length,
      })
    );
    if (scoped) {
      assert.ok(
        observations.samples.some(
          (sample) => JSON.stringify(sample.cardPaths) !== JSON.stringify(paths)
        ),
        "Fixture must reorder the underlying scoped list while the preview remains open"
      );
    }
    if (videoFixture) {
      assert.equal(
        await page
          .locator(".unified-media-native-scene-player video")
          .evaluate((video) => video.ended && video.currentTime > 3.9),
        true,
        `${path}: the VP8 fixture did not finish playing`
      );
    }
    const next = page.locator(".Lightbox-display > .Lightbox-navbutton").last();
    await page.getByTitle("Close Lightbox", { exact: true }).focus();
    if (width === 390) await page.keyboard.press("ArrowRight");
    else await next.click();
    if (!scoped) {
      await page.waitForFunction(
        (id) => window.previewQueuePlayerID === id,
        paths[1].split("/").at(-1)
      );
      await page.waitForTimeout(500);
      assert.equal(
        await page.evaluate(() => window.previewQueuePlayerID),
        paths[1].split("/").at(-1),
        `${path}: explicit Next did not keep its selected Video`
      );
    }
    await page.getByTitle("Close Lightbox", { exact: true }).focus();
    if (width === 390) await page.keyboard.press("ArrowLeft");
    else
      await page
        .locator(".Lightbox-display > .Lightbox-navbutton")
        .first()
        .click();
    await page.waitForFunction(
      (id) => window.previewQueuePlayerID === id,
      selectedPath.split("/").at(-1)
    );
    await page.waitForTimeout(500);
    assert.equal(
      await page.evaluate(() => window.previewQueuePlayerID),
      selectedPath.split("/").at(-1)
    );
    assert.ok(
      observations.samples.every(
        (sample) =>
          sample.href === selectedPath &&
          sample.playerID === selectedPath.split("/").at(-1) &&
          sample.players === 1
      ),
      `${path}: refreshing Video metadata changed the selected preview`
    );
    assert.deepEqual(errors, []);
    await page.getByTitle("Close Lightbox", { exact: true }).click();
    await page.locator(".Lightbox").waitFor({ state: "hidden" });
    const reopenedLink = await cards
      .last()
      .locator("a.scene-card-link")
      .getAttribute("href");
    const reopenedPath = new URL(reopenedLink, baseURL).pathname;
    await cards.last().hover();
    await cards
      .last()
      .locator(".unified-media-video-preview-button button")
      .click();
    await page.locator(".Lightbox").waitFor();
    console.log(
      "Reopen observation",
      path,
      await page.evaluate(() => ({
        href: document
          .querySelector(".Lightbox-footer-center .image-link")
          ?.getAttribute("href"),
        playerID: window.previewQueuePlayerID,
      })),
      "expected",
      reopenedPath
    );
    await page.waitForFunction(
      (id) => window.previewQueuePlayerID === id,
      reopenedPath.split("/").at(-1)
    );
    await page.keyboard.press("Escape");
    await page.locator(".Lightbox").waitFor({ state: "hidden" });
    assert.equal(
      await page.locator(".unified-media-native-scene-player").count(),
      0
    );
    await page.close();
  }
  console.log(
    "PASS: Video previews keep their selected identity through metadata refreshes, idle, explicit navigation and reopening on All, mobile Videos and reordered Character All"
  );
  if (videoFixture)
    console.log(
      "PASS: native ScenePlayer decoded and played the VP8 fixture to completion without advancing"
    );
} finally {
  await browser.close();
}
