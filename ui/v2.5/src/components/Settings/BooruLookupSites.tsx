import { useEffect, useState } from "react";
import { Button, Form } from "react-bootstrap";
import { faArrowDown, faArrowUp } from "@fortawesome/free-solid-svg-icons";
import { Icon } from "src/components/Shared/Icon";
import { useIntl } from "react-intl";

interface PublicBooruSource {
  id: string;
  name: string;
  engine: string;
  url: string;
  credentialsConfigured: boolean;
}

// Keep in step with booruEngineSupportsMD5 and defaultBooruLookupSources
// on the server.
const md5Engines = new Set(["danbooru", "gelbooru", "moebooru", "e621"]);
export const defaultBooruLookupSources = [
  "danbooru",
  "gelbooru",
  "yandere",
  "konachan",
  "safebooru",
];

function needsKey(source: PublicBooruSource) {
  try {
    const host = new URL(source.url).hostname;
    return (
      source.engine === "gelbooru" &&
      (host === "rule34.xxx" || host.endsWith(".rule34.xxx")) &&
      !source.credentialsConfigured
    );
  } catch {
    return false;
  }
}

/** Choose which Booru browser sources tagging's MD5 lookup asks. */
export function BooruLookupSites({
  value,
  onChange,
}: {
  value: string[] | null | undefined;
  onChange: (value: string[]) => void;
}) {
  const intl = useIntl();
  const [sources, setSources] = useState<PublicBooruSource[]>();
  const [error, setError] = useState("");
  const chosen = value ?? defaultBooruLookupSources;

  useEffect(() => {
    const controller = new AbortController();
    void fetch("booru-browser/sources", { signal: controller.signal })
      .then(async (response) => {
        if (!response.ok)
          throw new Error((await response.text()) || response.statusText);
        return response.json() as Promise<PublicBooruSource[]>;
      })
      .then((list) => {
        if (!controller.signal.aborted)
          setSources(list.filter((source) => md5Engines.has(source.engine)));
      })
      .catch((cause: unknown) => {
        if (!controller.signal.aborted)
          setError(cause instanceof Error ? cause.message : String(cause));
      });
    return () => controller.abort();
  }, []);

  // The stored list is the lookup priority. Ticked sites come first in that
  // order; IDs of removed or disabled sources are dropped from the list shown.
  const available = sources ?? [];
  const known = new Map(available.map((source) => [source.id, source]));
  const ordered = chosen.filter(
    (id) => known.has(id) && !needsKey(known.get(id)!)
  );
  const rows = [
    ...ordered.map((id) => known.get(id)!),
    ...available.filter((source) => !ordered.includes(source.id)),
  ];

  function toggle(id: string, checked: boolean) {
    onChange(
      checked ? [...ordered, id] : ordered.filter((other) => other !== id)
    );
  }

  function move(index: number, delta: -1 | 1) {
    const next = [...ordered];
    const target = index + delta;
    if (target < 0 || target >= next.length) return;
    [next[index], next[target]] = [next[target], next[index]];
    onChange(next);
  }

  return (
    <Form.Group className="booru-lookup-sites">
      <Form.Label>
        {intl.formatMessage({ id: "media_tagging.md5_sites" })}
      </Form.Label>
      {error && (
        <Form.Text className="text-danger">
          {intl.formatMessage(
            { id: "media_tagging.md5_sites_error" },
            { error }
          )}
        </Form.Text>
      )}
      {rows.map((source) => {
        const disabled = needsKey(source);
        const position = ordered.indexOf(source.id);
        return (
          <div
            key={source.id}
            className="booru-lookup-site d-flex align-items-center"
          >
            <Form.Check
              type="checkbox"
              id={`booru-lookup-site-${source.id}`}
              checked={position >= 0}
              disabled={disabled}
              onChange={(event) =>
                toggle(source.id, event.currentTarget.checked)
              }
              label={
                disabled
                  ? intl.formatMessage(
                      { id: "media_tagging.md5_site_needs_key" },
                      { name: source.name }
                    )
                  : position >= 0
                    ? `${position + 1}. ${source.name}`
                    : source.name
              }
            />
            {position >= 0 && (
              <span className="ml-auto booru-lookup-site-order">
                <Button
                  size="sm"
                  variant="minimal"
                  disabled={position === 0}
                  onClick={() => move(position, -1)}
                  aria-label={intl.formatMessage(
                    { id: "media_tagging.md5_site_up" },
                    { name: source.name }
                  )}
                  title={intl.formatMessage(
                    { id: "media_tagging.md5_site_up" },
                    { name: source.name }
                  )}
                >
                  <Icon icon={faArrowUp} />
                </Button>
                <Button
                  size="sm"
                  variant="minimal"
                  disabled={position === ordered.length - 1}
                  onClick={() => move(position, 1)}
                  aria-label={intl.formatMessage(
                    { id: "media_tagging.md5_site_down" },
                    { name: source.name }
                  )}
                  title={intl.formatMessage(
                    { id: "media_tagging.md5_site_down" },
                    { name: source.name }
                  )}
                >
                  <Icon icon={faArrowDown} />
                </Button>
              </span>
            )}
          </div>
        );
      })}
      {sources && ordered.length === 0 && (
        <Form.Text className="text-warning">
          {intl.formatMessage({ id: "media_tagging.md5_sites_none" })}
        </Form.Text>
      )}
      <Form.Text className="text-muted">
        {intl.formatMessage({ id: "media_tagging.md5_sites_help" })}
      </Form.Text>
    </Form.Group>
  );
}
