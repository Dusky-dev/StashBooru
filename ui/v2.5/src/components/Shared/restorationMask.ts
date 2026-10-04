// Finite, integer box feathering in source-pixel coordinates. Matches Go Feather.
export function featherMask(
  input: Uint8Array,
  width: number,
  height: number,
  radius: number
): Uint8Array {
  if (
    input.length !== width * height ||
    radius < 0 ||
    radius > 16 ||
    !Number.isInteger(radius)
  ) {
    throw new Error("Invalid mask dimensions or feather radius");
  }
  if (!radius) return input.slice();
  const temporary = new Uint8Array(input.length);
  const result = new Uint8Array(input.length);
  const divisor = 2 * radius + 1;
  for (let y = 0; y < height; y++) {
    let sum = 0;
    for (let x = 0; x <= Math.min(radius, width - 1); x++) {
      sum += input[y * width + x];
    }
    for (let x = 0; x < width; x++) {
      temporary[y * width + x] = Math.floor((sum + radius) / divisor);
      if (x - radius >= 0) sum -= input[y * width + x - radius];
      if (x + radius + 1 < width) sum += input[y * width + x + radius + 1];
    }
  }
  for (let x = 0; x < width; x++) {
    let sum = 0;
    for (let y = 0; y <= Math.min(radius, height - 1); y++) {
      sum += temporary[y * width + x];
    }
    for (let y = 0; y < height; y++) {
      result[y * width + x] = Math.floor((sum + radius) / divisor);
      if (y - radius >= 0) sum -= temporary[(y - radius) * width + x];
      if (y + radius + 1 < height)
        sum += temporary[(y + radius + 1) * width + x];
    }
  }
  return result;
}

export function maskPNG(canvas: HTMLCanvasElement): string {
  const data = canvas
    .getContext("2d")!
    .getImageData(0, 0, canvas.width, canvas.height);
  for (let i = 0; i < data.data.length; i += 4) {
    const value = data.data[i + 3];
    data.data[i] = value;
    data.data[i + 1] = value;
    data.data[i + 2] = value;
    data.data[i + 3] = 255;
  }
  const output = document.createElement("canvas");
  output.width = canvas.width;
  output.height = canvas.height;
  output.getContext("2d")!.putImageData(data, 0, 0);
  return output.toDataURL("image/png").split(",")[1];
}
