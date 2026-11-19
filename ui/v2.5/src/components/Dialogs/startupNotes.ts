import { compareStableVersions } from "../Settings/releaseVersion.ts";

// Kept free of Markdown imports so it can be unit-tested; callers pass the
// changelog and the newest release-note date.
export interface IStashBooruChange {
  version: string;
  date: string;
  content: string;
}

/** The version every install had before update notes were versioned. */
export const unversionedBaseline = "1.0.0";

export type StartupNotes =
  | { kind: "introduction" }
  | { kind: "changes"; entries: IStashBooruChange[] }
  | undefined;

export interface AcknowledgementState {
  lastNoteSeen?: number | null;
  lastVersionSeen?: string | null;
}

/**
 * What to show when the app opens:
 * - a fresh install (nothing acknowledged yet) gets the introduction;
 * - otherwise, the changelog entries newer than the last version seen, up to
 *   the running version. Installs from before versioned notes count as 1.0.0.
 */
export function startupNotes(
  ui: AcknowledgementState,
  currentVersion: string,
  changelog: IStashBooruChange[]
): StartupNotes {
  if (!ui.lastNoteSeen && !ui.lastVersionSeen) return { kind: "introduction" };
  let since = ui.lastVersionSeen ?? unversionedBaseline;
  if (compareStableVersions(since, currentVersion) === undefined)
    since = unversionedBaseline;
  const entries = changelog.filter(
    (entry) =>
      compareStableVersions(entry.version, since) === 1 &&
      compareStableVersions(entry.version, currentVersion) !== 1
  );
  return entries.length > 0 ? { kind: "changes", entries } : undefined;
}

/** UI settings that record every note up to this version as seen. */
export function acknowledgement(
  currentVersion: string,
  latestNoteDate: number
) {
  return { lastNoteSeen: latestNoteDate, lastVersionSeen: currentVersion };
}
