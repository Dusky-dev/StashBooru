import { useEffect, useState } from "react";
import { Form } from "react-bootstrap";
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

  function toggle(id: string, checked: boolean) {
    // Store the choice in the order shown, which is the lookup priority.
    const next = new Set(chosen);
    if (checked) next.add(id);
    else next.delete(id);
    onChange((sources ?? []).map((s) => s.id).filter((s) => next.has(s)));
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
      {sources?.map((source) => {
        const disabled = needsKey(source);
        return (
          <Form.Check
            key={source.id}
            type="checkbox"
            id={`booru-lookup-site-${source.id}`}
            checked={chosen.includes(source.id) && !disabled}
            disabled={disabled}
            onChange={(event) => toggle(source.id, event.currentTarget.checked)}
            label={
              disabled
                ? intl.formatMessage(
                    { id: "media_tagging.md5_site_needs_key" },
                    { name: source.name }
                  )
                : source.name
            }
          />
        );
      })}
      {sources && chosen.length === 0 && (
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
