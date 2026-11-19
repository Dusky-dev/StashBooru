import { useEffect, useRef, useState } from "react";
import { Alert, Badge, Button, ButtonGroup, Form } from "react-bootstrap";
import { FormattedMessage, useIntl } from "react-intl";
import { Link, useHistory, useLocation } from "react-router-dom";
import {
  type IdentityKind,
  type IdentityPage,
  type ProviderIdentity,
  identityMessages,
  identityName,
  identityPaths,
  identityRequest,
} from "./identityApi";
import { IdentityResolution } from "./IdentityResolution";
import { IdentitySiteLink } from "./IdentitySiteLink";
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
const scanStates = ["queued", "running", "completed", "cancelled", "failed"];

function siteName(row: ProviderIdentity, kind: IdentityKind) {
  return row.form === "account" && row.label === row.target
    ? ""
    : identityName(row.label, kind);
}

export default function Identities() {
  const intl = useIntl();
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
  const kindName = intl.formatMessage({ id: identityMessages[kind].one });
  const kindPlural = intl.formatMessage({ id: identityMessages[kind].many });
  const filtered = !!(
    params.get("q") ||
    params.get("state") ||
    params.has("identity") ||
    params.has("entity") ||
    params.has("after")
  );
  const m = (id: string, values?: Record<string, string | number>) =>
    intl.formatMessage({ id: `identity_sources.page.${id}` }, values);

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
    if (!window.confirm(m("collect.clear_confirm", { kind: kindName }))) return;
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
        setNotice(m("collect.cleared", { count: data.discarded }));
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
      <h2>
        <FormattedMessage id="identity_sources.title" />
      </h2>
      <p className="identity-intro">{m("intro", { kinds: m("kinds") })}</p>
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
      <ButtonGroup
        className="identity-kinds mb-3"
        aria-label={m("kind_switcher")}
      >
        {kinds.map((value) => (
          <Button
            key={value}
            variant={kind === value ? "primary" : "secondary"}
            aria-pressed={kind === value}
            disabled={busy}
            onClick={() => navigate({ kind: value })}
          >
            {intl.formatMessage({ id: identityMessages[value].many })}
          </Button>
        ))}
      </ButtonGroup>
      <section className="identity-collect mb-3">
        <h5>{m("collect.title")}</h5>
        <p className="text-muted">{m("collect.help")}</p>
        <div className="identity-toolbar">
          <Button
            variant="secondary"
            size="sm"
            disabled={busy || scanning}
            onClick={() => void scanAction("start")}
          >
            {m("collect.scan")}
          </Button>
          {scanning && (
            <Button
              variant="secondary"
              size="sm"
              disabled={busy}
              onClick={() => void scanAction("cancel")}
            >
              {m("collect.cancel")}
            </Button>
          )}
          <Button
            variant="outline-danger"
            size="sm"
            disabled={busy || scanning}
            onClick={() => void discard()}
          >
            {m("collect.clear", { kind: kindName })}
          </Button>
        </div>
        {scan && scanStates.includes(scan.status) && (
          <p className="mt-2 mb-0" role="status">
            {m(`collect.status.${scan.status}`, {
              scanned: scan.scanned,
              found: scan.observations,
              skipped: scan.skipped,
            })}{" "}
            {scan.error}
          </p>
        )}
      </section>
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
          aria-label={m("filter.label")}
          name="q"
          defaultValue={params.get("q") ?? ""}
          maxLength={80}
          placeholder={m("filter.placeholder")}
        />
        <Form.Control
          as="select"
          aria-label={m("filter.state")}
          name="state"
          defaultValue={params.get("state") ?? ""}
        >
          <option value="">{m("filter.all")}</option>
          <option value="unresolved">{m("filter.unconnected")}</option>
          <option value="linked">{m("filter.connected")}</option>
        </Form.Control>
        <Button type="submit" variant="secondary" disabled={busy}>
          {m("filter.submit")}
        </Button>
        {filtered && (
          <Button variant="secondary" onClick={() => navigate({ kind })}>
            {m("filter.show_all", { kinds: kindPlural })}
          </Button>
        )}
      </Form>
      <div className="identity-layout">
        <section aria-label={m("list_label")}>
          {!page && !error && <p>{m("loading")}</p>}
          {page?.items.length === 0 && (
            <p>
              {filtered ? m("empty_filtered") : m("empty", { kind: kindName })}
            </p>
          )}
          {page?.items.map((row) => {
            const name = siteName(row, kind) || m("row.unnamed_account");
            const active = selected.includes(row.id);
            return (
              <article
                key={row.id}
                className={`identity-row${active ? " selected" : ""}`}
              >
                <div className="identity-row-head">
                  <Form.Check
                    id={`identity-${row.id}`}
                    aria-label={m("row.select", {
                      name,
                      site: row.sourceName,
                    })}
                    checked={active}
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
                  <h5>{name}</h5>
                  {row.entityID ? (
                    <IdentitySiteLink row={row} />
                  ) : (
                    <Badge variant="secondary">{row.sourceName}</Badge>
                  )}
                  <Button
                    className="ml-auto"
                    variant={row.entityID ? "secondary" : "primary"}
                    size="sm"
                    disabled={busy}
                    onClick={() => review(row)}
                  >
                    {m(row.entityID ? "row.change" : "row.connect")}
                  </Button>
                </div>
                <p className="mb-1">
                  {row.entityID ? (
                    <Link to={`${identityPaths[kind]}/${row.entityID}`}>
                      {m("row.connected", { name: row.entityName })}
                    </Link>
                  ) : (
                    <span className="text-muted">{m("row.unconnected")}</span>
                  )}
                </p>
                {kind === "character" && row.context && (
                  <p className="mb-1">
                    {m("row.seen_with", { names: row.context })}
                  </p>
                )}
                <details>
                  <summary>
                    <FormattedMessage id="identity_sources.source_details" />
                  </summary>
                  <p>
                    {m("row.form", { site: row.sourceName, form: row.form })}
                  </p>
                  <code>{row.target}</code>
                  <p>{m("row.label", { label: row.label })}</p>
                </details>
              </article>
            );
          })}
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
              {m("next_page")}
            </Button>
          )}
        </section>
        <aside
          ref={resolution}
          className="identity-resolution-panel"
          aria-label={m("panel.label")}
        >
          {!anchor ? (
            <p className="identity-placeholder text-muted">
              {m("panel.placeholder", { kind: kindName })}
            </p>
          ) : (
            <>
              <IdentityResolution
                key={chosen.map((row) => row.id).join(":")}
                selected={chosen}
                kind={kind}
                onBusyChange={setBusy}
                onSaved={() => {
                  setBusy(false);
                  setNotice(m("saved"));
                  setSelected([]);
                  setEpoch((value) => value + 1);
                }}
              />
              <div className="identity-toolbar mt-2">
                <Button
                  variant="link"
                  size="sm"
                  disabled={busy}
                  onClick={() => setSelected([])}
                >
                  {m("panel.clear_selection")}
                </Button>
                <Button
                  variant="link"
                  size="sm"
                  disabled={busy}
                  onClick={() => {
                    const same = siteName(anchor, kind).toLowerCase();
                    setSelected(
                      page?.items
                        .filter(
                          (row) =>
                            row.id === anchor.id ||
                            (same && siteName(row, kind).toLowerCase() === same)
                        )
                        .map((row) => row.id) ?? []
                    );
                  }}
                >
                  {m("panel.select_same")}
                </Button>
              </div>
            </>
          )}
        </aside>
      </div>
    </div>
  );
}
