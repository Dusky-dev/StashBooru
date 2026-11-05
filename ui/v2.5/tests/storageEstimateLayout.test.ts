import assert from "node:assert/strict";
import test from "node:test";

import { formatStorageEstimateLayout } from "../src/components/Settings/Tasks/storageEstimateLayout.ts";

test("P12 string and P13 object layouts render as readable text", () => {
  assert.equal(formatStorageEstimateLayout("independent"), "independent");
  assert.equal(
    formatStorageEstimateLayout({ kind: "independent" }),
    "independent"
  );
  assert.equal(
    formatStorageEstimateLayout({ kind: "fixed", size: 65536 }),
    "fixed, size 65536"
  );
  assert.equal(
    formatStorageEstimateLayout({ kind: "cdc", size: 2048 }),
    "cdc, size 2048"
  );
  assert.equal(
    formatStorageEstimateLayout({ kind: "tile", size: 16 }),
    "tile, size 16"
  );
});

test("unexpected layout data never renders an object or causes a crash", () => {
  for (const layout of [
    null,
    undefined,
    [],
    ["fixed"],
    {},
    { kind: null },
    { size: 65536 },
    { kind: {} },
    12,
  ]) {
    assert.equal(formatStorageEstimateLayout(layout), "Unknown");
  }
  assert.equal(formatStorageEstimateLayout({ kind: "fixed", size: null }), "fixed");
  assert.equal(formatStorageEstimateLayout({ kind: "fixed", size: -1 }), "fixed");
});
