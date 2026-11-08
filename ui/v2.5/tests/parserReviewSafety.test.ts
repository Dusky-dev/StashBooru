import assert from "node:assert/strict";
import test from "node:test";

import { countParserAssociationReplacements } from "../src/components/SceneFilenameParser/parserReviewSafety.ts";

const field = (originalValue: string[], value: string[], isSet = true) => ({
  originalValue,
  value,
  isSet,
});

test("new filename associations never count as replacements", () => {
  assert.deepEqual(
    countParserAssociationReplacements([
      {
        performers: field([], ["7"]),
        tags: field(["12"], ["12", "13"]),
      },
    ]),
    { characters: 0, tags: 0 }
  );
});

test("filename apply reports removals by affected Video and field", () => {
  assert.deepEqual(
    countParserAssociationReplacements([
      {
        performers: field(["1", "2"], ["3"]),
        tags: field(["4", "5"], ["4"]),
      },
      {
        performers: field(["6"], ["6"]),
        tags: field(["7"], []),
      },
    ]),
    { characters: 1, tags: 2 }
  );
});

test("disabled fields do not trigger a replacement warning", () => {
  assert.deepEqual(
    countParserAssociationReplacements([
      {
        performers: field(["1"], ["2"], false),
        tags: field(["9"], [], false),
      },
    ]),
    { characters: 0, tags: 0 }
  );
});
