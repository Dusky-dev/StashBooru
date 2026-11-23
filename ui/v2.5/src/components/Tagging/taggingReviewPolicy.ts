export interface TargetCandidate {
  id: number;
  name: string;
  disambiguation?: string;
}

export interface TagPrediction {
  name: string;
  category: string;
  score: number;
  rawName?: string;
  source?: string;
  targetPath?: string;
  targetExists?: boolean;
  targetCandidates?: TargetCandidate[];
  /** The target is an ignored Tag, Character, Artist or Copyright. */
  ignored?: boolean;
  libraryVotes?: number;
  libraryMeanSimilarity?: number;
}

export interface MetadataSource {
  kind: string;
  label: string;
  detail?: string;
  priority: number;
}

export interface MetadataPrediction extends TagPrediction {
  provenance: MetadataSource[];
}

export interface PredictionSourceGroup {
  predictions: TagPrediction[];
  source: (prediction: TagPrediction) => MetadataSource;
}

const IDENTITY_CATEGORIES = new Set(["character", "artist", "copyright"]);
const CHARACTER_IDENTITY_PATTERN = /^(.+?)\s*\(([^()]*)\)\s*$/;
const POSSIBLE_CHARACTER_TARGET_PATTERN = /^\/performers\/\d+\/?$/;

export function normalizePredictionValue(value?: string) {
  return (value ?? "")
    .trim()
    .replaceAll("_", " ")
    .replace(/\s+/g, " ")
    .toLocaleLowerCase();
}

export function predictionIdentityKeys(prediction: TagPrediction) {
  const category = prediction.category.trim().toLocaleLowerCase();
  const targetPath = prediction.targetExists
    ? prediction.targetPath?.trim().replace(/\/+$/, "").toLocaleLowerCase()
    : undefined;
  if (targetPath) {
    return [`${category}\u0000target:${targetPath}`];
  }

  const values = [prediction.name, prediction.rawName]
    .map(normalizePredictionValue)
    .filter(Boolean);
  return [...new Set(values)].map((value) => `${category}\u0000name:${value}`);
}

export function predictionKey(prediction: TagPrediction) {
  return (
    predictionIdentityKeys(prediction)[0] ??
    `${prediction.category}\u0000${prediction.name}`
  );
}

export function sourceKey(source: MetadataSource) {
  return `${source.kind}\u0000${source.detail ?? ""}`;
}

function mergeSources(
  current: MetadataSource[],
  incoming: MetadataSource[]
): MetadataSource[] {
  const merged = new Map<string, MetadataSource>();
  for (const source of [...current, ...incoming]) {
    merged.set(sourceKey(source), source);
  }
  return [...merged.values()].sort((left, right) => {
    if (left.priority !== right.priority) return left.priority - right.priority;
    return left.label.localeCompare(right.label);
  });
}

export function mergeMetadataPredictions(
  sourceGroups: PredictionSourceGroup[]
): MetadataPrediction[] {
  const merged: MetadataPrediction[] = [];
  const indexByKey = new Map<string, number>();

  for (const group of sourceGroups) {
    for (const prediction of group.predictions) {
      const keys = predictionIdentityKeys(prediction);
      let existingIndex: number | undefined;
      for (const key of keys) {
        const index = indexByKey.get(key);
        if (index !== undefined) {
          existingIndex = index;
          break;
        }
      }

      const provenance = [group.source(prediction)];
      if (existingIndex === undefined) {
        const next: MetadataPrediction = { ...prediction, provenance };
        const index = merged.length;
        merged.push(next);
        for (const key of keys) indexByKey.set(key, index);
        continue;
      }

      // Source groups are supplied in priority order. Preserve the earlier
      // source's prediction and score; later sources only add provenance and
      // fill optional target information that the winning source lacked.
      const current = merged[existingIndex];
      const next: MetadataPrediction = {
        ...current,
        rawName: current.rawName || prediction.rawName,
        targetExists:
          current.targetExists === undefined
            ? prediction.targetExists
            : current.targetExists,
        targetPath: current.targetPath || prediction.targetPath,
        targetCandidates: current.targetCandidates?.length
          ? current.targetCandidates
          : prediction.targetCandidates,
        ignored: current.ignored || prediction.ignored,
        provenance: mergeSources(current.provenance, provenance),
      };
      merged[existingIndex] = next;

      for (const key of [
        ...predictionIdentityKeys(current),
        ...keys,
        ...predictionIdentityKeys(next),
      ]) {
        indexByKey.set(key, existingIndex);
      }
    }
  }

  // Ignored records are hidden from review and never applied.
  return merged.filter((prediction) => !prediction.ignored);
}

export function filterBooruPredictionsForLocalPriority(
  localPredictions: TagPrediction[],
  booruPredictions: TagPrediction[]
) {
  const localIdentityCategories = new Set(
    localPredictions
      .map((prediction) => prediction.category.trim().toLocaleLowerCase())
      .filter((category) => IDENTITY_CATEGORIES.has(category))
  );
  const localIdentityKeys = new Set(
    localPredictions.flatMap(predictionIdentityKeys)
  );

  return booruPredictions.filter((prediction) => {
    const category = prediction.category.trim().toLocaleLowerCase();
    if (!IDENTITY_CATEGORIES.has(category)) return true;
    if (!localIdentityCategories.has(category)) return true;

    return predictionIdentityKeys(prediction).some((key) =>
      localIdentityKeys.has(key)
    );
  });
}

export function addPredictionSelection(
  current: Set<string>,
  predictions: TagPrediction[]
) {
  const next = new Set(current);
  for (const prediction of predictions) next.add(predictionKey(prediction));
  return next;
}

export function togglePredictionSelection(
  current: Set<string>,
  prediction: TagPrediction
) {
  const next = new Set(current);
  const key = predictionKey(prediction);
  if (next.has(key)) next.delete(key);
  else next.add(key);
  return next;
}

export function selectedPredictionCount(
  predictions: TagPrediction[],
  selected: Set<string>
) {
  return predictions.filter((prediction) =>
    selected.has(predictionKey(prediction))
  ).length;
}

export function possibleBareCharacterMatch(prediction: TagPrediction) {
  if (
    prediction.category !== "character" ||
    prediction.targetExists ||
    !POSSIBLE_CHARACTER_TARGET_PATTERN.test(prediction.targetPath ?? "")
  ) {
    return undefined;
  }

  const match = prediction.name.match(CHARACTER_IDENTITY_PATTERN);
  if (!match) return undefined;

  const bareName = match[1].trim();
  const disambiguation = match[2].trim();
  if (!bareName || !disambiguation) return undefined;

  return { bareName, disambiguation };
}

export function ambiguousCharacterCandidates(prediction: TagPrediction) {
  if (prediction.category !== "character" || prediction.targetExists) {
    return [];
  }
  return prediction.targetCandidates ?? [];
}

export function characterCandidateLabel(candidate: TargetCandidate) {
  const disambiguation = candidate.disambiguation?.trim();
  return disambiguation
    ? `${candidate.name} (${disambiguation})`
    : candidate.name;
}

export function reusePossibleBareCharacter(
  prediction: TagPrediction
): TagPrediction {
  const possible = possibleBareCharacterMatch(prediction);
  if (!possible) return prediction;

  return {
    ...prediction,
    name: possible.bareName,
    rawName: possible.bareName,
    targetExists: true,
    targetCandidates: undefined,
  };
}

export function reuseCharacterCandidate(
  prediction: TagPrediction,
  candidate: TargetCandidate
): TagPrediction {
  const name = characterCandidateLabel(candidate);
  return {
    ...prediction,
    name,
    rawName: name,
    targetPath: `/performers/${candidate.id}`,
    targetExists: true,
    targetCandidates: undefined,
  };
}

export function nextCharacterResolutionIndex(
  tags: TagPrediction[],
  startIndex: number
) {
  return tags.findIndex(
    (prediction, index) =>
      index >= startIndex &&
      (possibleBareCharacterMatch(prediction) ||
        ambiguousCharacterCandidates(prediction).length > 1)
  );
}

export interface DuplicateDetectionSettings {
  enabled: boolean;
  reversedNames: boolean;
  onePerson: boolean;
}

export const defaultDuplicateDetection: DuplicateDetectionSettings = {
  enabled: true,
  reversedNames: true,
  onePerson: true,
};

const GROUP_COUNT_TAG =
  /^(?:\d+\+?(?:girls|boys|others)|multiple_(?:girls|boys|others))$/;
// Danbooru tags that let one person carry several Character tags. Keep in
// step with camieSeveralFigureTags on the server.
const SEVERAL_FIGURE_TAGS = new Set([
  "solo_focus",
  "cosplay",
  "multiple_views",
  "multiple_persona",
  "dual_persona",
  "character_doll",
  "crossover",
  "costume_switch",
  "split_screen",
  "reference_sheet",
  "character_sheet",
  "fusion",
  "genderswap",
  "alternate_form",
  "doppelganger",
  "clone",
]);

/** A two-part Character name in either order gives the same key. */
export function characterNameOrderKey(name: string) {
  const match = name.match(CHARACTER_IDENTITY_PATTERN);
  const bare = normalizePredictionValue(match ? match[1] : name);
  const qualifier = match ? normalizePredictionValue(match[2]) : "";
  const parts = bare.split(" ");
  if (parts.length === 2) parts.sort();
  return `${parts.join(" ")}\u0000${qualifier}`;
}

/**
 * The general tags clearly say the Image shows one person: `solo`, with no
 * group count and no tag that lets one person carry several Characters.
 */
export function depictsOneCharacter(predictions: readonly TagPrediction[]) {
  let solo = false;
  for (const prediction of predictions) {
    if (prediction.category !== "general") continue;
    const name = normalizePredictionValue(prediction.name).replaceAll(" ", "_");
    if (name === "solo") solo = true;
    else if (GROUP_COUNT_TAG.test(name) || SEVERAL_FIGURE_TAGS.has(name))
      return false;
  }
  return solo;
}

export interface DuplicateCharacterWarnings {
  /** Selected Characters that are the same name in another order. */
  reversed: [string, string][];
  /** Different selected Characters when the tags say one person; else 0. */
  soloConflict: number;
}

function characterIdentityKey(
  prediction: TagPrediction,
  reversedNames: boolean
) {
  if (prediction.targetExists && prediction.targetPath)
    return `target:${prediction.targetPath}`;
  return reversedNames
    ? `name:${characterNameOrderKey(prediction.name)}`
    : `name:${normalizePredictionValue(prediction.name)}`;
}

/** Warn about likely duplicate Characters in a selection. */
export function duplicateCharacterWarnings(
  selected: readonly TagPrediction[],
  settings: DuplicateDetectionSettings = defaultDuplicateDetection
): DuplicateCharacterWarnings {
  const none = { reversed: [], soloConflict: 0 };
  if (!settings.enabled) return none;
  const characters = selected.filter(
    (prediction) => prediction.category === "character"
  );
  const reversed: [string, string][] = [];
  if (settings.reversedNames) {
    const firstByKey = new Map<string, TagPrediction>();
    for (const character of characters) {
      const key = characterNameOrderKey(character.name);
      const first = firstByKey.get(key);
      if (!first) firstByKey.set(key, character);
      else if (
        normalizePredictionValue(first.name) !==
        normalizePredictionValue(character.name)
      )
        reversed.push([first.name, character.name]);
    }
  }
  let soloConflict = 0;
  if (settings.onePerson && depictsOneCharacter(selected)) {
    const distinct = new Set(
      characters.map((character) =>
        characterIdentityKey(character, settings.reversedNames)
      )
    );
    if (distinct.size > 1) soloConflict = distinct.size;
  }
  return { reversed, soloConflict };
}
