import assert from "node:assert/strict";
import { test } from "node:test";
import {
  mappedVideoTime,
  normalizeKeeperPreferences,
  suggestVideoKeeper,
  type VideoOverlapRow,
} from "../src/components/Shared/videoOverlap.ts";

function pair(): VideoOverlapRow {
  const a = {
    id: 1,
    title: "Original",
    basename: "original.mp4",
    metadataFields: 4,
    media: {
      duration: 24,
      width: 128,
      height: 96,
      codec: "h264",
      bitRate: 800000,
      rotation: 0,
      audioTracks: 1,
      subtitleTracks: 0,
      audioCodecs: ["aac"],
    },
  };
  return {
    a,
    b: { ...a, id: 2, media: { ...a.media } },
    match: {
      a: 1,
      b: 2,
      class: "partial-overlap",
      intervals: [],
      coverageA: 0.5,
      coverageB: 0.5,
      unmatchedA: [],
      unmatchedB: [],
      tolerance: 1.25,
      audio: "Unverified",
      evidence: "Samples",
      confidence: "Limited sampled evidence",
      limited: false,
    },
  };
}

test("keeper suggestions follow explicit priorities, preserve ties and consider tracks/metadata", () => {
  const row = pair(),
    defaults = normalizeKeeperPreferences(null);
  assert.equal(suggestVideoKeeper(row, defaults), undefined);
  row.a.media.width = 256;
  row.b.media.codec = "av1";
  assert.deepEqual(
    suggestVideoKeeper(row, { first: "resolution", codecs: ["av1", "h264"] }),
    { id: 1, reason: "Resolution" }
  );
  assert.deepEqual(
    suggestVideoKeeper(row, { first: "codec", codecs: ["av1", "h264"] }),
    { id: 2, reason: "Preferred codec" }
  );
  for (const [criterion, field] of [
    ["bitrate", "bitRate"],
    ["duration", "duration"],
    ["audio", "audioTracks"],
    ["subtitles", "subtitleTracks"],
  ] as const) {
    row.b.media[field] = row.a.media[field] + 1;
    assert.equal(
      suggestVideoKeeper(row, { first: criterion, codecs: [] })?.id,
      2
    );
  }
  row.b.metadataFields = 8;
  assert.deepEqual(suggestVideoKeeper(row, { first: "metadata", codecs: [] }), {
    id: 2,
    reason: "Populated metadata fields",
  });
  assert.equal(
    row.match.class,
    "partial-overlap",
    "a quality suggestion cannot turn a partial match into a whole-file duplicate"
  );
});

test("stored priorities reject malformed criteria and limit codec input", () => {
  assert.deepEqual(
    normalizeKeeperPreferences({
      first: "__proto__",
      codecs: [null, "av1", "<script>", "HEVC"],
    }),
    { first: "resolution", codecs: ["av1"] }
  );
  assert.deepEqual(normalizeKeeperPreferences("invalid"), {
    first: "resolution",
    codecs: [],
  });
  assert.equal(
    normalizeKeeperPreferences({ codecs: Array(20).fill("h264") }).codecs
      .length,
    8
  );
});

test("offset mapping clamps both ends of the selected timeline", () => {
  const interval = {
    a: { start: 5, end: 15 },
    b: { start: 13, end: 23 },
    offset: 8,
    samples: 10,
    distinctFrames: 10,
    meanDistance: 1,
    meanDHashDistance: 2,
    support: 1,
    gapsA: [],
    gapsB: [],
  };
  assert.equal(mappedVideoTime(9, interval), 17);
  assert.equal(mappedVideoTime(0, interval), 13);
  assert.equal(mappedVideoTime(50, interval), 23);
});
