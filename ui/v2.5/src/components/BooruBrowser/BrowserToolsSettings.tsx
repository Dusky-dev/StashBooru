import { useEffect, useState } from "react";
import { Alert, Button, Form } from "react-bootstrap";
import { type BooruSource, readBooruResponse } from "./browser";

interface TagDatabase {
  count: number;
  revision: string;
}
export function BrowserToolsSettings() {
  const [sources, setSources] = useState<BooruSource[]>([]);
  const [sourceID, setSourceID] = useState("");
  const [database, setDatabase] = useState<TagDatabase>();
  const [convert, setConvert] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [file, setFile] = useState<File>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [attempt, setAttempt] = useState(0);
  // biome-ignore lint/correctness/useExhaustiveDependencies: Reload explicitly retries persisted settings.
  useEffect(() => {
    const controller = new AbortController();
    void Promise.all([
      fetch("booru-browser/sources", { signal: controller.signal }).then(
        readBooruResponse<BooruSource[]>
      ),
      fetch("booru-browser/download-settings", {
        signal: controller.signal,
      }).then(readBooruResponse<{ jpegToJXL: boolean }>),
    ])
      .then(([list, settings]) => {
        if (!controller.signal.aborted) {
          setSources(list);
          setSourceID((old) =>
            list.some((s) => s.id === old) ? old : (list[0]?.id ?? "")
          );
          setConvert(settings.jpegToJXL);
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
  // biome-ignore lint/correctness/useExhaustiveDependencies: Reload explicitly retries persisted settings.
  useEffect(() => {
    const controller = new AbortController();
    setDatabase(undefined);
    setFile(undefined);
    if (source) {
      const params = new URLSearchParams({
        source: source.id,
        instance: source.instance,
      });
      void fetch(`booru-browser/tags?${params}`, { signal: controller.signal })
        .then(readBooruResponse<TagDatabase>)
        .then((value) => {
          if (!controller.signal.aborted) setDatabase(value);
        })
        .catch((cause: Error) => {
          if (!controller.signal.aborted) setError(cause.message);
        });
    }
    return () => controller.abort();
  }, [source, attempt]);
  async function saveConversion() {
    setBusy(true);
    setError("");
    setNotice("");
    try {
      await fetch("booru-browser/download-settings", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ jpegToJXL: convert }),
      }).then(readBooruResponse);
      setNotice("Download preference saved.");
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setBusy(false);
    }
  }
  async function upload() {
    if (!source || !file || !database) return;
    if (file.size > 64 * 1024 * 1024) {
      setError("Tag database must be at most 64 MiB.");
      return;
    }
    setBusy(true);
    setError("");
    setNotice("");
    try {
      const params = new URLSearchParams({
        source: source.id,
        instance: source.instance,
      });
      const result = await fetch(`booru-browser/tags?${params}`, {
        method: "POST",
        headers: {
          "Content-Type": "text/csv",
          "X-Booru-Revision": database.revision,
        },
        body: file,
      }).then(readBooruResponse<TagDatabase>);
      setDatabase(result);
      setFile(undefined);
      setNotice(
        `Imported ${result.count.toLocaleString()} tags for ${source.name}.`
      );
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setBusy(false);
    }
  }
  return (
    <section
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
      <Form.Check
        id="booru-default-jxl"
        label="Convert JPEG downloads to JPEG XL by default"
        checked={convert}
        disabled={!loaded || busy}
        onChange={(event) => setConvert(event.currentTarget.checked)}
      />
      <p className="text-muted">
        Uses the worker selected in Media converter settings, with JPEG
        reconstruction (quality 100). Other formats stay original. Downloads are
        saved to your computer; they do not import catalogue entries or change
        review marks.
      </p>
      <Button
        size="sm"
        disabled={!loaded || busy}
        onClick={() => void saveConversion()}
      >
        Save download preference
      </Button>
      <hr />
      <Form.Group controlId="booru-tag-source">
        <Form.Label>Tag database source</Form.Label>
        <Form.Control
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
          : "Loading tag database…"}{" "}
        Databases are shared by aliases of the same booru URL and engine.
      </p>
      <p className="text-muted">
        Upload a tagcomplete CSV: name,type,count,"aliases". Danbooru exports
        with name/category/post_count headers also work. Categories and aliases
        are kept for suggestions. Up to 64 MiB and one million tags; uploading
        replaces this source's database.
      </p>
      <input
        key={`${sourceID}:${database?.revision ?? ""}`}
        type="file"
        accept=".csv,text/csv"
        aria-label="Tag database CSV"
        disabled={busy}
        onChange={(event) => setFile(event.currentTarget.files?.[0])}
      />
      <div className="mt-2 d-flex flex-wrap">
        <Button
          size="sm"
          disabled={busy || !file || !database}
          onClick={() => void upload()}
        >
          Upload tag database
        </Button>
        <Button
          size="sm"
          variant="secondary"
          className="ml-2"
          disabled={busy}
          onClick={() => {
            setError("");
            setNotice("");
            setAttempt((n) => n + 1);
          }}
        >
          Reload download and tag settings
        </Button>
      </div>
    </section>
  );
}
