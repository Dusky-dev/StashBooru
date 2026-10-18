export function completionToken(value: string, caret: number, comma = false) {
  const separator = comma ? /,/ : /\s/;
  let start = caret;
  let end = caret;
  while (start > 0 && !separator.test(value[start - 1])) start--;
  while (end < value.length && !separator.test(value[end])) end++;
  while (start < caret && value[start] === " ") start++;
  const operator = /^[~-]/.test(value[start] ?? "") ? value[start++] : "";
  const prefix = value.slice(start, caret);
  return { start, end, prefix, operator };
}
export function applyCompletion(
  value: string,
  token: ReturnType<typeof completionToken>,
  name: string
) {
  return value.slice(0, token.start) + name + value.slice(token.end);
}

// Keep exact spans so removing a chip never rewrites native operators or quoting.
export function queryTerms(value: string, comma = false) {
  const terms: { name: string; start: number; end: number }[] = [];
  let start = -1,
    quoted = false,
    escaped = false;
  for (let i = 0; i <= value.length; i++) {
    const char = value[i];
    if (char === '"' && !escaped) quoted = !quoted;
    const boundary =
      i === value.length ||
      (!quoted && (comma ? char === "," : /\s/.test(char)));
    if (boundary && start >= 0) {
      const raw = value.slice(start, i),
        leading = raw.length - raw.trimStart().length;
      terms.push({ name: raw.trim(), start: start + leading, end: i });
      start = -1;
    } else if (!boundary && start < 0) start = i;
    escaped = char === "\\" && !escaped;
  }
  return terms.filter((t) => t.name);
}
export function removeQueryTerm(
  value: string,
  term: { start: number; end: number },
  comma = false
) {
  let start = term.start,
    end = term.end;
  if (comma) {
    while (end < value.length && /\s/.test(value[end])) end++;
    if (value[end] === ",") end++;
    else {
      while (start > 0 && /\s/.test(value[start - 1])) start--;
      if (value[start - 1] === ",") start--;
    }
  }
  return (value.slice(0, start) + value.slice(end)).trim();
}
