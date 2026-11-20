import type { IStashBooruChange } from "src/components/Dialogs/startupNotes";
import v110 from "./1.1.0.md";
import v111 from "./1.1.1.md";
import v112 from "./1.1.2.md";
import v113 from "./1.1.3.md";
import v120 from "./1.2.0.md";
import v130 from "./1.3.0.md";
import v140 from "./1.4.0.md";
import v150 from "./1.5.0.md";
import v151 from "./1.5.1.md";
import v160 from "./1.6.0.md";

// Newest first. Every user-visible change bumps
// internal/build/stashbooru-version.txt and adds an entry here.
export const stashBooruChangelog: IStashBooruChange[] = [
  { version: "1.6.0", date: "2026-11-20", content: v160 },
  { version: "1.5.1", date: "2026-11-20", content: v151 },
  { version: "1.5.0", date: "2026-11-20", content: v150 },
  { version: "1.4.0", date: "2026-11-19", content: v140 },
  { version: "1.3.0", date: "2026-11-19", content: v130 },
  { version: "1.2.0", date: "2026-11-19", content: v120 },
  { version: "1.1.3", date: "2026-11-19", content: v113 },
  { version: "1.1.2", date: "2026-11-19", content: v112 },
  { version: "1.1.1", date: "2026-11-19", content: v111 },
  { version: "1.1.0", date: "2026-11-19", content: v110 },
];
