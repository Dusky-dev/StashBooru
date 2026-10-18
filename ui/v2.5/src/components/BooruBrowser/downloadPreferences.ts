import { readBooruResponse } from "./browser";

interface DownloadPreferences {
  convert: boolean;
  defaultDirectory: string;
}
let pending: Promise<DownloadPreferences> | undefined;
export function invalidateDownloadPreferences() {
  pending = undefined;
}
// Cards mounted together share one settings request. New mounts and explicit
// settings saves read fresh preferences, rather than retaining stale paths.
export function loadDownloadPreferences() {
  if (!pending) {
    const request = fetch("booru-browser/download-settings").then(
      readBooruResponse<DownloadPreferences>
    );
    pending = request;
    void request
      .finally(() => {
        if (pending === request) pending = undefined;
      })
      .catch(() => {});
  }
  return pending;
}
