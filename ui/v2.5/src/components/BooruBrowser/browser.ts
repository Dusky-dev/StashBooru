export interface BooruSource {
  instance: string;
  id: string;
  name: string;
  engine: string;
  url: string;
  queryHelp: string;
  authentication: string;
  credentialsConfigured: boolean;
  categorizedTags: boolean;
}

export interface BooruPost {
  id: string;
  sourceID: string;
  postURL: string;
  previewURL?: string;
  sampleURL?: string;
  mediaKey?: string;
  downloadAvailable?: boolean;
  width?: number;
  height?: number;
  bytes?: number;
  md5?: string;
  rating?: string;
  format?: string;
  tags: { name: string; category: string }[];
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
  error?: string;
}

const validSource = /^[a-z0-9][a-z0-9_-]{0,63}$/;

export function readBrowserSearch(search: string): BrowserSearch {
  const params = new URLSearchParams(search);
  const sources = [...new Set(params.getAll("source"))];
  if (sources.length === 0) sources.push("danbooru");
  const query = params.get("q") ?? "";
  const result: BrowserSearch = { sources, query, pages: {} };
  if (
    sources.length > 8 ||
    sources.some((source) => !validSource.test(source)) ||
    new TextEncoder().encode(query).length > 1024 ||
    ["\r", "\n", "\0"].some((control) => query.includes(control))
  ) {
    result.error =
      "Invalid search URL. Choose up to eight sources and a search of at most 1024 bytes.";
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
  const inspect = params.get("inspect")?.split(":");
  if (
    inspect?.length === 2 &&
    sources.includes(inspect[0]) &&
    /^[1-9][0-9]{0,19}$/.test(inspect[1])
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
  if (value.showIgnored) params.set("ignored", "1");
  if (value.query) params.set("q", value.query);
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
