import { useEffect, useRef, useState } from "react";
import { Alert, Button, Form } from "react-bootstrap";
import { type BooruSource, readBooruResponse } from "./browser";
import { FolderPicker } from "./FolderPicker";
import { invalidateDownloadPreferences } from "./downloadPreferences";
interface TagDatabase {
  count: number;
  revision: string;
  path?: string;
}
export function BrowserToolsSettings() {
  const [sources, setSources] = useState<BooruSource[]>([]),
    [sourceID, setSourceID] = useState("");
  const [database, setDatabase] = useState<TagDatabase>(),
    [databasePath, setDatabasePath] = useState("");
  const [convert, setConvert] = useState(false),
    [directory, setDirectory] = useState("");
  const [filenameTemplate, setFilenameTemplate] = useState("");
  const [picker, setPicker] = useState(false),
    [loaded, setLoaded] = useState(false),
    [busy, setBusy] = useState(false);
  const [file, setFile] = useState<File>(),
    [error, setError] = useState(""),
    [notice, setNotice] = useState("");
  const [attempt, setAttempt] = useState(0);
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);
  // biome-ignore lint/correctness/useExhaustiveDependencies: Explicit reload retries the saved settings request.
  useEffect(() => {
    const controller = new AbortController();
    setLoaded(false);
    void Promise.all([
      fetch("booru-browser/sources", { signal: controller.signal }).then(
        readBooruResponse<BooruSource[]>
      ),
      fetch("booru-browser/download-settings", {
        signal: controller.signal,
      }).then(
        readBooruResponse<{
          convert: boolean;
          defaultDirectory: string;
          filenameTemplate?: string;
        }>
      ),
    ])
      .then(([list, settings]) => {
        if (!controller.signal.aborted) {
          setSources(list);
          setSourceID((old) =>
            list.some((s) => s.id === old) ? old : (list[0]?.id ?? "")
          );
          setConvert(settings.convert);
          setDirectory(settings.defaultDirectory);
          setFilenameTemplate(settings.filenameTemplate ?? "");
          setLoaded(true);
          setError("");
        }
      })
      .catch((cause: Error) => {
        if (!controller.signal.aborted) setError(cause.message);
      });
    return () => controller.abort();
  }, [attempt]);
  const source = sources.find((s) => s.id === sourceID);
  // biome-ignore lint/correctness/useExhaustiveDependencies: Explicit reload retries the selected database request.
  useEffect(() => {
    const controller = new AbortController();
    setDatabase(undefined);
    setFile(undefined);
    setDatabasePath("");
    if (source) {
      const params = new URLSearchParams({
        source: source.id,
        instance: source.instance,
      });
      void fetch(`booru-browser/tags?${params}`, { signal: controller.signal })
        .then(readBooruResponse<TagDatabase>)
        .then((value) => {
          if (!controller.signal.aborted) {
            setDatabase(value);
            setDatabasePath(value.path ?? "");
          }
        })
        .catch((cause: Error) => {
          if (!controller.signal.aborted) setError(cause.message);
        });
    }
    return () => controller.abort();
  }, [source, attempt]);
  async function action(work: () => Promise<void>) {
    setBusy(true);
    setError("");
    setNotice("");
    try {
      await work();
    } catch (cause) {
      if (mounted.current)
        setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      if (mounted.current) setBusy(false);
    }
  }
  async function save() {
    await fetch("booru-browser/download-settings", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        convert,
        defaultDirectory: directory,
        filenameTemplate,
      }),
    }).then(readBooruResponse);
    invalidateDownloadPreferences();
    window.dispatchEvent(new Event("booru-download-settings"));
    if (mounted.current) setNotice("Download settings saved.");
  }
  async function install(fromPath: boolean) {
    if (!source || !database || (!fromPath && !file)) return;
    if (file && !fromPath && file.size > 64 * 1024 * 1024)
      throw new Error("Tag database must be at most 64 MiB.");
    const params = new URLSearchParams({
      source: source.id,
      instance: source.instance,
    });
    const response = await fetch(
      `booru-browser/${fromPath ? "tag-path" : "tags"}?${params}`,
      fromPath
        ? {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
              path: databasePath,
              revision: database.revision,
            }),
          }
        : {
            method: "POST",
            headers: {
              "Content-Type": "text/csv",
              "X-Booru-Revision": database.revision,
            },
            body: file,
          }
    );
    const result = await readBooruResponse<TagDatabase>(response);
    if (mounted.current) {
      setDatabase(result);
      setDatabasePath(result.path ?? "");
      setFile(undefined);
      setNotice(
        `Imported ${result.count.toLocaleString()} tags for ${source.name}.`
      );
    }
  }
  return (
    <section
      id="booru-tools-settings"
      className="booru-tools-settings"
      aria-label="Downloads and autocomplete"
    >
      <h4>Downloads and autocomplete</h4>
      {error && <Alert variant="warning">{error}</Alert>}
      {notice && (
        <Alert variant="success" role="status">
          {notice}
        </Alert>
      )}
      <Form.Group controlId="booru-default-directory">
        <Form.Label>Default Stash download folder</Form.Label>
        <Form.Control
          className="text-input"
          value={directory}
          disabled={busy || !loaded}
          onChange={(event) => setDirectory(event.currentTarget.value)}
          placeholder="/path/to/stash/downloads"
        />
      </Form.Group>
      <Button
        variant="secondary"
        size="sm"
        disabled={busy || !loaded}
        onClick={() => setPicker(true)}
      >
        Choose default folder
      </Button>
      <Form.Group controlId="booru-filename-template" className="mt-3">
        <Form.Label>Download filename template</Form.Label>
        <Form.Control
          className="text-input"
          value={filenameTemplate}
          disabled={busy || !loaded}
          onChange={(event) => setFilenameTemplate(event.currentTarget.value)}
          placeholder="%artist%/%copyright%/%character%/%md5%.%ext%"
        />
        <Form.Text muted>
          Common Grabber tokens: %artist%, %copyright%, %character%, %id%,
          %md5%, %ext%, %filename%, %websitename%, %position%, %num%. Use / for
          subfolders. Blank keeps existing naming. Unknown metadata becomes
          unknown; existing files are never overwritten.
        </Form.Text>
      </Form.Group>
      <Form.Check
        type="switch"
        className="mt-3"
        id="booru-default-convert"
        label="Convert new downloads using Media converter defaults"
        checked={convert}
        disabled={busy || !loaded}
        onChange={(event) => setConvert(event.currentTarget.checked)}
      />
      <p className="text-muted">
        Files are saved into this server's Stash library and imported by its
        native scanner. Conversion uses the saved format, quality, effort,
        worker and savings policy in Media converter settings.
      </p>
      <Button
        size="sm"
        disabled={busy || !loaded}
        onClick={() => void action(save)}
      >
        Save download settings
      </Button>
      <hr />
      <h5>Autocomplete database</h5>
      <Form.Group controlId="booru-tag-source">
        <Form.Label>Tag database source</Form.Label>
        <Form.Control
          className="text-input"
          as="select"
          value={sourceID}
          disabled={busy}
          onChange={(event) => {
            setSourceID(event.currentTarget.value);
            setNotice("");
            setError("");
          }}
        >
          {sources.map((item) => (
            <option key={item.id} value={item.id}>
              {item.name}
            </option>
          ))}
        </Form.Control>
      </Form.Group>
      <p>
        {database
          ? `${database.count.toLocaleString()} tags installed.`
          : "Loading tag database…"}
      </p>
      <Form.Group controlId="booru-database-path">
        <Form.Label>Autocomplete database CSV path on the server</Form.Label>
        <Form.Control
          className="text-input"
          value={databasePath}
          disabled={busy || !database}
          onChange={(event) => setDatabasePath(event.currentTarget.value)}
          placeholder="/models/autocomplete/danbooru.csv"
        />
      </Form.Group>
      <Button
        size="sm"
        disabled={busy || !database || !databasePath}
        onClick={() => void action(() => install(true))}
      >
        Load database from path
      </Button>
      <p className="text-muted mt-3">
        Or upload a CSV from your computer. Format: name,type,count,"aliases",
        or a Danbooru CSV export. Up to 64 MiB and one million tags. Loading
        replaces this source's autocomplete database and leaves catalogue tags
        alone.
      </p>
      <input
        key={`${sourceID}:${database?.revision ?? ""}`}
        type="file"
        accept=".csv,text/csv"
        aria-label="Tag database CSV"
        disabled={busy || !database}
        onChange={(event) => setFile(event.currentTarget.files?.[0])}
      />
      <div className="mt-2 d-flex flex-wrap">
        <Button
          size="sm"
          disabled={busy || !file || !database}
          onClick={() => void action(() => install(false))}
        >
          Upload tag database
        </Button>
        <Button
          size="sm"
          variant="secondary"
          className="ml-2"
          disabled={busy}
          onClick={() => {
            setNotice("");
            setAttempt((n) => n + 1);
          }}
        >
          Reload download and tag settings
        </Button>
      </div>
      {picker && (
        <FolderPicker
          initial={directory}
          onClose={(value) => {
            setPicker(false);
            if (value) setDirectory(value);
          }}
        />
      )}
    </section>
  );
}
