import assert from "node:assert/strict";
import test from "node:test";
import {
  correctedListPage,
  parsePositiveListInteger,
} from "../src/models/list-filter/pageValidity.ts";

test("URL page and page-size values must be positive safe integers", () => {
  for (const invalid of [
    "",
    "0",
    "-2",
    "2garbage",
    "2.5",
    "Infinity",
    "NaN",
    "9999999999999999999999999999",
  ]) {
    assert.equal(parsePositiveListInteger(invalid), undefined, invalid);
  }
  assert.equal(parsePositiveListInteger(null), undefined);
  assert.equal(parsePositiveListInteger("1"), 1);
  assert.equal(parsePositiveListInteger("0003"), 3);
  assert.equal(parsePositiveListInteger("500"), 500);
});

test("a successful empty query always normalizes to page one", () => {
  assert.equal(correctedListPage(8, 0, 40), 1);
  assert.equal(correctedListPage(1, 0, 40), undefined);
});

test("out-of-range or invalid pages clamp to the valid result range", () => {
  assert.equal(correctedListPage(9, 81, 40), 3);
  assert.equal(correctedListPage(3, 81, 40), undefined);
  assert.equal(correctedListPage(0, 81, 40), 1);
  assert.equal(correctedListPage(Number.NaN, 81, 40), 1);
});

test("pagination does not clamp against untrusted/incomplete counts", () => {
  assert.equal(correctedListPage(9, Number.NaN, 40), undefined);
  assert.equal(correctedListPage(9, -1, 40), undefined);
  assert.equal(correctedListPage(9, 1, 0), undefined);
});
