import { useEffect, useState } from "react";
import { readBooruResponse } from "../BooruBrowser/browser";

export type MediaUploadSettings = {
  enabled: boolean;
  allowURL: boolean;
  allowFolderCreation: boolean;
  defaultDirectory: string;
};

export async function readMediaUploadSettings(
  signal?: AbortSignal
): Promise<MediaUploadSettings> {
  const response = await fetch("media-upload/settings", {
    signal,
    cache: "no-store",
  });
  return readBooruResponse<MediaUploadSettings>(response);
}

export async function saveMediaUploadSettings(
  settings: MediaUploadSettings
): Promise<MediaUploadSettings> {
  const response = await fetch("media-upload/settings", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(settings),
  });
  return readBooruResponse<MediaUploadSettings>(response);
}

// Fail closed while settings load, when unavailable, and when disabled.
// Both the Images and Videos toolbar use this hook.
export function useMediaUploadEnabled() {
  const [enabled, setEnabled] = useState(false);
  useEffect(() => {
    const controller = new AbortController();
    void readMediaUploadSettings(controller.signal)
      .then((settings) => {
        if (!controller.signal.aborted) setEnabled(settings.enabled);
      })
      .catch(() => {
        if (!controller.signal.aborted) setEnabled(false);
      });
    return () => controller.abort();
  }, []);
  return enabled;
}
