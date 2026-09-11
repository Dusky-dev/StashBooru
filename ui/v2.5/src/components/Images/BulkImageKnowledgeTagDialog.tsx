import React, { useCallback, useState } from "react";
import { Button, Form, Modal } from "react-bootstrap";

import { useToast } from "src/hooks/Toast";

interface IProps {
  imageIds: string[];
  all?: boolean;
  totalCount?: number;
  onHide: () => void;
}

interface CamieBulkResponse {
  jobID: number;
}

const DEFAULT_THRESHOLD = "0.492";
const DEFAULT_LIMIT = "50";

async function readResponse<T>(response: Response): Promise<T> {
  if (!response.ok) {
    throw new Error((await response.text()) || response.statusText);
  }
  return response.json() as Promise<T>;
}

export const BulkImageKnowledgeTagDialog: React.FC<IProps> = ({
  imageIds,
  all = false,
  totalCount,
  onHide,
}) => {
  const Toast = useToast();
  const [threshold, setThreshold] = useState(DEFAULT_THRESHOLD);
  const [limit, setLimit] = useState(DEFAULT_LIMIT);
  const [applyCharacters, setApplyCharacters] = useState(true);
  const [applyArtist, setApplyArtist] = useState(true);
  const [applyTags, setApplyTags] = useState(true);
  const [replaceArtist, setReplaceArtist] = useState(false);
  const [starting, setStarting] = useState(false);
  const [error, setError] = useState<string>();

  const imageCount = all ? totalCount : imageIds.length;

  const start = useCallback(async () => {
    const parsedThreshold = Number.parseFloat(threshold);
    const parsedLimit = Number.parseInt(limit, 10);
    if (!(parsedThreshold > 0 && parsedThreshold < 1)) {
      setError("Threshold must be greater than 0 and less than 1.");
      return;
    }
    if (!(parsedLimit >= 1 && parsedLimit <= 200)) {
      setError("Per-category limit must be between 1 and 200.");
      return;
    }
    if (!applyCharacters && !applyArtist && !applyTags) {
      setError("Enable at least one metadata category.");
      return;
    }

    setStarting(true);
    setError(undefined);
    try {
      const response = await fetch("image/visual-similarity/camie/tag", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          imageIDs: all ? [] : imageIds.map((id) => Number.parseInt(id, 10)),
          all,
          threshold: parsedThreshold,
          limit: parsedLimit,
          applyCharacters,
          applyArtist,
          applyTags,
          replaceArtist,
        }),
      });
      const result = await readResponse<CamieBulkResponse>(response);
      Toast.success(`Started Camie tagging job #${result.jobID}.`);
      onHide();
    } catch (cause) {
      const message = cause instanceof Error ? cause.message : String(cause);
      setError(message);
      Toast.error(cause);
    } finally {
      setStarting(false);
    }
  }, [
    Toast,
    all,
    applyArtist,
    applyCharacters,
    applyTags,
    imageIds,
    limit,
    onHide,
    replaceArtist,
    threshold,
  ]);

  return (
    <Modal show onHide={onHide} centered>
      <Modal.Header closeButton>
        <Modal.Title>Camie tag images</Modal.Title>
      </Modal.Header>
      <Modal.Body>
        <p>
          {all
            ? `Run Camie on all ${imageCount ?? ""} images in the library.`
            : `Run Camie on the ${imageIds.length} selected image${imageIds.length === 1 ? "" : "s"}.`}
          {" "}This runs as a background task, so you can leave this page after
          starting it.
        </p>

        <div className="d-flex flex-wrap align-items-end mb-3">
          <Form.Group className="mr-3 mb-2">
            <Form.Label>Threshold</Form.Label>
            <Form.Control
              type="number"
              min="0.001"
              max="0.999"
              step="0.01"
              value={threshold}
              onChange={(event) => setThreshold(event.currentTarget.value)}
              style={{ width: "8rem" }}
            />
          </Form.Group>
          <Form.Group className="mb-2">
            <Form.Label>Per-category limit</Form.Label>
            <Form.Control
              type="number"
              min="1"
              max="200"
              value={limit}
              onChange={(event) => setLimit(event.currentTarget.value)}
              style={{ width: "8rem" }}
            />
          </Form.Group>
        </div>

        <Form.Check
          className="mb-2"
          type="checkbox"
          id="camie-bulk-characters"
          checked={applyCharacters}
          onChange={(event) => setApplyCharacters(event.currentTarget.checked)}
          label="Apply character predictions as Characters"
        />
        <Form.Check
          className="mb-2"
          type="checkbox"
          id="camie-bulk-artist"
          checked={applyArtist}
          onChange={(event) => setApplyArtist(event.currentTarget.checked)}
          label="Apply the highest-confidence artist as Artist"
        />
        <Form.Check
          className="mb-3"
          type="checkbox"
          id="camie-bulk-tags"
          checked={applyTags}
          onChange={(event) => setApplyTags(event.currentTarget.checked)}
          label="Apply copyright, general, meta and other predictions as Tags"
        />
        <Form.Check
          className="mb-3"
          type="checkbox"
          id="camie-bulk-replace-artist"
          checked={replaceArtist}
          disabled={!applyArtist}
          onChange={(event) => setReplaceArtist(event.currentTarget.checked)}
          label="Replace an existing Artist when Camie predicts one"
        />

        <div className="small text-muted mb-3">
          Existing Characters and Tags are preserved and only added to. Existing
          Artists are preserved unless replacement is enabled above.
        </div>

        {all ? (
          <div className="alert alert-warning">
            This will automatically apply Camie predictions to the entire image
            library. Review the threshold before starting.
          </div>
        ) : null}
        {error ? <div className="alert alert-danger">{error}</div> : null}
      </Modal.Body>
      <Modal.Footer>
        <Button variant="secondary" disabled={starting} onClick={onHide}>
          Cancel
        </Button>
        <Button variant="primary" disabled={starting} onClick={() => void start()}>
          {starting ? "Starting…" : "Start tagging job"}
        </Button>
      </Modal.Footer>
    </Modal>
  );
};
