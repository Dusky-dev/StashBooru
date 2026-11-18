import terminology from "../../../src/locales/booru-terminology.json";
import english from "../../../src/locales/en-GB.json";

// The app flattens nested locale messages and overlays fork terminology.
function flatten(value: object, prefix = ""): Record<string, string> {
  return Object.entries(value).reduce<Record<string, string>>(
    (result, [key, item]) =>
      typeof item === "string"
        ? Object.assign(result, { [prefix + key]: item })
        : Object.assign(result, flatten(item, `${prefix + key}.`)),
    {}
  );
}

export const messages = { ...flatten(english), ...terminology };
