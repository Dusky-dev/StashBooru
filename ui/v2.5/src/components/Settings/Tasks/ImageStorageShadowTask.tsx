import React, { useCallback, useEffect, useState } from "react";
import { Alert, Button, Col, Form, Row } from "react-bootstrap";

interface CandidateList {
  candidates?: {
    id: string;
    group: string;
    method: string;
    snapshot: string;
  }[];
}

interface ShadowMemberList {
  readOnly: boolean;
  sourcesRetained: boolean;
  canActivate: boolean;
  originalEvictionEnabled: boolean;
  candidateID: string;
  stackID: number;
  members: { imageID: number }[];
}

interface ShadowSelectionStatus {
  State: string;
  CandidateID: string;
  StackID: number;
  Revision: number;
  EvidenceVerifiedNow: boolean;
  RequiresFreshByteCheck: boolean;
  OriginalsAuthoritative: boolean;
  NativeRoutingEnabled: boolean;
  CanActivate: boolean;
  OriginalEvictionEnabled: boolean;
  TransactionalRollbackVerified: boolean;
}

interface Comparison {
  key: string;
  originalURL: string;
  shadowURL: string;
  downloadURL: string;
  clipPreviewURL: string;
  thumbnailURL: string;
  detailPreviewURL: string;
  nativeImageURL: string;
  nativeThumbnailURL: string;
  imageID: string;
  method: string;
}

export const ImageStorageShadowTask: React.FC = () => {
  const [candidates, setCandidates] = useState<
    NonNullable<CandidateList["candidates"]>
  >([]);
  const [selectedID, setSelectedID] = useState("");
  const [imageID, setImageID] = useState("");
  const [members, setMembers] = useState<number[]>([]);
  const [memberLoading, setMemberLoading] = useState(false);
  const [memberError, setMemberError] = useState("");
  const [error, setError] = useState("");
  const [comparison, setComparison] = useState<Comparison>();
  const [originalStatus, setOriginalStatus] = useState("pending");
  const [shadowStatus, setShadowStatus] = useState("pending");
  const [thumbnailStatus, setThumbnailStatus] = useState("pending");
  const [shadowSelection, setShadowSelection] =
    useState<ShadowSelectionStatus>();
  const [selectionBusy, setSelectionBusy] = useState(false);
  const [selectionError, setSelectionError] = useState("");
  const [selectionAcknowledged, setSelectionAcknowledged] = useState(false);

  const refresh = useCallback(async (signal: AbortSignal) => {
    const response = await fetch("image/storage-candidate", { signal });
    if (!response.ok) throw new Error("Saved candidate list is unavailable.");
    const data = (await response.json()) as CandidateList;
    if (signal.aborted) return;
    const items = data.candidates ?? [];
    setCandidates(items);
    setSelectedID((previous) =>
      items.some((item) => item.id === previous)
        ? previous
        : (items[0]?.id ?? "")
    );
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    void refresh(controller.signal).catch((reason: unknown) => {
      if (!controller.signal.aborted) {
        setError(reason instanceof Error ? reason.message : String(reason));
      }
    });
    return () => controller.abort();
  }, [refresh]);

  const candidate = candidates.find((item) => item.id === selectedID);
  const stackID = candidate?.group.match(/^stack:([1-9]\d*)$/)?.[1];

  const loadShadowSelection = useCallback(async (signal?: AbortSignal) => {
    const response = await fetch("image/storage-shadow-selection", { signal });
    if (!response.ok)
      throw new Error("Saved shadow trial state is unavailable or unsafe.");
    const status = (await response.json()) as ShadowSelectionStatus;
    if (
      !status.OriginalsAuthoritative ||
      status.NativeRoutingEnabled ||
      status.CanActivate ||
      status.OriginalEvictionEnabled ||
      status.TransactionalRollbackVerified
    ) {
      throw new Error(
        "Saved shadow trial state unexpectedly enables storage activation."
      );
    }
    if (!signal?.aborted) setShadowSelection(status);
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    void loadShadowSelection(controller.signal).catch((reason: unknown) => {
      if (!controller.signal.aborted) {
        setSelectionError(
          reason instanceof Error ? reason.message : String(reason)
        );
      }
    });
    return () => controller.abort();
  }, [loadShadowSelection]);

  async function changeShadowSelection(action: "prepare" | "disarm") {
    if (
      selectionBusy ||
      !selectionAcknowledged ||
      (action === "prepare" && (!selectedID || !stackID || members.length < 2))
    )
      return;
    setSelectionBusy(true);
    setSelectionError("");
    try {
      const response = await fetch("image/storage-shadow-selection", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          Action: action,
          ...(action === "prepare"
            ? { CandidateID: selectedID, StackID: Number(stackID) }
            : {}),
          Acknowledged: true,
        }),
      });
      if (!response.ok)
        throw new Error("Could not safely " + action + " shadow trial intent.");
      const status = (await response.json()) as ShadowSelectionStatus;
      if (
        !status.OriginalsAuthoritative ||
        status.NativeRoutingEnabled ||
        status.CanActivate ||
        status.OriginalEvictionEnabled ||
        status.TransactionalRollbackVerified
      ) {
        throw new Error("Unsafe shadow trial response was refused.");
      }
      setShadowSelection(status);
      setSelectionAcknowledged(false);
    } catch (reason) {
      setSelectionError(
        reason instanceof Error ? reason.message : String(reason)
      );
    } finally {
      setSelectionBusy(false);
    }
  }

  useEffect(() => {
    const controller = new AbortController();
    setMembers([]);
    setMemberError("");
    setImageID("");
    setComparison(undefined);
    if (!stackID || !selectedID) {
      setMemberLoading(false);
      return () => controller.abort();
    }
    setMemberLoading(true);
    const params = new URLSearchParams({ candidateID: selectedID, stackID });
    void fetch("image/storage-shadow-members?" + params, {
      signal: controller.signal,
    })
      .then(async (response) => {
        if (!response.ok) {
          throw new Error("Saved candidate members could not be verified.");
        }
        const data = (await response.json()) as ShadowMemberList;
        if (
          data.candidateID !== selectedID ||
          data.stackID !== Number(stackID) ||
          !data.readOnly ||
          !data.sourcesRetained ||
          data.canActivate ||
          data.originalEvictionEnabled ||
          !Array.isArray(data.members) ||
          data.members.length < 2 ||
          data.members.length > 16
        ) {
          throw new Error("Candidate member proof was unexpected or unsafe.");
        }
        const ids = data.members.map((entry) => entry.imageID);
        if (
          ids.some((id) => !Number.isSafeInteger(id) || id <= 0) ||
          new Set(ids).size !== ids.length
        ) {
          throw new Error("Candidate returned invalid image IDs.");
        }
        if (!controller.signal.aborted) {
          setMembers(ids);
          setImageID(String(ids[0]));
        }
      })
      .catch((reason: unknown) => {
        if (!controller.signal.aborted) {
          setMemberError(
            reason instanceof Error ? reason.message : String(reason)
          );
        }
      })
      .finally(() => {
        if (!controller.signal.aborted) setMemberLoading(false);
      });
    return () => controller.abort();
  }, [selectedID, stackID]);

  const validImageID =
    /^[1-9]\d*$/.test(imageID) && Number.isSafeInteger(Number(imageID));
  const params = new URLSearchParams();
  if (stackID && validImageID && candidate) {
    params.set("stackID", stackID);
    params.set("imageID", imageID);
    params.set("candidateID", candidate.id);
  }
  const shadowURL = params.size === 3 ? "image/storage-shadow?" + params : "";

  function showComparison() {
    if (!shadowURL || !candidate) return;
    setOriginalStatus("pending");
    setShadowStatus("pending");
    setThumbnailStatus("pending");
    setComparison({
      key: candidate.id + ":" + imageID,
      imageID,
      method: candidate.method,
      originalURL: "image/" + imageID + "/image",
      shadowURL,
      downloadURL: "image/storage-shadow-download?" + params,
      clipPreviewURL: "image/" + imageID + "/preview",
      thumbnailURL: "image/storage-shadow-thumbnail?" + params,
      nativeImageURL:
        "image/" +
        imageID +
        "/image?storageShadowCandidate=" +
        encodeURIComponent(candidate.id) +
        "&storageShadowStack=" +
        encodeURIComponent(stackID ?? ""),
      nativeThumbnailURL:
        "image/" +
        imageID +
        "/thumbnail?storageShadowCandidate=" +
        encodeURIComponent(candidate.id) +
        "&storageShadowStack=" +
        encodeURIComponent(stackID ?? ""),
      detailPreviewURL:
        "/images/" +
        imageID +
        "?storageShadowCandidate=" +
        encodeURIComponent(candidate.id) +
        "&storageShadowStack=" +
        encodeURIComponent(stackID ?? ""),
    });
  }

  function clearComparison() {
    setComparison(undefined);
    setOriginalStatus("pending");
    setShadowStatus("pending");
    setThumbnailStatus("pending");
  }

  return (
    <section aria-labelledby="image-storage-shadow-title">
      <h2 id="image-storage-shadow-title">Compare packed shadow image</h2>
      <p>
        Browse the verified image members of a saved P12/P13 candidate. Compare
        the original and byte-exact archived reconstruction side by side. Each
        shadow request independently rechecks the candidate and current original
        before returning reconstructed bytes.
      </p>
      <Alert variant="warning">
        Experimental per-request testing only. Opt-in links also exercise normal
        image and thumbnail URLs, but other requests still use originals. The
        native thumbnail probe is isolated and does not write cached files.
        Global packed-image serving, generated previews, and original deletion
        remain disabled.
      </Alert>
      <div className="border rounded p-3 mb-3">
        <h3>Saved shadow trial intent (does not activate storage)</h3>
        <p className="text-muted small">
          Preparing saves a reversible selection for future validation. It never
          redirects ordinary image requests, changes originals, or reclaims disk
          space. Disarm cancels that saved intent.
        </p>
        <p role="status">
          Saved intent: {shadowSelection?.State ?? "loading"}.
          {shadowSelection?.CandidateID &&
            " Candidate: " + shadowSelection.CandidateID + "."}
          {shadowSelection?.State === "prepared" &&
            (shadowSelection.EvidenceVerifiedNow
              ? " Candidate archive and catalogue evidence checked; byte reconstruction is still required."
              : " Candidate evidence is stale or cannot be verified.")}
        </p>
        {selectionError && <Alert variant="danger">{selectionError}</Alert>}
        <Form.Check
          id="shadow-intent-acknowledge"
          checked={selectionAcknowledged}
          onChange={(event) =>
            setSelectionAcknowledged(event.currentTarget.checked)
          }
          label="I understand this is a non-activating experiment and originals remain authoritative."
        />
        <div className="d-flex flex-wrap gap-2 mt-2">
          <Button
            variant="outline-secondary"
            disabled={
              selectionBusy ||
              !selectionAcknowledged ||
              !selectedID ||
              !stackID ||
              memberLoading ||
              members.length < 2
            }
            onClick={() => void changeShadowSelection("prepare")}
          >
            Prepare candidate (no activation)
          </Button>
          <Button
            variant="outline-secondary"
            disabled={
              selectionBusy ||
              !selectionAcknowledged ||
              !shadowSelection ||
              shadowSelection.State === "not_prepared"
            }
            onClick={() => void changeShadowSelection("disarm")}
          >
            Disarm saved trial
          </Button>
        </div>
      </div>

      {error && <Alert variant="danger">{error}</Alert>}
      {memberError && <Alert variant="warning">{memberError}</Alert>}
      <Form.Group>
        <Form.Label htmlFor="shadow-candidate">Verified candidate</Form.Label>
        <Form.Control
          id="shadow-candidate"
          as="select"
          value={selectedID}
          onChange={(event) => {
            setSelectedID(event.currentTarget.value);
            setImageID("");
            clearComparison();
          }}
        >
          {!selectedID && <option value="">No saved candidates</option>}
          {candidates.map((item) => (
            <option key={item.id} value={item.id}>
              {item.group} — {item.method} — {item.snapshot}
            </option>
          ))}
        </Form.Control>
      </Form.Group>
      <Form.Group>
        <Form.Label htmlFor="shadow-image-id">
          Image in this saved candidate
        </Form.Label>
        <Form.Control
          id="shadow-image-id"
          as="select"
          value={imageID}
          disabled={memberLoading || members.length === 0}
          onChange={(event) => {
            setImageID(event.currentTarget.value);
            clearComparison();
          }}
        >
          {members.length === 0 && (
            <option value="">
              {memberLoading
                ? "Verifying candidate members…"
                : "No verified members"}
            </option>
          )}
          {members.map((id, index) => (
            <option key={id} value={String(id)}>
              Image {id} ({index + 1} of {members.length})
            </option>
          ))}
        </Form.Control>
      </Form.Group>
      <div className="d-flex flex-wrap gap-2 mt-2">
        <Button
          variant="secondary"
          disabled={!shadowURL}
          onClick={showComparison}
        >
          Compare original and packed reconstruction
        </Button>
        <Button
          variant="outline-secondary"
          disabled={members.indexOf(Number(imageID)) <= 0}
          onClick={() => {
            const index = members.indexOf(Number(imageID));
            if (index > 0) {
              setImageID(String(members[index - 1]));
              clearComparison();
            }
          }}
        >
          Previous image
        </Button>
        <Button
          variant="outline-secondary"
          disabled={
            members.length === 0 ||
            members.indexOf(Number(imageID)) < 0 ||
            members.indexOf(Number(imageID)) >= members.length - 1
          }
          onClick={() => {
            const index = members.indexOf(Number(imageID));
            if (index >= 0 && index < members.length - 1) {
              setImageID(String(members[index + 1]));
              clearComparison();
            }
          }}
        >
          Next image
        </Button>
      </div>
      {!shadowURL && (
        <p className="text-muted">
          Select a saved candidate to browse its verified image members.
        </p>
      )}
      {comparison && (
        <div className="mt-3" key={comparison.key}>
          <p>
            Image {comparison.imageID}, {comparison.method} candidate. Both
            previews are fetched on demand; the existing original is never
            replaced.
          </p>
          <Row>
            <Col md={6}>
              <h3>Original image</h3>
              <img
                src={comparison.originalURL}
                alt={"Original image " + comparison.imageID}
                style={{ width: "100%", maxHeight: 420, objectFit: "contain" }}
                onLoad={() => setOriginalStatus("loaded")}
                onError={() => setOriginalStatus("failed or unsupported")}
              />
              <p role="status">Original preview: {originalStatus}</p>
              <a
                href={comparison.originalURL}
                target="_blank"
                rel="noopener noreferrer"
              >
                Open original
              </a>
            </Col>
            <Col md={6}>
              <h3>Archived reconstruction</h3>
              <img
                src={comparison.shadowURL}
                alt={
                  "Byte-exact archived reconstruction of " + comparison.imageID
                }
                style={{ width: "100%", maxHeight: 420, objectFit: "contain" }}
                onLoad={() => setShadowStatus("loaded")}
                onError={() => setShadowStatus("failed or unsupported")}
              />
              <p role="status">Shadow preview: {shadowStatus}</p>
              <a
                href={comparison.shadowURL}
                target="_blank"
                rel="noopener noreferrer"
              >
                Open reconstructed image
              </a>
              {" · "}
              <a href={comparison.detailPreviewURL}>
                Test packed preview in image details
              </a>
              {" · "}
              <a
                href={comparison.nativeImageURL}
                target="_blank"
                rel="noopener noreferrer"
              >
                Test normal image route (explicit packed read)
              </a>
              {" · "}
              <a href={comparison.downloadURL}>
                Download verified original-format bytes
              </a>
            </Col>
          </Row>
          <Row className="mt-3">
            <Col md={6}>
              <h3>Native thumbnail from archived bytes</h3>
              <img
                src={comparison.thumbnailURL}
                alt={"Isolated native thumbnail for " + comparison.imageID}
                style={{ width: "100%", maxHeight: 320, objectFit: "contain" }}
                onLoad={() => setThumbnailStatus("loaded")}
                onError={() =>
                  setThumbnailStatus("unsupported format or decoding error")
                }
              />
              <p role="status">Thumbnail probe: {thumbnailStatus}</p>
              <a
                href={comparison.thumbnailURL}
                target="_blank"
                rel="noopener noreferrer"
              >
                Open isolated JPEG thumbnail
              </a>
              {" · "}
              <a
                href={comparison.nativeThumbnailURL}
                target="_blank"
                rel="noopener noreferrer"
              >
                Test normal thumbnail route (explicit packed read)
              </a>
            </Col>
          </Row>
          <p className="text-muted small">
            This probe uses StashBooru's native in-memory thumbnail encoder. GIF
            animation, animated WebP, JXL, and AVIF path-only decoding are not
            supported in this isolated mode. The regular thumbnail and generated
            preview routes remain unchanged.
          </p>
          <p className="text-muted small mt-2">
            The normal clip preview is a separately generated cache file.
            Packed-byte preview generation is not enabled here. The existing
            native clip preview can be inspected without changing it:{" "}
            <a
              href={comparison.clipPreviewURL}
              target="_blank"
              rel="noopener noreferrer"
            >
              Open existing clip preview (may be missing)
            </a>
          </p>
          <p className="text-muted small mt-2">
            A browser decoding failure can also mean an unsupported image format
            (such as JXL or AVIF), not necessarily corrupt archive data. Use the
            links above for format-specific inspection. Visual comparison is not
            a substitute for the backend SHA-256 proof.
          </p>
        </div>
      )}
    </section>
  );
};
