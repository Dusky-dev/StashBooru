import { type BooruSource } from "./browser";

type RatingSource = Pick<BooruSource, "engine" | "url">;

export function normalizedRating(rating: string, source: RatingSource) {
  const value = rating.toLowerCase();
  if (value === "s")
    return supportsSensitiveRating(source) ? "sensitive" : "safe";
  if (["safe", "general", "g"].includes(value)) return "safe";
  if (["sensitive", "questionable", "explicit"].includes(value)) return value;
  if (value === "q") return "questionable";
  if (value === "e") return "explicit";
  return "unknown";
}

export function supportsSensitiveRating(source: RatingSource) {
  return (
    source.engine === "danbooru" ||
    (source.engine === "gelbooru" &&
      ![
        "rule34.xxx",
        "api.rule34.xxx",
        "safebooru.org",
        "www.safebooru.org",
      ].includes(new URL(source.url).hostname))
  );
}
