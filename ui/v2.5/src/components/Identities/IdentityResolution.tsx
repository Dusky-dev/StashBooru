import { useEffect, useRef, useState } from "react";
import { Alert, Button, Form } from "react-bootstrap";
import { Link } from "react-router-dom";
import {
  type IdentityCandidate,
  type IdentityKind,
  type IdentityPage,
  type ProviderIdentity,
  IdentityRequestError,
  identityLabels,
  identityName,
  identityPaths,
  identityRequest,
} from "./identityApi";

export function IdentityResolution({
  selected,
  kind,
  entity,
  onSaved,
  onBusyChange,
}: {
  selected: ProviderIdentity[];
  kind: IdentityKind;
  entity?: { id: number; name: string };
  onSaved: (rows: ProviderIdentity[]) => void;
  onBusyChange?: (busy: boolean) => void;
}) {
  const [chosen, setChosen] = useState(selected);
  const anchor = chosen[0];
  const anchorID = anchor.id;
  const readable =
    anchor.form === "account" && anchor.label === anchor.target
      ? ""
      : identityName(anchor.label, kind);
  const [query, setQuery] = useState(readable.slice(0, 80));
  const [name, setName] = useState(readable);
  const [disambiguation, setDisambiguation] = useState("");
  const [candidates, setCandidates] = useState<IdentityCandidate[]>([]);
  const [truncated, setTruncated] = useState(false);
  const [candidateError, setCandidateError] = useState("");
  const [candidateLoading, setCandidateLoading] = useState(false);
  const [rejected, setRejected] = useState(false);
  const [busy, setLocalBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [needsReload, setNeedsReload] = useState(false);
  const [epoch, setEpoch] = useState(0);

  const pending = useRef<AbortController>();
  useEffect(() => () => pending.current?.abort(), []);
  function setBusy(value: boolean) {
    setLocalBusy(value);
    onBusyChange?.(value);
  }
  useEffect(() => {
    void epoch; // Refresh suggestions after a decision or version reload.
    if (entity) return;
    const controller = new AbortController();
    setCandidates([]);
    setCandidateError("");
    if (!anchorID) {
      setCandidateLoading(false);
      return () => controller.abort();
    }
    setCandidateLoading(true);
    const timer = setTimeout(() => {
      const p = new URLSearchParams({ kind, identity: anchorID, q: query });
      if (rejected) p.set("rejected", "1");
      void identityRequest<{ items: IdentityCandidate[]; truncated: boolean }>(
        `identities/candidates?${p}`,
        controller.signal
      )
        .then((data) => {
          if (!controller.signal.aborted) {
            setCandidates(data.items);
            setTruncated(data.truncated);
          }
        })
        .catch((e: unknown) => {
          if (!controller.signal.aborted) setCandidateError(String(e));
        })
        .finally(() => {
          if (!controller.signal.aborted) setCandidateLoading(false);
        });
    }, 250);
    return () => {
      clearTimeout(timer);
      controller.abort();
    };
  }, [anchorID, kind, query, rejected, epoch, entity]);

  async function reload() {
    const controller = new AbortController();
    pending.current?.abort();
    pending.current = controller;
    setBusy(true);
    setError("");
    setNotice("");
    try {
      const latest = await Promise.all(
        chosen.map(async (row) => {
          const page = await identityRequest<IdentityPage>(
            `identities/list?${new URLSearchParams({ kind, identity: row.id })}`,
            controller.signal
          );
          const current = page.items.find((item) => item.id === row.id);
          if (!current)
            throw new Error(
              "An identifier is no longer available. Close and review again."
            );
          return current;
        })
      );
      if (!controller.signal.aborted) {
        setChosen(latest);
        setNeedsReload(false);
        setEpoch((value) => value + 1);
        setNotice(
          "Latest connections loaded. Your draft is kept; review before saving again."
        );
      }
    } catch (e) {
      if (!controller.signal.aborted) setError(String(e));
    } finally {
      if (!controller.signal.aborted) setBusy(false);
    }
  }
  async function mutate(action: string, entityID = 0) {
    if (busy || needsReload) return;
    const controller = new AbortController();
    pending.current?.abort();
    pending.current = controller;
    setBusy(true);
    setError("");
    setNotice("");
    try {
      const rows = await identityRequest<ProviderIdentity[]>(
        "identities/update",
        controller.signal,
        {
          action,
          kind,
          name,
          disambiguation,
          changes: chosen.map((row) => ({
            id: row.id,
            version: row.version,
            entityID,
          })),
        }
      );
      if (!controller.signal.aborted) {
        if (action === "reject" || action === "restore") {
          setChosen(rows);
          setNotice("Decision saved.");
          setEpoch((value) => value + 1);
        } else {
          setBusy(false);
          onSaved(rows);
        }
      }
    } catch (e) {
      if (!controller.signal.aborted) {
        setError(String(e));
        // Conflicts and uncertain writes require an explicit refresh, never a blind retry.
        setNeedsReload(
          !(e instanceof IdentityRequestError) || e.status !== 400
        );
      }
    } finally {
      if (!controller.signal.aborted) setBusy(false);
    }
  }
  return (
    <section className="identity-resolution" aria-label="Connection decision">
      <h4>
        Connect {chosen.length} {identityLabels[kind]} identifier
        {chosen.length === 1 ? "" : "s"}
      </h4>
      <ul className="identity-targets">
        {chosen.map((row) => (
          <li key={row.id}>
            {row.sourceName}: <code>{row.target}</code>
            {row.entityID > 0 && (
              <div>Currently connected to {row.entityName}</div>
            )}
            {kind === "character" && row.context && (
              <div>Observed with: {row.context}</div>
            )}
          </li>
        ))}
      </ul>
      <p className="text-muted">
        Matching names are suggestions. Only the selected source connections
        change; media assignments stay as they are.
      </p>
      {error && (
        <Alert variant="danger" role="alert">
          {error}
        </Alert>
      )}
      {notice && (
        <Alert variant="success" role="status">
          {notice}
        </Alert>
      )}
      {needsReload && (
        <Button
          variant="secondary"
          disabled={busy}
          onClick={() => void reload()}
        >
          Reload latest connections
        </Button>
      )}
      <fieldset disabled={busy || needsReload}>
        <div className="identity-toolbar mb-3">
          {chosen.some((row) => row.entityID) && (
            <Button
              variant="secondary"
              size="sm"
              disabled={busy || needsReload}
              onClick={() => void mutate("unlink")}
            >
              Remove connection
            </Button>
          )}
        </div>
        {entity ? (
          <>
            {chosen.some(
              (row) => row.entityID > 0 && row.entityID !== entity.id
            ) && <p>This moves the selected source name to {entity.name}.</p>}
            <Button
              disabled={busy || needsReload}
              onClick={() => void mutate("link", entity.id)}
            >
              Connect to {entity.name}
            </Button>
          </>
        ) : (
          <>
            <Form.Group>
              <Form.Label htmlFor="identity-candidate-query">
                Find an existing {identityLabels[kind]}
              </Form.Label>
              <Form.Control
                id="identity-candidate-query"
                value={query}
                maxLength={80}
                onChange={(event) => setQuery(event.currentTarget.value)}
                placeholder="Search native names and aliases"
              />
            </Form.Group>
            <Form.Check
              type="switch"
              id="identity-rejected"
              label="Show rejected matches"
              checked={rejected}
              onChange={(event) => setRejected(event.currentTarget.checked)}
            />
            {candidateLoading && <p>Loading matches…</p>}
            {candidateError && <Alert variant="danger">{candidateError}</Alert>}
            {!candidateLoading &&
              !candidateError &&
              candidates.length === 0 && (
                <p>
                  No matching records. Try a different name, or create one
                  below.
                </p>
              )}
            {truncated && (
              <p>First 50 matches. Refine the name to find more.</p>
            )}
            {candidates.map((candidate) => (
              <div className="identity-candidate my-2" key={candidate.id}>
                <Link
                  target="_blank"
                  rel="noreferrer"
                  to={`${identityPaths[kind]}/${candidate.id}`}
                >
                  {candidate.name}
                  {candidate.disambiguation
                    ? ` (${candidate.disambiguation})`
                    : ""}
                </Link>
                <div className="identity-toolbar mt-2">
                  {candidate.rejected ? (
                    <Button
                      variant="secondary"
                      size="sm"
                      disabled={busy || needsReload || chosen.length !== 1}
                      onClick={() => void mutate("restore", candidate.id)}
                    >
                      Allow suggestion again
                    </Button>
                  ) : (
                    <>
                      <Button
                        variant="primary"
                        size="sm"
                        disabled={busy || needsReload}
                        onClick={() => void mutate("link", candidate.id)}
                      >
                        Connect to {candidate.name}
                      </Button>
                      {chosen.length === 1 && (
                        <Button
                          variant="secondary"
                          size="sm"
                          disabled={busy || needsReload}
                          onClick={() => void mutate("reject", candidate.id)}
                        >
                          Different {identityLabels[kind]}
                        </Button>
                      )}
                    </>
                  )}
                </div>
              </div>
            ))}
            <details className="mt-3">
              <summary>Create a new {identityLabels[kind]}</summary>
              <Form
                onSubmit={(event) => {
                  event.preventDefault();
                  void mutate("create");
                }}
              >
                <Form.Group>
                  <Form.Label htmlFor="identity-name">Display name</Form.Label>
                  <Form.Control
                    id="identity-name"
                    required
                    maxLength={80}
                    value={name}
                    onChange={(event) => setName(event.currentTarget.value)}
                  />
                </Form.Group>
                {kind === "character" && (
                  <Form.Group>
                    <Form.Label htmlFor="identity-disambiguation">
                      Disambiguation (optional)
                    </Form.Label>
                    <Form.Control
                      id="identity-disambiguation"
                      maxLength={80}
                      value={disambiguation}
                      onChange={(event) =>
                        setDisambiguation(event.currentTarget.value)
                      }
                    />
                  </Form.Group>
                )}
                <Button
                  type="submit"
                  disabled={busy || needsReload || !name.trim()}
                >
                  Create and connect {chosen.length}
                </Button>
              </Form>
            </details>
          </>
        )}
      </fieldset>
    </section>
  );
}
