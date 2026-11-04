import { useEffect, useState } from "react";
import { Alert, Button, Form, Modal } from "react-bootstrap";
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
  convertedEligible: number;
  review: number;
  truncated: boolean;
  items: Item[];
}

const endpoint = "booru-browser/filename-migration";
const laterKey = "booru-filename-migration-v1-later";

function postponeForSession(): void {
  try {
    sessionStorage.setItem(laterKey, "1");
  } catch {
    // Storage can be unavailable; the dialog remains dismissible.
  }
}

function postponedThisSession(): boolean {
  try {
    return sessionStorage.getItem(laterKey) === "1";
  } catch {
    return false;
  }
}

async function readReport(): Promise<Report> {
  const data = await fetch(endpoint, { cache: "no-store" }).then(
    readBooruResponse<Report>
  );
  if (!data || !Array.isArray(data.items)) {
    throw new Error("Filename migration preview is unavailable.");
  }
  return data;
}

async function sendAction<T>(
  action: string,
  receipts?: string[],
  acceptHistoryMigration = false
): Promise<T> {
  return fetch(endpoint, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ action, receipts, acceptHistoryMigration }),
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
  const [selected, setSelected] = useState<string[]>([]);
  const [selectedConverted, setSelectedConverted] = useState<string[]>([]);
  const [acceptHistoryMigration, setAcceptHistoryMigration] = useState(false);
  const [visibleCount, setVisibleCount] = useState(150);

  useEffect(() => {
    let mounted = true;
    void readReport()
      .then((data) => {
        if (!mounted) return;
        setReport(data);
        if (
          !data.dismissed &&
          data.items.length > 0 &&
          !postponedThisSession()
        ) {
          setOpen(true);
        }
      })
      .catch(() => {
        // Browsing must still work if the optional migration API is absent.
      });
    return () => {
      mounted = false;
    };
  }, []);

  const closeForSession = () => {
    postponeForSession();
    setOpen(false);
  };

  const review = async () => {
    setError("");
    try {
      setReport(await readReport());
      setSelected([]);
      setSelectedConverted([]);
      setAcceptHistoryMigration(false);
      setVisibleCount(150);
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

  const apply = async (converted = false) => {
    const ready = converted ? selectedConvertedEligible : selectedEligible;
    if (
      !report ||
      busy ||
      ready.length === 0 ||
      (converted && !acceptHistoryMigration)
    ) {
      return;
    }
    setBusy(true);
    setError("");
    setNotice("");
    let succeeded = 0;
    let failures = 0;
    try {
      for (let offset = 0; offset < ready.length; offset += 20) {
        const response = await sendAction<{
          applied: string[];
          failures: Record<string, string>;
        }>(
          converted ? "apply-converted" : "apply",
          ready.slice(offset, offset + 20),
          converted && acceptHistoryMigration
        );
        succeeded += response.applied.length;
        failures += Object.keys(response.failures).length;
      }
      setReport(await readReport());
      setSelected([]);
      setSelectedConverted([]);
      setAcceptHistoryMigration(false);
      setNotice(
        "Applied " +
          succeeded +
          " verified filename/receipt update(s) without re-encoding." +
          (converted
            ? " Their conversion-history paths were updated; restore/reapply still requires the inactive-format cache."
            : "")
      );
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

  const eligible = (report?.items ?? []).filter(
    (item) => item.status === "ready" || item.status === "ready-receipt"
  );
  const eligibleNames = new Set(eligible.map((item) => item.receipt));
  const convertedEligible = (report?.items ?? []).filter(
    (item) => item.status === "ready-converted"
  );
  const convertedNames = new Set(convertedEligible.map((item) => item.receipt));
  const selectedEligible = selected.filter((receipt) =>
    eligibleNames.has(receipt)
  );
  const selectedConvertedEligible = selectedConverted.filter((receipt) =>
    convertedNames.has(receipt)
  );
  const toggleSelected = (receipt: string, converted = false) => {
    const setter = converted ? setSelectedConverted : setSelected;
    setter((current) =>
      current.includes(receipt)
        ? current.filter((item) => item !== receipt)
        : [...current, receipt]
    );
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
      <Modal show={open} onHide={closeForSession} size="lg" centered>
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
            Eligible original-media updates: {report?.eligible ?? 0}. Eligible
            converted renames: {report?.convertedEligible ?? 0}. Additional
            review: {report?.review ?? 0}.
          </p>
          <Alert variant="info">
            Filename changes do not re-encode media or alter tags. Files already
            verified under their proposed names can have their stale download
            receipts repaired without moving media. Missing files, directory
            moves and ambiguous metadata remain review-only.
          </Alert>
          {(report?.convertedEligible ?? 0) > 0 && (
            <Alert variant="warning">
              <strong>Converted or restored media:</strong> Both stored paths in
              conversion history will be migrated with the native file and
              receipt. Restore and reapply remain available while the required
              inactive version is cached. Back up media, database, receipts, and
              conversion history: these changes are not crash-atomic. Nothing is
              selected automatically.
            </Alert>
          )}
          {error && <Alert variant="warning">{error}</Alert>}
          {notice && <Alert variant="success">{notice}</Alert>}
          {report?.truncated && (
            <Alert variant="warning">Only 2,000 receipts were checked.</Alert>
          )}
          <div className="d-flex align-items-center justify-content-between gap-2 mb-2">
            <span className="small">
              Selected {selectedEligible.length} original-media updates and{" "}
              {selectedConvertedEligible.length} converted media
            </span>
            <div className="d-flex gap-2">
              <Button
                variant="outline-secondary"
                size="sm"
                disabled={busy || eligible.length === 0}
                onClick={() =>
                  setSelected(eligible.map((item) => item.receipt))
                }
              >
                Select eligible
              </Button>
              <Button
                variant="outline-secondary"
                size="sm"
                disabled={busy || convertedEligible.length === 0}
                onClick={() =>
                  setSelectedConverted(
                    convertedEligible.map((item) => item.receipt)
                  )
                }
              >
                Select eligible converted
              </Button>
              <Button
                variant="outline-secondary"
                size="sm"
                disabled={
                  busy ||
                  (selected.length === 0 && selectedConverted.length === 0)
                }
                onClick={() => {
                  setSelected([]);
                  setSelectedConverted([]);
                }}
              >
                Clear
              </Button>
            </div>
          </div>
          <div style={{ maxHeight: 320, overflowY: "auto" }}>
            {(report?.items ?? []).slice(0, visibleCount).map((item) => (
              <div className="border-bottom py-2" key={item.receipt}>
                <div className="d-flex align-items-center gap-2">
                  {(item.status === "ready" ||
                    item.status === "ready-receipt" ||
                    item.status === "ready-converted") && (
                    <Button
                      variant={
                        (item.status === "ready-converted"
                          ? selectedConverted
                          : selected
                        ).includes(item.receipt)
                          ? "primary"
                          : "outline-secondary"
                      }
                      size="sm"
                      disabled={busy}
                      aria-pressed={(item.status === "ready-converted"
                        ? selectedConverted
                        : selected
                      ).includes(item.receipt)}
                      onClick={() =>
                        toggleSelected(
                          item.receipt,
                          item.status === "ready-converted"
                        )
                      }
                    >
                      {(item.status === "ready-converted"
                        ? selectedConverted
                        : selected
                      ).includes(item.receipt)
                        ? "Selected"
                        : "Select"}
                    </Button>
                  )}
                  <div className="small text-break">{item.oldPath}</div>
                </div>
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
          {(report?.items.length ?? 0) > visibleCount && (
            <Button
              variant="outline-secondary"
              size="sm"
              disabled={busy}
              onClick={() => setVisibleCount((count) => count + 150)}
            >
              Show more results
            </Button>
          )}
          {(report?.convertedEligible ?? 0) > 0 && (
            <Form.Check
              id="booru-conversion-history-migration"
              type="checkbox"
              disabled={busy}
              checked={acceptHistoryMigration}
              onChange={(event) =>
                setAcceptHistoryMigration(event.target.checked)
              }
              label="I backed up my library and understand that converted-file renaming updates both native media and conversion history, but is not crash-atomic."
            />
          )}
        </Modal.Body>
        <Modal.Footer>
          <Button
            variant="outline-secondary"
            disabled={busy}
            onClick={closeForSession}
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
            disabled={busy || selectedEligible.length === 0}
            onClick={() => void apply(false)}
          >
            Apply {selectedEligible.length} original-media update(s)
          </Button>
          <Button
            variant="warning"
            disabled={
              busy ||
              !acceptHistoryMigration ||
              selectedConvertedEligible.length === 0
            }
            onClick={() => void apply(true)}
          >
            Rename {selectedConvertedEligible.length} converted file(s)
          </Button>
        </Modal.Footer>
      </Modal>
    </>
  );
}
