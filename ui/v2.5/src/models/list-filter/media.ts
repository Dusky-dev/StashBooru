import { BooleanCriterionOption } from "./criteria/criterion";
import { ImageListFilterOptions } from "./images";
import { DurationCriterionOption } from "./scenes";
import { MediaKindCriterionOption } from "./criteria/media-types";
import { ListFilterOptions } from "./filter-options";
import { DisplayMode } from "./types";

export const CollapseStacksOption = new BooleanCriterionOption(
  "visual_stacks",
  "collapse_stacks"
);

const shared = new Set([
  "title",
  "details",
  "path",
  "rating100",
  "date",
  "created_at",
  "updated_at",
  "organized",
  "performer_favorite",
  "tags",
  "performers",
  "studios",
  "copyrights",
]);

export const MediaListFilterOptions = new ListFilterOptions(
  "date",
  ["date", "title", "rating", "path", "filesize"].map(
    ListFilterOptions.createSortBy
  ),
  [DisplayMode.Grid, DisplayMode.Wall],
  [
    MediaKindCriterionOption,
    ...ImageListFilterOptions.criterionOptions.filter((c) =>
      shared.has(c.type)
    ),
    DurationCriterionOption,
    CollapseStacksOption,
  ]
);
