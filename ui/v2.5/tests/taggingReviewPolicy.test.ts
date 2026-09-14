import assert from "node:assert/strict";
import test from "node:test";

import {
  ambiguousCharacterCandidates,
  buildTaggingApplyPayload,
  filterBooruPredictionsForLocalPriority,
  mergeMetadataPredictions,
  possibleBareCharacterMatch,
  reuseCharacterCandidate,
  reusePossibleBareCharacter,
  type TagPrediction,
} from "../src/components/Tagging/taggingReviewPolicy.ts";

const prediction = (
  name: string,
  category: string,
  overrides: Partial<TagPrediction> = {}
): TagPrediction => ({
  name,
  category,
  score: 1,
  ...overrides,
});

test("local identity metadata suppresses conflicting booru identities", () => {
  const local = [
    prediction("Lana", "character", { rawName: "lana" }),
    prediction("Artist A", "artist"),
  ];
  const booru = [
    prediction("Lana", "character", { source: "booru:danbooru" }),
    prediction("Other Lana", "character", { source: "booru:danbooru" }),
    prediction("Artist B", "artist", { source: "booru:danbooru" }),
    prediction("Pokemon", "copyright", { source: "booru:danbooru" }),
    prediction("smile", "general", { source: "booru:danbooru" }),
    prediction("highres", "meta", { source: "booru:danbooru" }),
  ];

  const allowed = filterBooruPredictionsForLocalPriority(local, booru);
  assert.deepEqual(
    allowed.map(({ name, category }) => [name, category]),
    [
      ["Lana", "character"],
      ["Pokemon", "copyright"],
      ["smile", "general"],
      ["highres", "meta"],
    ]
  );
});

test("canonical and raw alias identities merge provenance without replacing local data", () => {
  const local = [
    prediction("Lana", "character", {
      rawName: "lana",
      score: 1,
      targetPath: "/performers/12",
      targetExists: true,
    }),
  ];
  const booru = [
    prediction("lana", "character", {
      rawName: "Lana",
      score: 0.7,
      source: "booru:danbooru",
    }),
  ];

  const merged = mergeMetadataPredictions(local, booru);
  assert.equal(merged.length, 1);
  assert.equal(merged[0].name, "Lana");
  assert.equal(merged[0].score, 1);
  assert.equal(merged[0].targetPath, "/performers/12");
  assert.equal(merged[0].targetExists, true);
  assert.deepEqual(
    merged[0].provenance.map(({ kind }) => kind),
    ["local", "booru"]
  );
});

test("same-name Character candidates stay ambiguous until an explicit choice is made", () => {
  const lana = prediction("Lana", "character", {
    targetCandidates: [
      { id: 1, name: "Lana", disambiguation: "Pokemon" },
      { id: 2, name: "Lana", disambiguation: "Fire Emblem" },
    ],
  });

  assert.equal(ambiguousCharacterCandidates(lana).length, 2);

  const resolved = reuseCharacterCandidate(lana, {
    id: 2,
    name: "Lana",
    disambiguation: "Fire Emblem",
  });
  assert.equal(resolved.name, "Lana (Fire Emblem)");
  assert.equal(resolved.rawName, "Lana (Fire Emblem)");
  assert.equal(resolved.targetPath, "/performers/2");
  assert.equal(resolved.targetExists, true);
  assert.equal(resolved.targetCandidates, undefined);
});

test("a disambiguated possible bare Character can be explicitly reused", () => {
  const lana = prediction("Lana (Pokemon)", "character", {
    targetPath: "/performers/1",
    targetExists: false,
  });

  assert.deepEqual(possibleBareCharacterMatch(lana), {
    bareName: "Lana",
    disambiguation: "Pokemon",
  });

  const reused = reusePossibleBareCharacter(lana);
  assert.equal(reused.name, "Lana");
  assert.equal(reused.rawName, "Lana");
  assert.equal(reused.targetExists, true);
});

test("Artist apply payload preserves add-vs-replace intent", () => {
  const tags = [prediction("Artist A", "artist")];

  assert.deepEqual(buildTaggingApplyPayload(tags, false), {
    tags,
    replaceArtist: false,
  });
  assert.deepEqual(buildTaggingApplyPayload(tags, true), {
    tags,
    replaceArtist: true,
  });
});
