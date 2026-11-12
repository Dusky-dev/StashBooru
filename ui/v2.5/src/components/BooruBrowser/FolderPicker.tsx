import { useEffect, useState } from "react";
import { Alert, Button, Form, Modal } from "react-bootstrap";
import { faHouse } from "@fortawesome/free-solid-svg-icons";
import { Icon } from "../Shared/Icon";
import { readBooruResponse } from "./browser";

export function FolderPicker({
  initial,
  onClose,
  title = "Choose Stash download folder",
  allowCreate = false,
}: {
  initial: string;
  onClose: (value?: string) => void;
  title?: string;
  allowCreate?: boolean;
}) {
  const [path, setPath] = useState(initial);
  const [directory, setDirectory] = useState(initial);
  const [listing, setListing] = useState<{
    directories: string[];
    parent: string;
  }>();
  const [error, setError] = useState("");
  const [newFolder, setNewFolder] = useState("");
  const [creating, setCreating] = useState(false);
  useEffect(() => {
    if (path === directory) return;
    const timer = window.setTimeout(() => setDirectory(path), 400);
    return () => window.clearTimeout(timer);
  }, [path, directory]);
  useEffect(() => {
    const controller = new AbortController();
    setError("");
    setListing(undefined);
    void fetch(
      `booru-browser/directories?${new URLSearchParams({ path: directory })}`,
      { signal: controller.signal }
    )
      .then(readBooruResponse<{ directories: string[]; parent: string }>)
      .then((result) => {
        if (!controller.signal.aborted) setListing(result);
      })
      .catch((cause: Error) => {
        if (!controller.signal.aborted) setError(cause.message);
      });
    return () => controller.abort();
  }, [directory]);
  function navigate(value: string) {
    setPath(value);
    setDirectory(value);
  }
  async function createFolder() {
    if (!listing || !directory || !newFolder.trim() || creating) return;
    setCreating(true);
    setError("");
    try {
      const response = await fetch("media-upload/folder", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ parent: directory, name: newFolder.trim() }),
      });
      const result = await readBooruResponse<{ path: string }>(response);
      setNewFolder("");
      navigate(result.path);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setCreating(false);
    }
  }
  return (
    <Modal show onHide={() => onClose()}>
      <Modal.Header closeButton>
        <Modal.Title>{title}</Modal.Title>
        <Button
          size="sm"
          variant="secondary"
          className="ml-auto"
          onClick={() => navigate("")}
          title="Root"
        >
          <Icon icon={faHouse} /> Root
        </Button>
      </Modal.Header>
      <Modal.Body>
        <Form.Group controlId="booru-folder-path">
          <Form.Label>Server directory</Form.Label>
          <Form.Control
            className="text-input"
            value={path}
            onChange={(event) => setPath(event.currentTarget.value)}
            onKeyDown={(event) => {
              if (event.key === "Enter") {
                event.preventDefault();
                setDirectory(path);
              }
            }}
          />
        </Form.Group>
        {error && (
          <Alert variant="warning" className="mt-2">
            {error}
          </Alert>
        )}
        <ul className="booru-folders">
          {listing?.parent && (
            <li>
              <Button variant="link" onClick={() => navigate(listing.parent)}>
                Parent folder
              </Button>
            </li>
          )}
          {listing?.directories.map((entry) => (
            <li key={entry}>
              <Button variant="link" onClick={() => navigate(entry)}>
                {entry}
              </Button>
            </li>
          ))}
        </ul>
        {allowCreate && directory && listing && (
          <Form.Group controlId="upload-create-subfolder">
            <Form.Label>New subfolder</Form.Label>
            <div className="d-flex">
              <Form.Control
                value={newFolder}
                disabled={creating}
                placeholder="Folder name"
                onChange={(event) => setNewFolder(event.currentTarget.value)}
                onKeyDown={(event) => {
                  if (event.key === "Enter") {
                    event.preventDefault();
                    void createFolder();
                  }
                }}
              />
              <Button
                variant="secondary"
                className="ml-2"
                disabled={creating || !newFolder.trim()}
                onClick={() => void createFolder()}
              >
                Create
              </Button>
            </div>
          </Form.Group>
        )}
        <small>
          {allowCreate
            ? "Choose or create a folder inside a configured Stash library."
            : "Choose an existing folder inside a configured Stash library."}
        </small>
      </Modal.Body>
      <Modal.Footer>
        <Button variant="secondary" onClick={() => onClose()}>
          Cancel
        </Button>
        <Button
          disabled={!path || path !== directory || !listing}
          onClick={() => onClose(path)}
        >
          Use this folder
        </Button>
      </Modal.Footer>
    </Modal>
  );
}
