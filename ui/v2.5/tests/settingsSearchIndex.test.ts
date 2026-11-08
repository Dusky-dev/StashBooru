import assert from "node:assert/strict";
import test from "node:test";

import { searchSettings } from "../src/components/Settings/settingsSearchIndex.ts";

test("Settings search finds options across tabs and their synonyms", () => {
  assert.ok(
    searchSettings("convert").some(
      (entry) =>
        entry.tab === "processing" && entry.anchor === "media-converter"
    )
  );
  assert.deepEqual(
    searchSettings("filenames").map((entry) => [entry.tab, entry.anchor]),
    [["interface", "image-filename-display"]]
  );
  assert.ok(
    searchSettings("api key").some(
      (entry) => entry.tab === "security" && entry.anchor === "apikey"
    )
  );
  assert.ok(
    searchSettings("booru").some((entry) => entry.anchor === "booru-sources")
  );
  assert.deepEqual(searchSettings("unknown-xyz-setting"), []);
  assert.deepEqual(searchSettings(" "), []);
});

test("Settings search combines terms and does not match unrelated sections", () => {
  assert.deepEqual(
    searchSettings("image titles").map((entry) => entry.anchor),
    ["image-filename-display"]
  );
  assert.equal(
    searchSettings("video extensions")[0]?.anchor,
    "video-extensions"
  );
  assert.equal(searchSettings("  MEDIA   CONVERTER ")[0]?.tab, "processing");
});

test("P12/P13 storage settings search goes to Experimental", () => {
  const storage = searchSettings("p12");
  assert.ok(storage.length > 0);
  assert.ok(storage.every((entry) => entry.tab === "experimental"));
  assert.ok(
    searchSettings("shadow").some(
      (entry) =>
        entry.tab === "experimental" &&
        entry.anchor === "experimental-storage-candidates"
    )
  );
  assert.ok(
    searchSettings("missing originals").some(
      (entry) => entry.anchor === "experimental-storage-recovery"
    )
  );
});
