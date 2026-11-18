export const settingsTabs = [
  "tasks",
  "library",
  "processing",
  "interface",
  "security",
  "metadata-providers",
  "services",
  "system",
  "plugins",
  "about",
  "experimental",
] as const;

export type SettingsTab = (typeof settingsTabs)[number];

const movedSections: Record<string, SettingsTab> = {
  "#inference-worker": "processing",
  "#visual-similarity": "processing",
  "#media-converter": "processing",
  "#media-upscaling": "processing",
  "#association-inheritance-settings": "library",
};

// Existing P12/P13 and Video overlap bookmarks should follow their relocated panels.
// Task Queue and original-media backup remain on the Tasks page.
const experimentalAnchors = new Set([
  "#experimental-image-storage",
  "#experimental-storage-assess",
  "#experimental-storage-candidates",
  "#experimental-storage-recovery",
  "#experimental-video-overlap",
  "#video-overlap-task",
  "#tasks-media",
  "#image-storage-estimate-task",
  "#image-storage-estimate-heading",
  "#image-storage-backup-preflight-task",
  "#image-storage-backup-preflight-heading",
  "#image-storage-candidate-task",
  "#image-storage-candidate-title",
  "#image-storage-recovery-task",
  "#image-storage-recovery-title",
  "#image-storage-restore-plan-task",
  "#image-storage-restore-plan-title",
  "#image-missing-original-title",
  "#image-storage-shadow-title",
  "#image-consumer-audit-title",
]);

// Tabs folded into others: Tools into Tasks; Logs and Changelog into About.
const foldedTabs: Record<string, { tab: SettingsTab; anchor: string }> = {
  tools: { tab: "tasks", anchor: "#tasks-tools" },
  logs: { tab: "about", anchor: "#about-logs" },
  changelog: { tab: "about", anchor: "#about-changelog" },
};

/** The section an old Tools, Logs or Changelog link opens when it has no hash. */
export function foldedTabHash(search: string, hash: string): string {
  const folded = foldedTabs[new URLSearchParams(search).get("tab") ?? ""];
  return hash || !folded ? hash : folded.anchor;
}

// Keep bookmarks from before processing settings moved out of System.
export function resolveSettingsTab(search: string, hash: string): SettingsTab {
  const tab = new URLSearchParams(search).get("tab");
  const folded = tab ? foldedTabs[tab] : undefined;
  if (folded) return folded.tab;
  if ((!tab || tab === "tasks") && experimentalAnchors.has(hash)) {
    return "experimental";
  }
  if ((!tab || tab === "system") && movedSections[hash]) {
    return movedSections[hash];
  }
  return settingsTabs.includes(tab as SettingsTab)
    ? (tab as SettingsTab)
    : "tasks";
}

/** Legacy Tools bookmark opens the consolidated duplicate/related-image review. */
export function legacySimilarityReviewDestination(
  search: string,
  hash: string
): string | undefined {
  const params = new URLSearchParams(search);
  const tab = params.get("tab");
  if (hash !== "#tools-similarity" || (tab && tab !== "tools")) {
    return undefined;
  }
  params.delete("tab");
  params.set("method", "embedding");
  return `/image-duplicates?${params.toString()}`;
}

export function settingsSearch(search: string, tab: SettingsTab): string {
  const params = new URLSearchParams(search);
  params.set("tab", tab);
  return `?${params.toString()}`;
}
