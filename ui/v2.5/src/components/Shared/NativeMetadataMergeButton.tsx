import React, { useState } from "react";
import { Button, Modal } from "react-bootstrap";
import {
  Copyright,
  CopyrightSelect,
} from "src/components/Copyrights/CopyrightSelect";
import { Studio, StudioSelect } from "src/components/Studios/StudioSelect";

interface Props {
  kind: "artist" | "copyright";
  source: { id: string; name: string };
  onMerged: (destinationID: string) => void;
}

/** The current profile is always the source and is removed after merging. */
export const NativeMetadataMergeButton: React.FC<Props> = ({
  kind,
  source,
  onMerged,
}) => {
  const [show, setShow] = useState(false);
  const [destination, setDestination] = useState<{
    id: string;
    name: string;
  }>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const label = kind === "artist" ? "Artist" : "Copyright";

  async function merge() {
    if (!destination || busy || source.id === destination.id) return;
    setBusy(true);
    setError("");
    try {
      const endpoint =
        kind === "artist" ? "/studio/merge" : "/tag/copyright/merge";
      const result = await fetch(endpoint, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          sourceID: Number(source.id),
          destinationID: Number(destination.id),
        }),
      });
      if (!result.ok) throw new Error(await result.text());
      setShow(false);
      onMerged(destination.id);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <Button
        variant="secondary"
        onClick={() => {
          setError("");
          setDestination(undefined);
          setShow(true);
        }}
      >
        Merge {label}…
      </Button>
      <Modal show={show} onHide={() => !busy && setShow(false)}>
        <Modal.Header closeButton={!busy}>
          <Modal.Title>Merge {label}s</Modal.Title>
        </Modal.Header>
        <Modal.Body>
          <p>
            Merge <strong>{source.name}</strong> into another {label}. The
            destination is retained and this source profile is removed.
          </p>
          <p className="text-muted">
            Linked images, videos, identities, and compatible metadata are
            transferred. Conflicting profile fields prevent the merge rather
            than being overwritten.
          </p>
          <label
            className="d-block"
            htmlFor="native-metadata-merge-destination"
          >
            Destination {label}
          </label>
          <div id="native-metadata-merge-destination">
            {kind === "artist" ? (
              <StudioSelect
                values={destination ? [destination as Studio] : []}
                onSelect={(items) => setDestination(items[0])}
                excludeIds={[source.id]}
                creatable={false}
              />
            ) : (
              <CopyrightSelect
                values={destination ? [destination as Copyright] : []}
                onSelect={(items) => setDestination(items[0])}
                excludeIds={[source.id]}
                creatable={false}
              />
            )}
          </div>
          {destination && (
            <p className="mt-3">
              Source <strong>{source.name}</strong> → Destination{" "}
              <strong>{destination.name}</strong>
            </p>
          )}
          {error && <div className="alert alert-danger">{error}</div>}
        </Modal.Body>
        <Modal.Footer>
          <Button
            variant="secondary"
            disabled={busy}
            onClick={() => setShow(false)}
          >
            Cancel
          </Button>
          <Button
            variant="danger"
            disabled={!destination || busy || destination.id === source.id}
            onClick={() => void merge()}
          >
            {busy ? "Merging…" : "Confirm merge"}
          </Button>
        </Modal.Footer>
      </Modal>
    </>
  );
};
