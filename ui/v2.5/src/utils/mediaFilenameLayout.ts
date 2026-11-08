/**
 * The shared image filename layout uses the Booru/Grabber %token% vocabulary.
 * This parser only interprets filenames for display; it does not tag or rename
 * media. The Booru downloader has the authoritative renderer and path checks.
 */
export const exampleImageFilenameLayout =
  "[%artist%]/(%copyright%).%character%_%md5%.%ext%";

const supportedTokens = new Set([
  "id", "asset_id", "md5", "ext", "artist", "copyright", "character",
  "filename", "rating", "website", "websitename", "width", "height",
  "author", "authorid", "position", "score", "date", "current_date",
  "name", "title", "num",
]);
const categoryTokens = new Set(["copyright", "character", "artist"]);
const templateToken = /%([a-z_][a-z0-9_]*)%/g;

export interface FilenameCategoryFields {
  copyright?: string;
  character?: string;
  artist?: string;
}

export function validateImageFilenameLayout(layout: string): string | undefined {
  if (!layout) return undefined;
  if (
    layout.length > 512 ||
    layout.startsWith("/") ||
    layout.includes("\\") ||
    layout.includes(":") ||
    [...layout].some((char) => {
      const code = char.charCodeAt(0);
      return code < 32 || code === 127;
    }) ||
    !layout.endsWith(".%ext%") ||
    layout.split("/").length > 8 ||
    layout.split("/").some((part) => !part || part === "." || part === "..")
  ) {
    return "Use a relative layout ending with .%ext% (up to eight path components).";
  }
  const matches = [...layout.matchAll(templateToken)];
  const names = new Set<string>();
  const left = layout.replace(templateToken, "");
  if (left.includes("%")) return "Use valid %token% placeholders.";
  for (const match of matches) {
    const token = match[1];
    if (!supportedTokens.has(token)) return `Unsupported token: %${token}%`;
    if (categoryTokens.has(token) && names.has(token)) {
      return `Use %${token}% only once to avoid ambiguous titles.`;
    }
    names.add(token);
  }
  // Adjacent captures have no unique boundary, so cannot be decoded safely.
  if (/%[a-z_][a-z0-9_]*%%[a-z_][a-z0-9_]*%/.test(layout)) {
    return "Separate adjacent placeholders with literal characters.";
  }
  return undefined;
}

function regexEscape(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function displayCategory(value: string): string | undefined {
  const names = value
    .split("+")
    .map((part) => part.trim())
    .filter((part) => part && part.toLowerCase() !== "unknown")
    .map((part) => part.replace(/_+/g, " ").replace(/\s+/g, " ").trim())
    .filter(Boolean);
  if (!names.length) return undefined;
  const unique = [...new Set(names)];
  const shown = unique.slice(0, 2).join(", ");
  return unique.length > 2 ? `${shown} +${unique.length - 2}` : shown;
}

/** Only decode fields if the complete path suffix matches the configured layout. */
export function parseImageFilenameLayout(
  filenamePath: string,
  layout: string
): FilenameCategoryFields | undefined {
  if (!filenamePath || !layout || validateImageFilenameLayout(layout)) {
    return undefined;
  }
  const captureNames: string[] = [];
  let cursor = 0;
  let expression = "";
  for (const match of layout.matchAll(templateToken)) {
    expression += regexEscape(layout.slice(cursor, match.index));
    const name = match[1];
    captureNames.push(name);
    expression +=
      name === "ext"
        ? "([A-Za-z0-9]+)"
        : name === "md5"
          ? "([a-fA-F0-9]{32}|unknown)"
          : "([^/]+?)";
    cursor = match.index + match[0].length;
  }
  expression += regexEscape(layout.slice(cursor));
  const input = filenamePath.replace(/\\/g, "/");
  const result = new RegExp(`(?:^|/)${expression}$`, "i").exec(input);
  if (!result) return undefined;

  const fields: FilenameCategoryFields = {};
  captureNames.forEach((name, index) => {
    if (categoryTokens.has(name)) {
      const value = displayCategory(result[index + 1]);
      if (value) fields[name as keyof FilenameCategoryFields] = value;
    }
  });
  return fields;
}
