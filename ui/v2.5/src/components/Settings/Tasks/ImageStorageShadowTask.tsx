import React, { useCallback, useEffect, useState } from "react";
import { Alert, Form } from "react-bootstrap";

interface CandidateList {
  candidates?: {
    id: string;
    group: string;
    method: string;
    snapshot: string;
  }[];
}

export const ImageStorageShadowTask: React.FC = () => {
  const [candidates, setCandidates] = useState<
    NonNullable<CandidateList["candidates"]>
  >([]);
  const [selectedID, setSelectedID] = useState("");
  const [imageID, setImageID] = useState("");
  const [error, setError] = useState("");

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
      if (!controller.signal.aborted)
        setError(reason instanceof Error ? reason.message : String(reason));
    });
    return () => controller.abort();
  }, [refresh]);

  const candidate = candidates.find((item) => item.id === selectedID);
  const stackID = candidate?.group.match(/^stack:([1-9]\d*)$/)?.[1];
  const validImageID =
    /^[1-9]\d*$/.test(imageID) && Number.isSafeInteger(Number(imageID));
  const params = new URLSearchParams();
  if (stackID && validImageID && candidate) {
    params.set("stackID", stackID);
    params.set("imageID", imageID);
    params.set("candidateID", candidate.id);
  }
  const url = params.size === 3 ? "image/storage-shadow?" + params : "";

  return (
    <section aria-labelledby="image-storage-shadow-title">
      <h2 id="image-storage-shadow-title">Compare packed shadow image</h2>
      <p>
        Open a single P12/P13 archived image for byte-exact comparison against
        its still-active original. The archive is rehashed and the original is
        read again before any bytes are delivered.
      </p>
      <Alert variant="warning">
        Experimental, on-demand comparison only. This does not enable packed
        thumbnails, previews, normal image serving, live activation or
        original-file removal.
      </Alert>
      {error && <Alert variant="danger">{error}</Alert>}
      <Form.Group>
        <Form.Label htmlFor="shadow-candidate">Verified candidate</Form.Label>
        <Form.Control
          id="shadow-candidate"
          as="select"
          value={selectedID}
          onChange={(event) => {
            setSelectedID(event.currentTarget.value);
            setImageID("");
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
          Image ID in this stack
        </Form.Label>
        <Form.Control
          id="shadow-image-id"
          inputMode="numeric"
          value={imageID}
          placeholder="123"
          onChange={(event) => setImageID(event.currentTarget.value)}
        />
      </Form.Group>
      {url ? (
        <a href={url} target="_blank" rel="noopener noreferrer">
          Open shadow image (read-only)
        </a>
      ) : (
        <p className="text-muted">
          Select a saved candidate and enter the ID of one image in its stack.
        </p>
      )}
    </section>
  );
};
