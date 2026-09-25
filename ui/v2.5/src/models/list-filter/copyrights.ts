import { ListFilterOptions } from "./filter-options";
import { DisplayMode } from "./types";

const defaultSortBy = "sort_name";
const sortByOptions = [
  "name",
  "sort_name",
  "created_at",
  "updated_at",
  "image_count",
  "scene_count",
  "performer_count",
].map(ListFilterOptions.createSortBy);
const displayModeOptions = [DisplayMode.Grid, DisplayMode.List];

export const CopyrightListFilterOptions = new ListFilterOptions(
  defaultSortBy,
  sortByOptions,
  displayModeOptions,
  []
);
