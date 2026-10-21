import { useEffect, useRef, useState } from "react";
import { Alert, Dropdown } from "react-bootstrap";
import { Link, useHistory } from "react-router-dom";
import type { BooruSource } from "../BooruBrowser/browser";

export type IdentityKind = "artist" | "character" | "copyright";
export const identityLabels = {
  artist: "Artist",
  character: "Character",
  copyright: "Copyright",
};
export const identityPaths = {
  artist: "/studios",
  character: "/performers",
  copyright: "/copyrights",
};
export interface ProviderIdentity {
  id: string;
  kind: IdentityKind;
  instance: string;
  sourceID: string;
  sourceName: string;
  engine: string;
  sourceURL: string;
  form: string;
  target: string;
  label: string;
  context: string;
  version: number;
  entityID: number;
  entityName: string;
}
export interface IdentityPage {
  items: ProviderIdentity[];
  next: string;
}
export interface IdentityCandidate {
  id: number;
  name: string;
  disambiguation: string;
  rejected: boolean;
}

export function identityName(value: string, kind: IdentityKind) {
  const name = value.replace(/_/g, " ").trim();
  return kind === "artist" ? name.replace(/\s*\(artist\)$/i, "").trim() : name;
}
export async function identityRequest<T>(
  path: string,
  signal: AbortSignal,
  input?: unknown
): Promise<T> {
  const response = await fetch(new URL(path, document.baseURI), {
    signal,
    credentials: "same-origin",
    ...(input === undefined
      ? {}
      : {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(input),
        }),
  });
  const data = await response.json();
  if (!response.ok)
    throw new Error(
      typeof data.error === "string"
        ? data.error
        : "Identity request failed. Reload and retry."
    );
  return data as T;
}

export function IdentityLink({
  source,
  kind,
  target,
  label,
  identityID,
}: {
  source: BooruSource;
  kind: string;
  target: string;
  label?: string;
  identityID?: string;
}) {
  const history = useHistory();
  const pending = useRef<AbortController>();
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  useEffect(() => () => pending.current?.abort(), []);
  if (kind !== "artist" && kind !== "character" && kind !== "copyright")
    return null;
  async function open() {
    pending.current?.abort();
    const controller = new AbortController();
    pending.current = controller;
    setBusy(true);
    setError("");
    try {
      const id =
        identityID ??
        (
          await identityRequest<ProviderIdentity>(
            "identities/update",
            controller.signal,
            {
              action: "observe",
              kind,
              target,
              label,
              source: source.id,
              instance: source.instance,
            }
          )
        ).id;
      if (!controller.signal.aborted)
        history.push(`/identities?kind=${kind}&identity=${id}`);
    } catch (e) {
      if (!controller.signal.aborted) setError(String(e));
    } finally {
      if (!controller.signal.aborted) setBusy(false);
    }
  }
  return (
    <>
      <Dropdown.Item
        role="menuitem"
        disabled={busy}
        onClick={(event) => {
          event.stopPropagation();
          void open();
        }}
      >
        Connect {identityLabels[kind]}
      </Dropdown.Item>
      {error && <Alert variant="danger">{error}</Alert>}
    </>
  );
}

export function IdentitySources({
  kind,
  entityID,
}: {
  kind: IdentityKind;
  entityID: string;
}) {
  const [open, setOpen] = useState(false);
  const [page, setPage] = useState<IdentityPage>();
  const [error, setError] = useState("");
  useEffect(() => {
    if (!open) return;
    const controller = new AbortController();
    setPage(undefined);
    setError("");
    void identityRequest<IdentityPage>(
      `identities/list?kind=${kind}&entity=${encodeURIComponent(entityID)}`,
      controller.signal
    )
      .then((data) => {
        if (!controller.signal.aborted) setPage(data);
      })
      .catch((e: unknown) => {
        if (!controller.signal.aborted) setError(String(e));
      });
    return () => controller.abort();
  }, [kind, entityID, open]);
  return (
    <details
      className="identity-sources my-3"
      onToggle={(event) => setOpen(event.currentTarget.open)}
    >
      <summary>Sources &amp; aliases</summary>
      {open && (
        <>
          {error && <Alert variant="danger">{error}</Alert>}
          {!page && !error && <p>Loading provider connections…</p>}
          {page?.items.length === 0 && (
            <p>No connected provider identifiers.</p>
          )}
          <ul>
            {page?.items.map((item) => (
              <li key={item.id}>
                {item.sourceName}: <code>{item.target}</code>
              </li>
            ))}
          </ul>
          <Link to={`/identities?kind=${kind}&entity=${entityID}`}>
            {page?.next
              ? "View all and manage connections"
              : "Manage connections"}
          </Link>
        </>
      )}
    </details>
  );
}
