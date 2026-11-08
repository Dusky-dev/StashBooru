export const settingsTabs = [
  "tasks",
  "experimental",
  "library",
  "processing",
  "interface",
  "security",
  "metadata-providers",
  "services",
  "system",
  "plugins",
  "logs",
  "tools",
  "changelog",
  "about",
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

// Keep bookmarks from before processing settings moved out of System.
export function resolveSettingsTab(search: string, hash: string): SettingsTab {
  const tab = new URLSearchParams(search).get("tab");
  // Preserve saved Tools links and profile inspector bookmarks after relocation.
  if ((!tab || tab === "tools") && hash === "#tools-aliases") {
    return "experimental";
  }
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

export function settingsSearch(search: string, tab: SettingsTab): string {
  const params = new URLSearchParams(search);
  params.set("tab", tab);
  return `?${params.toString()}`;
}
