import { useEffect, useRef, useState } from "react";
import { Button } from "react-bootstrap";
import { faUpload } from "@fortawesome/free-solid-svg-icons";
import { Icon } from "./Icon";
import { MediaUploadDialog } from "./MediaUploadDialog";
import {
  MEDIA_UPLOAD_IMPORTED_EVENT,
  useMediaUploadEnabled,
} from "./mediaUploadSettings";

type FileBatch = { id: number; files: File[] };

function isFileDrag(event: DragEvent) {
  return (
    Array.from(event.dataTransfer?.types ?? []).includes("Files") ||
    (event.type === "drop" && !!event.dataTransfer?.files.length)
  );
}

/** One upload entry point across all pages, including file drops onto the page. */
export function GlobalMediaUpload() {
  const enabled = useMediaUploadEnabled();
  const [open, setOpen] = useState(false);
  const [dropActive, setDropActive] = useState(false);
  const [incomingBatch, setIncomingBatch] = useState<FileBatch>();
  const nextBatchId = useRef(0);
  const dragDepth = useRef(0);

  useEffect(() => {
    if (!enabled) return;

    const resetDrag = () => {
      dragDepth.current = 0;
      setDropActive(false);
    };
    const dragEnter = (event: DragEvent) => {
      if (!isFileDrag(event)) return;
      event.preventDefault();
      dragDepth.current += 1;
      setDropActive(true);
    };
    const dragOver = (event: DragEvent) => {
      if (!isFileDrag(event)) return;
      event.preventDefault();
      if (event.dataTransfer) event.dataTransfer.dropEffect = "copy";
    };
    const dragLeave = (event: DragEvent) => {
      if (!isFileDrag(event)) return;
      dragDepth.current = Math.max(0, dragDepth.current - 1);
      if (dragDepth.current === 0) setDropActive(false);
    };
    const drop = (event: DragEvent) => {
      if (!isFileDrag(event)) return;
      event.preventDefault(); // Never navigate to a dropped file.
      resetDrag();
      const files = Array.from(event.dataTransfer?.files ?? []);
      if (!files.length) return;
      setIncomingBatch({ id: ++nextBatchId.current, files });
      setOpen(true);
    };

    // Capture ensures a drop never opens the media file in the browser.
    window.addEventListener("dragenter", dragEnter, true);
    window.addEventListener("dragover", dragOver, true);
    window.addEventListener("dragleave", dragLeave, true);
    window.addEventListener("drop", drop, true);
    window.addEventListener("blur", resetDrag);
    return () => {
      window.removeEventListener("dragenter", dragEnter, true);
      window.removeEventListener("dragover", dragOver, true);
      window.removeEventListener("dragleave", dragLeave, true);
      window.removeEventListener("drop", drop, true);
      window.removeEventListener("blur", resetDrag);
    };
  }, [enabled]);

  if (!enabled) return null;

  return (
    <>
      <Button
        variant="primary"
        className="mr-2 d-flex align-items-center"
        title="Upload images or videos"
        aria-label="Upload images or videos"
        onClick={() => setOpen(true)}
      >
        <Icon icon={faUpload} />
        <span className="ml-1 d-none d-sm-inline">Upload</span>
      </Button>
      {dropActive && (
        <div
          aria-hidden="true"
          style={{
            position: "fixed",
            inset: 0,
            zIndex: 2000,
            pointerEvents: "none",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            background: "rgba(0, 0, 0, 0.55)",
          }}
        >
          <div
            className="rounded p-5 text-center h4"
            style={{ border: "2px dashed currentColor" }}
          >
            <Icon icon={faUpload} /> Drop images or videos to upload
          </div>
        </div>
      )}
      {open && (
        <MediaUploadDialog
          incomingBatch={incomingBatch}
          dropActive={dropActive}
          onHide={() => {
            setOpen(false);
            setIncomingBatch(undefined);
          }}
          onImported={() =>
            window.dispatchEvent(new Event(MEDIA_UPLOAD_IMPORTED_EVENT))
          }
        />
      )}
    </>
  );
}
