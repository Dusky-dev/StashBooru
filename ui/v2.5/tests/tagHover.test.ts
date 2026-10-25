import assert from "node:assert/strict";
import test from "node:test";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";

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
