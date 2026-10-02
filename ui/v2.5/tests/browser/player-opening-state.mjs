import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

// Use an isolated HTTP fixture with Videos 1 and 2. Browser substitutions are
// read-only and cover native source/poster ownership and asynchronous thumbnails.
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

async function fixturePage(width = 1440) {
  const page = await browser.newPage({
    viewport: { width, height: width < 500 ? 844 : 1000 },
  });
  const errors = [];
  page.on("pageerror", (error) => errors.push(error.stack ?? error.message));
  await page.route("**/browser-opening/scene/*/stream", (route) => {
    const range = route
      .request()
      .headers()
      .range?.match(/bytes=(\d+)-(\d*)/);
    if (!range) return route.fulfill({ contentType: "video/webm", body: clip });
    const start = Number(range[1]);
    const end = Math.min(Number(range[2] || clip.length - 1), clip.length - 1);
    return route.fulfill({
      status: 206,
      contentType: "video/webm",
      headers: {
        "accept-ranges": "bytes",
        "content-range": `bytes ${start}-${end}/${clip.length}`,
      },
      body: clip.subarray(start, end + 1),
    });
  });
  await page.route("**/browser-opening/poster-*.svg", (route) =>
    route.fulfill({
      contentType: "image/svg+xml",
      body: '<svg xmlns="http://www.w3.org/2000/svg" width="320" height="180"><rect width="320" height="180" fill="#234"/></svg>',
    })
  );
  let delayedID;
  let release;
  let requested;
  let waiting;
  const delayScene = (id) => {
    delayedID = id;
    waiting = new Promise((resolve) => {
      release = resolve;
    });
    return new Promise((resolve) => {
      requested = resolve;
    });
  };
  await page.route("**/graphql", async (route) => {
    const request = route.request().postDataJSON();
    if (request.operationName === "SceneSaveActivity")
      return route.fulfill({ json: { data: { sceneSaveActivity: true } } });
    if (request.operationName === "SceneAddPlay")
      return route.fulfill({
        json: { data: { sceneAddPlay: { count: 1, history: [] } } },
      });
    const response = await route.fetch();
    const payload = await response.json();
    function visit(value) {
      if (!value || typeof value !== "object") return;
      if (value.__typename === "Scene" && value.paths) {
        value.paths.screenshot = `${baseURL}/browser-opening/poster-${value.id}.svg`;
        value.paths.vtt = null;
        value.resume_time = 0;
      }
      if (value.__typename === "Scene" && value.sceneStreams) {
        value.sceneStreams = [
          {
            __typename: "SceneStreamEndpoint",
            url: `${baseURL}/browser-opening/scene/${value.id}/stream`,
            mime_type: "video/webm",
            label: "Browser VP8 fixture",
          },
        ];
      }
      for (const child of Object.values(value)) visit(child);
    }
    visit(payload);
    if (
      request.operationName === "FindScene" &&
      request.variables.id === delayedID
    ) {
      requested();
      await waiting;
    }
    await route.fulfill({ response, json: payload });
  });
  await page.goto(`${baseURL}/scenes`);
  await page.waitForFunction(() => Boolean(window.PluginApi));
  await page.evaluate(() => {
    const api = window.PluginApi;
    window.openingRenders = [];
    function Probe({ id, children }) {
      window.openingHistory = api.libraries.ReactRouterDOM.useHistory();
      window.openingClient = api.libraries.Apollo.useApolloClient();
      window.openingRenders.push({
        id,
        path: window.openingHistory.location.pathname,
      });
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
  await page.locator('a.scene-card-link[href^="/scenes/1"]').first().click();
  await page.locator(".VideoPlayer .video-js").waitFor();
  await page.waitForFunction(() => Boolean(window.openingHistory));
  // Clicking a thumbnail can open the shared viewer. Close it before using the
  // native router to test the actual detail page beneath it.
  if (await page.locator(".Lightbox").count()) {
    await page.getByTitle("Close Lightbox", { exact: true }).click();
    await page.locator(".Lightbox").waitFor({ state: "hidden" });
  }
  await page.evaluate(() => window.openingHistory.push("/scenes/1"));
  await page
    .locator(".scene-player-container .VideoPlayer .video-js")
    .waitFor();
  return {
    page,
    errors,
    delayScene,
    release: () => {
      delayedID = undefined;
      release?.();
    },
  };
}

const chosen = process.env.STASH_BROWSER_OPENING_CASE;
try {
  if (!chosen || chosen === "route") {
    for (const width of [1440, 390]) {
      const fixture = await fixturePage(width);
      const { page, errors } = fixture;
      try {
        await page.evaluate(async () => {
          window.oldPlayer = document.querySelector(
            ".VideoPlayer .video-js"
          ).player;
          window.oldPlayer.muted(true);
          window.oldPlayer.loop(true);
          await window.oldPlayer.play();
        });
        await page.waitForFunction(() => window.oldPlayer.currentTime() > 0.2);
        const requested = fixture.delayScene("2");
        await page.evaluate(() => window.openingHistory.push("/scenes/2"));
        await requested;
        // The request starts during React's render; observe the committed UI.
        await page.evaluate(
          () =>
            new Promise((resolve) =>
              requestAnimationFrame(() => requestAnimationFrame(resolve))
            )
        );
        const duringLoad = await page.evaluate(() => ({
          path: location.pathname,
          sources: [...document.querySelectorAll(".VideoPlayer video")].map(
            (video) => video.currentSrc
          ),
          posters: [...document.querySelectorAll(".vjs-poster")].map(
            (poster) => getComputedStyle(poster).backgroundImage
          ),
          oldDisposed: window.oldPlayer.isDisposed(),
          renders: window.openingRenders.slice(-5),
          assets: performance
            .getEntriesByType("resource")
            .filter((entry) => /\/Scene-.*\.js$/.test(entry.name))
            .map((entry) => entry.name.split("/").at(-1)),
        }));
        console.log(`${engineName} ${width}px delayed route`, duringLoad);
        assert.equal(duringLoad.path, "/scenes/2");
        assert.deepEqual(
          duringLoad.sources,
          [],
          "Old Video remained visible on the new route"
        );
        assert.equal(duringLoad.oldDisposed, true);
        fixture.release();
        await page.waitForFunction(() => {
          const player = document.querySelector(
            ".VideoPlayer .video-js"
          )?.player;
          return player?.currentSource().src?.includes("/scene/2/stream");
        });
        // A cache-hit transition must also dispose the previous native player.
        await page.evaluate(() => {
          window.secondPlayer = document.querySelector(
            ".VideoPlayer .video-js"
          ).player;
          window.openingHistory.push("/scenes/1");
        });
        await page.waitForFunction(() => {
          const player = document.querySelector(
            ".VideoPlayer .video-js"
          )?.player;
          return player?.currentSource().src?.includes("/scene/1/stream");
        });
        assert.equal(
          await page.evaluate(() => window.secondPlayer.isDisposed()),
          true
        );
        assert.deepEqual(errors, []);
        console.log(
          `PASS (${engineName}, ${width}px): slow and cached detail transitions isolate sources/posters`
        );
      } finally {
        fixture.release();
        await page.unrouteAll({ behavior: "ignoreErrors" });
        await page.close();
      }
    }
  }
  if (!chosen || chosen === "config") {
    const { page, errors } = await fixturePage();
    try {
      for (const area of ["detail", "preview"]) {
        if (area === "preview") {
          await page.evaluate(() =>
            window.dispatchEvent(
              new CustomEvent("unified-media:preview-scene", {
                detail: { id: "2" },
              })
            )
          );
          await page
            .locator(".unified-media-native-scene-player .video-js")
            .waitFor();
        }
        await page.evaluate(
          async (selector) => {
            window.configSelector = selector;
            window.originalElement = document.querySelector(selector);
            const player = window.originalElement.player;
            player.muted(true);
            player.loop(true);
            await player.play();
          },
          area === "preview"
            ? ".unified-media-native-scene-player .video-js"
            : ".scene-player-container .video-js"
        );
        await page.waitForFunction(
          () => window.originalElement.player.currentTime() > 0.4
        );
        await page.evaluate(() => {
          const { cache } = window.openingClient;
          const query = window.PluginApi.GQL.ConfigurationDocument;
          const data = cache.readQuery({ query });
          window.enabledAutostart =
            !data.configuration.interface.autostartVideo;
          cache.writeQuery({
            query,
            data: {
              configuration: {
                ...data.configuration,
                interface: {
                  ...data.configuration.interface,
                  autostartVideo: window.enabledAutostart,
                },
              },
            },
          });
        });
        await page.waitForFunction(() => {
          const player = document.querySelector(window.configSelector)?.player;
          return (
            player?.autostartButton().getEnabled() === window.enabledAutostart
          );
        });
        assert.equal(
          await page.evaluate(() => window.originalElement.isConnected),
          true,
          "Autostart preference replaced the active native player"
        );
        assert.equal(
          await page.evaluate(
            () => window.originalElement.player.currentTime() > 0.4
          ),
          true
        );
        assert.deepEqual(errors, []);
        console.log(
          `PASS (${engineName}, ${area}): autostart updates in place without a source/player restart`
        );
      }
    } finally {
      await page.unrouteAll({ behavior: "ignoreErrors" });
      await page.close();
    }
  }
  if (!chosen || chosen === "sprites") {
    const { page, errors } = await fixturePage();
    let releaseA;
    let releaseC;
    const waiting = new Promise((resolve) => {
      releaseA = resolve;
    });
    const waitingC = new Promise((resolve) => {
      releaseC = resolve;
    });
    await page.route("**/browser-opening/*.vtt", async (route) => {
      const id = new URL(route.request().url()).pathname.split("/").at(-1)[0];
      if (id === "a") await waiting;
      if (id === "c") await waitingC;
      if (id === "e") return route.abort("failed");
      await route.fulfill({
        contentType: "text/vtt",
        body: `WEBVTT\n\n00:00:00.000 --> 00:00:01.000\n${id}.jpg#xywh=0,0,160,90\n\n`,
      });
    });
    try {
      const requestedA = page.waitForRequest("**/browser-opening/a.vtt");
      await page.evaluate((base) => {
        const api = window.PluginApi;
        window.spriteHost = document.createElement("div");
        document.body.appendChild(window.spriteHost);
        window.spriteSamples = [];
        function SpriteProbe({ path }) {
          const sprites = api.hooks.useSpriteInfo(path);
          window.spriteSamples.push({
            path,
            urls: sprites?.map((sprite) => sprite.url),
          });
          return api.React.createElement(
            "div",
            { id: "sprite-result" },
            sprites === null
              ? "missing"
              : sprites?.map((sprite) => sprite.url).join(",")
          );
        }
        window.setSpritePath = (id) =>
          api.ReactDOM.render(
            api.React.createElement(SpriteProbe, {
              path: id ? `${base}/browser-opening/${id}.vtt` : undefined,
            }),
            window.spriteHost
          );
        window.setSpritePath("a");
      }, baseURL);
      await requestedA;
      await page.evaluate(() => window.setSpritePath("b"));
      await page.waitForFunction(() =>
        document.querySelector("#sprite-result").textContent.endsWith("b.jpg")
      );
      releaseA();
      await page.waitForTimeout(250);
      assert.equal(
        await page.locator("#sprite-result").textContent(),
        `${baseURL}/browser-opening/b.jpg`,
        "A late VTT response replaced the current Video's thumbnails"
      );
      await page.evaluate(() => window.setSpritePath(undefined));
      assert.equal(await page.locator("#sprite-result").textContent(), "");
      await page.evaluate(() => window.setSpritePath("e"));
      await page.waitForFunction(
        () => document.querySelector("#sprite-result").textContent === "missing"
      );
      // Exercise the actual detail scrubber on a same-ID metadata refresh.
      // Its rendered items must clear with the source, including a failed VTT.
      if (
        !(await page
          .locator(".scene-player-container .scrubber-wrapper")
          .count())
      )
        await page.keyboard.press(".");
      await page.locator(".scene-player-container .scrubber-wrapper").waitFor();
      await page.evaluate((base) => {
        window.setNativeVtt = (id) => {
          const { cache } = window.openingClient;
          const query = window.PluginApi.GQL.FindSceneDocument;
          const variables = { id: "1" };
          const data = cache.readQuery({ query, variables });
          cache.writeQuery({
            query,
            variables,
            data: {
              findScene: {
                ...data.findScene,
                paths: {
                  ...data.findScene.paths,
                  vtt: `${base}/browser-opening/${id}.vtt`,
                },
              },
            },
          });
        };
        window.setNativeVtt("d");
      }, baseURL);
      await page.waitForFunction(() =>
        document
          .querySelector(".scrubber-item")
          ?.style.backgroundImage.includes("d.jpg")
      );
      const requestedC = page.waitForRequest("**/browser-opening/c.vtt");
      await page.evaluate(() => window.setNativeVtt("c"));
      await requestedC;
      assert.equal(
        await page.locator(".scrubber-item").count(),
        0,
        "The detail scrubber retained old thumbnails while the new VTT loaded"
      );
      releaseC();
      await page.waitForFunction(() =>
        document
          .querySelector(".scrubber-item")
          ?.style.backgroundImage.includes("c.jpg")
      );
      await page.evaluate(() => window.setNativeVtt("e"));
      await page.waitForFunction(
        () => !document.querySelector(".scrubber-item")
      );
      assert.deepEqual(errors, []);
      console.log(
        `PASS (${engineName}): late, cleared and failed VTT loads cannot expose unrelated thumbnails`
      );
    } finally {
      releaseA();
      releaseC();
      await page.evaluate(() => {
        window.PluginApi.ReactDOM.unmountComponentAtNode(window.spriteHost);
        window.spriteHost.remove();
      });
      await page.unrouteAll({ behavior: "ignoreErrors" });
      await page.close();
    }
  }
} finally {
  await browser.close();
}
