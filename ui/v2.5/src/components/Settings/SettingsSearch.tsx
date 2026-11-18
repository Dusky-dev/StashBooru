import { useMemo, useState } from "react";
import { Form } from "react-bootstrap";
import { Link, useLocation } from "react-router-dom";
import { useIntl } from "react-intl";
import { settingsSearch } from "./settingsNavigation";
import {
  type SettingsSearchEntry,
  searchSettings,
  settingsSearchLabelID,
} from "./settingsSearchIndex";

export function SettingsSearch({
  showExperimental,
}: {
  showExperimental: boolean;
}) {
  const [query, setQuery] = useState("");
  const location = useLocation();
  const intl = useIntl();
  const labelOf = useMemo(
    () => (entry: SettingsSearchEntry) =>
      intl.formatMessage({
        id: settingsSearchLabelID(entry),
        defaultMessage: entry.label,
      }),
    [intl]
  );
  const matches = useMemo(
    () =>
      searchSettings(query, undefined, labelOf)
        .filter(
          (entry) =>
            showExperimental ||
            (entry.tab !== "experimental" && !entry.advanced)
        )
        .slice(0, 12),
    [query, showExperimental, labelOf]
  );
  const active = query.trim().length > 0;

  return (
    <div className="settings-search">
      <Form.Label htmlFor="settings-search-input">
        {intl.formatMessage({ id: "config.settings_search.find" })}
      </Form.Label>
      <Form.Control
        id="settings-search-input"
        type="search"
        className="input-control"
        autoComplete="off"
        value={query}
        placeholder={intl.formatMessage({
          id: "config.settings_search.placeholder",
        })}
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
          aria-label={intl.formatMessage({
            id: "config.settings_search.results",
          })}
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
                <span>{labelOf(entry)}</span>
                <small className="text-muted">
                  {intl.formatMessage({
                    id: `config.categories.${entry.tab.replaceAll("-", "_")}`,
                    defaultMessage:
                      entry.tab === "experimental"
                        ? "Experimental"
                        : entry.tab.replaceAll("-", " "),
                  })}
                </small>
              </Link>
            ))
          ) : (
            <div className="settings-search-empty" role="status">
              {intl.formatMessage({ id: "config.settings_search.none" })}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
