import { useEffect, useState } from "react";
import { Alert, Button, Form, Modal } from "react-bootstrap";
import { readBooruResponse } from "./browser";

interface MigrationItem {
  receipt: string;
  oldPath: string;
  newPath?: string;
  status: "ready" | "review" | "audit";
  reason?: string;
  warnings?: string[];
}
interface MigrationReport {
  version: string;
  dismissed: boolean;
  eligible: number;
  review: number;
  truncated: boolean;
  items: MigrationItem[];
}
interface MigrationApplyResult {
  applied: string[];
  failures: Record<string, string>;
}
const endpoint = "booru-browser/filename-migration";

async function migrationAction<T>(payload: object): Promise<T> {
  const response = await fetch(endpoint, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  return readBooruResponse<T>(response);
}

export function BooruFilenameMigration({ showSettings }: { showSettings: boolean }) {
  const [report, setReport] = useState<MigrationReport>();
  const [open, setOpen] = useState(false);
  const [selected, setSelected] = useState<string[]>([]);
  const [busy, setBusy] = useState(false);
  const [completed, setCompleted] = useState(0);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");

  const refresh = async (): Promise<MigrationReport> => {
    const next = await fetch(endpoint, { cache: "no-store" }).then(
      readBooruResponse<MigrationReport>
    );
    setReport(next);
    setSelected(next.items.filter((item) => item.status === "ready").map((item) => item.receipt));
    return next;
  };

  useEffect(() => {
    let mounted = true;
    void fetch(endpoint, { cache: "no-store" })
      .then(readBooruResponse<MigrationReport>)
      .then((value) => {
        if (!mounted) return;
        setReport(value);
        setSelected(value.items.filter((item) => item.status === "ready").map((item) => item.receipt));
        if (!value.dismissed && value.items.length > 0) setOpen(true);
      })
      .catch(() => {
        // This optional upgrade UI must never stop ordinary browsing.
      });
    return () => { mounted = false; };
  }, []);

  const apply = async () => {
    if (busy || selected.length === 0) return;
    setBusy(true);
    setError("");
    setNotice("");
    setCompleted(0);
    let succeeded = 0;
    const failures: string[] = [];
    try {
      for (let i = 0; i < selected.length; i += 20) {
        const batch = selected.slice(i, i + 20);
        const result = await migrationAction<MigrationApplyResult>({ action: "apply", receipts: batch });
        succeeded += result.applied.length;
        failures.push(...Object.entries(result.failures).map(([name, reason]) => name + ": " + reason));
        setCompleted(Math.min(i + batch.length, selected.length));
      }
      await refresh();
      setNotice("Renamed " + succeeded + " verified file(s). No media was re-encoded.");
      if (failures.length > 0) setError(failures.length + " file(s) need review: " + failures.slice(0, 3).join("; "));
    } catch (cause) {
      setError("Migration stopped. Already completed moves are retained. " + String(cause));
      try { await refresh(); } catch { /* Leave the last preview visible. */ }
    } finally {
      setBusy(false);
    }
  };

  const dismiss = async () => {
    setBusy(true);
    setError("");
    try {
      await migrationAction<{ dismissed: boolean }>({ action: "dismiss" });
      setReport((value) => value ? { ...value, dismissed: true } : value);
      setOpen(false);
    } catch (cause) {
      setError(String(cause));
    } finally {
      setBusy(false);
    }
  };

  return (
    <>
      {showSettings && (
        <div className="mb-3">
          <Button variant="outline-secondary" size="sm" onClick={() => {
            setError("");
            void refresh().then(() => setOpen(true)).catch((cause) => setError(String(cause)));
          }}>
            Review legacy download filenames
          </Button>
        </div>
      )}
      <Modal show={open} onHide={() => { if (!busy) setOpen(false); }} size="lg" centered>
        <Modal.Header closeButton={!busy}>
          <Modal.Title>Review older Booru downloads</Modal.Title>
        </Modal.Header>
        <Modal.Body>
          <p>Older StashBooru versions joined multiple Artist, Character and Copyright
            names with a comma. This upgrade checks the saved filename template
            and original tag snapshot before suggesting a rename.</p>
          <p><strong>{report?.eligible ?? 0}</strong> verified, same-folder, unconverted
            file(s) can be renamed; <strong>{report?.review ?? 0}</strong> need review.
            Filename migration never changes metadata relationships.</p>
          {report?.truncated && <Alert variant="warning">Only the first 2,000 download receipts were checked.</Alert>}
          <Alert variant="info">Converted JXL files and directory changes are review-only
            until restoration journals can be migrated safely. Metadata audit is read-only.</Alert>
          {error && <Alert variant="warning" role="alert">{error}</Alert>}
          {notice && <Alert variant="success" role="status">{notice}</Alert>}
          {busy && <p role="status">Checked {completed} of {selected.length} selected files...</p>}
          <div style={{ maxHeight: 320, overflowY: "auto" }} aria-label="Proposed legacy filename changes">
            {(report?.items ?? []).slice(0, 150).map((item) => (
              <div key={item.receipt} className="border-bottom py-2">
                <div className="d-flex align-items-start">
                  {item.status === "ready" ? (
                    <Form.Check
                      id={"migration-" + item.receipt}
                      label="Rename verified file"
                      checked={selected.includes(item.receipt)}
                      disabled={busy}
                      onChange={(event) => {
                        const checked = event.currentTarget.checked;
                        setSelected((current) => checked
                          ? [...current, item.receipt]
                          : current.filter((name) => name !== item.receipt));
                      }}
                    />
                  ) : <span className="small text-muted">Review only</span>}
                </div>
                <div className="small text-break">{item.oldPath}</div>
                {item.newPath && <div className="small text-break text-muted">Proposed: {item.newPath}</div>}
                {item.reason && <div className="small text-warning">{item.reason}</div>}
                {item.warnings?.map((warning) => <div key={warning} className="small text-warning">{warning}</div>)}
              </div>
            ))}
            {(report?.items.length ?? 0) > 150 && (
              <p className="small text-muted">Showing 150 examples. All eligible matches may still be selected.</p>
            )}
          </div>
          {report?.items.length === 0 && <p>No affected downloads found.</p>}
        </Modal.Body>
        <Modal.Footer>
          <Button variant="outline-secondary" disabled={busy} onClick={() => setOpen(false)}>Later</Button>
          <Button variant="outline-secondary" disabled={busy} onClick={() => void dismiss()}>Don't show again</Button>
          <Button disabled={busy || selected.length === 0} onClick={() => void apply()}>
            Rename {selected.length} verified file(s)
          </Button>
        </Modal.Footer>
      </Modal>
    </>
  );
}
