import assert from "node:assert/strict";
import test from "node:test";
import { compareStableVersions } from "../src/components/Settings/releaseVersion.ts";

test("release checks compare numeric versions without offering a downgrade", () => {
  assert.equal(compareStableVersions("1.10.0", "1.9.0"), 1);
  assert.equal(compareStableVersions("1.0.0", "1.0.0"), 0);
  assert.equal(compareStableVersions("1.0.0", "1.1.0"), -1);
  assert.equal(compareStableVersions("2.0.0", "1.99.99"), 1);
});

test("upstream tags, snapshots, prereleases and invalid versions stay unknown", () => {
  for (const value of [
    "v0.30.0",
    "latest_develop",
    "1.1.0-rc.1",
    "1.0",
    "01.0.0",
    "9007199254740992.0.0",
  ]) {
    assert.equal(compareStableVersions(value, "1.0.0"), undefined);
    assert.equal(compareStableVersions("1.0.0", value), undefined);
  }
});
