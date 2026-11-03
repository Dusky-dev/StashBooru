import { useEffect, useState } from "react";
import { Alert, Button, Modal } from "react-bootstrap";
import { readBooruResponse } from "./browser";

interface Item {
  receipt: string;
  oldPath: string;
  newPath?: string;
  status: string;
  reason?: string;
  warnings?: string[];
}

interface Report {
  dismissed: boolean;
  eligible: number;
  review: number;
  truncated: boolean;
  items: Item[];
}

const endpoint = "booru-browser/filename-migration";

async function readReport(): Promise<Report> {
  const data = await fetch(endpoint, { cache: "no-store" }).then(
    readBooruResponse<Report>
  );
  if (!data || !Array.isArray(data.items)) {
    throw new Error("Filename migration preview is unavailable.");
  }
  return data;
}

async function sendAction<T>(action: string, receipts?: string[]): Promise<T> {
  return fetch(endpoint, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ action, receipts }),
  }).then(readBooruResponse<T>);
}

export function BooruFilenameMigration({
  showSettings,
}: {
  showSettings: boolean;
}) {
  const [report, setReport] = useState<Report>();
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");

  useEffect(() => {
    let mounted = true;
    void readReport()
      .then((data) => {
        if (!mounted) return;
        setReport(data);
        if (!data.dismissed && data.items.length > 0) setOpen(true);
      })
      .catch(() => {
        // Browsing must still work if the optional migration API is absent.
      });
    return () => {
      mounted = false;
    };
  }, []);

  const review = async () => {
    setError("");
    try {
      setReport(await readReport());
      setOpen(true);
    } catch (cause) {
      setError(String(cause));
    }
  };

  const dismiss = async () => {
    setBusy(true);
    setError("");
    try {
      await sendAction<{ dismissed: boolean }>("dismiss");
      setOpen(false);
    } catch (cause) {
      setError(String(cause));
    } finally {
      setBusy(false);
    }
  };

  const apply = async () => {
    if (!report || busy) return;
    setBusy(true);
    setError("");
    setNotice("");
    const ready = report.items
      .filter((item) => item.status === "ready")
      .map((item) => item.receipt);
    let succeeded = 0;
    let failures = 0;
    try {
      for (let offset = 0; offset < ready.length; offset += 20) {
        const response = await sendAction<{
          applied: string[];
          failures: Record<string, string>;
        }>("apply", ready.slice(offset, offset + 20));
        succeeded += response.applied.length;
        failures += Object.keys(response.failures).length;
      }
      setReport(await readReport());
      setNotice("Renamed " + succeeded + " file(s); no media re-encoding.");
      if (failures) setError(failures + " file(s) need manual review.");
    } catch (cause) {
      setError(String(cause));
      try {
        setReport(await readReport());
      } catch {
        // Preserve the last available preview.
      }
    } finally {
      setBusy(false);
    }
  };

  return (
    <>
      {showSettings && (
        <Button
          variant="outline-secondary"
          size="sm"
          onClick={() => void review()}
        >
          Review legacy download filenames
        </Button>
      )}
      <Modal show={open} onHide={() => setOpen(false)} size="lg" centered>
        <Modal.Header closeButton={!busy}>
          <Modal.Title>Legacy Booru filename upgrade</Modal.Title>
        </Modal.Header>
        <Modal.Body>
          <p>
            Older downloads joined Artist, Character and Copyright lists with
            commas. This preview uses each saved download receipt, not your
            current tagging layout.
          </p>
          <p>
            Verified safe renames: {report?.eligible ?? 0}. Additional review:{" "}
            {report?.review ?? 0}.
          </p>
          <Alert variant="info">
            Converted JXL files, directory moves and ambiguous metadata are
            review-only. No metadata or media contents will be changed.
          </Alert>
          {error && <Alert variant="warning">{error}</Alert>}
          {notice && <Alert variant="success">{notice}</Alert>}
          {report?.truncated && (
            <Alert variant="warning">Only 2,000 receipts were checked.</Alert>
          )}
          <div style={{ maxHeight: 320, overflowY: "auto" }}>
            {(report?.items ?? []).slice(0, 150).map((item) => (
              <div className="border-bottom py-2" key={item.receipt}>
                <div className="small text-break">{item.oldPath}</div>
                {item.newPath && (
                  <div className="small text-muted text-break">
                    Proposed: {item.newPath}
                  </div>
                )}
                {item.reason && (
                  <div className="small text-warning">{item.reason}</div>
                )}
                {item.warnings?.map((warning) => (
                  <div className="small text-warning" key={warning}>
                    {warning}
                  </div>
                ))}
              </div>
            ))}
          </div>
        </Modal.Body>
        <Modal.Footer>
          <Button
            variant="outline-secondary"
            disabled={busy}
            onClick={() => setOpen(false)}
          >
            Later
          </Button>
          <Button
            variant="outline-secondary"
            disabled={busy}
            onClick={() => void dismiss()}
          >
            Don't show again
          </Button>
          <Button
            disabled={busy || !report?.eligible}
            onClick={() => void apply()}
          >
            Rename {report?.eligible ?? 0} verified file(s)
          </Button>
        </Modal.Footer>
      </Modal>
    </>
  );
}
