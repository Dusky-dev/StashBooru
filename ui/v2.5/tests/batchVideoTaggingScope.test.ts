import assert from "node:assert/strict";
import test from "node:test";

import { collectFilteredSceneIDs } from "../src/components/Scenes/batchVideoTaggingScope.ts";

class TestFilter {
  currentPage = 3;
  itemsPerPage = 40;
  searchTerm = "selected Character";

  clone() {
    return Object.assign(new TestFilter(), this);
  }
}

function queryMatchingIDs(ids: string[], calls: TestFilter[]) {
  return async (filter: TestFilter) => {
    calls.push(filter);
    const offset = (filter.currentPage - 1) * filter.itemsPerPage;
    return {
      data: {
        findScenes: {
          count: ids.length,
          scenes: ids
            .slice(offset, offset + filter.itemsPerPage)
            .map((id) => ({ id })),
        },
      },
    };
  };
}

test("current-filter scope collects every page without changing the list filter", async () => {
  const filter = new TestFilter();
  const original = filter.clone();
  const calls: TestFilter[] = [];
  const ids = Array.from({ length: 501 }, (_, i) => String(i + 1));

  const result = await collectFilteredSceneIDs(
    filter,
    queryMatchingIDs(ids, calls)
  );

  assert.deepEqual(result, ids);
  assert.deepEqual(filter, original);
  assert.deepEqual(
    calls.map((page) => page.currentPage),
    [1, 2, 3]
  );
  assert.ok(calls.every((page) => page.searchTerm === filter.searchTerm));
});

test("restricted scope intersects the filter even when early pages have no targets", async () => {
  const calls: TestFilter[] = [];
  const ids = Array.from({ length: 501 }, (_, i) => String(i + 1));

  const result = await collectFilteredSceneIDs(
    new TestFilter(),
    queryMatchingIDs(ids, calls),
    [501, 700]
  );

  assert.deepEqual(result, ["501"]);
  assert.equal(calls.length, 3);
});

test("empty restricted scope never queries the library", async () => {
  const calls: TestFilter[] = [];
  const result = await collectFilteredSceneIDs(
    new TestFilter(),
    queryMatchingIDs(["1", "2"], calls),
    []
  );

  assert.deepEqual(result, []);
  assert.deepEqual(calls, []);
});

test("duplicate results are tagged once and do not cause extra page queries", async () => {
  const calls: TestFilter[] = [];
  const result = await collectFilteredSceneIDs(
    new TestFilter(),
    queryMatchingIDs(
      Array.from({ length: 500 }, () => "1"),
      calls
    ),
    [1, 2]
  );

  assert.deepEqual(result, ["1"]);
  assert.equal(calls.length, 2);
});
