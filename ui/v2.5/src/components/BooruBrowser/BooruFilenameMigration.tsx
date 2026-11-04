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
  acceptRestoreLoss = false
): Promise<T> {
  return fetch(endpoint, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ action, receipts, acceptRestoreLoss }),
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
  const [selectedJXL, setSelectedJXL] = useState<string[]>([]);
  const [acceptRestoreLoss, setAcceptRestoreLoss] = useState(false);
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
      setSelectedJXL([]);
      setAcceptRestoreLoss(false);
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
      (converted && !acceptRestoreLoss)
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
          converted ? "apply-jxl" : "apply",
          ready.slice(offset, offset + 20),
          converted && acceptRestoreLoss
        );
        succeeded += response.applied.length;
        failures += Object.keys(response.failures).length;
      }
      setReport(await readReport());
      setSelected([]);
      setSelectedJXL([]);
      setAcceptRestoreLoss(false);
      setNotice(
        "Renamed " +
          succeeded +
          " file(s) without re-encoding." +
          (converted
            ? " Conversion restoration for these JXL files is no longer supported."
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
    (item) => item.status === "ready"
  );
  const eligibleNames = new Set(eligible.map((item) => item.receipt));
  const convertedEligible = (report?.items ?? []).filter(
    (item) => item.status === "ready-converted"
  );
  const convertedNames = new Set(convertedEligible.map((item) => item.receipt));
  const selectedEligible = selected.filter((receipt) =>
    eligibleNames.has(receipt)
  );
  const selectedConvertedEligible = selectedJXL.filter((receipt) =>
    convertedNames.has(receipt)
  );
  const toggleSelected = (receipt: string, converted = false) => {
    const setter = converted ? setSelectedJXL : setSelected;
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
            Eligible unconverted renames: {report?.eligible ?? 0}. Eligible JXL
            renames: {report?.convertedEligible ?? 0}. Additional review:{" "}
            {report?.review ?? 0}.
          </p>
          <Alert variant="info">
            Filename changes do not re-encode media or alter tags. Directory
            moves and ambiguous metadata remain review-only.
          </Alert>
          {(report?.convertedEligible ?? 0) > 0 && (
            <Alert variant="warning">
              <strong>JXL restore warning:</strong> Renaming a converted JXL
              keeps its image and native library associations, but its saved
              conversion journal will still reference the old filename.
              Restoring the original JPEG through StashBooru will no longer work
              for renamed JXL files. Back up the database, media, and conversion
              history first. JXL files are never selected automatically.
            </Alert>
          )}
          {error && <Alert variant="warning">{error}</Alert>}
          {notice && <Alert variant="success">{notice}</Alert>}
          {report?.truncated && (
            <Alert variant="warning">Only 2,000 receipts were checked.</Alert>
          )}
          <div className="d-flex align-items-center justify-content-between gap-2 mb-2">
            <span className="small">
              Selected {selectedEligible.length} regular and{" "}
              {selectedConvertedEligible.length} JXL
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
                  setSelectedJXL(convertedEligible.map((item) => item.receipt))
                }
              >
                Select eligible JXL
              </Button>
              <Button
                variant="outline-secondary"
                size="sm"
                disabled={
                  busy || (selected.length === 0 && selectedJXL.length === 0)
                }
                onClick={() => {
                  setSelected([]);
                  setSelectedJXL([]);
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
                    item.status === "ready-converted") && (
                    <Button
                      variant={
                        (item.status === "ready-converted"
                          ? selectedJXL
                          : selected
                        ).includes(item.receipt)
                          ? "primary"
                          : "outline-secondary"
                      }
                      size="sm"
                      disabled={busy}
                      aria-pressed={(item.status === "ready-converted"
                        ? selectedJXL
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
                        ? selectedJXL
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
              id="booru-jxl-restore-loss"
              type="checkbox"
              disabled={busy}
              checked={acceptRestoreLoss}
              onChange={(event) => setAcceptRestoreLoss(event.target.checked)}
              label="I understand that restoring originals through conversion history will no longer work for renamed JXL files."
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
            Rename {selectedEligible.length} regular file(s)
          </Button>
          <Button
            variant="warning"
            disabled={
              busy ||
              !acceptRestoreLoss ||
              selectedConvertedEligible.length === 0
            }
            onClick={() => void apply(true)}
          >
            Rename {selectedConvertedEligible.length} JXL file(s)
          </Button>
        </Modal.Footer>
      </Modal>
    </>
  );
}
