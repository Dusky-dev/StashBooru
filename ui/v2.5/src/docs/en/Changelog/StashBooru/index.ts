import type { IStashBooruChange } from "src/components/Dialogs/startupNotes";
import v110 from "./1.1.0.md";
import v111 from "./1.1.1.md";

// Newest first. Every user-visible change bumps
// internal/build/stashbooru-version.txt and adds an entry here.
export const stashBooruChangelog: IStashBooruChange[] = [
  { version: "1.1.1", date: "2026-11-19", content: v111 },
  { version: "1.1.0", date: "2026-11-19", content: v110 },
];
