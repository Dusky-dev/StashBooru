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
