import { useEffect, useState } from "react";
import { Alert, Button, Form, Spinner } from "react-bootstrap";
import { type BooruSource, readBooruResponse } from "./browser";

interface Creator {
  id: string;
  name: string;
  service: string;
}
interface Directory {
  creators: Creator[];
  page: number;
  hasMore: boolean;
}

export function CreatorSearch({
  source,
  onChoose,
}: {
  source: BooruSource;
  onChoose: (query: string, label: string) => void;
}) {
  const [query, setQuery] = useState("");
  const [page, setPage] = useState(1);
  const [result, setResult] = useState<Directory>();
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [attempt, setAttempt] = useState(0);
  // biome-ignore lint/correctness/useExhaustiveDependencies: Retry explicitly reloads the same directory query.
  useEffect(() => {
    const controller = new AbortController();
    setResult(undefined);
    setError("");
    setLoading(false);
    if (new TextEncoder().encode(query.trim()).length < 2)
      return () => controller.abort();
    setLoading(true);
    const timer = window.setTimeout(() => {
      const params = new URLSearchParams({
        source: source.id,
        instance: source.instance,
        q: query.trim(),
        page: String(page),
      });
      void fetch(`booru-browser/creators?${params}`, {
        signal: controller.signal,
      })
        .then(readBooruResponse<Directory>)
        .then((value) => {
          if (!controller.signal.aborted) setResult(value);
        })
        .catch((cause: Error) => {
          if (!controller.signal.aborted) setError(cause.message);
        })
        .finally(() => {
          if (!controller.signal.aborted) setLoading(false);
        });
    }, 350);
    return () => {
      controller.abort();
      window.clearTimeout(timer);
    };
  }, [source.id, source.instance, query, page, attempt]);
  return (
    <details open className="booru-creator-search">
      <summary>Find an artist on {source.name}</summary>
      <Form.Group controlId="booru-creator-query">
        <Form.Label>Artist name</Form.Label>
        <Form.Control
          className="text-input"
          placeholder="Search artists…"
          maxLength={128}
          value={query}
          onChange={(event) => {
            setQuery(event.currentTarget.value);
            setPage(1);
          }}
        />
      </Form.Group>
      {loading && (
        <p role="status">
          <Spinner size="sm" animation="border" /> Searching artist directory…
        </p>
      )}
      {error && (
        <Alert variant="warning">
          {error}{" "}
          <Button
            size="sm"
            variant="secondary"
            onClick={() => setAttempt((v) => v + 1)}
          >
            Retry artist search
          </Button>
        </Alert>
      )}
      {result && (
        <>
          {!result.creators.length && <p>No artists matched this name.</p>}
          <ul className="booru-creator-results">
            {result.creators.map((creator) => (
              <li key={`${creator.service}:${creator.id}`}>
                <Button
                  variant="link"
                  onClick={() =>
                    onChoose(
                      `creator:${creator.service}:${creator.id}`,
                      creator.name
                    )
                  }
                >
                  {creator.name}{" "}
                  <small>
                    {creator.service} · {creator.id}
                  </small>
                </Button>
              </li>
            ))}
          </ul>
          <div className="d-flex justify-content-between">
            <Button
              size="sm"
              variant="secondary"
              disabled={page === 1 || loading}
              onClick={() => setPage((p) => p - 1)}
            >
              Previous artists
            </Button>
            <Button
              size="sm"
              variant="secondary"
              disabled={!result.hasMore || loading}
              onClick={() => setPage((p) => p + 1)}
            >
              More artists
            </Button>
          </div>
        </>
      )}
    </details>
  );
}
