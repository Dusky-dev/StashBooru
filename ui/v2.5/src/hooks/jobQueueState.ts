// A job's numeric ID is reused after a server restart; its creation time keeps
// retained failures and delayed removals separate from a new job with that ID.
export interface JobNotice {
  id: string;
  addTime: string;
  status: string;
  description: string;
  subTasks?: string[] | null;
  progress?: number | null;
  error?: string | null;
  startTime?: string | null;
}

export const jobKey = (job: JobNotice) => `${job.id}@${job.addTime}`;
export const completedJobDelay = 10000;

interface Entry {
  job: JobNotice;
  revision: number;
  expiresAt?: number;
}
export interface JobQueueState {
  entries: Entry[];
  dismissed: Set<string>;
  expired: Map<string, number>;
}
export type JobQueueAction =
  | { type: "snapshot"; jobs: JobNotice[]; revision: number }
  | {
      type: "event";
      job: JobNotice;
      removed: boolean;
      revision: number;
      now: number;
    }
  | { type: "expire"; now: number }
  | { type: "dismiss"; key?: string };

export function readJobNotices(serialized: string | null): JobQueueState {
  const empty = {
    entries: [],
    dismissed: new Set<string>(),
    expired: new Map<string, number>(),
  };
  if (!serialized) return empty;
  try {
    const value = JSON.parse(serialized);
    if (!Array.isArray(value?.failures) || !Array.isArray(value?.dismissed))
      return empty;
    const dismissed = new Set<string>(
      value.dismissed.filter((key: unknown) => typeof key === "string")
    );
    const failures = new Map<string, JobNotice>();
    for (const job of value.failures) {
      if (
        typeof job?.id !== "string" ||
        typeof job.addTime !== "string" ||
        typeof job.description !== "string" ||
        job.status !== "FAILED" ||
        (job.error != null && typeof job.error !== "string")
      )
        continue;
      const notice: JobNotice = {
        id: job.id,
        addTime: job.addTime,
        status: "FAILED",
        description: job.description,
        error: job.error,
      };
      if (!dismissed.has(jobKey(notice))) failures.set(jobKey(notice), notice);
    }
    return {
      entries: Array.from(failures.values(), (job) => ({ job, revision: 0 })),
      dismissed,
      expired: new Map<string, number>(),
    };
  } catch {
    return empty;
  }
}

export function serializeJobNotices(state: JobQueueState) {
  return JSON.stringify({
    failures: state.entries
      .filter(({ job }) => job.status === "FAILED")
      .map(({ job }) => job),
    dismissed: Array.from(state.dismissed),
  });
}

export function jobQueueReducer(
  state: JobQueueState,
  action: JobQueueAction
): JobQueueState {
  if (action.type === "dismiss") {
    const removed = state.entries.filter(
      ({ job }) =>
        job.status === "FAILED" &&
        (action.key === undefined || jobKey(job) === action.key)
    );
    if (!removed.length) return state;
    const dismissed = new Set(state.dismissed);
    for (const { job } of removed) dismissed.add(jobKey(job));
    return {
      ...state,
      dismissed,
      entries: state.entries.filter(
        ({ job }) => job.status !== "FAILED" || !dismissed.has(jobKey(job))
      ),
    };
  }
  if (action.type === "expire") {
    const expired = new Map(state.expired);
    for (const entry of state.entries) {
      if (entry.expiresAt !== undefined && entry.expiresAt <= action.now)
        expired.set(jobKey(entry.job), entry.revision);
    }
    const entries = state.entries.filter(
      ({ expiresAt }) => expiresAt === undefined || expiresAt > action.now
    );
    return entries.length === state.entries.length
      ? state
      : { ...state, entries, expired };
  }
  if (action.type === "event") {
    const key = jobKey(action.job);
    if (state.dismissed.has(key)) return state;
    const entry: Entry = {
      job: action.job,
      revision: action.revision,
      expiresAt:
        action.removed && action.job.status !== "FAILED"
          ? action.now + completedJobDelay
          : undefined,
    };
    const existing = state.entries.some(({ job }) => jobKey(job) === key);
    const expired = new Map(state.expired);
    expired.delete(key);
    return {
      ...state,
      expired,
      entries: existing
        ? state.entries.map((old) => (jobKey(old.job) === key ? entry : old))
        : [...state.entries, entry],
    };
  }

  const current = new Map(
    state.entries.map((entry) => [jobKey(entry.job), entry])
  );
  const entries: Entry[] = [];
  for (const job of action.jobs) {
    const key = jobKey(job);
    if (state.dismissed.has(key)) continue;
    if ((state.expired.get(key) ?? -1) > action.revision) continue;
    const old = current.get(key);
    // Subscription updates received while the query was in flight win.
    entries.push(
      old && old.revision > action.revision
        ? old
        : { job, revision: action.revision }
    );
    current.delete(key);
  }
  for (const entry of current.values()) {
    if (
      entry.job.status === "FAILED" ||
      entry.expiresAt !== undefined ||
      entry.revision > action.revision
    )
      entries.push(entry);
  }
  return {
    ...state,
    entries,
    expired: new Map(
      Array.from(state.expired).filter(
        ([, revision]) => revision > action.revision
      )
    ),
  };
}
