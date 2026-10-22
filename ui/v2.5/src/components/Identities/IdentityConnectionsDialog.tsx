import { useEffect, useRef, useState } from "react";
import { Alert, Button, Form, Modal } from "react-bootstrap";
import { Link } from "react-router-dom";
import type { BooruSource } from "../BooruBrowser/browser";
import { IdentityResolution } from "./IdentityResolution";
import {
  type IdentityKind,
  type IdentityPage,
  type ProviderIdentity,
  identityLabels,
  identityRequest,
} from "./identityApi";
import "./styles.scss";

export interface SourceTarget {
  source: BooruSource;
  target: string;
  label?: string;
  identityID?: string;
}
export function IdentityConnectionsDialog({
  kind,
  target,
  entity,
  onHide,
  onSaved,
}: {
  kind: IdentityKind;
  target?: SourceTarget;
  entity?: { id: number; name: string };
  onHide: () => void;
  onSaved?: () => void;
}) {
  const [current, setCurrent] = useState<ProviderIdentity>();
  const [items, setItems] = useState<ProviderIdentity[]>([]);
  const [next, setNext] = useState("");
  const [after, setAfter] = useState("");
  const [epoch, setEpoch] = useState(0);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [adding, setAdding] = useState(false);
  const [sources, setSources] = useState<BooruSource[]>([]);
  const [sourceError, setSourceError] = useState("");
  const [sourceID, setSourceID] = useState("");
  const [sourceName, setSourceName] = useState("");
  const pending = useRef<AbortController>();
  const entityID = entity?.id;
  const identityID = target?.identityID;
  const targetText = target?.target;
  const targetLabel = target?.label;
  const targetSource = target?.source.id;
  const targetInstance = target?.source.instance;
  useEffect(() => () => pending.current?.abort(), []);
  useEffect(() => {
    void epoch; // Reload after a saved decision or an explicit retry.
    const controller = new AbortController();
    setLoading(true);
    setError("");
    async function load() {
      if (targetSource) {
        const row = identityID
          ? (
              await identityRequest<IdentityPage>(
                `identities/list?${new URLSearchParams({ kind, identity: identityID })}`,
                controller.signal
              )
            ).items[0]
          : await identityRequest<ProviderIdentity>(
              "identities/update",
              controller.signal,
              {
                action: "observe",
                kind,
                source: targetSource,
                instance: targetInstance,
                target: targetText,
                label: targetLabel,
              }
            );
        if (
          !row ||
          row.target !== targetText ||
          row.instance !== targetInstance
        )
          throw new Error(
            "This source name is no longer available. Reload the provider post and retry."
          );
        if (!controller.signal.aborted) setCurrent(row);
      } else if (entityID) {
        const params = new URLSearchParams({ kind, entity: String(entityID) });
        if (after) params.set("after", after);
        const page = await identityRequest<IdentityPage>(
          `identities/list?${params}`,
          controller.signal
        );
        if (!controller.signal.aborted) {
          setItems((old) => (after ? [...old, ...page.items] : page.items));
          setNext(page.next);
        }
      }
    }
    void load()
      .catch((e: unknown) => {
        if (!controller.signal.aborted) setError(String(e));
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [
    kind,
    entityID,
    identityID,
    targetText,
    targetLabel,
    targetSource,
    targetInstance,
    after,
    epoch,
  ]);

  useEffect(() => {
    if (!adding) return;
    const controller = new AbortController();
    setSources([]);
    setSourceError("");
    void identityRequest<BooruSource[]>(
      "booru-browser/sources",
      controller.signal
    )
      .then((data) => {
        if (!controller.signal.aborted) setSources(data);
      })
      .catch((e: unknown) => {
        if (!controller.signal.aborted) setSourceError(String(e));
      });
    return () => controller.abort();
  }, [adding]);

  async function observe(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const source = sources.find((item) => item.id === sourceID);
    if (!source || busy) return;
    const controller = new AbortController();
    pending.current = controller;
    setBusy(true);
    setError("");
    try {
      const row = await identityRequest<ProviderIdentity>(
        "identities/update",
        controller.signal,
        {
          action: "observe",
          kind,
          source: source.id,
          instance: source.instance,
          target: sourceName,
        }
      );
      if (!controller.signal.aborted) {
        setCurrent(row);
        setAdding(false);
        setNotice("");
      }
    } catch (e) {
      if (!controller.signal.aborted) setError(String(e));
    } finally {
      if (!controller.signal.aborted) setBusy(false);
    }
  }
  function saved(rows: ProviderIdentity[]) {
    setBusy(false);
    setCurrent(undefined);
    setNotice(
      rows[0]?.entityID
        ? `Connected to ${rows[0].entityName}.`
        : "Connection removed."
    );
    if (entityID) {
      setAfter("");
      setEpoch((value) => value + 1);
    }
    onSaved?.();
  }
  return (
    <Modal
      show
      size="lg"
      scrollable
      keyboard={!busy}
      backdrop={busy ? "static" : true}
      backdropClassName="identity-dialog-backdrop"
      className="identity-dialog"
      aria-labelledby="identity-dialog-title"
      onHide={() => {
        if (!busy) onHide();
      }}
    >
      <Modal.Header closeButton={!busy}>
        <Modal.Title id="identity-dialog-title">
          {entity
            ? `Sources & aliases — ${entity.name}`
            : `Connect ${identityLabels[kind]}`}
        </Modal.Title>
      </Modal.Header>
      <Modal.Body>
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
        {loading && <p>Loading provider connections…</p>}
        {!loading && error && (
          <Button
            variant="secondary"
            onClick={() => setEpoch((value) => value + 1)}
          >
            Retry loading connections
          </Button>
        )}
        {!loading && current ? (
          <>
            {entity && (
              <Button
                variant="secondary"
                className="mb-3"
                disabled={busy}
                onClick={() => setCurrent(undefined)}
              >
                Back to connections
              </Button>
            )}
            <IdentityResolution
              key={current.id}
              selected={[current]}
              kind={kind}
              entity={entity}
              onBusyChange={setBusy}
              onSaved={saved}
            />
          </>
        ) : !loading && entity ? (
          <>
            {items.length === 0 && (
              <p>
                No connected provider identifiers. Add a source name here or
                connect it from a provider preview.
              </p>
            )}
            <ul className="identity-targets">
              {items.map((row) => (
                <li key={row.id}>
                  {row.sourceName}: <code>{row.target}</code>{" "}
                  <Button
                    size="sm"
                    variant="secondary"
                    disabled={busy}
                    onClick={() => {
                      setCurrent(row);
                      setNotice("");
                    }}
                  >
                    Manage {row.target}
                  </Button>
                </li>
              ))}
            </ul>
            {next && (
              <Button
                className="mb-3"
                variant="secondary"
                disabled={busy}
                onClick={() => setAfter(next)}
              >
                More connections
              </Button>
            )}
            <div>
              <Button
                variant="secondary"
                disabled={busy}
                onClick={() => {
                  setAdding((value) => !value);
                  setError("");
                }}
              >
                {adding ? "Cancel adding source name" : "Add source name"}
              </Button>
            </div>
            {adding && (
              <Form className="mt-3" onSubmit={(event) => void observe(event)}>
                <fieldset disabled={busy}>
                  {sourceError && (
                    <Alert variant="danger">
                      {sourceError} Close and reopen Add source name to retry.
                    </Alert>
                  )}
                  <Form.Group>
                    <Form.Label htmlFor="identity-source">Site</Form.Label>
                    <Form.Control
                      as="select"
                      id="identity-source"
                      value={sourceID}
                      required
                      onChange={(event) =>
                        setSourceID(event.currentTarget.value)
                      }
                    >
                      <option value="">Choose a site</option>
                      {sources.map((source) => (
                        <option key={source.id} value={source.id}>
                          {source.name}
                        </option>
                      ))}
                    </Form.Control>
                  </Form.Group>
                  <Form.Group>
                    <Form.Label htmlFor="identity-source-name">
                      Exact provider tag or creator expression
                    </Form.Label>
                    <Form.Control
                      id="identity-source-name"
                      value={sourceName}
                      maxLength={512}
                      required
                      onChange={(event) =>
                        setSourceName(event.currentTarget.value)
                      }
                    />
                    <Form.Text>
                      Kept exactly as entered. This adds a local name for review
                      and does not contact the provider.
                    </Form.Text>
                  </Form.Group>
                  <Button
                    type="submit"
                    disabled={
                      !sourceID ||
                      !sourceName.trim() ||
                      !sources.some((source) => source.id === sourceID)
                    }
                  >
                    Review connection
                  </Button>
                </fieldset>
              </Form>
            )}
          </>
        ) : null}
      </Modal.Body>
      <Modal.Footer>
        <Link
          className={`btn btn-link${busy ? " disabled" : ""}`}
          aria-disabled={busy}
          tabIndex={busy ? -1 : 0}
          to={`/identities?kind=${kind}${entity ? `&entity=${entity.id}` : identityID ? `&identity=${identityID}` : ""}`}
        >
          Advanced reconciliation
        </Link>
        <Button variant="secondary" disabled={busy} onClick={onHide}>
          Close
        </Button>
      </Modal.Footer>
    </Modal>
  );
}
