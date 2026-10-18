import assert from "node:assert/strict";
import { test } from "node:test";
import {
  browserSearchURL,
  readBooruResponse,
  readBrowserSearch,
  remoteURL,
  motionPreviewURL,
} from "../src/components/BooruBrowser/browser.ts";

test("Browser filters round-trip with native expressions and independent paging", () => {
  const value = {
    sources: ["gelbooru"],
    query: "cat -blue_hair",
    pages: { gelbooru: 3 },
    sort: "popular" as const,
    rating: "safe" as const,
    media: "videos" as const,
    pageSize: 40,
  };
  assert.deepEqual(readBrowserSearch(browserSearchURL(value)), value);
  assert.ok(readBrowserSearch("?sort=unsupported").error);
  assert.ok(readBrowserSearch("?rating=unsupported").error);
});

test("Page size rejects unsafe provider requests and survives refresh", () => {
  for (const limit of ["0", "51", "-1", "1.5", "1e2", "NaN", ""])
    assert.ok(readBrowserSearch(`?limit=${limit}`).error);
  assert.equal(
    readBrowserSearch("?source=danbooru&limit=50&p.danbooru=2").pageSize,
    50
  );
});

test("Animated previews require an opaque key, a supported format and bounded size", () => {
  const post = {
    id: "1",
    sourceID: "danbooru",
    postURL: "https://example.org/1",
    tags: [],
    mediaKey: "a".repeat(48),
    downloadAvailable: true,
    format: "webm",
    bytes: 24 * 1048576,
  };
  assert.match(motionPreviewURL(post) ?? "", /kind=motion/);
  assert.ok(motionPreviewURL({ ...post, format: "gif" }));
  assert.equal(motionPreviewURL({ ...post, bytes: post.bytes + 1 }), undefined);
  assert.equal(motionPreviewURL({ ...post, format: "mkv" }), undefined);
  assert.equal(motionPreviewURL({ ...post, mediaKey: "forged" }), undefined);
  assert.equal(
    motionPreviewURL({ ...post, downloadAvailable: false }),
    undefined
  );
});

test("Search chips preserve quoted expressions, exclusions and comma tags", async () => {
  const { queryTerms, removeQueryTerm } = await import(
    "../src/components/BooruBrowser/completion.ts"
  );
  const query = 'cat source:"two words" -blue_hair';
  const terms = queryTerms(query);
  assert.deepEqual(
    terms.map((t) => t.name),
    ["cat", 'source:"two words"', "-blue_hair"]
  );
  assert.equal(removeQueryTerm(query, terms[1]), "cat  -blue_hair");
  const comma = "artist:a, blue hair, safe";
  assert.equal(
    removeQueryTerm(comma, queryTerms(comma, true)[1], true),
    "artist:a,  safe"
  );
});

test("Loaded-page media filters leave unknown formats distinct", async () => {
  const { postMediaKind } = await import(
    "../src/components/BooruBrowser/browser.ts"
  );
  const post = {
    id: "1",
    sourceID: "danbooru",
    postURL: "https://example.org/1",
    tags: [],
  };
  assert.equal(postMediaKind({ ...post, format: "webm" }), "videos");
  assert.equal(postMediaKind({ ...post, format: "jxl" }), "images");
  assert.equal(postMediaKind(post), "unknown");
});

test("Booru URL preserves source order, literal query, per-source page and preview identity", () => {
  const value = {
    sources: ["gelbooru", "custom"],
    query: "猫 + a&b -excluded",
    pages: { gelbooru: 2, custom: 1 },
    inspect: { source: "custom", id: "9007199254740993" },
  };
  assert.deepEqual(readBrowserSearch(browserSearchURL(value)), value);
  const updated = readBrowserSearch(
    browserSearchURL({
      ...value,
      pages: { ...value.pages, custom: 3 },
      inspect: undefined,
    })
  );
  assert.equal(updated.pages.gelbooru, 2);
  assert.equal(updated.pages.custom, 3);
  assert.equal(updated.inspect, undefined);
});

test("Booru URL validation rejects invalid/broad states without changing query semantics", () => {
  assert.equal(readBrowserSearch("").query, "");
  assert.deepEqual(readBrowserSearch("").sources, ["danbooru"]);
  for (const search of [
    "?source=../local",
    "?p.danbooru=-1",
    "?p.danbooru=751",
    "?p.danbooru=1.5",
    "?q=%00",
    `?q=${"猫".repeat(342)}`,
    "?" + Array.from({ length: 9 }, (_, i) => `source=s${i}`).join("&"),
  ]) {
    assert.ok(readBrowserSearch(search).error, search);
  }
  const deduplicated = readBrowserSearch(
    "?source=gelbooru&source=gelbooru&q=cat&inspect=other:123"
  );
  assert.deepEqual(deduplicated.sources, ["gelbooru"]);
  assert.equal(deduplicated.inspect, undefined);
});

test("Remote links cannot run scripts or include account credentials", () => {
  for (const value of [
    undefined,
    "javascript:alert(1)",
    "data:image/svg+xml,x",
    "/local",
    "http://board.example.org/x",
    "https://owner:secret@board.example.org/x",
  ]) {
    assert.equal(remoteURL(value), undefined);
  }
  assert.equal(
    remoteURL("https://board.example.org/posts/1"),
    "https://board.example.org/posts/1"
  );
});

test("Search failures show JSON messages and retry hints, never provider HTML", async () => {
  await assert.rejects(
    readBooruResponse(
      new Response(JSON.stringify({ error: "Rate limited", retryAfter: 60 }), {
        status: 429,
      })
    ),
    /Rate limited.*60 seconds/
  );
  await assert.rejects(
    readBooruResponse(
      new Response("<html>private-provider-page</html>", { status: 502 })
    ),
    (error: Error) =>
      !error.message.includes("private-provider-page") &&
      error.message.includes("HTTP 502")
  );
  assert.deepEqual(await readBooruResponse(new Response('{"posts":[]}')), {
    posts: [],
  });
});

test("Ignored-post filters round-trip without changing the provider query or pages", () => {
  const active = readBrowserSearch(
    "?source=danbooru&q=literal+-tag&p.danbooru=3&ignored=1"
  );
  assert.equal(active.showIgnored, true);
  assert.equal(readBrowserSearch(browserSearchURL(active)).showIgnored, true);
  const hidden = readBrowserSearch(
    browserSearchURL({ ...active, showIgnored: undefined })
  );
  assert.equal(hidden.showIgnored, undefined);
  assert.equal(hidden.query, active.query);
  assert.equal(hidden.pages.danbooru, 3);
  assert.match(
    readBrowserSearch("?ignored=oops").error ?? "",
    /Invalid ignored-post filter/
  );
});

test("Autocomplete replaces only the caret token and preserves exclusions and Philomena commas", async () => {
  const { completionToken, applyCompletion } = await import(
    "../src/components/BooruBrowser/completion.ts"
  );
  for (const [query, caret, comma, name, expected] of [
    ["cat -blu dog", 8, false, "blue_hair", "cat -blue_hair dog"],
    ["~blu fox", 4, false, "blue_eyes", "~blue_eyes fox"],
    [
      "artist_a, blue h, rating:safe",
      16,
      true,
      "blue hair",
      "artist_a, blue hair, rating:safe",
    ],
  ] as const) {
    assert.equal(
      applyCompletion(query, completionToken(query, caret, comma), name),
      expected
    );
  }
});

test("Browser memory keeps a submitted page and unfinished draft separately", async () => {
  const { readBrowserMemory, saveBrowserMemory } = await import(
    "../src/components/BooruBrowser/browser.ts"
  );
  let stored: string | null = null;
  const previous = Object.getOwnPropertyDescriptor(globalThis, "localStorage");
  Object.defineProperty(globalThis, "localStorage", {
    configurable: true,
    value: {
      getItem: () => stored,
      setItem: (_key: string, value: string) => {
        stored = value;
      },
    },
  });
  try {
    const state = {
      search: "?source=gelbooru&q=landscape&p.gelbooru=3",
      query: "unfinished -blue",
      sources: ["gelbooru", "danbooru"],
      settings: false,
      tagSource: "gelbooru",
      scroll: 612,
    };
    saveBrowserMemory(state);
    assert.deepEqual(readBrowserMemory(), state);
    stored =
      '{"search":"?source=../bad","query":"a","sources":[],"settings":false,"tagSource":"","scroll":0}';
    assert.equal(readBrowserMemory(), undefined);
  } finally {
    if (previous) Object.defineProperty(globalThis, "localStorage", previous);
    else Reflect.deleteProperty(globalThis, "localStorage");
  }
});
