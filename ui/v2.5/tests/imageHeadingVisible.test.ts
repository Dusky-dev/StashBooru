import assert from "node:assert/strict";
import test from "node:test";

import {
  imageAssociationHeading,
  imageDisplayHeading,
  imageHeadingVisible,
} from "../src/utils/imageHeadingVisible.ts";

test("image filename headings are visible by default", () => {
  assert.equal(imageHeadingVisible({ title: null }), true);
  assert.equal(imageHeadingVisible({}), true);
  assert.equal(imageHeadingVisible({ title: "" }), true);
});

test("hide filename fallback but keep explicit image titles", () => {
  assert.equal(imageHeadingVisible({ title: null }, true), false);
  assert.equal(imageHeadingVisible({ title: "" }, true), false);
  assert.equal(imageHeadingVisible({}, true), false);
  assert.equal(imageHeadingVisible({ title: "Holiday photo" }, true), true);
  assert.equal(imageHeadingVisible({ title: "custom.jpg" }, true), true);
  assert.equal(imageHeadingVisible({ title: "Holiday photo" }, false), true);
});

test("metadata headings replace filename fallbacks in Copyright / Character / Artist order", () => {
  const image = {
    id: "42",
    title: null,
    primary_copyright: { name: "Re:Zero" },
    copyrights: [{ name: "Other series" }],
    performers: [{ name: "Emilia" }, { name: "Rem" }],
    artists: [{ name: "Illustrator" }],
    studio: { name: "Legacy Artist" },
  };
  assert.equal(
    imageAssociationHeading(image),
    "Re:Zero · Emilia, Rem · Illustrator"
  );
  assert.equal(
    imageDisplayHeading(image, "4a1c.jpg", {
      showImageAssociationTitles: true,
    }),
    "Re:Zero · Emilia, Rem · Illustrator"
  );
  assert.equal(imageDisplayHeading(image, "4a1c.jpg"), "4a1c.jpg");
  assert.equal(
    imageDisplayHeading(image, "4a1c.jpg", { hideImageFilenames: true }),
    null
  );
  assert.equal(
    imageDisplayHeading({ ...image, title: "My custom title" }, "4a1c.jpg", {
      showImageAssociationTitles: true,
    }),
    "My custom title"
  );
  assert.equal(
    imageDisplayHeading({ ...image, title: "My custom title" }, "4a1c.jpg", {
      hideImageFilenames: true,
    }),
    "My custom title"
  );
});

test("metadata titles handle missing associations, duplicate names and older Artist links", () => {
  assert.equal(
    imageAssociationHeading({ id: "8", copyrights: [], performers: [] }),
    "Image #8"
  );
  assert.equal(imageAssociationHeading({}), "Image");
  assert.equal(
    imageAssociationHeading({
      id: "3",
      copyrights: [{ name: "Fate" }, { name: "Fate" }],
      performers: [{ name: "Saber" }, { name: "Rin" }, { name: "Archer" }],
      artists: [],
      studio: { name: "Some Artist" },
    }),
    "Fate · Saber, Rin +1 · Some Artist"
  );
  assert.equal(
    imageAssociationHeading({
      id: "5",
      copyrights: [{ name: "  " }],
      performers: [{ name: "Hero" }],
    }),
    "Hero"
  );
  assert.equal(
    imageDisplayHeading({ id: 99, title: "" }, "literal.png", {
      showImageAssociationTitles: true,
      hideImageFilenames: true,
    }),
    "Image #99"
  );
});
