import assert from "node:assert/strict";
import test from "node:test";

import {
  imageAssociationHeading,
  imageDisplayHeading,
  formattedImageFilenameHeading,
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

test("formatted filename fallback is opt-in and only applies without association names", () => {
  const untagged = { id: "42", title: null };
  const options = {
    showImageAssociationTitles: true,
    showFormattedImageFilenameFallback: true,
  };
  assert.equal(
    imageDisplayHeading(untagged, "emilia_winter_outfit.PNG", options),
    "Emilia winter outfit"
  );
  assert.equal(
    imageDisplayHeading(untagged, "emilia_winter_outfit.PNG", {
      showImageAssociationTitles: true,
    }),
    "Image #42"
  );
  assert.equal(
    imageDisplayHeading(untagged, "emilia_winter_outfit.PNG", {
      showFormattedImageFilenameFallback: true,
    }),
    "emilia_winter_outfit.PNG"
  );
  assert.equal(
    imageDisplayHeading(untagged, "emilia_winter_outfit.PNG", {
      hideImageFilenames: true,
      showFormattedImageFilenameFallback: true,
    }),
    null
  );
  assert.equal(
    imageDisplayHeading(
      { ...untagged, performers: [{ name: "Emilia" }] },
      "emilia_winter_outfit.PNG",
      options
    ),
    "Emilia"
  );
  assert.equal(
    imageDisplayHeading(
      { ...untagged, title: "Handpicked title" },
      "emilia_winter_outfit.PNG",
      options
    ),
    "Handpicked title"
  );
  assert.equal(imageDisplayHeading(untagged, "", options), "Image #42");
  assert.equal(
    imageDisplayHeading(untagged, "No File Name", options),
    "Image #42"
  );
});

test("filename fallback cleans separators, extensions and machine hashes", () => {
  assert.equal(
    formattedImageFilenameHeading("/media/emilia_winter--outfit_2024.JXL"),
    "Emilia winter outfit 2024"
  );
  assert.equal(
    formattedImageFilenameHeading(
      "booru-danbooru-post_014_aabbccddeeff00112233445566778899.webp"
    ),
    "Booru danbooru post 014"
  );
  assert.equal(
    formattedImageFilenameHeading("aabbccddeeff00112233445566778899.png"),
    "Aabbccddeeff…"
  );
  assert.equal(formattedImageFilenameHeading("2024-10-25.jpg"), "2024-10-25");
  assert.equal(formattedImageFilenameHeading(""), "");
});

test("configured filename layout fills missing image title fields without tagging media", () => {
  const md5 = "2c20e78d4c3163a761873936d4380ead";
  const layout = "%copyright% - %character% - %artist%_%md5%.%ext%";
  const filename = `re_zero - emilia - source_artist_${md5}.jxl`;
  const image = {
    id: "7",
    title: null,
    artists: [{ name: "Canonical Artist" }],
    visual_files: [{ path: `/media/${filename}` }],
  };
  const options = {
    showImageAssociationTitles: true,
    showFormattedImageFilenameFallback: true,
    imageFilenameLayout: layout,
  };
  assert.equal(
    imageDisplayHeading(image, filename, options),
    "re zero · emilia · Canonical Artist"
  );
  assert.equal(
    imageDisplayHeading({ ...image, title: "My custom title" }, filename, options),
    "My custom title"
  );
  assert.equal(
    imageDisplayHeading(image, filename, {
      ...options,
      showFormattedImageFilenameFallback: false,
    }),
    "Canonical Artist"
  );
  assert.equal(
    imageDisplayHeading(image, "different.jpg", {
      ...options,
      imageFilenameLayout: "%artist%/%copyright%.%character%_%md5%.%ext%",
    }),
    "Canonical Artist"
  );
});

test("folder-based layout parses full file path and fills only missing slots", () => {
  const md5 = "2c20e78d4c3163a761873936d4380ead";
  const image = {
    title: null,
    id: "8",
    primary_copyright: { name: "Canonical Series" },
    visual_files: [{ path: `/download/[source_artist]/(re_zero).emilia_${md5}.jxl` }],
  };
  const options = {
    showImageAssociationTitles: true,
    showFormattedImageFilenameFallback: true,
    imageFilenameLayout: "[%artist%]/(%copyright%).%character%_%md5%.%ext%",
  };
  assert.equal(
    imageDisplayHeading(image, `(re_zero).emilia_${md5}.jxl`, options),
    "Canonical Series · emilia · source artist"
  );
  const mismatch = {
    ...image,
    visual_files: [{ path: "/media/unmatched_random_name.jxl" }],
  };
  assert.equal(
    imageDisplayHeading(mismatch, "unmatched_random_name.jxl", options),
    "Canonical Series"
  );
});
