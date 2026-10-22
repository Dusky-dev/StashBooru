import { useEffect, useRef, useState } from "react";
import { Alert, Badge, Button, Form } from "react-bootstrap";
import { Link, useHistory, useLocation } from "react-router-dom";
import {
  type IdentityKind,
  type IdentityPage,
  type ProviderIdentity,
  identityLabels,
  identityName,
  identityPaths,
  identityRequest,
} from "./identityApi";
import { IdentityResolution } from "./IdentityResolution";
import "./styles.scss";

interface ScanState {
  jobID: number;
  status: string;
  scanned: number;
  observations: number;
  skipped: number;
  error?: string;
}
const kinds: IdentityKind[] = ["artist", "character", "copyright"];

export default function Identities() {
  const location = useLocation();
  const history = useHistory();
  const params = new URLSearchParams(location.search);
  const kind = kinds.find((k) => k === params.get("kind")) ?? "artist";
  const [page, setPage] = useState<IdentityPage>();
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [epoch, setEpoch] = useState(0);
  const [selected, setSelected] = useState<string[]>([]);
  const [busy, setBusy] = useState(false);
  const [scan, setScan] = useState<ScanState>();
  const pending = useRef<AbortController>();
  const resolution = useRef<HTMLElement>(null);
  const autoSelected = useRef("");
  const chosen = page?.items.filter((row) => selected.includes(row.id)) ?? [];
  const anchor = chosen[0];
  const search = location.search;
  const scanning = scan?.status === "queued" || scan?.status === "running";

  useEffect(() => () => pending.current?.abort(), []);
  useEffect(() => {
    const controller = new AbortController();
    setPage(undefined);
    setSelected([]);
    // An explicit refresh after a decision must reload even with unchanged URL.
    void epoch;
    void identityRequest<IdentityPage>(
      `identities/list${search || "?kind=artist"}`,
      controller.signal
    )
      .then((data) => {
        if (controller.signal.aborted) return;
        setPage(data);
        const id = new URLSearchParams(search).get("identity");
        const row = data.items.find((item) => item.id === id);
        if (row && autoSelected.current !== search) {
          autoSelected.current = search;
          setSelected([row.id]);
        }
      })
      .catch((e: unknown) => {
        if (!controller.signal.aborted) setError(String(e));
      });
    return () => controller.abort();
  }, [search, epoch]);

  useEffect(() => {
    const controller = new AbortController();
    let timer: ReturnType<typeof setTimeout>;
    async function poll() {
      try {
        const data = await identityRequest<ScanState>(
          "identities/scan",
          controller.signal
        );
        if (controller.signal.aborted) return;
        setScan(data);
        if (data.status === "queued" || data.status === "running")
          timer = setTimeout(() => void poll(), 1000);
        else if (scanning) setEpoch((v) => v + 1);
      } catch (e) {
        if (!controller.signal.aborted) setError(String(e));
      }
    }
    void poll();
    return () => {
      clearTimeout(timer);
      controller.abort();
    };
  }, [scanning]);

  function navigate(values: Record<string, string>) {
    setError("");
    const p = new URLSearchParams(values);
    history.push(`/identities?${p}`);
  }
  function review(row: ProviderIdentity, scroll = true) {
    setNotice("");
    setError("");
    setSelected([row.id]);
    if (scroll)
      requestAnimationFrame(() =>
        resolution.current?.scrollIntoView({
          behavior: "smooth",
          block: "start",
        })
      );
  }
  async function scanAction(action: string) {
    const controller = new AbortController();
    pending.current?.abort();
    pending.current = controller;
    setBusy(true);
    setError("");
    try {
      const data = await identityRequest<ScanState>(
        "identities/scan",
        controller.signal,
        { action, jobID: scan?.jobID }
      );
      if (!controller.signal.aborted) setScan(data);
    } catch (e) {
      if (!controller.signal.aborted) setError(String(e));
    } finally {
      if (!controller.signal.aborted) setBusy(false);
    }
  }
  async function discard() {
    if (
      !window.confirm(
        `Discard unreviewed ${identityLabels[kind]} metadata? Connections and explicit decisions are kept. A scan can restore discarded observations.`
      )
    )
      return;
    const controller = new AbortController();
    pending.current = controller;
    setBusy(true);
    setError("");
    try {
      const data = await identityRequest<{ discarded: number }>(
        "identities/update",
        controller.signal,
        { action: "discard", kind }
      );
      if (!controller.signal.aborted) {
        setNotice(`Discarded ${data.discarded} unreviewed identifiers.`);
        setEpoch((v) => v + 1);
      }
    } catch (e) {
      if (!controller.signal.aborted) setError(String(e));
    } finally {
      if (!controller.signal.aborted) setBusy(false);
    }
  }

  return (
    <div className="identity-page container-fluid py-3">
      <h2>Identities</h2>
      <p>
        Connect site names and accounts to one local Artist, Character or
        Copyright. Matching names are suggestions; you decide who is the same.
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
      <div className="identity-toolbar mb-3">
        {kinds.map((value) => (
          <Button
            key={value}
            variant={kind === value ? "primary" : "secondary"}
            disabled={busy}
            onClick={() => navigate({ kind: value })}
          >
            {identityLabels[value]}s
          </Button>
        ))}
        <Button
          variant="secondary"
          disabled={busy || scanning}
          onClick={() => void scanAction("start")}
        >
          Scan saved metadata
        </Button>
        {scanning && (
          <Button
            variant="secondary"
            disabled={busy}
            onClick={() => void scanAction("cancel")}
          >
            Cancel scan
          </Button>
        )}
      </div>
      <details className="mb-3">
        <summary>About the saved metadata scan</summary>
        <p>
          Reads local download receipts and cached Following posts. No remote
          requests or automatic linking. Missing metadata is skipped. Repeat
          scans are safe; cancellation keeps saved observations. Status resets
          after a server restart.
        </p>
        <Button
          variant="secondary"
          size="sm"
          disabled={busy || scanning}
          onClick={() => void discard()}
        >
          Discard unreviewed {identityLabels[kind]} metadata
        </Button>
      </details>
      {scan && scan.status !== "idle" && (
        <p role="status">
          Scan {scan.status}: {scan.scanned} records, {scan.observations}{" "}
          observations, {scan.skipped} skipped. {scan.error}
        </p>
      )}
      <Form
        className="identity-toolbar mb-3"
        key={search}
        onSubmit={(event) => {
          event.preventDefault();
          const data = new FormData(event.currentTarget);
          navigate({
            kind,
            q: String(data.get("q") ?? ""),
            state: String(data.get("state") ?? ""),
          });
        }}
      >
        <Form.Control
          aria-label="Filter identifiers"
          name="q"
          defaultValue={params.get("q") ?? ""}
          maxLength={80}
          placeholder="Name or original provider identifier"
        />
        <Form.Control
          as="select"
          aria-label="Connection state"
          name="state"
          defaultValue={params.get("state") ?? ""}
        >
          <option value="">All identifiers</option>
          <option value="unresolved">Unresolved</option>
          <option value="linked">Connected</option>
        </Form.Control>
        <Button type="submit" variant="secondary" disabled={busy}>
          Filter
        </Button>
        {(params.has("identity") ||
          params.has("entity") ||
          params.has("after")) && (
          <Button variant="secondary" onClick={() => navigate({ kind })}>
            Show all {identityLabels[kind]}s
          </Button>
        )}
      </Form>
      <div className="identity-layout">
        <section aria-label="Provider identifiers">
          {!page && !error && <p>Loading identifiers…</p>}
          {page?.items.length === 0 && (
            <p>
              No identifiers found. Browse posts or scan saved metadata to
              collect provider names.
            </p>
          )}
          {page?.items.map((row) => (
            <article key={row.id} className="identity-row">
              <div className="identity-toolbar">
                <Form.Check
                  type="switch"
                  id={`identity-${row.id}`}
                  label={`Select ${row.entityName || (row.form === "account" && row.label === row.target ? "Unresolved artist account" : identityName(row.label, kind))}`}
                  checked={selected.includes(row.id)}
                  disabled={busy}
                  onChange={(event) => {
                    if (event.currentTarget.checked && selected.length === 0)
                      review(row, false);
                    else
                      setSelected((old) =>
                        event.currentTarget.checked
                          ? [...old, row.id]
                          : old.filter((id) => id !== row.id)
                      );
                  }}
                />
                <Badge variant="secondary">{row.sourceName}</Badge>
              </div>
              <h5>
                {row.entityName ||
                  (row.form === "account" && row.label === row.target
                    ? "Unresolved artist account"
                    : identityName(row.label, kind))}
              </h5>
              <p>
                {row.entityID ? (
                  <Link to={`${identityPaths[kind]}/${row.entityID}`}>
                    Connected to {row.entityName}
                  </Link>
                ) : (
                  "Unresolved"
                )}
              </p>
              {kind === "character" && row.context && (
                <p>Observed with: {row.context}</p>
              )}
              <details>
                <summary>Sources &amp; aliases</summary>
                <p>
                  {row.sourceName} · {row.form}
                </p>
                <code>{row.target}</code>
                <p>Provider label: {row.label}</p>
              </details>
              <Button
                className="mt-2"
                variant="secondary"
                size="sm"
                disabled={busy}
                onClick={() => review(row)}
              >
                Review {identityLabels[kind]}
              </Button>
            </article>
          ))}
          {page?.next && (
            <Button
              variant="secondary"
              disabled={busy}
              onClick={() => {
                const p = new URLSearchParams(search);
                p.set("after", page.next);
                history.push(`/identities?${p}`);
              }}
            >
              Next page
            </Button>
          )}
        </section>
        <aside
          ref={resolution}
          className="identity-resolution-panel"
          aria-label="Connect identifiers"
        >
          {!anchor ? (
            <p>
              Select an identifier to connect it. Select several to connect them
              together.
            </p>
          ) : (
            <>
              <div className="identity-toolbar mb-3">
                <Button
                  variant="secondary"
                  size="sm"
                  disabled={busy}
                  onClick={() => setSelected([])}
                >
                  Later / deselect
                </Button>
                <Button
                  variant="secondary"
                  size="sm"
                  disabled={busy}
                  onClick={() =>
                    setSelected(
                      page?.items
                        .filter(
                          (row) =>
                            identityName(row.label, kind).toLowerCase() ===
                            identityName(anchor.label, kind).toLowerCase()
                        )
                        .map((row) => row.id) ?? []
                    )
                  }
                >
                  Select matching labels on this page
                </Button>
              </div>
              <IdentityResolution
                key={chosen.map((row) => row.id).join(":")}
                selected={chosen}
                kind={kind}
                onBusyChange={setBusy}
                onSaved={() => {
                  setBusy(false);
                  setNotice("Decision saved.");
                  setSelected([]);
                  setEpoch((value) => value + 1);
                }}
              />
            </>
          )}
        </aside>
      </div>
    </div>
  );
}
