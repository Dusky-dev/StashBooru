import assert from "node:assert/strict";
import test from "node:test";

import {
  addPredictionSelection,
  ambiguousCharacterCandidates,
  filterBooruPredictionsForLocalPriority,
  mergeMetadataPredictions,
  possibleBareCharacterMatch,
  reuseCharacterCandidate,
  reusePossibleBareCharacter,
  selectedPredictionCount,
  togglePredictionSelection,
  type PredictionSourceGroup,
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

const sourceGroup = (
  kind: string,
  priority: number,
  predictions: TagPrediction[]
): PredictionSourceGroup => ({
  predictions,
  source: () => ({ kind, label: kind, priority }),
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

test("matching exact native target merges provenance without replacing local data", () => {
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
      targetPath: "/performers/12",
      targetExists: true,
    }),
  ];

  const merged = mergeMetadataPredictions([
    sourceGroup("local", 0, local),
    sourceGroup("booru", 10, booru),
  ]);
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

test("same-name native targets remain distinct", () => {
  const local = [
    prediction("Lana", "character", {
      targetPath: "/performers/12",
      targetExists: true,
    }),
  ];
  const booru = [
    prediction("Lana", "character", {
      source: "booru:danbooru",
      targetPath: "/performers/13",
      targetExists: true,
    }),
  ];

  const merged = mergeMetadataPredictions([
    sourceGroup("local", 0, local),
    sourceGroup("booru", 10, booru),
  ]);
  assert.equal(merged.length, 2);
  assert.deepEqual(
    merged.map(({ targetPath }) => targetPath),
    ["/performers/12", "/performers/13"]
  );
});

test("resolved local native target suppresses unresolved same-name lower-priority identity", () => {
  const local = [
    prediction("Lana", "character", {
      targetPath: "/performers/12",
      targetExists: true,
    }),
  ];
  const booru = [
    prediction("Lana", "character", {
      source: "booru:danbooru",
    }),
  ];

  assert.deepEqual(filterBooruPredictionsForLocalPriority(local, booru), []);
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

test("selection helpers add, toggle, and count exact prediction identities", () => {
  const first = prediction("Lana", "character", {
    targetPath: "/performers/12",
    targetExists: true,
  });
  const second = prediction("Lana", "character", {
    targetPath: "/performers/13",
    targetExists: true,
  });

  let selected = addPredictionSelection(new Set<string>(), [first, second]);
  assert.equal(selectedPredictionCount([first, second], selected), 2);

  selected = togglePredictionSelection(selected, first);
  assert.equal(selectedPredictionCount([first, second], selected), 1);
  assert.equal(selectedPredictionCount([first], selected), 0);
  assert.equal(selectedPredictionCount([second], selected), 1);
});
