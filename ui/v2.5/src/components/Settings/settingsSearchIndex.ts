import type { SettingsTab } from "./settingsNavigation";

export interface SettingsSearchEntry {
  tab: SettingsTab;
  label: string;
  anchor?: string;
  keywords?: string;
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
    label: "Profiles and associations",
    anchor: "tasks-profiles",
    keywords: "character artist copyright tagging",
  },
  {
    tab: "tasks",
    label: "Image and Video processing tasks",
    anchor: "tasks-media",
    keywords: "convert duplicate restore storage",
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
    label: "Inherited Copyright and Character associations",
    anchor: "association-inheritance-settings",
    keywords: "metadata artist link",
  },
  {
    tab: "processing",
    label: "Worker connection",
    anchor: "inference-worker",
    keywords: "remote cpu gpu url token",
  },
  {
    tab: "processing",
    label: "Similarity and AI tagging",
    anchor: "visual-similarity",
    keywords: "embeddings phash camie eva02",
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
    keywords: "filename filenames character copyright artist formatted fallback hide",
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
    keywords: "theme style",
  },
  {
    tab: "interface",
    label: "Custom JavaScript",
    anchor: "interface-custom-javascript",
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
    label: "Plugins and package manager",
    keywords: "install extensions update",
  },
  {
    tab: "logs",
    label: "Application logs",
    keywords: "errors debug logging",
  },
  {
    tab: "tools",
    label: "Metadata health",
    anchor: "tools-health",
    keywords: "diagnostics",
  },
  {
    tab: "tools",
    label: "Visual similarity",
    anchor: "tools-similarity",
    keywords: "duplicates phash clusters",
  },
  {
    tab: "tools",
    label: "Alias collision inspector",
    anchor: "tools-aliases",
    keywords: "tags character artist merge",
  },
  {
    tab: "tools",
    label: "GraphQL and developer tools",
    anchor: "tools-developer",
    keywords: "api playground",
  },
  {
    tab: "changelog",
    label: "Changelog and release notes",
    keywords: "version updates",
  },
  {
    tab: "about",
    label: "About StashBooru and version",
    keywords: "upstream build release",
  },
];

export function searchSettings(
  query: string,
  entries = settingsSearchIndex
): SettingsSearchEntry[] {
  const terms = query.toLocaleLowerCase().trim().split(/\s+/).filter(Boolean);
  if (!terms.length) return [];
  return entries.filter(({ label, tab, keywords }) => {
    const text = `${label} ${tab} ${keywords ?? ""}`.toLocaleLowerCase();
    return terms.every((term) => text.includes(term));
  });
}
