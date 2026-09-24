import assert from "node:assert/strict";
import test from "node:test";

import {
  computeImageDifference,
  createComparisonGeometry,
} from "../src/hooks/Lightbox/imageDifference.ts";

function pixels(width: number, height: number, value = 0) {
  const data = new Uint8ClampedArray(width * height * 4);
  for (let i = 0; i < data.length; i += 4) {
    data[i] = value;
    data[i + 1] = value;
    data[i + 2] = value;
    data[i + 3] = 255;
  }
  return data;
}

function setPixel(
  data: Uint8ClampedArray,
  width: number,
  x: number,
  y: number,
  value: number
) {
  const offset = (y * width + x) * 4;
  data[offset] = value;
  data[offset + 1] = value;
  data[offset + 2] = value;
}

test("identical pixels produce no difference regions", () => {
  const reference = pixels(4, 4, 32);
  const result = computeImageDifference(reference, reference.slice(), 4, 4, {
    threshold: 0,
    minRegionArea: 1,
  });

  assert.equal(result.changedPixels, 0);
  assert.equal(result.maxDelta, 0);
  assert.deepEqual(result.boxes, []);
  assert.ok(result.heatmap.every((value) => value === 0));
});

test("connected changed pixels become one bounded region", () => {
  const reference = pixels(5, 5);
  const selected = reference.slice();
  setPixel(selected, 5, 1, 2, 120);
  setPixel(selected, 5, 2, 2, 120);
  setPixel(selected, 5, 1, 3, 120);
  setPixel(selected, 5, 2, 3, 120);

  const result = computeImageDifference(reference, selected, 5, 5, {
    threshold: 10,
    minRegionArea: 1,
    boxPadding: 0,
    mergeGap: 0,
  });

  assert.equal(result.changedPixels, 4);
  assert.equal(result.boxes.length, 1);
  assert.deepEqual(result.boxes[0], {
    x: 1,
    y: 2,
    width: 2,
    height: 2,
    pixels: 4,
    maxDelta: 120,
  });
});

test("noise suppression removes isolated regions below the configured area", () => {
  const reference = pixels(6, 4);
  const selected = reference.slice();
  setPixel(selected, 6, 0, 0, 255);
  setPixel(selected, 6, 3, 1, 80);
  setPixel(selected, 6, 4, 1, 80);
  setPixel(selected, 6, 3, 2, 80);
  setPixel(selected, 6, 4, 2, 80);

  const result = computeImageDifference(reference, selected, 6, 4, {
    threshold: 10,
    minRegionArea: 2,
    boxPadding: 0,
    mergeGap: 0,
  });

  assert.equal(result.changedPixels, 4);
  assert.equal(result.boxes.length, 1);
  assert.equal(result.heatmap[3], 0);
});

test("nearby boxes merge after padding/gap while preserving changed-pixel counts", () => {
  const reference = pixels(8, 3);
  const selected = reference.slice();
  setPixel(selected, 8, 1, 1, 100);
  setPixel(selected, 8, 4, 1, 100);

  const result = computeImageDifference(reference, selected, 8, 3, {
    threshold: 5,
    minRegionArea: 1,
    boxPadding: 0,
    mergeGap: 2,
  });

  assert.equal(result.changedPixels, 2);
  assert.equal(result.boxes.length, 1);
  assert.equal(result.boxes[0].pixels, 2);
  assert.deepEqual(
    {
      x: result.boxes[0].x,
      y: result.boxes[0].y,
      width: result.boxes[0].width,
      height: result.boxes[0].height,
    },
    { x: 1, y: 1, width: 4, height: 1 }
  );
});

test("same-aspect images with different resolutions align to the same normalized rect", () => {
  const geometry = createComparisonGeometry(1000, 500, 2000, 1000);
  assert.deepEqual(geometry.reference, geometry.selected);
  assert.ok(geometry.width <= 2048);
  assert.ok(geometry.width * geometry.height <= 1_500_000);
});

test("different aspect ratios are centered without stretching", () => {
  const geometry = createComparisonGeometry(1600, 900, 900, 1600, 1_500_000);

  const referenceRatio = geometry.reference.width / geometry.reference.height;
  const selectedRatio = geometry.selected.width / geometry.selected.height;
  assert.ok(Math.abs(referenceRatio - 1600 / 900) < 1e-9);
  assert.ok(Math.abs(selectedRatio - 900 / 1600) < 1e-9);
  assert.ok(geometry.reference.y > 0);
  assert.ok(geometry.selected.x > 0);
});
