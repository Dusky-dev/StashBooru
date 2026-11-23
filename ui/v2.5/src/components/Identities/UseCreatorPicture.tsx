import React, { useState } from "react";
import { Button } from "react-bootstrap";
import { useIntl } from "react-intl";
import { useStudioUpdate } from "src/core/StashService";
import { useToast } from "src/hooks/Toast";
import type { ProviderIdentity } from "./identityApi";

/** Pawchive and Kemono creator accounts have a profile picture. */
export function hasCreatorPicture(row: ProviderIdentity) {
  return (
    row.kind === "artist" &&
    row.form === "account" &&
    (row.engine === "pawchive" || row.engine === "kemono") &&
    /^creator:[^:/\\\s]+:[^:/\\\s]+$/.test(row.target)
  );
}

function asDataURL(blob: Blob) {
  return new Promise<string>((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result));
    reader.onerror = () => reject(reader.error);
    reader.readAsDataURL(blob);
  });
}

/**
 * Sets the Artist image from a connected creator account's profile picture.
 * Never replaces an existing image without asking.
 */
export const UseCreatorPicture: React.FC<{
  row: ProviderIdentity;
  artistID: string;
  hasImage: boolean;
}> = ({ row, artistID, hasImage }) => {
  const intl = useIntl();
  const Toast = useToast();
  const [updateStudio] = useStudioUpdate();
  const [busy, setBusy] = useState(false);
  const site = row.sourceName || row.engine;

  async function apply() {
    if (
      hasImage &&
      !window.confirm(
        intl.formatMessage({ id: "identity_sources.picture_replace" }, { site })
      )
    )
      return;
    setBusy(true);
    try {
      const response = await fetch(
        `booru-browser/creator-icon?instance=${encodeURIComponent(row.instance)}&target=${encodeURIComponent(row.target)}`
      );
      if (!response.ok) {
        let message = response.statusText;
        try {
          message =
            ((await response.json()) as { message?: string }).message ??
            message;
        } catch {
          // Keep the status text.
        }
        throw new Error(message);
      }
      const image = await asDataURL(await response.blob());
      await updateStudio({ variables: { input: { id: artistID, image } } });
      Toast.success(
        intl.formatMessage({ id: "identity_sources.picture_saved" }, { site })
      );
    } catch (error) {
      Toast.error(error);
    } finally {
      setBusy(false);
    }
  }

  return (
    <Button
      size="sm"
      variant="secondary"
      className="mx-1 identity-use-picture"
      disabled={busy}
      onClick={() => void apply()}
    >
      {intl.formatMessage({ id: "identity_sources.picture_use" }, { site })}
    </Button>
  );
};
