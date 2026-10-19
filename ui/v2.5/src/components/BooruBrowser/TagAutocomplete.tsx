import { useEffect, useRef, useState } from "react";
import { type BooruSource, readBooruResponse } from "./browser";
import {
  completionToken,
  applyCompletion,
  queryTerms,
  removeQueryTerm,
} from "./completion";

interface Suggestion {
  name: string;
  category: string;
  count: number;
  alias?: string;
  source: string;
}

export function TagAutocomplete({
  value,
  onChange,
  sources,
  mode = "live",
  submittedValue,
  submissionKey,
}: {
  value: string;
  onChange: (value: string) => void;
  sources: BooruSource[];
  mode?: "live" | "database";
  submittedValue?: string;
  submissionKey?: string;
}) {
  const input = useRef<HTMLInputElement>(null);
  const list = useRef<HTMLUListElement>(null);
  const [caret, setCaret] = useState(value.length);
  const [items, setItems] = useState<Suggestion[]>([]);
  const [active, setActive] = useState(-1);
  const [dismissed, setDismissed] = useState(false);
  const [focused, setFocused] = useState(false);
  const [notice, setNotice] = useState("");
  useEffect(() => {
    if (submittedValue !== undefined || submissionKey !== undefined)
      setDismissed(true);
  }, [submittedValue, submissionKey]);
  const comma =
    sources.length > 0 && sources.every((s) => s.engine === "philomena");
  useEffect(
    () => setCaret((current) => Math.min(current, value.length)),
    [value]
  );
  const token = completionToken(value, caret, comma);
  const sourceKey = sources.map((s) => `${s.id}:${s.instance}`).join(",");
  // biome-ignore lint/correctness/useExhaustiveDependencies: Stable source identity list governs requests.
  useEffect(() => {
    const controller = new AbortController();
    setItems([]);
    setNotice("");
    if (
      token.prefix.length < 2 ||
      (!comma && token.prefix.includes(":")) ||
      sources.length === 0
    )
      return () => controller.abort();
    const timer = window.setTimeout(() => {
      void Promise.allSettled(
        sources.map(async (source) => {
          const params = new URLSearchParams({
            source: source.id,
            instance: source.instance,
            q: token.prefix,
            mode,
          });
          const result = await fetch(`booru-browser/autocomplete?${params}`, {
            signal: controller.signal,
          }).then(
            readBooruResponse<{
              mode: string;
              notice?: string;
              suggestions: Omit<Suggestion, "source">[];
            }>
          );
          return {
            notice: result.notice,
            items: result.suggestions.map((item) => ({
              ...item,
              source: source.name,
            })),
          };
        })
      )
        .then((results) => {
          if (controller.signal.aborted) return;
          const available = results.flatMap((result) =>
            result.status === "fulfilled" ? [result.value] : []
          );
          if (available.length === 0)
            setNotice(
              "Autocomplete unavailable. Check source access or select Installed CSV only in the Autocomplete menu."
            );
          else if (available.length < results.length)
            setNotice(
              "Some sources are unavailable. Suggestions from the others are shown."
            );
          const combined = available
            .flatMap((result) => result.items)
            .sort((a, b) => b.count - a.count || a.name.localeCompare(b.name));
          const unique = combined
            .filter(
              (item, index) =>
                combined.findIndex((other) => other.name === item.name) ===
                index
            )
            .slice(0, 20);
          setItems(unique);
          setActive(-1);
          if (
            available.length === results.length &&
            available.some((result) => result.notice)
          )
            setNotice(
              available
                .map((result) => result.notice)
                .filter(Boolean)
                .join(" ")
            );
        })
        .catch(() => {
          if (!controller.signal.aborted)
            setNotice(
              "Autocomplete unavailable. Check source access or the optional CSV database in settings."
            );
        });
    }, 350);
    return () => {
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [token.prefix, sourceKey, mode]);
  function choose(item: Suggestion) {
    const next = applyCompletion(value, token, item.name);
    if (new TextEncoder().encode(next).length > 1024) {
      setNotice("Completed search would exceed 1024 bytes.");
      return;
    }
    const position = token.start + item.name.length;
    onChange(next);
    setCaret(position);
    setDismissed(true);
    window.requestAnimationFrame(() => {
      input.current?.focus();
      input.current?.setSelectionRange(position, position);
    });
  }
  const exact =
    token.end === caret &&
    items.some(
      (item) => item.name.toLowerCase() === token.prefix.toLowerCase()
    );
  const open = focused && items.length > 0 && !dismissed && !exact;
  useEffect(() => {
    if (open)
      list.current?.children.item(active)?.scrollIntoView({ block: "nearest" });
  }, [active, open]);
  return (
    <div className="booru-autocomplete">
      {value.trim() && (
        <div className="booru-query-chips" aria-label="Search terms">
          {queryTerms(value, comma)
            .slice(0, 30)
            .map((term) => (
              <span
                className={`booru-query-chip ${term.name.startsWith("-") ? "is-excluded" : ""}`}
                key={`${term.start}:${term.name}`}
              >
                <span>{term.name.replace(/_/g, " ")}</span>
                <button
                  type="button"
                  aria-label={`Remove search term ${term.name}`}
                  onClick={() => onChange(removeQueryTerm(value, term, comma))}
                >
                  ×
                </button>
              </span>
            ))}
        </div>
      )}

      <input
        ref={input}
        id="booru-query"
        className="form-control text-input"
        role="combobox"
        aria-label="Search tags (native source syntax)"
        aria-autocomplete="list"
        aria-expanded={open}
        aria-controls="booru-tag-suggestions"
        aria-activedescendant={
          open && active >= 0 ? `booru-suggestion-${active}` : undefined
        }
        autoComplete="off"
        value={value}
        maxLength={1024}
        placeholder="Search tags…"
        onChange={(event) => {
          onChange(event.currentTarget.value);
          setCaret(
            event.currentTarget.selectionStart ??
              event.currentTarget.value.length
          );
          setDismissed(false);
          setActive(-1);
          setItems([]);
        }}
        onSelect={(event) =>
          setCaret(event.currentTarget.selectionStart ?? value.length)
        }
        onBlur={() => setFocused(false)}
        onFocus={() => setFocused(true)}
        onKeyDown={(event) => {
          if (event.ctrlKey && event.key === "Enter") {
            event.preventDefault();
            setDismissed(true);
            event.currentTarget.form?.requestSubmit();
            return;
          }
          if (event.key === "Escape") {
            setDismissed(true);
            return;
          }
          if (event.key === "Enter" && (!open || active < 0)) {
            setDismissed(true);
            return;
          }
          if (!open) return;
          if (event.key === "ArrowDown" || event.key === "ArrowUp") {
            event.preventDefault();
            setActive((index) =>
              index < 0
                ? event.key === "ArrowDown"
                  ? 0
                  : items.length - 1
                : (index + (event.key === "ArrowDown" ? 1 : items.length - 1)) %
                  items.length
            );
          }
          if (active >= 0 && (event.key === "Enter" || event.key === "Tab")) {
            event.preventDefault();
            choose(items[active]);
          }
        }}
      />
      {open && (
        <ul
          ref={list}
          id="booru-tag-suggestions"
          role="listbox"
          aria-label="Tag suggestions"
        >
          {items.map((item, index) => (
            <li
              id={`booru-suggestion-${index}`}
              key={item.name}
              role="option"
              aria-selected={active === index}
              onMouseDown={(event) => event.preventDefault()}
            >
              <button
                type="button"
                onClick={() => choose(item)}
                tabIndex={-1}
                className={active === index ? "active" : ""}
              >
                <span
                  className={`booru-category-label tag-${["artist", "character", "copyright", "general", "meta", "species"].includes(item.category) ? item.category : "unknown"}`}
                >
                  {item.category}
                </span>{" "}
                <strong>{item.name}</strong>
                <small>
                  {item.category} · {item.count.toLocaleString()} ·{" "}
                  {item.source}
                  {item.alias && ` · alias: ${item.alias}`}
                </small>
              </button>
            </li>
          ))}
        </ul>
      )}
      {notice && !dismissed && focused && !exact && (
        <small className="text-muted" role="status">
          {notice}
        </small>
      )}
    </div>
  );
}
