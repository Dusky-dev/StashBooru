// Paste into the browser console BEFORE opening a buggy preview. Reproduce
// once, then stashPreviewDebug.download(). No network writes.
(() => {
  window.stashPreviewDebug?.stop();
  const started = performance.now();
  const events = [];
  const cleanups = [];
  const identities = new WeakMap();
  const sourceIdentities = new Map();
  let nextIdentity = 1;
  let dropped = 0;
  let stopped = false;
  let lastView = "";
  let report;

  const rounded = (value) =>
    Number.isFinite(value) ? Math.round(value * 1000) / 1000 : null;
  function mediaPath(value) {
    if (!value) return null;
    try {
      const path = new URL(value, location.href).pathname;
      if (
        /^\/(?:media|scenes|images|performers|studios|copyrights|tags)(?:\/\d+)?(?:\/(?:scenes|videos|images|all|galleries|variants))?\/?$/.test(
          path
        )
      )
        return path;
      return (
        path.match(
          /^\/(?:scene|scenes|images|performers)\/\d+(?:\/(?:preview|stream|screenshot))?$/
        )?.[0] ?? "other"
      );
    } catch {
      return "other";
    }
  }
  function record(kind, data) {
    if (stopped) return;
    if (events.length === 2000) {
      events.shift();
      dropped++;
    }
    events.push({ ms: rounded(performance.now() - started), kind, ...data });
  }
  function videoState(video) {
    if (!identities.has(video)) identities.set(video, nextIdentity++);
    const host = video.closest(".unified-media-native-scene-player");
    const rect = video.getBoundingClientRect();
    const style = getComputedStyle(video);
    const source = video.currentSrc || video.src;
    if (source && !sourceIdentities.has(source) && sourceIdentities.size < 2000)
      sourceIdentities.set(source, sourceIdentities.size + 1);
    let playerTime = null;
    try {
      playerTime = video.closest(".video-js")?.player?.currentTime();
    } catch {
      // A player being disposed may no longer expose its logical stream time.
    }
    return {
      node: identities.get(video),
      player: video.closest(".video-js")?.id ?? null,
      scene: host?.dataset.sceneId ?? null,
      area: video.closest(".Lightbox")
        ? "lightbox"
        : video.closest(".scene-card")
          ? "card"
          : video.closest(".scene-wall, .media-wall, .unified-media-wall")
            ? "wall"
            : "other",
      source: mediaPath(source),
      sourceRevision: sourceIdentities.get(source) ?? null,
      connected: video.isConnected,
      time: rounded(video.currentTime),
      playerTime: rounded(playerTime),
      duration: rounded(video.duration),
      paused: video.paused,
      ready: video.readyState,
      network: video.networkState,
      error: video.error?.code ?? null,
      rate: video.playbackRate,
      loop: video.loop,
      opacity: style.opacity,
      visibility: style.visibility,
      display: style.display,
      width: rounded(rect.width),
      height: rounded(rect.height),
      decoded: video.getVideoPlaybackQuality?.().totalVideoFrames ?? null,
      droppedFrames:
        video.getVideoPlaybackQuality?.().droppedVideoFrames ?? null,
    };
  }
  function viewState() {
    const box = document.querySelector(".Lightbox");
    const carousel = box?.querySelector(".Lightbox-carousel");
    return {
      visible: Boolean(box),
      scenes: [
        ...document.querySelectorAll(".unified-media-native-scene-player"),
      ].map((node) => node.dataset.sceneId),
      players: [...document.querySelectorAll(".VideoPlayer .video-js")].map(
        (node) => node.id
      ),
      footer: mediaPath(
        box?.querySelector(".image-link")?.getAttribute("href") ?? ""
      ),
      transform: carousel ? getComputedStyle(carousel).transform : null,
    };
  }
  function observeView() {
    const view = viewState();
    const signature = JSON.stringify(view);
    if (signature !== lastView) {
      lastView = signature;
      record("viewer", view);
    }
  }
  function listen(target, name, handler) {
    target.addEventListener(name, handler, true);
    cleanups.push(() => target.removeEventListener(name, handler, true));
  }
  for (const name of [
    "loadstart",
    "loadedmetadata",
    "loadeddata",
    "play",
    "playing",
    "pause",
    "waiting",
    "stalled",
    "seeking",
    "seeked",
    "ended",
    "emptied",
    "abort",
    "error",
  ]) {
    listen(document, name, (event) => {
      if (event.target instanceof HTMLVideoElement)
        record(name, videoState(event.target));
    });
  }
  for (const name of [
    "keydown",
    "click",
    "wheel",
    "pointerover",
    "pointerout",
  ]) {
    listen(document, name, (event) => {
      if (!(event.target instanceof Element)) return;
      const relevant = event.target.closest(
        ".Lightbox, .scene-card, .scene-wall, .media-wall, .unified-media-wall"
      );
      if (!relevant && name !== "keydown") return;
      if (
        name === "keydown" &&
        !["ArrowLeft", "ArrowRight", "Escape", " "].includes(event.key)
      )
        return;
      const cardLink = event.target
        .closest(".scene-card")
        ?.querySelector("a.scene-card-link");
      record("input", {
        event: name,
        key: name === "keydown" ? event.key : null,
        repeat: Boolean(event.repeat),
        trusted: event.isTrusted,
        wheelY: name === "wheel" ? rounded(event.deltaY) : null,
        area: event.target.closest(".Lightbox") ? "lightbox" : "background",
        scene: cardLink ? mediaPath(cardLink.getAttribute("href")) : null,
      });
    });
  }
  listen(document, "visibilitychange", () =>
    record("visibility", { state: document.visibilityState })
  );
  const observer = new MutationObserver((records) => {
    for (const mutation of records) {
      for (const [kind, nodes] of [
        ["video-added", mutation.addedNodes],
        ["video-removed", mutation.removedNodes],
      ]) {
        for (const node of nodes) {
          if (!(node instanceof Element)) continue;
          const videos = node.matches("video")
            ? [node]
            : [...node.querySelectorAll("video")];
          for (const video of videos) record(kind, videoState(video));
        }
      }
      if (
        mutation.type === "attributes" &&
        mutation.target instanceof HTMLVideoElement
      )
        record("video-attribute", videoState(mutation.target));
    }
    observeView();
  });
  observer.observe(document.body, {
    childList: true,
    subtree: true,
    attributes: true,
    attributeFilter: ["src", "data-scene-id", "id", "class"],
  });
  cleanups.push(() => observer.disconnect());
  try {
    const resources = new PerformanceObserver((list) => {
      for (const entry of list.getEntries()) {
        const path = mediaPath(entry.name);
        if (!path?.startsWith("/scene/")) continue;
        record("resource", {
          path,
          duration: rounded(entry.duration),
          status: entry.responseStatus ?? null,
        });
      }
    });
    resources.observe({ type: "resource" });
    cleanups.push(() => resources.disconnect());
  } catch {
    // Older browsers may not expose resource observation; DOM/media still work.
  }
  const timer = setInterval(() => {
    observeView();
    for (const video of document.querySelectorAll(
      ".Lightbox video, .VideoPlayer video"
    ))
      record("sample", videoState(video));
  }, 250);
  cleanups.push(() => clearInterval(timer));
  window.stashPreviewDebug = {
    download() {
      const contents = JSON.stringify(this.stop(), null, 2);
      const url = URL.createObjectURL(
        new Blob([contents], { type: "application/json" })
      );
      const link = document.createElement("a");
      link.href = url;
      link.download = "stashbooru-preview-debug.json";
      document.body.append(link);
      link.click();
      link.remove();
      setTimeout(() => URL.revokeObjectURL(url), 1000);
    },
    stop() {
      if (report) return report;
      observeView();
      record("stop", {});
      stopped = true;
      for (const cleanup of cleanups) cleanup();
      sourceIdentities.clear();
      report = {
        format: "stashbooru-preview-v1",
        browser: navigator.userAgent,
        page: mediaPath(location.href),
        viewport: {
          width: innerWidth,
          height: innerHeight,
          ratio: devicePixelRatio,
        },
        assets: [
          ...new Set(
            [
              ...[...document.querySelectorAll("script[src]")].map(
                (script) => script.src
              ),
              ...performance
                .getEntriesByType("resource")
                .filter((entry) => new URL(entry.name).pathname.endsWith(".js"))
                .map((entry) => entry.name),
            ].map((value) => new URL(value).pathname.split("/").at(-1))
          ),
        ],
        durationMs: rounded(performance.now() - started),
        dropped,
        events,
      };
      return report;
    },
  };
  const limit = setTimeout(() => window.stashPreviewDebug.stop(), 60000);
  cleanups.push(() => clearTimeout(limit));
  observeView();
  for (const video of document.querySelectorAll("video"))
    record("initial", videoState(video));
  console.info(
    "Preview recorder active for up to 60 seconds. Reproduce once, then stashPreviewDebug.download()."
  );
})();
