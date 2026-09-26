import assert from "node:assert/strict";
import test from "node:test";

import {
  addFileFilterInput,
  imageFormatFileFilterInput,
  sceneFormatFileFilterInput,
} from "../src/models/list-filter/criteria/file-filter.ts";

const format = { modifier: "EQUALS", value: "jxl" };

test("image format criterion matches image and video-backed image files", () => {
  const filter: Record<string, unknown> = {};
  addFileFilterInput(filter, imageFormatFileFilterInput(format));

  assert.deepEqual(filter, {
    files_filter: {
      video_file_filter: {
        format: { modifier: "EQUALS", value: "jxl" },
      },
      OR: {
        image_file_filter: {
          format: { modifier: "EQUALS", value: "jxl" },
        },
      },
    },
  });
});

test("scene format criterion matches the scene video file format", () => {
  const filter: Record<string, unknown> = {};
  addFileFilterInput(filter, sceneFormatFileFilterInput(format));

  assert.deepEqual(filter, {
    files_filter: {
      video_file_filter: {
        format: { modifier: "EQUALS", value: "jxl" },
      },
    },
  });
});

test("format and folder criteria preserve both regardless of selection order", () => {
  const formatFirst: Record<string, unknown> = {};
  addFileFilterInput(formatFirst, imageFormatFileFilterInput(format));
  addFileFilterInput(formatFirst, {
    parent_folder: { modifier: "INCLUDES", value: ["folder-id"] },
  });

  const folderFirst: Record<string, unknown> = {};
  addFileFilterInput(folderFirst, {
    parent_folder: { modifier: "INCLUDES", value: ["folder-id"] },
  });
  addFileFilterInput(folderFirst, imageFormatFileFilterInput(format));

  assert.deepEqual(formatFirst, folderFirst);
  assert.deepEqual(formatFirst, {
    files_filter: {
      parent_folder: {
        modifier: "INCLUDES",
        value: ["folder-id"],
      },
      AND: {
        video_file_filter: {
          format: { modifier: "EQUALS", value: "jxl" },
        },
        OR: {
          image_file_filter: {
            format: { modifier: "EQUALS", value: "jxl" },
          },
        },
      },
    },
  });
});
