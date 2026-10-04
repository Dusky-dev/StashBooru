import assert from "node:assert/strict";
import test from "node:test";
import { featherMask } from "../src/components/Shared/restorationMask.ts";

test("mask feathering has finite support and matches source-pixel integer blending", () => {
  const input = new Uint8Array(49);
  input[3 * 7 + 3] = 255;
  const result = featherMask(input, 7, 7, 1);
  for (let y = 0; y < 7; y++) {
    for (let x = 0; x < 7; x++) {
      assert.equal(
        result[y * 7 + x],
        x >= 2 && x <= 4 && y >= 2 && y <= 4 ? 28 : 0
      );
    }
  }
  assert.deepEqual(featherMask(input, 7, 7, 0), input);
  assert.throws(() => featherMask(input, 7, 7, 17));
});

test("feathering uses zero padding at canvas boundaries and preserves the raw mask", () => {
  const input = new Uint8Array(9);
  input[0] = 255;
  assert.deepEqual(
    [...featherMask(input, 3, 3, 1)],
    [28, 28, 0, 28, 28, 0, 0, 0, 0]
  );
  assert.equal(input[0], 255);
  assert.equal(input[1], 0);
});
