import { useEffect, useState } from "react";
import { useIntl } from "react-intl";
import { faUserCircle } from "@fortawesome/free-solid-svg-icons";
import { useToast } from "src/hooks/Toast";
import type { IImageInputExtraAction } from "src/components/Shared/ImageInput";
import {
  identityRequest,
  type IdentityPage,
  type ProviderIdentity,
} from "./identityApi";

/** Pixiv, Pawchive and Kemono creator accounts have a profile picture. */
export function hasCreatorPicture(row: ProviderIdentity) {
  if (row.kind !== "artist" || row.form !== "account") return false;
  if (row.engine === "pixiv") return /^user:\d+$/.test(row.target);
  return (
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

async function fetchCreatorPicture(row: ProviderIdentity) {
  const response = await fetch(
    `booru-browser/creator-icon?instance=${encodeURIComponent(row.instance)}&target=${encodeURIComponent(row.target)}`
  );
  if (!response.ok) {
    let message = response.statusText;
    try {
      message =
        ((await response.json()) as { error?: string }).error || message;
    } catch {
      // Keep the status text.
    }
    throw new Error(message);
  }
  return asDataURL(await response.blob());
}

// The list is paged by 50; an Artist with more connections than this is
// unusual, and the menu stays bounded.
const maxPages = 4;

/**
 * Set image menu entries for an existing Artist: one per connected creator
 * account that has a profile picture. Choosing one loads the picture into the
 * edit form; Save keeps it, like any other image source.
 */
export function useCreatorPictureActions(
  artistID: string | undefined,
  onImage: (image: string) => void
): IImageInputExtraAction[] {
  const intl = useIntl();
  const Toast = useToast();
  const [rows, setRows] = useState<ProviderIdentity[]>([]);

  useEffect(() => {
    setRows([]);
    if (!artistID) return;
    const controller = new AbortController();
    void (async () => {
      const found: ProviderIdentity[] = [];
      let after = "";
      for (let page = 0; page < maxPages; page++) {
        const data = await identityRequest<IdentityPage>(
          `identities/list?kind=artist&entity=${encodeURIComponent(artistID)}${after ? `&after=${encodeURIComponent(after)}` : ""}`,
          controller.signal
        );
        found.push(...data.items.filter(hasCreatorPicture));
        if (!data.next) break;
        after = data.next;
      }
      if (!controller.signal.aborted) setRows(found);
    })().catch(() => {
      // Without identities the menu simply has no profile picture entries.
    });
    return () => controller.abort();
  }, [artistID]);

  return rows.map((row) => {
    const site = row.sourceName || row.engine;
    const name = row.label && row.label !== row.target ? row.label : "";
    return {
      icon: faUserCircle,
      labelId: `creator-picture-${row.id}`,
      label: intl.formatMessage(
        {
          id: name
            ? "identity_sources.picture_use_named"
            : "identity_sources.picture_use",
        },
        { site, name }
      ),
      onClick: async () => {
        try {
          onImage(await fetchCreatorPicture(row));
          Toast.success(
            intl.formatMessage(
              { id: "identity_sources.picture_loaded" },
              { site }
            )
          );
        } catch (error) {
          Toast.error(error);
        }
      },
    };
  });
}
