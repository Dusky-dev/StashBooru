import assert from "node:assert/strict";
import { test } from "node:test";
import {
  JobNotice,
  jobKey,
  jobQueueReducer,
  readJobNotices,
  serializeJobNotices,
} from "../src/hooks/jobQueueState.ts";

const job = (
  id: string,
  status = "RUNNING",
  addTime = "2026-10-04T12:00:00Z"
): JobNotice => ({
  id,
  status,
  addTime,
  description: `Job ${id}`,
  error: status === "FAILED" ? "First line\nDetails" : null,
});
const event = (j: JobNotice, revision = 1, removed = true, now = 0) => ({
  type: "event" as const,
  job: j,
  revision,
  removed,
  now,
});

test("failed removals remain across expiry, empty snapshots and session reload until dismissed", () => {
  const failed = job("1", "FAILED");
  let state = jobQueueReducer(readJobNotices(null), event(failed));
  state = jobQueueReducer(state, { type: "expire", now: 100000 });
  state = jobQueueReducer(state, { type: "snapshot", jobs: [], revision: 1 });
  state = readJobNotices(serializeJobNotices(state));
  assert.deepEqual(
    state.entries.map((entry) => entry.job.error),
    [failed.error]
  );
  state = jobQueueReducer(state, { type: "dismiss", key: jobKey(failed) });
  state = readJobNotices(serializeJobNotices(state));
  state = jobQueueReducer(state, event(failed, 2));
  state = jobQueueReducer(state, {
    type: "snapshot",
    jobs: [failed],
    revision: 2,
  });
  state = jobQueueReducer(state, {
    type: "snapshot",
    jobs: [{ ...failed, status: "RUNNING" }],
    revision: 0,
  });
  assert.equal(state.entries.length, 0);
});

test("successful/cancelled jobs expire, duplicates deduplicate and active jobs survive dismissal", () => {
  let state = readJobNotices(null);
  for (const j of [
    job("1", "FAILED"),
    job("2", "FINISHED"),
    job("3", "CANCELLED"),
    job("4"),
  ]) {
    state = jobQueueReducer(state, event(j, 1, j.status !== "RUNNING"));
    state = jobQueueReducer(state, event(j, 2, j.status !== "RUNNING"));
  }
  assert.equal(state.entries.length, 4);
  state = jobQueueReducer(state, { type: "expire", now: 9999 });
  assert.equal(state.entries.length, 4);
  state = jobQueueReducer(state, { type: "expire", now: 10000 });
  assert.deepEqual(
    state.entries.map(({ job: j }) => j.id),
    ["1", "4"]
  );
  state = jobQueueReducer(state, { type: "dismiss" });
  assert.deepEqual(
    state.entries.map(({ job: j }) => j.id),
    ["4"]
  );
});

test("a delayed queue snapshot cannot overwrite newer subscription updates or removals", () => {
  let state = jobQueueReducer(
    readJobNotices(null),
    event(job("1", "FAILED"), 2)
  );
  state = jobQueueReducer(state, event(job("2", "FINISHED"), 3));
  state = jobQueueReducer(state, {
    type: "snapshot",
    jobs: [job("1"), job("2")],
    revision: 1,
  });
  assert.deepEqual(
    state.entries.map(({ job: j }) => j.status),
    ["FAILED", "FINISHED"]
  );
  state = jobQueueReducer(state, { type: "expire", now: 10000 });
  assert.equal(state.entries.length, 1);
});

test("reused numeric job IDs cannot dismiss or expire a new job after a server restart", () => {
  const old = job("1", "FINISHED");
  const fresh = job("1", "RUNNING", "2026-10-04T14:00:00Z");
  let state = jobQueueReducer(readJobNotices(null), event(old));
  state = jobQueueReducer(state, event(fresh, 2, false));
  state = jobQueueReducer(state, { type: "expire", now: 10000 });
  assert.deepEqual(
    state.entries.map(({ job: j }) => jobKey(j)),
    [jobKey(fresh)]
  );
  state = jobQueueReducer(state, event({ ...fresh, status: "FAILED" }, 3));
  state = jobQueueReducer(state, { type: "dismiss", key: jobKey(old) });
  assert.equal(state.entries.length, 1);
});

test("an expired completion is not resurrected by a slower in-flight snapshot", () => {
  let state = jobQueueReducer(
    readJobNotices(null),
    event(job("1", "FINISHED"), 2)
  );
  state = jobQueueReducer(state, { type: "expire", now: 10000 });
  state = jobQueueReducer(state, {
    type: "snapshot",
    jobs: [job("1")],
    revision: 1,
  });
  assert.equal(state.entries.length, 0);
  assert.equal(state.expired.size, 1);
  state = jobQueueReducer(state, { type: "snapshot", jobs: [], revision: 2 });
  assert.equal(state.expired.size, 0);
});

test("malformed stored notices are ignored and only failures are persisted", () => {
  assert.equal(readJobNotices("not JSON").entries.length, 0);
  let state = jobQueueReducer(readJobNotices(null), event(job("1"), 1, false));
  state = jobQueueReducer(state, event(job("2", "FAILED"), 2));
  const serialized = serializeJobNotices(state);
  assert.deepEqual(
    readJobNotices(serialized).entries.map(({ job: j }) => j.id),
    ["2"]
  );
  const value = JSON.parse(serialized);
  value.failures.push(null, { ...job("3", "FAILED"), error: {} }, job("4"));
  assert.equal(readJobNotices(JSON.stringify(value)).entries.length, 1);
});
