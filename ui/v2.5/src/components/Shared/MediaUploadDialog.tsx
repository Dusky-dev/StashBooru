import { useEffect, useState } from "react";
import { Alert, Button, Form, Modal } from "react-bootstrap";
import { Link } from "react-router-dom";
import { FolderPicker } from "../BooruBrowser/FolderPicker";
import { readBooruResponse } from "../BooruBrowser/browser";
import { loadDownloadPreferences } from "../BooruBrowser/downloadPreferences";
import { StudioIDSelect } from "../Studios/StudioSelect";
import {
  GalleryIDSelect,
  excludeFileBasedGalleries,
} from "../Galleries/GallerySelect";
import { TagIDSelect } from "../Tags/TagSelect";

const maxUploadSize = 512 * 1024 * 1024;

type UploadResult = {
  kind: "image" | "scene";
  id: string;
  path: string;
};

export function MediaUploadDialog({
  onHide,
  onImported,
}: {
  onHide: () => void;
  onImported: () => void;
}) {
  const [files, setFiles] = useState<File[]>([]);
  const [directory, setDirectory] = useState("");
  const [artistIDs, setArtistIDs] = useState<string[]>([]);
  const [galleryIDs, setGalleryIDs] = useState<string[]>([]);
  const [tagIDs, setTagIDs] = useState<string[]>([]);
  const [picker, setPicker] = useState(false);
  const [working, setWorking] = useState(false);
  const [current, setCurrent] = useState("");
  const [error, setError] = useState("");
  const [ready, setReady] = useState(false);
  const [imported, setImported] = useState<
    { filename: string; result: UploadResult }[]
  >([]);

  useEffect(() => {
    let active = true;
    void loadDownloadPreferences()
      .then((settings) => {
        if (active) {
          setDirectory(settings.defaultDirectory ?? "");
          setReady(true);
        }
      })
      .catch((reason: unknown) => {
        if (active) {
          setError(reason instanceof Error ? reason.message : String(reason));
          setReady(true);
        }
      });
    return () => {
      active = false;
    };
  }, []);

  async function upload() {
    if (!directory || files.length === 0 || working) return;
    setWorking(true);
    setError("");
    const batch = [...files];
    for (let i = 0; i < batch.length; i++) {
      const file = batch[i];
      if (file.size === 0 || file.size > maxUploadSize) {
        setError(
          file.name + ": file must be between 1 byte and 512 MiB. Remaining files were not uploaded."
        );
        break;
      }
      setCurrent(
        "Uploading and importing " +
          (i + 1) +
          " of " +
          batch.length +
          ": " +
          file.name
      );
      const form = new FormData();
      // Metadata and destination precede the file so the backend can stream
      // directly to a validated library folder without buffering large data.
      form.append("destination", directory);
      form.append("artistIDs", artistIDs.join(","));
      form.append("galleryIDs", galleryIDs.join(","));
      form.append("tagIDs", tagIDs.join(","));
      form.append("file", file, file.name);
      try {
        const response = await fetch("media-upload", {
          method: "POST",
          body: form,
        });
        const result = await readBooruResponse<UploadResult>(response);
        setImported((previous) => [
          ...previous,
          { filename: file.name, result },
        ]);
        setFiles(batch.slice(i + 1));
        onImported();
      } catch (reason) {
        setError(
          file.name +
            ": " +
            (reason instanceof Error ? reason.message : String(reason)) +
            ". Remaining files were not uploaded."
        );
        break;
      }
    }
    setCurrent("");
    setWorking(false);
  }

  return (
    <>
      {!picker && (
        <Modal
          show
          size="lg"
          onHide={() => {
            if (!working) onHide();
          }}
        >
          <Modal.Header closeButton={!working}>
            <Modal.Title>Upload images or videos to Stash</Modal.Title>
          </Modal.Header>
          <Modal.Body>
            <p className="text-muted">
              Upload media from this device to an existing Stash library
              folder. Each file is scanned and imported after saving.
            </p>
            {error && <Alert variant="warning">{error}</Alert>}
            {current && (
              <Alert variant="info" role="status">
                {current}
              </Alert>
            )}
            <Form.Group controlId="media-upload-files">
              <Form.Label>Media files</Form.Label>
              <Form.Control
                type="file"
                accept="image/*,video/*,.jxl,.heic,.heif,.mkv,.avi,.wmv,.ts"
                multiple
                disabled={working}
                onChange={(event) =>
                  setFiles(
                    Array.from(
                      (event.currentTarget as HTMLInputElement).files ?? []
                    )
                  )
                }
              />
              <Form.Text className="text-muted">
                Multiple files supported, uploaded one at a time, maximum 512
                MiB each. Existing media files are never overwritten.
              </Form.Text>
              {files.length > 0 && (
                <small className="d-block mt-1">
                  {files.length} selected: {files.map((f) => f.name).join(", ")}
                </small>
              )}
            </Form.Group>
            <Form.Group controlId="media-upload-destination">
              <Form.Label>Destination folder</Form.Label>
              <div className="d-flex align-items-center">
                <Form.Control
                  type="text"
                  value={directory}
                  placeholder="Choose a configured library folder"
                  readOnly
                />
                <Button
                  variant="secondary"
                  className="ml-2"
                  disabled={working}
                  onClick={() => setPicker(true)}
                >
                  Browse…
                </Button>
              </div>
            </Form.Group>
            <Form.Group>
              <Form.Label>Artists (optional)</Form.Label>
              <StudioIDSelect
                ids={artistIDs}
                isMulti
                isDisabled={working}
                isClearable
                onSelect={(items) =>
                  setArtistIDs(items.map((item) => item.id))
                }
                menuPortalTarget={document.body}
              />
            </Form.Group>
            <Form.Group>
              <Form.Label>Galleries (optional)</Form.Label>
              <GalleryIDSelect
                ids={galleryIDs}
                isMulti
                isDisabled={working}
                isClearable
                extraCriteria={excludeFileBasedGalleries}
                onSelect={(items) =>
                  setGalleryIDs(items.map((item) => item.id))
                }
                menuPortalTarget={document.body}
              />
              <Form.Text className="text-muted">
                Only user-created galleries can be selected.
              </Form.Text>
            </Form.Group>
            <Form.Group>
              <Form.Label>Tags (optional)</Form.Label>
              <TagIDSelect
                ids={tagIDs}
                isMulti
                isDisabled={working}
                isClearable
                onSelect={(items) => setTagIDs(items.map((item) => item.id))}
                menuPortalTarget={document.body}
              />
            </Form.Group>
            {imported.length > 0 && (
              <section aria-label="Imported files">
                <strong>Imported files</strong>
                <ul className="mt-2">
                  {imported.map(({ filename, result }, index) => (
                    <li key={result.kind + "-" + result.id + "-" + index}>
                      <Link
                        to={
                          result.kind === "image"
                            ? "/images/" + result.id
                            : "/scenes/" + result.id
                        }
                        onClick={onHide}
                      >
                        {filename}
                      </Link>
                    </li>
                  ))}
                </ul>
              </section>
            )}
          </Modal.Body>
          <Modal.Footer>
            <Button
              variant="secondary"
              onClick={onHide}
              disabled={working}
            >
              {imported.length > 0 ? "Close" : "Cancel"}
            </Button>
            <Button
              variant="primary"
              disabled={!ready || working || !directory || files.length === 0}
              onClick={() => void upload()}
            >
              {working ? "Uploading and importing…" : "Upload to Stash"}
            </Button>
          </Modal.Footer>
        </Modal>
      )}
      {picker && (
        <FolderPicker
          initial={directory}
          title="Choose upload destination"
          onClose={(path) => {
            if (path) setDirectory(path);
            setPicker(false);
          }}
        />
      )}
    </>
  );
}
