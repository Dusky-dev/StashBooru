import type { SettingsTab } from "./settingsNavigation";

export interface SettingsSearchEntry {
  tab: SettingsTab;
  label: string;
  anchor?: string;
  keywords?: string;
  /** Only listed in Advanced mode, like the setting itself. */
  advanced?: boolean;
}

/** Searchable destinations, not executable settings. Keep links to stable IDs. */
export const settingsSearchIndex: SettingsSearchEntry[] = [
  {
    tab: "tasks",
    label: "Tasks and jobs",
    anchor: "job-queue",
    keywords: "progress queue",
  },
  {
    tab: "tasks",
    label: "Scan, identify and import library",
    anchor: "tasks-library",
    keywords: "import scan metadata generate thumbnails",
  },
  {
    tab: "tasks",
    label: "Review Characters",
    anchor: "tasks-profiles",
    keywords: "character duplicates merge",
  },
  {
    tab: "tasks",
    label: "Animation inspection",
    anchor: "animation-inspection-task",
    keywords: "scan library update image animated backfill",
  },
  {
    tab: "tasks",
    label: "Maintenance and database backups",
    anchor: "tasks-maintenance",
    keywords: "backup clean migrate restore",
  },
  {
    tab: "tasks",
    label: "Plugin tasks",
    anchor: "tasks-plugins",
  },
  {
    tab: "experimental",
    label: "Video overlap / containment",
    anchor: "experimental-video-overlap",
    keywords:
      "duplicate clip compilation matching index timeline partial video analysis",
  },
  {
    tab: "experimental",
    label: "Media uploads",
    anchor: "experimental-media-uploads",
    keywords:
      "quick upload drag and drop direct url import default folder destination conversion tagging",
  },
  {
    tab: "experimental",
    label: "Opt-in thumbnail compression",
    anchor: "experimental-thumbnail-compression",
    keywords:
      "jpeg jpg webp jxl avif format quality resolution slider compression thumbnails video previews mp4 webm",
  },
  {
    tab: "experimental",
    label: "P12/P13 image storage experiments",
    anchor: "experimental-image-storage",
    keywords: "experimental delta shared tiles compression packed archive",
  },
  {
    tab: "experimental",
    label: "Estimate P12/P13 image storage savings",
    anchor: "experimental-storage-assess",
    keywords: "estimate sample read only backup coverage restore audit",
  },
  {
    tab: "experimental",
    label: "Create candidates and compare shadow images",
    anchor: "experimental-storage-candidates",
    keywords: "p12 p13 publish pack native preview serving compare",
  },
  {
    tab: "experimental",
    label: "Rehearse recovery of packed images",
    anchor: "experimental-storage-recovery",
    keywords: "p12 p13 restore reconstruct missing originals scratch",
  },
  {
    tab: "library",
    label: "Video extensions",
    anchor: "video-extensions",
    keywords: "scene file format",
  },
  {
    tab: "library",
    label: "Image extensions",
    anchor: "image-extensions",
    keywords: "jpg jxl png webp",
  },
  {
    tab: "library",
    label: "Library exclusions",
    anchor: "excluded-video-patterns",
    keywords: "exclude skip folders videos",
  },
  {
    tab: "library",
    label: "Image and gallery exclusions",
    anchor: "excluded-image-gallery-patterns",
  },
  {
    tab: "library",
    label: "Galleries from folders",
    anchor: "create-galleries-from-folders",
  },
  {
    tab: "library",
    label: "Image thumbnails",
    anchor: "write-image-thumbnails",
  },
  {
    tab: "library",
    label: "Delete original files",
    anchor: "delete-file-default",
  },
  {
    tab: "library",
    label:
      "Include parents (inherited Characters, Artists, Copyrights and Tags)",
    anchor: "association-inheritance-settings",
    keywords: "metadata artist link hierarchy inheritance parents",
  },
  {
    tab: "processing",
    label: "Worker connection",
    anchor: "inference-worker",
    keywords: "remote local fallback enable disable cpu gpu url token",
  },
  {
    tab: "processing",
    label: "Similarity and AI tagging",
    anchor: "visual-similarity",
    keywords: "embeddings phash camie eva02",
  },
  {
    tab: "processing",
    label: "Camie Tagger model",
    anchor: "camie-model",
    keywords: "install delete optional tagging",
  },
  {
    tab: "processing",
    label: "PixAI Tagger model",
    anchor: "pixai-model",
    keywords: "install delete optional anime tagging",
  },
  {
    tab: "processing",
    label: "Media converter and format rules",
    anchor: "media-converter",
    keywords: "encode mp4 jxl av1 codec defaults quality enabled",
  },
  {
    tab: "processing",
    label: "Upscaling",
    anchor: "media-upscaling",
    keywords: "waifu2x video image restoration",
  },
  {
    tab: "interface",
    label: "Language and basic interface options",
    anchor: "interface-basic",
    keywords: "theme appearance sfw",
  },
  {
    tab: "interface",
    label: "Video player",
    anchor: "interface-video-player",
    keywords: "scene playback loop subtitles scrubber",
  },
  {
    tab: "interface",
    label: "Video wall and previews",
    anchor: "interface-video-wall",
    keywords: "scene thumbnails",
  },
  {
    tab: "interface",
    label: "Image titles",
    anchor: "image-filename-display",
    keywords:
      "filename filenames character copyright artist formatted fallback hide",
  },
  {
    tab: "interface",
    label: "Shared filename layout",
    anchor: "shared-image-filename-layout",
    keywords: "grabber imggrabber pattern template booru download image",
  },
  {
    tab: "interface",
    label: "Image wall and lightbox",
    anchor: "interface-image-lightbox",
    keywords: "slideshow previews zoom",
  },
  {
    tab: "interface",
    label: "Metadata detail panels",
    anchor: "interface-detail",
    keywords: "character artist tags panels",
  },
  {
    tab: "interface",
    label: "Editing preferences",
    anchor: "interface-editing",
    keywords: "forms autocomplete ratings",
  },
  {
    tab: "interface",
    label: "Custom CSS",
    anchor: "interface-custom-css",
    advanced: true,
    keywords: "theme style",
  },
  {
    tab: "interface",
    label: "Custom JavaScript",
    anchor: "interface-custom-javascript",
    advanced: true,
    keywords: "script extension",
  },
  {
    tab: "security",
    label: "Authentication credentials",
    anchor: "authentication-settings",
    keywords: "login username password",
  },
  {
    tab: "security",
    label: "API key",
    anchor: "apikey",
    keywords: "token authorization",
  },
  {
    tab: "security",
    label: "Session expiry",
    anchor: "maxSessionAge",
    keywords: "timeout",
  },
  {
    tab: "metadata-providers",
    label: "Booru sources and downloads",
    anchor: "booru-sources",
    keywords: "danbooru rule34 paheal pixiv pawchive kemono autocomplete",
  },
  {
    tab: "metadata-providers",
    anchor: "metadata-group-scrapers",
    label: "Scrapers",
    keywords: "metadata scraping plugin",
  },
  {
    tab: "metadata-providers",
    label: "Scraper user agent",
    anchor: "scraperUserAgent",
  },
  {
    tab: "metadata-providers",
    label: "Excluded tag patterns",
    anchor: "excluded-tag-patterns",
  },
  {
    tab: "services",
    label: "DLNA service and networking",
    anchor: "settings-dlna",
    keywords: "server whitelist interfaces port",
  },
  {
    tab: "system",
    label: "File and application paths",
    anchor: "generated-path",
    keywords: "cache generated scrapers plugins ffmpeg",
  },
  {
    tab: "system",
    label: "Database and storage",
    anchor: "database-path",
    keywords: "sqlite backup blobs",
  },
  {
    tab: "system",
    label: "Hardware acceleration and transcoding",
    anchor: "hardware-encoding",
    keywords: "ffmpeg gpu",
  },
  {
    tab: "system",
    label: "Preview generation",
    anchor: "video-preview-settings",
    keywords: "video scene sprites",
  },
  {
    tab: "system",
    label: "Parallel scan tasks",
    anchor: "parallel-tasks",
  },
  {
    tab: "plugins",
    anchor: "plugins-group-installed",
    label: "Plugins and package manager",
    keywords: "install extensions update",
  },
  {
    tab: "about",
    label: "Application logs",
    anchor: "about-logs",
    keywords: "errors debug logging",
  },
  {
    tab: "tasks",
    label: "Metadata health",
    anchor: "tools-health",
    keywords: "diagnostics",
  },
  {
    tab: "tasks",
    label: "GraphQL and developer tools",
    anchor: "tools-developer",
    keywords: "api playground",
  },
  {
    tab: "about",
    label: "Changelog and release notes",
    anchor: "about-changelog",
    keywords: "version updates",
  },
  {
    tab: "about",
    label: "About StashBooru and version",
    keywords: "upstream build release",
  },
];

/** Message ID for an entry's label; its English text is the default. */
export function settingsSearchLabelID(entry: SettingsSearchEntry): string {
  const key = entry.label
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "_")
    .replace(/^_|_$/g, "");
  return `config.settings_search.entries.${key}`;
}

/**
 * Matches the displayed (translated) label as well as the English label and
 * keywords, so both the UI language and English terms find a setting.
 */
export function searchSettings(
  query: string,
  entries = settingsSearchIndex,
  labelOf: (entry: SettingsSearchEntry) => string = (entry) => entry.label
): SettingsSearchEntry[] {
  const terms = query.toLocaleLowerCase().trim().split(/\s+/).filter(Boolean);
  if (!terms.length) return [];
  return entries.filter((entry) => {
    const { label, tab, keywords } = entry;
    const text =
      `${labelOf(entry)} ${label} ${tab} ${keywords ?? ""}`.toLocaleLowerCase();
    return terms.every((term) => text.includes(term));
  });
}
