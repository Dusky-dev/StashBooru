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
      (entry) => entry.tab === "system" && entry.anchor === "apikey"
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

test("Animation inspection search leads to scanning and Video overlap to Experimental", () => {
  assert.ok(
    searchSettings("animation inspection").some(
      (entry) =>
        entry.tab === "tasks" && entry.anchor === "animation-inspection-task"
    )
  );
  assert.ok(
    searchSettings("video overlap").some(
      (entry) =>
        entry.tab === "experimental" &&
        entry.anchor === "experimental-video-overlap"
    )
  );
});

test("retired similarity triage is not a standalone Tools search result", () => {
  assert.equal(
    searchSettings("visual similarity").some(
      (entry) => entry.anchor === "tools-similarity"
    ),
    false
  );
});

test("Settings search labels are translatable and match in the UI language", async () => {
  const { settingsSearchIndex, settingsSearchLabelID } = await import(
    "../src/components/Settings/settingsSearchIndex.ts"
  );
  const { readFile } = await import("node:fs/promises");
  const messages = JSON.parse(
    await readFile(
      new URL("../src/locales/en-GB.json", import.meta.url),
      "utf8"
    )
  );
  const lookup = (id: string) =>
    id.split(".").reduce((value, key) => value?.[key], messages);
  for (const entry of settingsSearchIndex)
    assert.equal(lookup(settingsSearchLabelID(entry)), entry.label);
  // A translated label is both shown and searchable; English still matches.
  const translated = (entry: { label: string }) =>
    entry.label === "Custom CSS" ? "Benutzerdefiniertes CSS" : entry.label;
  assert.ok(
    searchSettings("benutzerdefiniertes", undefined, translated).some(
      (entry) => entry.anchor === "interface-custom-css"
    )
  );
  assert.ok(
    searchSettings("custom css", undefined, translated).some(
      (entry) => entry.anchor === "interface-custom-css"
    )
  );
});
