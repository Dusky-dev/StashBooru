import assert from "node:assert/strict";
import test from "node:test";
import { runUploadBatch } from "../src/components/Shared/mediaUploadQueue.ts";

test("pause finishes the active item without starting the next", async () => {
  let paused = false;
  let finishCurrent = () => {};
  const current = new Promise<void>((resolve) => {
    finishCurrent = resolve;
  });
  const started: string[] = [];
  const completed: string[] = [];
  const run = runUploadBatch(
    ["current", "next"],
    () => paused,
    async (item) => {
      started.push(item);
      if (item === "current") await current;
      completed.push(item);
      return true;
    }
  );

  assert.deepEqual(started, ["current"]);
  paused = true;
  finishCurrent();
  await run;
  assert.deepEqual(completed, ["current"]);
  assert.deepEqual(started, ["current"]);
});

test("a failed import stops its batch even when not paused", async () => {
  const started: string[] = [];
  await runUploadBatch(
    ["saved", "needs attention", "pending"],
    () => false,
    async (item, hasMore) => {
      started.push(item);
      assert.equal(hasMore, true);
      return item !== "needs attention";
    }
  );
  assert.deepEqual(started, ["saved", "needs attention"]);
});
