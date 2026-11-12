import { useEffect, useState } from "react";
import { readBooruResponse } from "../BooruBrowser/browser";

export type MediaUploadSettings = {
  enabled: boolean;
  allowURL: boolean;
  allowFolderCreation: boolean;
  defaultDirectory: string;
  quickUpload: boolean;
  quickTagDefaults: boolean;
  quickConvertDefaults: boolean;
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
  const saved = await readBooruResponse<MediaUploadSettings>(response);
  window.dispatchEvent(new Event(MEDIA_UPLOAD_SETTINGS_CHANGED_EVENT));
  return saved;
}

export const MEDIA_UPLOAD_IMPORTED_EVENT = "stashbooru:media-upload-imported";
const MEDIA_UPLOAD_SETTINGS_CHANGED_EVENT =
  "stashbooru:media-upload-settings-changed";

// Fail closed while settings load, when unavailable, and when disabled.
// The main navigation button updates when Experimental settings change.
export function useMediaUploadEnabled() {
  const [enabled, setEnabled] = useState(false);
  useEffect(() => {
    const controller = new AbortController();
    const refresh = () => {
      void readMediaUploadSettings(controller.signal)
        .then((settings) => {
          if (!controller.signal.aborted) setEnabled(settings.enabled);
        })
        .catch(() => {
          if (!controller.signal.aborted) setEnabled(false);
        });
    };
    refresh();
    window.addEventListener(MEDIA_UPLOAD_SETTINGS_CHANGED_EVENT, refresh);
    return () => {
      controller.abort();
      window.removeEventListener(MEDIA_UPLOAD_SETTINGS_CHANGED_EVENT, refresh);
    };
  }, []);
  return enabled;
}
