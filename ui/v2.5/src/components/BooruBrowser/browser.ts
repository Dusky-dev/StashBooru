export interface BooruSource {
  instance: string;
  id: string;
  name: string;
  engine: string;
  url: string;
  queryHelp: string;
  authentication: string;
  credentialsConfigured: boolean;
  accountFavorites?: boolean;
  categorizedTags: boolean;
}

export interface BooruPost {
  id: string;
  sourceID: string;
  postURL: string;
  previewURL?: string;
  sampleURL?: string;
  directPreviewURL?: string;
  directSampleURL?: string;
  mediaKey?: string;
  downloadAvailable?: boolean;
  width?: number;
  height?: number;
  bytes?: number;
  md5?: string;
  rating?: string;
  format?: string;
  tags: { name: string; category: string; label?: string }[];
  attachments?: BooruPost[];
  remoteID?: string;
  title?: string;
  favorite?: boolean;
  mediaNotice?: string;
}

export interface BooruResult {
  sourceID: string;
  posts: BooruPost[];
  page: number;
  limit: number;
  hasMore: boolean;
  total?: number;
  fetchedAt: string;
}

export interface BrowserSearch {
  sources: string[];
  query: string;
  pages: Record<string, number>;
  inspect?: { source: string; id: string };
  showIgnored?: boolean;
  pageSize?: number;
  favorites?: boolean;
  sort?:
    | "newest"
    | "popular"
    | "updated"
    | "oldest"
    | "score_asc"
    | "score_desc";
  rating?: "safe" | "sensitive" | "questionable" | "explicit";
  media?: "images" | "videos";
  hideAI?: boolean;
  error?: string;
}

const validSource = /^[a-z0-9][a-z0-9_-]{0,63}$/;

export function readBrowserSearch(search: string): BrowserSearch {
  const params = new URLSearchParams(search);
  const requestedSources = [...new Set(params.getAll("source"))];
  const sources = requestedSources.slice(0, 1);
  if (sources.length === 0) sources.push("danbooru");
  const query = params.get("q") ?? "";
  const result: BrowserSearch = { sources, query, pages: {} };
  if (
    requestedSources.length > 8 ||
    requestedSources.some((source) => !validSource.test(source)) ||
    new TextEncoder().encode(query).length > 1024 ||
    ["\r", "\n", "\0"].some((control) => query.includes(control))
  ) {
    result.error =
      "Invalid search URL. Choose one source and a search of at most 1024 bytes.";
  }
  for (const source of sources) {
    const raw = params.get(`p.${source}`) ?? "1";
    const page = Number(raw);
    if (
      !/^\d+$/.test(raw) ||
      !Number.isInteger(page) ||
      page < 1 ||
      page > 750
    ) {
      result.error = "Invalid page in search URL. Pages must be from 1 to 750.";
    }
    result.pages[source] = page;
  }
  if (params.get("ignored") === "1") result.showIgnored = true;
  else if (params.has("ignored") && params.get("ignored") !== "0")
    result.error = "Invalid ignored-post filter.";
  for (const [key, choices] of Object.entries({
    sort: ["newest", "popular", "updated", "oldest", "score_asc", "score_desc"],
    rating: ["safe", "sensitive", "questionable", "explicit"],
    media: ["images", "videos"],
  })) {
    const value = params.get(key);
    if (value && !choices.includes(value))
      result.error = "Invalid browser filter.";
    else if (value) Object.assign(result, { [key]: value });
  }
  if (params.has("limit")) {
    const limit = params.get("limit") ?? "";
    if (!/^\d+$/.test(limit) || Number(limit) < 1 || Number(limit) > 50)
      result.error = "Page size must be from 1 to 50.";
    else result.pageSize = Number(limit);
  }
  if (params.get("ai") === "hide") result.hideAI = true;
  else if (params.has("ai")) result.error = "Invalid AI-generated filter.";
  if (params.get("favorites") === "1") result.favorites = true;
  const inspect = params.get("inspect")?.split(":");
  if (
    inspect?.length === 2 &&
    sources.includes(inspect[0]) &&
    /^(?:[1-9][0-9]{0,19}|[ap][a-f0-9]{32})$/.test(inspect[1])
  ) {
    result.inspect = { source: inspect[0], id: inspect[1] };
  }
  return result;
}

export function browserSearchURL(value: BrowserSearch): string {
  const params = new URLSearchParams();
  for (const source of value.sources) {
    params.append("source", source);
    if ((value.pages[source] ?? 1) > 1) {
      params.set(`p.${source}`, String(value.pages[source]));
    }
  }
  if (value.favorites) params.set("favorites", "1");
  if (value.pageSize) params.set("limit", String(value.pageSize));
  if (value.showIgnored) params.set("ignored", "1");
  if (value.hideAI) params.set("ai", "hide");
  for (const key of ["sort", "rating", "media"] as const)
    if (value[key]) params.set(key, value[key]);
  params.set("q", value.query);
  if (value.inspect) {
    params.set("inspect", `${value.inspect.source}:${value.inspect.id}`);
  }
  return `?${params}`;
}

// Defence in depth for browser-rendered links. The server also validates source
// and CDN hosts; never let malformed API responses create active script URLs.
export function remoteURL(value?: string): string | undefined {
  if (!value) return undefined;
  try {
    const parsed = new URL(value);
    if (parsed.protocol !== "https:" || parsed.username || parsed.password) {
      return undefined;
    }
    return parsed.href;
  } catch {
    return undefined;
  }
}

export async function readBooruResponse<T>(response: Response): Promise<T> {
  if (!response.ok) {
    let error = `Search failed (HTTP ${response.status}).`;
    try {
      const body = (await response.json()) as {
        error?: string;
        retryAfter?: number;
      };
      if (typeof body.error === "string") error = body.error;
      if (body.retryAfter && body.retryAfter > 0) {
        error += ` Retry after ${body.retryAfter} seconds.`;
      }
    } catch {
      // Never display HTML or reflected provider URLs from a failed response.
    }
    throw new Error(error);
  }
  return response.json() as Promise<T>;
}

// Keys are issued only for server-owned provider snapshots. Never proxy a URL supplied by the browser.
export function booruMediaURL(
  post: BooruPost,
  kind: "preview" | "sample"
): string | undefined {
  if (!post.mediaKey || !/^[a-f0-9]{48}$/.test(post.mediaKey)) return undefined;
  if (!(kind === "preview" ? post.previewURL : post.sampleURL))
    return undefined;
  return `booru-browser/media?${new URLSearchParams({ key: post.mediaKey, kind })}`;
}

// Animated originals are fetched only after an explicit Play action, bounded by
// the server's preview route. Unsupported codecs remain remote/download-only.
export function motionPreviewURL(post: BooruPost): string | undefined {
  if (
    !post.mediaKey ||
    !/^[a-f0-9]{48}$/.test(post.mediaKey) ||
    !post.downloadAvailable ||
    (post.bytes ?? 0) > 24 * 1048576 ||
    !["mp4", "webm", "gif", "apng"].includes(post.format?.toLowerCase() ?? "")
  )
    return undefined;
  return `booru-browser/media?${new URLSearchParams({ key: post.mediaKey, kind: "motion" })}`;
}

export interface BrowserMemory {
  search: string;
  query: string;
  sources: string[];
  settings: boolean;
  tagSource: string;
  scroll: number;
  thumbnails?: boolean;
  autocomplete?: "live" | "database";
  displayMode?: "grid" | "wall";
  zoom?: number;
  sourceOrder?: string[];
  searchOnTagClick?: boolean;
}
const key = "booru-browser-state-v1";
export function readBrowserMemory(): BrowserMemory | undefined {
  try {
    const raw = localStorage.getItem(key);
    if (!raw || raw.length > 16384) return undefined;
    const value = JSON.parse(raw) as BrowserMemory;
    if (
      typeof value.search !== "string" ||
      readBrowserSearch(value.search).error ||
      typeof value.query !== "string" ||
      value.query.length > 1024 ||
      !Array.isArray(value.sources) ||
      value.sources.length > 8 ||
      value.sources.some(
        (s) => typeof s !== "string" || !/^[a-z0-9][a-z0-9_-]{0,63}$/.test(s)
      ) ||
      typeof value.settings !== "boolean" ||
      typeof value.tagSource !== "string" ||
      (value.thumbnails !== undefined &&
        typeof value.thumbnails !== "boolean") ||
      (value.autocomplete !== undefined &&
        !["live", "database"].includes(value.autocomplete)) ||
      (value.displayMode !== undefined &&
        !["grid", "wall"].includes(value.displayMode)) ||
      (value.zoom !== undefined &&
        (!Number.isInteger(value.zoom) || value.zoom < 0 || value.zoom > 3)) ||
      (value.searchOnTagClick !== undefined &&
        typeof value.searchOnTagClick !== "boolean") ||
      (value.sourceOrder !== undefined &&
        (!Array.isArray(value.sourceOrder) ||
          value.sourceOrder.length > 128 ||
          value.sourceOrder.some(
            (id) => typeof id !== "string" || !validSource.test(id)
          ) ||
          new Set(value.sourceOrder).size !== value.sourceOrder.length)) ||
      !Number.isFinite(value.scroll)
    )
      return undefined;
    return value;
  } catch {
    return undefined;
  }
}
export function saveBrowserMemory(value: BrowserMemory) {
  try {
    localStorage.setItem(key, JSON.stringify(value));
  } catch {
    /* Browsing still works when storage is unavailable. */
  }
}

export function postMediaKind(
  post: BooruPost
): "images" | "videos" | "unknown" {
  const ext = post.format?.toLowerCase();
  if (ext && ["mp4", "webm", "mkv", "mov", "avi", "m4v"].includes(ext))
    return "videos";
  if (
    ext &&
    [
      "jpg",
      "jpeg",
      "png",
      "gif",
      "apng",
      "webp",
      "avif",
      "jxl",
      "bmp",
      "tiff",
    ].includes(ext)
  )
    return "images";
  return "unknown";
}

// Match only explicit provider tags. AI-assisted and AI-generated backgrounds
// are distinct tags and do not assert that the whole post is AI-generated.
export function isAIGeneratedPost(post: BooruPost): boolean {
  return post.tags.some(
    (tag) => tag.name.toLowerCase().replace(/[-\s]/g, "_") === "ai_generated"
  );
}

export function browserResultSignature(
  query: string,
  page: number,
  sort?: string,
  rating?: string,
  media?: string,
  pageSize?: number,
  favorites?: boolean,
  hideAI?: boolean
) {
  return JSON.stringify([
    query,
    page,
    sort ?? "",
    rating ?? "",
    media ?? "",
    pageSize ?? 24,
    !!favorites,
    !!hideAI,
  ]);
}
