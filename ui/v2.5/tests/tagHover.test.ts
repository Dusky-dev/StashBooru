import assert from "node:assert/strict";
import test from "node:test";
import React from "react";
import { createRequire } from "node:module";

type Renderer = typeof import("react-dom/server");
const require = createRequire(import.meta.url);
const serverRenderer: Renderer = require("react-dom/server");
const { renderToStaticMarkup } = serverRenderer;

import { TagPopover } from "../src/components/Tags/TagPopover.tsx";

test("ordinary Tag hover wrapper never renders a popup or extra DOM", () => {
  const label = React.createElement(
    "span",
    { className: "tag-name" },
    "General Tag"
  );

  for (const props of [
    { id: "general:123" },
    { id: "general:123", placement: "right" as const },
    { id: "general:123", hide: false },
  ]) {
    assert.equal(
      renderToStaticMarkup(React.createElement(TagPopover, props, label)),
      '<span class="tag-name">General Tag</span>'
    );
  }
});
