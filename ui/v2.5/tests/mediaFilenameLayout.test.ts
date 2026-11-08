import assert from "node:assert/strict";
import test from "node:test";

import {
  exampleImageFilenameLayout,
  parseImageFilenameLayout,
  validateImageFilenameLayout,
} from "../src/utils/mediaFilenameLayout.ts";

const hash = "2c20e78d4c3163a761873936d4380ead";

test("a Grabber filename layout decodes all three categories from a path suffix", () => {
  assert.equal(
    validateImageFilenameLayout(exampleImageFilenameLayout),
    undefined
  );
  assert.deepEqual(
    parseImageFilenameLayout(
      `/library/downloads/[waligner]/(re_zero).emilia_${hash}.jxl`,
      exampleImageFilenameLayout
    ),
    { artist: "waligner", copyright: "re zero", character: "emilia" }
  );
});

test("multiple labels, unknown fields and converted file extensions", () => {
  assert.deepEqual(
    parseImageFilenameLayout(
      `/library/[waligner+another_artist]/(unknown).emilia+rem_${hash}.JXL`,
      exampleImageFilenameLayout
    ),
    { artist: "waligner, another artist", character: "emilia, rem" }
  );
  assert.deepEqual(
    parseImageFilenameLayout(
      `[unknown]/(unknown).unknown_${hash}.png`,
      exampleImageFilenameLayout
    ),
    {}
  );
});

test("unmatched and incomplete names are never interpreted as category data", () => {
  assert.equal(
    parseImageFilenameLayout(
      `/library/re_zero-emilia-${hash}.jxl`,
      exampleImageFilenameLayout
    ),
    undefined
  );
  assert.equal(
    parseImageFilenameLayout(
      "waligner-emilia_00000000000000000000000000000000.png",
      exampleImageFilenameLayout
    ),
    undefined
  );
});

test("reject invalid layouts and ambiguous adjacent token captures", () => {
  for (const bad of [
    "/%artist%/%md5%.%ext%",
    "../%artist%/%md5%.%ext%",
    "%artist%\\%md5%.%ext%",
    "%artist%_%bogus%.%ext%",
    "%artist%%copyright%.%ext%",
    "%artist%-%artist%.%ext%",
    "%artist%_%md5%.jpg",
  ]) {
    assert.ok(
      validateImageFilenameLayout(bad),
      `unexpected valid layout: ${bad}`
    );
    assert.equal(parseImageFilenameLayout("any.jpg", bad), undefined);
  }
  assert.equal(validateImageFilenameLayout(""), undefined);
});

test("single-file layouts use the same token vocabulary", () => {
  assert.deepEqual(
    parseImageFilenameLayout(
      `re_zero - emilia - waligner_${hash}.png`,
      "%copyright% - %character% - %artist%_%md5%.%ext%"
    ),
    { copyright: "re zero", character: "emilia", artist: "waligner" }
  );
});
