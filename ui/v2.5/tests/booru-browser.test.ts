import assert from "node:assert/strict";
import { test } from "node:test";
import {
  browserSearchURL,
  readBooruResponse,
  readBrowserSearch,
  remoteURL,
} from "../src/components/BooruBrowser/browser.ts";

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
