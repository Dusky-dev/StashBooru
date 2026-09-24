export interface DifferenceBox {
  x: number;
  y: number;
  width: number;
  height: number;
  pixels: number;
  maxDelta: number;
}

export interface ImageDifferenceOptions {
  threshold: number;
  minRegionArea: number;
  boxPadding?: number;
  mergeGap?: number;
}

export interface ImageDifferenceResult {
  heatmap: Uint8ClampedArray;
  boxes: DifferenceBox[];
  changedPixels: number;
  totalPixels: number;
  meanDelta: number;
  maxDelta: number;
}

export interface ComparisonRect {
  x: number;
  y: number;
  width: number;
  height: number;
}

export interface ComparisonGeometry {
  width: number;
  height: number;
  reference: ComparisonRect;
  selected: ComparisonRect;
}

const DEFAULT_MAX_PIXELS = 1_500_000;
const DEFAULT_MAX_DIMENSION = 2048;

function clamp(value: number, min: number, max: number) {
  return Math.max(min, Math.min(max, value));
}

function fitRect(
  sourceWidth: number,
  sourceHeight: number,
  targetWidth: number,
  targetHeight: number
): ComparisonRect {
  const scale = Math.min(
    targetWidth / sourceWidth,
    targetHeight / sourceHeight
  );
  const width = sourceWidth * scale;
  const height = sourceHeight * scale;
  return {
    x: (targetWidth - width) / 2,
    y: (targetHeight - height) / 2,
    width,
    height,
  };
}

export function createComparisonGeometry(
  referenceWidth: number,
  referenceHeight: number,
  selectedWidth: number,
  selectedHeight: number,
  maxPixels = DEFAULT_MAX_PIXELS,
  maxDimension = DEFAULT_MAX_DIMENSION
): ComparisonGeometry {
  const dimensions = [
    referenceWidth,
    referenceHeight,
    selectedWidth,
    selectedHeight,
  ];
  if (dimensions.some((value) => !Number.isFinite(value) || value <= 0)) {
    throw new Error("comparison images must have positive dimensions");
  }

  const logicalWidth = Math.max(referenceWidth, selectedWidth);
  const logicalHeight = Math.max(referenceHeight, selectedHeight);
  const scale = Math.min(
    1,
    maxDimension / logicalWidth,
    maxDimension / logicalHeight,
    Math.sqrt(maxPixels / (logicalWidth * logicalHeight))
  );
  const width = Math.max(1, Math.round(logicalWidth * scale));
  const height = Math.max(1, Math.round(logicalHeight * scale));

  return {
    width,
    height,
    reference: fitRect(referenceWidth, referenceHeight, width, height),
    selected: fitRect(selectedWidth, selectedHeight, width, height),
  };
}

function boxesTouch(a: DifferenceBox, b: DifferenceBox, gap: number) {
  return !(
    a.x + a.width + gap < b.x ||
    b.x + b.width + gap < a.x ||
    a.y + a.height + gap < b.y ||
    b.y + b.height + gap < a.y
  );
}

function unionBoxes(a: DifferenceBox, b: DifferenceBox): DifferenceBox {
  const x = Math.min(a.x, b.x);
  const y = Math.min(a.y, b.y);
  const right = Math.max(a.x + a.width, b.x + b.width);
  const bottom = Math.max(a.y + a.height, b.y + b.height);
  return {
    x,
    y,
    width: right - x,
    height: bottom - y,
    pixels: a.pixels + b.pixels,
    maxDelta: Math.max(a.maxDelta, b.maxDelta),
  };
}

function mergeBoxes(boxes: DifferenceBox[], gap: number) {
  const merged = [...boxes];
  let changed = true;
  while (changed) {
    changed = false;
    outer: for (let i = 0; i < merged.length; i++) {
      for (let j = i + 1; j < merged.length; j++) {
        if (!boxesTouch(merged[i], merged[j], gap)) continue;
        merged[i] = unionBoxes(merged[i], merged[j]);
        merged.splice(j, 1);
        changed = true;
        break outer;
      }
    }
  }
  return merged;
}

export function computeImageDifference(
  reference: Uint8ClampedArray,
  selected: Uint8ClampedArray,
  width: number,
  height: number,
  options: ImageDifferenceOptions
): ImageDifferenceResult {
  if (!Number.isInteger(width) || !Number.isInteger(height) || width <= 0 || height <= 0) {
    throw new Error("comparison canvas must have positive integer dimensions");
  }

  const totalPixels = width * height;
  const expectedLength = totalPixels * 4;
  if (reference.length !== expectedLength || selected.length !== expectedLength) {
    throw new Error("comparison pixel buffers do not match canvas dimensions");
  }

  const threshold = clamp(Math.round(options.threshold), 0, 255);
  const minRegionArea = Math.max(1, Math.round(options.minRegionArea));
  const padding = Math.max(0, Math.round(options.boxPadding ?? 2));
  const mergeGap = Math.max(0, Math.round(options.mergeGap ?? 4));

  const deltas = new Uint8Array(totalPixels);
  const changed = new Uint8Array(totalPixels);

  for (let pixel = 0; pixel < totalPixels; pixel++) {
    const offset = pixel * 4;
    const delta = Math.max(
      Math.abs(reference[offset] - selected[offset]),
      Math.abs(reference[offset + 1] - selected[offset + 1]),
      Math.abs(reference[offset + 2] - selected[offset + 2]),
      Math.abs(reference[offset + 3] - selected[offset + 3])
    );
    deltas[pixel] = delta;
    if (delta > threshold) changed[pixel] = 1;
  }

  const visited = new Uint8Array(totalPixels);
  const queue = new Int32Array(totalPixels);
  const boxes: DifferenceBox[] = [];

  for (let start = 0; start < totalPixels; start++) {
    if (!changed[start] || visited[start]) continue;

    let head = 0;
    let tail = 0;
    queue[tail++] = start;
    visited[start] = 1;

    let minX = width;
    let minY = height;
    let maxX = 0;
    let maxY = 0;
    let maxDelta = 0;

    while (head < tail) {
      const pixel = queue[head++];
      const x = pixel % width;
      const y = Math.floor(pixel / width);
      minX = Math.min(minX, x);
      minY = Math.min(minY, y);
      maxX = Math.max(maxX, x);
      maxY = Math.max(maxY, y);
      maxDelta = Math.max(maxDelta, deltas[pixel]);

      const neighbors = [
        x > 0 ? pixel - 1 : -1,
        x + 1 < width ? pixel + 1 : -1,
        y > 0 ? pixel - width : -1,
        y + 1 < height ? pixel + width : -1,
      ];
      for (const neighbor of neighbors) {
        if (neighbor < 0 || visited[neighbor] || !changed[neighbor]) continue;
        visited[neighbor] = 1;
        queue[tail++] = neighbor;
      }
    }

    if (tail < minRegionArea) {
      for (let i = 0; i < tail; i++) changed[queue[i]] = 0;
      continue;
    }

    const x = Math.max(0, minX - padding);
    const y = Math.max(0, minY - padding);
    const right = Math.min(width, maxX + 1 + padding);
    const bottom = Math.min(height, maxY + 1 + padding);
    boxes.push({
      x,
      y,
      width: right - x,
      height: bottom - y,
      pixels: tail,
      maxDelta,
    });
  }

  const heatmap = new Uint8ClampedArray(expectedLength);
  let changedPixels = 0;
  let deltaSum = 0;
  let maxDelta = 0;

  for (let pixel = 0; pixel < totalPixels; pixel++) {
    if (!changed[pixel]) continue;
    const delta = deltas[pixel];
    const offset = pixel * 4;
    heatmap[offset] = 255;
    heatmap[offset + 1] = Math.max(0, 255 - delta * 2);
    heatmap[offset + 2] = 0;
    heatmap[offset + 3] = Math.max(96, delta);
    changedPixels++;
    deltaSum += delta;
    maxDelta = Math.max(maxDelta, delta);
  }

  return {
    heatmap,
    boxes: mergeBoxes(boxes, mergeGap),
    changedPixels,
    totalPixels,
    meanDelta: changedPixels > 0 ? deltaSum / changedPixels : 0,
    maxDelta,
  };
}
