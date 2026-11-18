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

// Downloads before a connection existed created one record per spelling, such
// as "aki99 (artist)". Names are compared the way those records were named.
function sourceRecordName(target: string) {
  return target.replace(/_/g, " ").replace(/\s+/g, " ").trim().toLowerCase();
}

// A disambiguated Character is a deliberate separate record unless the source
// name includes its disambiguation.
function matchesSourceName(candidate: IdentityCandidate, names: Set<string>) {
  const name = candidate.name.toLowerCase();
  return names.has(
    candidate.disambiguation
      ? `${name} (${candidate.disambiguation.toLowerCase()})`
      : name
  );
}

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

  useEffect(() => {
    const names = new Set(
      items.map((item) => sourceRecordName(item.target)).filter(Boolean)
    );
    if (names.size === 0) {
      setRecords([]);
      return;
    }
    const controller = new AbortController();
    // Candidate search accepts up to 200 characters. Names are searched in
    // batches of 20; a failed lookup only hides its own suggestions.
    const search = (name: string) =>
      identityRequest<{ items: IdentityCandidate[] }>(
        `identities/candidates?kind=${kind}&q=${encodeURIComponent(name)}`,
        controller.signal
      )
        .then((page) => page.items)
        .catch(() => [] as IdentityCandidate[]);
    void (async () => {
      const searchable = [...names].filter((name) => name.length <= 200);
      const found = new Map<number, IdentityCandidate>();
      for (let start = 0; start < searchable.length; start += 20) {
        const pages = await Promise.all(
          searchable.slice(start, start + 20).map(search)
        );
        if (controller.signal.aborted) return;
        for (const candidate of pages.flat()) {
          if (candidate.id !== entity.id && matchesSourceName(candidate, names))
            found.set(candidate.id, candidate);
        }
      }
      setRecords([...found.values()]);
    })();
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
