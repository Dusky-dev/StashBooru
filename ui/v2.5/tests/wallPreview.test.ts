import assert from "node:assert/strict";
import test from "node:test";
import {
  getFirstValidPreviewSource,
  getScenePreviewSources,
  getWallDimensions,
} from "../src/utils/wallPreview.ts";

test("default video wall playback and failed animations fall back to native screenshots", () => {
  const paths = {
    preview: "video.mp4",
    webp: "animated.webp",
    screenshot: "still.jpg",
  };
  assert.deepEqual(
    getFirstValidPreviewSource(getScenePreviewSources(paths), []),
    { src: "video.mp4", mediaType: "video" }
  );
  assert.deepEqual(
    getFirstValidPreviewSource(getScenePreviewSources(paths, "animation"), [
      "animated.webp",
    ]),
    { src: "still.jpg", mediaType: "image" }
  );
  assert.deepEqual(
    getFirstValidPreviewSource(getScenePreviewSources(paths, "image"), []),
    { src: "still.jpg", mediaType: "image" }
  );
});

test("missing previews and exhausted fallback sources always return a usable source shape", () => {
  assert.deepEqual(
    getFirstValidPreviewSource(
      getScenePreviewSources({ preview: null, screenshot: "still.jpg" }),
      []
    ),
    { src: "still.jpg", mediaType: "image" }
  );
  assert.deepEqual(getFirstValidPreviewSource(getScenePreviewSources({}), []), {
    src: "",
    mediaType: "image",
  });
  assert.deepEqual(
    getFirstValidPreviewSource(
      getScenePreviewSources({ preview: "video.mp4", screenshot: "still.jpg" }),
      ["video.mp4", "still.jpg"]
    ),
    { src: "still.jpg", mediaType: "image" }
  );
});

test("invalid file dimensions cannot reach gallery layout as zero, NaN or infinity", () => {
  for (const dimension of [0, -1, NaN, Infinity, null, undefined]) {
    assert.deepEqual(
      getWallDimensions({ width: dimension, height: dimension }),
      { width: 1, height: 1 }
    );
  }
  assert.deepEqual(getWallDimensions(undefined, { width: 1280, height: 720 }), {
    width: 1280,
    height: 720,
  });
  assert.deepEqual(getWallDimensions({ width: 1920, height: 1080 }), {
    width: 1920,
    height: 1080,
  });
});
