import { useCallback, useState } from "react";

export interface ProcessingJobIdentity {
  jobID: number;
  batch: string;
}
export function processingJobIdentity(
  value: unknown
): ProcessingJobIdentity | undefined {
  if (!value || typeof value !== "object") return undefined;
  const candidate = value as Partial<ProcessingJobIdentity>;
  return Number.isSafeInteger(candidate.jobID) &&
    candidate.jobID! > 0 &&
    typeof candidate.batch === "string" &&
    /^[a-f0-9]{32}$/.test(candidate.batch)
    ? { jobID: candidate.jobID!, batch: candidate.batch }
    : undefined;
}
export function sameProcessingJob(a: ProcessingJobIdentity, b: unknown) {
  const candidate = processingJobIdentity(b);
  return (
    !!candidate && a.jobID === candidate.jobID && a.batch === candidate.batch
  );
}

export function useProcessingJob(endpoint: string) {
  // Match the exact address used by relative fetches, including a server prefix.
  const storageKey = `stashbooru.processingJob:${new URL(endpoint, document.baseURI).href}`;
  const [job, setJob] = useState<ProcessingJobIdentity | undefined>(() => {
    try {
      return processingJobIdentity(
        JSON.parse(localStorage.getItem(storageKey) ?? "null")
      );
    } catch {
      return undefined;
    }
  });
  const [notice, setNotice] = useState("");
  const remember = useCallback(
    (value: unknown) => {
      const identity = processingJobIdentity(value);
      setJob(identity);
      setNotice(
        identity
          ? ""
          : "Job submitted, but this server did not return a resumable job reference. Check the job queue; saved results remain available."
      );
      try {
        if (identity)
          localStorage.setItem(storageKey, JSON.stringify(identity));
        else localStorage.removeItem(storageKey);
      } catch {
        setNotice(
          "Browser storage is unavailable. Work can continue, but this live job cannot be resumed after refresh."
        );
      }
      return identity;
    },
    [storageKey]
  );
  const unavailable = useCallback(() => {
    setJob(undefined);
    setNotice(
      "Live job details expired or the server restarted. Saved results and available originals remain in their history tabs."
    );
    try {
      localStorage.removeItem(storageKey);
    } catch {
      /* In-memory identity is still cleared. */
    }
  }, [storageKey]);
  return { job, remember, unavailable, notice };
}
