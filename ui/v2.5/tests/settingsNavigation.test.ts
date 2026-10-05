import assert from "node:assert/strict";
import test from "node:test";
import {
  resolveSettingsTab,
  settingsSearch,
} from "../src/components/Settings/settingsNavigation.ts";

test("old processing and inheritance bookmarks follow their moved sections", () => {
  assert.equal(
    resolveSettingsTab("?tab=system", "#media-upscaling"),
    "processing"
  );
  assert.equal(resolveSettingsTab("", "#visual-similarity"), "processing");
  assert.equal(
    resolveSettingsTab("?tab=system", "#association-inheritance-settings"),
    "library"
  );
});

test("ordinary settings routes and explicit tabs keep their destination", () => {
  assert.equal(resolveSettingsTab("?tab=system", "#transcoding"), "system");
  assert.equal(resolveSettingsTab("?tab=tasks", "#media-converter"), "tasks");
  assert.equal(resolveSettingsTab("?tab=processing", ""), "processing");
  assert.equal(resolveSettingsTab("?tab=invalid", ""), "tasks");
});

test("normalizing a settings URL preserves unrelated query parameters", () => {
  const params = new URLSearchParams(
    settingsSearch("?tab=system&return=images%2F42&hint=a&hint=b", "processing")
  );
  assert.equal(params.get("tab"), "processing");
  assert.equal(params.get("return"), "images/42");
  assert.deepEqual(params.getAll("hint"), ["a", "b"]);
});
