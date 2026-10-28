import assert from "node:assert/strict";
import { test } from "node:test";
import {
  defaultDuplicateMethod,
  duplicateClusterURL,
  duplicateGroupKey,
  duplicateReviewPlan,
  keptImageIDs,
  readableFileSize,
} from "../src/components/ImageDuplicates/review.ts";

const group = {
  ids: [9, 2, 5],
  preferredID: 5,
  members: [
    { id: 9, width: 800, height: 800, fileSize: 200, preferred: false },
    { id: 2, width: 1024, height: 1024, fileSize: 400, preferred: false },
    { id: 5, width: 2048, height: 2048, fileSize: 900, preferred: true },
  ],
};

test("duplicate review defaults to the server-preferred image", () => {
  assert.deepEqual(duplicateReviewPlan(group), {
    keep: [5],
    remove: [9, 2],
  });
  assert.equal(duplicateGroupKey(group), "2:5:9");
});

test("keeping multiple images only marks explicit leftovers for deletion", () => {
  assert.deepEqual(duplicateReviewPlan(group, [2, 9]), {
    keep: [2, 9],
    remove: [5],
  });
  assert.deepEqual(duplicateReviewPlan(group, [2, 5, 9]).remove, []);
});

test("review selection never deletes every image, even with bad persisted state", () => {
  assert.deepEqual(keptImageIDs(group, []), [9]);
  assert.deepEqual(duplicateReviewPlan(group, [123, 123]), {
    keep: [9],
    remove: [2, 5],
  });
  assert.deepEqual(keptImageIDs(group, [5, 5, 2, 123]), [5, 2]);
});

test("sizes are readable and invalid values cannot produce NaN labels", () => {
  assert.equal(readableFileSize(2 * 1024 * 1024), "2.00 MiB");
  assert.equal(readableFileSize(Number.NaN), "Unknown size");
});

test("pHash is the default duplicate mode and uses Hamming distance", () => {
  assert.equal(defaultDuplicateMethod, "phash");
  assert.equal(
    duplicateClusterURL(defaultDuplicateMethod, "5", "0.94"),
    "image/visual-similarity/clusters?method=phash&distance=5"
  );
  assert.equal(
    duplicateClusterURL("phash", "0", "0.88"),
    "image/visual-similarity/clusters?method=phash&distance=0"
  );
});

test("related-content mode uses EVA02 without pHash distance", () => {
  assert.equal(
    duplicateClusterURL("embedding", "5", "0.95"),
    "image/visual-similarity/clusters?method=embedding&threshold=0.95&neighbors=20"
  );
});
