import { useCallback, useEffect, useMemo, useState } from "react";
import { Alert, Button, Form, Modal, Spinner } from "react-bootstrap";
import { Link } from "react-router-dom";

import * as GQL from "src/core/generated-graphql";
import {
  ReferenceComparison,
  type ReferenceComparisonMode,
} from "src/hooks/Lightbox/ReferenceComparison";
import type { ILightboxImage } from "src/hooks/Lightbox/types";
import "src/hooks/Lightbox/lightbox.scss";

import {
  duplicateComparisonPair,
  readableFileSize,
  type SimilarImageGroup,
} from "./review";

type ComparisonMode = Exclude<ReferenceComparisonMode, "selected">;

interface IProps {
  group: SimilarImageGroup;
  initialImageID: number;
  keptIDs: number[];
  onToggleKeep: (imageID: number) => void;
  onClose: () => void;
}

function comparisonImage(image?: ILightboxImage): ILightboxImage | undefined {
  if (!image) return undefined;
  // Browser-native JPEG XL support is not universal; prefer Stash's generated
  // preview for JXL rather than showing a blank original in the comparison.
  const filename = image.visual_files?.[0]?.path ?? "";
  if (/\.jxl$/i.test(filename) && image.paths.preview) {
    return { ...image, paths: { ...image.paths, image: image.paths.preview } };
  }
  return image;
}

export function DuplicateComparison({
  group,
  initialImageID,
  keptIDs,
  onToggleKeep,
  onClose,
}: IProps) {
  const initialPair = duplicateComparisonPair(group, initialImageID);
  const [referenceID, setReferenceID] = useState(initialPair.referenceID);
  const [selectedID, setSelectedID] = useState(initialPair.selectedID);
  const [mode, setMode] = useState<ComparisonMode>("both");
  const [zoom, setZoom] = useState(1);
  const [resetPosition, setResetPosition] = useState(false);
  const ids = useMemo(() => group.members.map((member) => member.id), [group]);
  const referenceMember = group.members.find((member) => member.id === referenceID);
  const selectedMember = group.members.find((member) => member.id === selectedID);
  const selectedIndex = ids.indexOf(selectedID);

  const referenceQuery = GQL.useFindImageQuery({
    variables: { id: String(referenceID) },
    skip: !referenceID,
  });
  const selectedQuery = GQL.useFindImageQuery({
    variables: { id: String(selectedID) },
    skip: !selectedID,
  });
  const reference = comparisonImage(
    referenceQuery.data?.findImage as ILightboxImage | undefined
  );
  const selected = comparisonImage(
    selectedQuery.data?.findImage as ILightboxImage | undefined
  );

  const changeReference = (id: number) => {
    if (!ids.includes(id) || id === referenceID) return;
    if (id === selectedID) {
      setSelectedID(referenceID);
    }
    setReferenceID(id);
    setZoom(1);
    setResetPosition((value) => !value);
  };

  const changeSelected = useCallback(
    (id: number) => {
      if (id !== referenceID && ids.includes(id)) setSelectedID(id);
    },
    [ids, referenceID]
  );

  const stepSelected = useCallback(
    (direction: number) => {
      if (ids.length < 2) return;
      let next = selectedIndex;
      do {
        next = (next + direction + ids.length) % ids.length;
      } while (ids[next] === referenceID && next !== selectedIndex);
      changeSelected(ids[next]);
    },
    [changeSelected, ids, referenceID, selectedIndex]
  );

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.defaultPrevented || event.altKey || event.ctrlKey || event.metaKey) return;
      const target = event.target;
      if (
        target instanceof HTMLElement &&
        (target.closest("button, input, select, textarea, [role='slider']") ||
          target.isContentEditable)
      ) {
        return;
      }
      if (event.key === "ArrowLeft" || event.key === "ArrowRight") {
        event.preventDefault();
        stepSelected(event.key === "ArrowRight" ? 1 : -1);
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [stepSelected]);

  const loading = referenceQuery.loading || selectedQuery.loading;
  const error = referenceQuery.error || selectedQuery.error;
  const comparisonReady = reference && selected;

  return (
    <Modal
      show
      onHide={onClose}
      centered
      size="xl"
      dialogClassName="duplicate-review-preview-modal"
    >
      <Modal.Header closeButton>
        <Modal.Title>Compare duplicates</Modal.Title>
      </Modal.Header>
      <Modal.Body className="duplicate-compare-body">
        <div className="duplicate-compare-toolbar">
          <Form.Group controlId="duplicate-compare-reference">
            <Form.Label>Reference image</Form.Label>
            <Form.Control
              as="select"
              value={referenceID}
              onChange={(event) => changeReference(Number(event.currentTarget.value))}
            >
              {group.members.map((member) => (
                <option key={member.id} value={member.id}>
                  Image #{member.id}{member.preferred ? " (suggested)" : ""}
                </option>
              ))}
            </Form.Control>
          </Form.Group>
          <Form.Group controlId="duplicate-compare-candidate">
            <Form.Label>Compare against</Form.Label>
            <Form.Control
              as="select"
              value={selectedID}
              onChange={(event) => changeSelected(Number(event.currentTarget.value))}
            >
              {group.members.map((member) => (
                <option key={member.id} value={member.id} disabled={member.id === referenceID}>
                  Image #{member.id}{member.preferred ? " (suggested)" : ""}
                </option>
              ))}
            </Form.Control>
          </Form.Group>
          <Form.Group controlId="duplicate-compare-mode">
            <Form.Label>View</Form.Label>
            <Form.Control
              as="select"
              value={mode}
              onChange={(event) => setMode(event.currentTarget.value as ComparisonMode)}
            >
              <option value="both">Side by side</option>
              <option value="slider">Wipe slider</option>
              <option value="blink">Blink</option>
              <option value="difference">Highlight differences</option>
            </Form.Control>
          </Form.Group>
          <div className="duplicate-compare-zoom" aria-label="Comparison zoom">
            <Button
              size="sm"
              variant="secondary"
              disabled={zoom <= 1}
              onClick={() => setZoom(Math.max(1, zoom / 1.5))}
              aria-label="Zoom out"
            >
              −
            </Button>
            <span>{Math.round(zoom * 100)}%</span>
            <Button
              size="sm"
              variant="secondary"
              disabled={zoom >= 8}
              onClick={() => setZoom(Math.min(8, zoom * 1.5))}
              aria-label="Zoom in"
            >
              +
            </Button>
            <Button
              size="sm"
              variant="outline-secondary"
              onClick={() => {
                setZoom(1);
                setResetPosition((value) => !value);
              }}
            >
              Fit / Reset
            </Button>
          </div>
        </div>
        <div className="duplicate-compare-stage">
          {loading ? (
            <div className="duplicate-compare-status" role="status">
              <Spinner animation="border" className="mr-2" />
              Loading full-size image previews…
            </div>
          ) : error || !comparisonReady ? (
            <Alert variant="warning" className="duplicate-compare-status">
              Could not load the comparison images. {error?.message}
            </Alert>
          ) : (
            <ReferenceComparison
              referenceImage={reference}
              selectedImage={selected}
              mode={mode}
              direction="right"
              animateSelected={false}
              zoom={zoom}
              resetPosition={resetPosition}
              setZoom={(value) => setZoom(Math.max(1, Math.min(8, value)))}
            />
          )}
        </div>
        <div className="duplicate-compare-navigation">
          <Button
            size="sm"
            variant="outline-secondary"
            onClick={() => stepSelected(-1)}
            aria-label="Previous comparison image"
          >
            ← Previous
          </Button>
          <span>
            {selectedIndex + 1} of {group.members.length} · Scroll to zoom and drag to pan.
            Arrow keys switch candidates.
          </span>
          <Button
            size="sm"
            variant="outline-secondary"
            onClick={() => stepSelected(1)}
            aria-label="Next comparison image"
          >
            Next →
          </Button>
        </div>
        <div className="duplicate-compare-details">
          {[referenceMember, selectedMember].map((member, index) => (
            <div key={index} className="duplicate-compare-metrics">
              <strong>{index === 0 ? "Reference" : "Candidate"}</strong>
              {member && (
                <>
                  <Link to={"/images/" + member.id} target="_blank" rel="noopener noreferrer">
                    Image #{member.id}
                  </Link>
                  <span>{member.width} × {member.height} · {readableFileSize(member.fileSize)}</span>
                  <Form.Check
                    type="checkbox"
                    id={"compare-keep-" + index}
                    label={keptIDs.includes(member.id) ? "Keep" : "Marked for removal"}
                    checked={keptIDs.includes(member.id)}
                    onChange={() => onToggleKeep(member.id)}
                  />
                </>
              )}
            </div>
          ))}
        </div>
        <div className="duplicate-compare-filmstrip" aria-label="Images in duplicate group">
          {group.members.map((member) => (
            <button
              type="button"
              key={member.id}
              className={
                "duplicate-compare-film-item" +
                (member.id === selectedID ? " is-current" : "") +
                (keptIDs.includes(member.id) ? " is-kept" : "")
              }
              disabled={member.id === referenceID}
              onClick={() => changeSelected(member.id)}
              aria-label={"Compare image " + member.id}
              aria-pressed={member.id === selectedID}
            >
              <img src={"image/" + member.id + "/thumbnail"} alt="" loading="lazy" />
              <span>#{member.id}{keptIDs.includes(member.id) ? " · Keep" : ""}</span>
            </button>
          ))}
        </div>
      </Modal.Body>
      <Modal.Footer>
        <span className="text-muted mr-auto">No files are changed by comparing or choosing what to keep.</span>
        <Button variant="secondary" onClick={onClose}>Back to duplicate groups</Button>
      </Modal.Footer>
    </Modal>
  );
}
