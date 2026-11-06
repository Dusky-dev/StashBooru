import { useMemo, useState } from "react";
import { Form } from "react-bootstrap";
import { Link, useLocation } from "react-router-dom";
import { useIntl } from "react-intl";
import { settingsSearch } from "./settingsNavigation";
import { searchSettings } from "./settingsSearchIndex";

export function SettingsSearch() {
  const [query, setQuery] = useState("");
  const location = useLocation();
  const intl = useIntl();
  const matches = useMemo(() => searchSettings(query).slice(0, 12), [query]);
  const active = query.trim().length > 0;

  return (
    <div className="settings-search">
      <Form.Label htmlFor="settings-search-input">Find a setting</Form.Label>
      <Form.Control
        id="settings-search-input"
        type="search"
        className="input-control"
        autoComplete="off"
        value={query}
        placeholder="Search all settings…"
        aria-controls="settings-search-results"
        onChange={(event) => setQuery(event.currentTarget.value)}
        onKeyDown={(event) => {
          if (event.key === "Escape") setQuery("");
        }}
      />
      {active && (
        <div
          id="settings-search-results"
          className="settings-search-results"
          aria-label="Matching settings"
        >
          {matches.length ? (
            matches.map((entry) => (
              <Link
                key={`${entry.tab}:${entry.anchor ?? entry.label}`}
                to={{
                  pathname: location.pathname,
                  search: settingsSearch(location.search, entry.tab),
                  hash: entry.anchor ? `#${entry.anchor}` : "",
                }}
                className="settings-search-result"
                onClick={() => setQuery("")}
              >
                <span>{entry.label}</span>
                <small className="text-muted">
                  {intl.formatMessage({
                    id: `config.categories.${entry.tab.replaceAll("-", "_")}`,
                    defaultMessage: entry.tab.replaceAll("-", " "),
                  })}
                </small>
              </Link>
            ))
          ) : (
            <div className="settings-search-empty" role="status">
              No matching settings.
            </div>
          )}
        </div>
      )}
    </div>
  );
}
