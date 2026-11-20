import type { IntlShape } from "react-intl";
import type { FilterMode } from "./generated-graphql";

// Message IDs for each list type; the fork's names come from the locale.
// Keys are FilterMode values (kept as strings so this has no runtime import).
export const FilterModeToMessageID: Record<FilterMode, string> = {
  COPYRIGHTS: "copyrights",
  GALLERIES: "galleries",
  GROUPS: "groups",
  IMAGES: "images",
  MEDIA: "media",
  MOVIES: "groups",
  PERFORMERS: "performers",
  SCENES: "scenes",
  SCENE_MARKERS: "markers",
  STUDIOS: "studios",
  TAGS: "tags",
} as Record<FilterMode, string>;

const objectTitleMessages = [
  "recently_released_objects",
  "recently_added_objects",
];

interface TitledRow {
  message?: { id: string; values: { [key: string]: string } };
  title?: string;
  mode: FilterMode;
}

/**
 * A main-page row's title. Premade rows saved their list name as text
 * ("Scenes"); translate it from the row's type instead, so older layouts
 * use the current names (Videos, Characters, Artists…).
 */
export function customFilterTitle(intl: IntlShape, filter: TitledRow) {
  if (!filter.message) return filter.title ?? "";
  const values = { ...filter.message.values };
  const objects = FilterModeToMessageID[filter.mode];
  if (objectTitleMessages.includes(filter.message.id) && objects)
    values.objects = intl.formatMessage({ id: objects });
  return intl.formatMessage({ id: filter.message.id }, values);
}
