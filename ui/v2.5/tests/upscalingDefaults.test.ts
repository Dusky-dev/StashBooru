import assert from "node:assert/strict";
import test from "node:test";
import { choiceFormat } from "../src/components/Shared/mediaConversion.ts";

test("saved upscaling formats map the old animated choices to JPEG XL and PNG", () => {
  assert.equal(choiceFormat("ajxl"), "jxl");
  assert.equal(choiceFormat("apng"), "png");
  for (const format of ["jxl", "png", "webp", "auto"])
    assert.equal(choiceFormat(format), format);
});
