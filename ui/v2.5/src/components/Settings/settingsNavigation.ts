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

// Keep bookmarks from before processing settings moved out of System.
export function resolveSettingsTab(search: string, hash: string): SettingsTab {
  const tab = new URLSearchParams(search).get("tab");
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
