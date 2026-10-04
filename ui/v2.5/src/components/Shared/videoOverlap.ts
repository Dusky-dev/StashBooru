export const videoOverlapEndpoint = "scene/overlap";

export interface VideoOverlapConfig {
  backend: "auto" | "local" | "remote";
  sampleSeconds: number;
  audioDigest: boolean;
}

export interface VideoOverlapMedia {
  duration: number;
  width: number;
  height: number;
  codec: string;
  bitRate: number;
  rotation: number;
  audioTracks: number;
  subtitleTracks: number;
  audioCodecs: string[];
}

export interface VideoOverlapInfo {
  id: number;
  title: string;
  basename: string;
  media: VideoOverlapMedia;
  metadataFields: number;
}

export interface VideoRange {
  start: number;
  end: number;
}

export interface VideoInterval {
  a: VideoRange;
  b: VideoRange;
  offset: number;
  samples: number;
  distinctFrames: number;
  meanDistance: number;
  meanDHashDistance: number;
  support: number;
  gapsA: VideoRange[];
  gapsB: VideoRange[];
}

export interface VideoOverlapMatch {
  a: number;
  b: number;
  class: string;
  intervals: VideoInterval[];
  coverageA: number;
  coverageB: number;
  unmatchedA: VideoRange[];
  unmatchedB: VideoRange[];
  tolerance: number;
  audio: string;
  evidence: string;
  confidence: string;
  limited: boolean;
}

export interface VideoOverlapRow {
  match: VideoOverlapMatch;
  a: VideoOverlapInfo;
  b: VideoOverlapInfo;
  stale?: string;
}

export interface VideoOverlapJob {
  id: string;
  jobID: number;
  action: string;
  status: string;
  backend: string;
  total: number;
  processed: number;
  indexed: number;
  skipped: number;
  failed: number;
  error?: string;
  items: { id: number; status: string; error?: string }[];
}

export interface VideoOverlapResponse {
  config: VideoOverlapConfig;
  indexed: number;
  totalVideos: number;
  algorithm: string;
  jobs: VideoOverlapJob[];
  job?: VideoOverlapJob;
  review?: {
    id: string;
    reference: number;
    status: string;
    total: number;
    rows: VideoOverlapRow[];
    skipped: number;
    alignmentLimited: number;
    errors: string[];
    candidates: {
      total: number;
      ids: number[];
      limited: boolean;
      omittedCommonBands: number;
    };
  };
}

export const overlapClassLabels: Record<string, string> = {
  "exact-file": "Exact file duplicate",
  "near-complete-visual": "Near-complete visual duplicate",
  "contained-clip": "Contained clip",
  "partial-overlap": "Partial overlap",
  "compilation-segments": "Compilation / multiple segment mappings",
};

export const keeperCriteria = {
  resolution: "Resolution",
  codec: "Preferred codec",
  bitrate: "Bitrate",
  duration: "Duration",
  audio: "Audio tracks",
  subtitles: "Subtitle tracks",
  metadata: "Populated metadata fields",
};

export type KeeperCriterion = keyof typeof keeperCriteria;
export interface KeeperPreferences {
  first: KeeperCriterion;
  codecs: string[];
}

export function suggestVideoKeeper(
  row: VideoOverlapRow,
  prefs: KeeperPreferences
) {
  const order: KeeperCriterion[] = [
    prefs.first,
    ...(Object.keys(keeperCriteria).filter(
      (c) => c !== prefs.first
    ) as KeeperCriterion[]),
  ];
  const score = (info: VideoOverlapInfo, c: KeeperCriterion) => {
    switch (c) {
      case "resolution":
        return info.media.width * info.media.height;
      case "codec": {
        const rank = prefs.codecs.indexOf(info.media.codec);
        return rank < 0 ? 0 : prefs.codecs.length - rank;
      }
      case "bitrate":
        return info.media.bitRate;
      case "duration":
        return info.media.duration;
      case "audio":
        return info.media.audioTracks;
      case "subtitles":
        return info.media.subtitleTracks;
      case "metadata":
        return info.metadataFields;
    }
  };
  for (const c of order) {
    const a = score(row.a, c),
      b = score(row.b, c);
    if (a !== b)
      return { id: a > b ? row.a.id : row.b.id, reason: keeperCriteria[c] };
  }
  return undefined;
}

export function mappedVideoTime(time: number, interval: VideoInterval) {
  return Math.min(
    interval.b.end,
    Math.max(interval.b.start, time + interval.offset)
  );
}

export function videoTime(value: number) {
  const minutes = Math.floor(value / 60);
  return `${minutes}:${(value - minutes * 60).toFixed(1).padStart(4, "0")}`;
}

export function videoRanges(ranges: VideoRange[]) {
  return ranges.length
    ? ranges.map((r) => `${videoTime(r.start)}–${videoTime(r.end)}`).join(", ")
    : "None at this sampling interval";
}

export function videoOverlapActive(job?: VideoOverlapJob) {
  return !!job && ["queued", "running"].includes(job.status);
}

export function normalizeKeeperPreferences(raw: unknown): KeeperPreferences {
  const value = raw as Partial<KeeperPreferences> | null;
  const first =
    value?.first && Object.hasOwn(keeperCriteria, value.first)
      ? value.first
      : "resolution";
  const codecs = Array.isArray(value?.codecs)
    ? value.codecs
        .filter(
          (c): c is string =>
            typeof c === "string" && /^[a-z0-9_-]{1,32}$/.test(c)
        )
        .slice(0, 8)
    : [];
  return { first, codecs };
}
