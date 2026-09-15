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
  return [...new Set(values)].map(
    (value) => `${category}\u0000name:${value}`
  );
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

  return merged;
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
