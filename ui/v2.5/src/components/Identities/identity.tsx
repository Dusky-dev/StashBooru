import { useEffect, useMemo, useRef, useState } from "react";
import { Alert, Button, Dropdown } from "react-bootstrap";
import type { BooruSource } from "../BooruBrowser/browser";
import { IdentityConnectionsDialog } from "./IdentityConnectionsDialog";
import {
  type IdentityKind,
  type IdentityPage,
  identityLabels,
  identityRequest,
} from "./identityApi";
import "./styles.scss";
export * from "./identityApi";

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
  const [open, setOpen] = useState(false);
  const returnFocus = useRef<HTMLElement | null>(null);
  const sourceTarget = useMemo(
    () => ({ source, target, label, identityID }),
    [source, target, label, identityID]
  );
  if (kind !== "artist" && kind !== "character" && kind !== "copyright")
    return null;
  function close() {
    setOpen(false);
    requestAnimationFrame(() => {
      if (returnFocus.current?.isConnected)
        returnFocus.current.focus({ preventScroll: true });
    });
  }
  return (
    <>
      <Dropdown.Item
        role="menuitem"
        onClick={(event) => {
          event.stopPropagation();
          // Return keyboard navigation to the preview; this menu item becomes hidden.
          returnFocus.current = (
            event.currentTarget as HTMLElement
          ).closest<HTMLElement>(".modal-content");
          setOpen(true);
        }}
      >
        Connect {identityLabels[kind]}
      </Dropdown.Item>
      {open && (
        <IdentityConnectionsDialog
          key={`${source.instance}:${kind}:${target}`}
          kind={kind}
          target={sourceTarget}
          onHide={close}
        />
      )}
    </>
  );
}

export function IdentitySources({
  kind,
  entityID,
  entityName,
}: {
  kind: IdentityKind;
  entityID: string;
  entityName: string;
}) {
  const [open, setOpen] = useState(false);
  const [manage, setManage] = useState(false);
  const [epoch, setEpoch] = useState(0);
  const entity = useMemo(
    () => ({ id: Number(entityID), name: entityName }),
    [entityID, entityName]
  );
  const [page, setPage] = useState<IdentityPage>();
  const [error, setError] = useState("");
  useEffect(() => {
    void epoch; // A saved decision invalidates the profile summary.
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
  }, [kind, entityID, open, epoch]);
  return (
    <>
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
            <Button
              size="sm"
              variant="secondary"
              onClick={() => setManage(true)}
            >
              {page?.next
                ? "View all and manage connections"
                : "Manage connections"}
            </Button>
          </>
        )}
      </details>
      {manage && (
        <IdentityConnectionsDialog
          key={`${kind}:${entityID}`}
          kind={kind}
          entity={entity}
          onHide={() => setManage(false)}
          onSaved={() => setEpoch((value) => value + 1)}
        />
      )}
    </>
  );
}
