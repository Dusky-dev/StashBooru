import { useEffect, useState } from "react";
import { Alert, Button, Modal } from "react-bootstrap";
import { defineMessages, useIntl } from "react-intl";
import { Link } from "react-router-dom";
import { getClient, mutatePerformerMerge } from "src/core/StashService";
import {
  type IdentityCandidate,
  type IdentityKind,
  identityPaths,
  identityRequest,
  type ProviderIdentity,
} from "./identityApi";

const messages = defineMessages({
  description: { id: "identity_sources.split.description" },
  merge: { id: "identity_sources.split.merge" },
  title: { id: "identity_sources.split.confirm_title" },
  body: { id: "identity_sources.split.confirm_body" },
  confirm: { id: "identity_sources.split.confirm" },
  merging: { id: "identity_sources.split.merging" },
  cancel: { id: "actions.cancel" },
});

const kindMessages = {
  artist: "studio",
  character: "performer",
  copyright: "copyright",
} as const;

async function mergeInto(
  kind: IdentityKind,
  source: number,
  destination: number
) {
  if (kind === "character") {
    await mutatePerformerMerge(String(destination), [String(source)], {
      id: String(destination),
    });
    return;
  }
  const response = await fetch(
    kind === "artist" ? "/studio/merge" : "/tag/copyright/merge",
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ sourceID: source, destinationID: destination }),
    }
  );
  if (!response.ok) throw new Error(await response.text());
  // The REST merge bypasses Apollo; refresh this profile and its lists.
  await getClient().refetchQueries({ include: "active" });
}

/**
 * Lists other native records named after a connected source name and offers
 * the existing merge into this profile. Nothing is merged without a confirm.
 * `items` only signals that connections exist or changed.
 */
export function IdentitySplitRecords({
  kind,
  entity,
  items,
  onMerged,
}: {
  kind: IdentityKind;
  entity: { id: number; name: string };
  items: ProviderIdentity[];
  onMerged: () => void;
}) {
  const intl = useIntl();
  const [records, setRecords] = useState<IdentityCandidate[]>([]);
  const [merging, setMerging] = useState<IdentityCandidate>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  // Downloads before a connection existed created one record per spelling,
  // such as "aki99 (artist)"; the server compares every connected name.
  useEffect(() => {
    if (items.length === 0) {
      setRecords([]);
      return;
    }
    const controller = new AbortController();
    identityRequest<{ items: IdentityCandidate[] }>(
      `identities/split?kind=${kind}&entity=${entity.id}`,
      controller.signal
    )
      .then((page) => {
        if (!controller.signal.aborted) setRecords(page.items);
      })
      .catch(() => {
        if (!controller.signal.aborted) setRecords([]);
      });
    return () => controller.abort();
  }, [kind, entity.id, items]);

  async function confirm() {
    if (!merging || busy) return;
    setBusy(true);
    setError("");
    try {
      await mergeInto(kind, merging.id, entity.id);
      setMerging(undefined);
      onMerged();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setBusy(false);
    }
  }

  if (records.length === 0) return null;
  const kindName = intl.formatMessage({ id: kindMessages[kind] });
  const values = (source: string) => ({
    kind: kindName,
    source,
    destination: entity.name,
    name: entity.name,
  });
  return (
    <div className="identity-split-records">
      <p>{intl.formatMessage(messages.description, values(""))}</p>
      <ul>
        {records.map((record) => (
          <li key={record.id}>
            <Link to={`${identityPaths[kind]}/${record.id}`}>
              {record.name}
              {record.disambiguation ? ` (${record.disambiguation})` : ""}
            </Link>{" "}
            <Button
              size="sm"
              variant="secondary"
              onClick={() => {
                setError("");
                setMerging(record);
              }}
            >
              {intl.formatMessage(messages.merge, values(record.name))}
            </Button>
          </li>
        ))}
      </ul>
      <Modal show={!!merging} onHide={() => !busy && setMerging(undefined)}>
        <Modal.Header closeButton={!busy}>
          <Modal.Title>
            {intl.formatMessage(messages.title, values(merging?.name ?? ""))}
          </Modal.Title>
        </Modal.Header>
        <Modal.Body>
          <p>
            {intl.formatMessage(messages.body, values(merging?.name ?? ""))}
          </p>
          {error && <Alert variant="danger">{error}</Alert>}
        </Modal.Body>
        <Modal.Footer>
          <Button
            variant="secondary"
            disabled={busy}
            onClick={() => setMerging(undefined)}
          >
            {intl.formatMessage(messages.cancel)}
          </Button>
          <Button
            variant="danger"
            disabled={busy}
            onClick={() => void confirm()}
          >
            {intl.formatMessage(busy ? messages.merging : messages.confirm)}
          </Button>
        </Modal.Footer>
      </Modal>
    </div>
  );
}
