import assert from "node:assert/strict";
import test from "node:test";
import { createIntl } from "react-intl";

import { customFilterTitle } from "../src/core/frontPageTitles.ts";
import type { FilterMode } from "../src/core/generated-graphql.ts";

const intl = createIntl({
  locale: "en-GB",
  messages: {
    recently_released_objects: "Recently Released {objects}",
    recently_added_objects: "Recently Added {objects}",
    scenes: "Videos",
    performers: "Characters",
    studios: "Artists",
  },
});

test("main-page rows saved with old list names show the current names", () => {
  const row = (mode: string, id: string, objects: string) => ({
    message: { id, values: { objects } },
    mode: mode as FilterMode,
  });
  assert.equal(
    customFilterTitle(
      intl,
      row("SCENES", "recently_released_objects", "Scenes")
    ),
    "Recently Released Videos"
  );
  assert.equal(
    customFilterTitle(
      intl,
      row("PERFORMERS", "recently_added_objects", "Performers")
    ),
    "Recently Added Characters"
  );
  assert.equal(
    customFilterTitle(intl, { title: "My row", mode: "STUDIOS" as FilterMode }),
    "My row"
  );
});
