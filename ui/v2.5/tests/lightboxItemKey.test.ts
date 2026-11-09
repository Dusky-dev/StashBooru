import assert from "node:assert/strict";
import test from "node:test";
import { lightboxEntryKey } from "../src/hooks/Lightbox/itemKey.ts";

test("typed Image and Video identities cannot collide", () => {
  const image = {
    id: "7",
    mediaReference: { kind: "IMAGE" as const, id: "7" },
    paths: { image: "/image/7" },
  };
  const video = {
    mediaReference: { kind: "VIDEO" as const, id: "7" },
    paths: { image: "/image/7" },
  };
  assert.equal(lightboxEntryKey(image, 0), "IMAGE:7");
  assert.equal(lightboxEntryKey(video, 1), "VIDEO:7");
});

test("videos without screenshots still get independent React slide keys", () => {
  const blank = { paths: {} };
  assert.notEqual(lightboxEntryKey(blank, 0), lightboxEntryKey(blank, 1));
  assert.equal(
    lightboxEntryKey({ ...blank, href: "/scenes/17" }, 0),
    "href:/scenes/17"
  );
});

test("native Image-only previews continue to key by Image identity", () => {
  assert.equal(lightboxEntryKey({ id: "21", paths: {} }, 0), "IMAGE:21");
});
