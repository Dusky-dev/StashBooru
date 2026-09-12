import { ListFilterOptions } from "./filter-options";
import { DisplayMode } from "./types";

const defaultSortBy = "name";
const sortByOptions = ["name", "created_at", "updated_at"].map(
  ListFilterOptions.createSortBy
);
const displayModeOptions = [DisplayMode.Grid, DisplayMode.List];
const criterionOptions = [];

export const CopyrightListFilterOptions = new ListFilterOptions(
  defaultSortBy,
  sortByOptions,
  displayModeOptions,
  criterionOptions
);
