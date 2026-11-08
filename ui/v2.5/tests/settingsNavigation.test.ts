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

test("P12/P13 bookmarks move from Tasks to Experimental", () => {
  assert.equal(
    resolveSettingsTab("?tab=tasks", "#image-storage-estimate-task"),
    "experimental"
  );
  assert.equal(
    resolveSettingsTab("?tab=tasks", "#image-storage-shadow-title"),
    "experimental"
  );
  assert.equal(
    resolveSettingsTab("", "#image-storage-recovery-task"),
    "experimental"
  );
  assert.equal(
    resolveSettingsTab("?tab=experimental", "#experimental-storage-candidates"),
    "experimental"
  );
  assert.equal(resolveSettingsTab("?tab=tasks", "#job-queue"), "tasks");
  assert.equal(resolveSettingsTab("?tab=tasks", "#media-backup-task"), "tasks");
});

test("animation inspection stays with Tasks and legacy overlap bookmarks redirect", () => {
  assert.equal(
    resolveSettingsTab("?tab=tasks", "#animation-inspection-task"),
    "tasks"
  );
  assert.equal(
    resolveSettingsTab("?tab=tasks", "#video-overlap-task"),
    "experimental"
  );
  assert.equal(
    resolveSettingsTab("?tab=tasks", "#tasks-media"),
    "experimental"
  );
  assert.equal(
    resolveSettingsTab("?tab=experimental", "#experimental-video-overlap"),
    "experimental"
  );
});

test("saved Alias Collision Inspector links follow the Experimental move", () => {
  assert.equal(resolveSettingsTab("?tab=tools", "#tools-aliases"), "experimental");
  assert.equal(resolveSettingsTab("", "#tools-aliases"), "experimental");
  assert.equal(
    resolveSettingsTab("?tab=experimental", "#experimental-alias-collisions"),
    "experimental"
  );
  assert.equal(resolveSettingsTab("?tab=tools", "#tools-health"), "tools");
});

test("ordinary settings routes and explicit tabs keep their destination", () => {
  assert.equal(resolveSettingsTab("?tab=system", "#transcoding"), "system");
  assert.equal(resolveSettingsTab("?tab=tasks", "#media-converter"), "tasks");
  assert.equal(resolveSettingsTab("?tab=processing", ""), "processing");
  assert.equal(resolveSettingsTab("?tab=experimental", ""), "experimental");
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
