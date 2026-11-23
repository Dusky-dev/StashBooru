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
import v161 from "./1.6.1.md";
import v162 from "./1.6.2.md";
import v170 from "./1.7.0.md";
import v180 from "./1.8.0.md";
import v190 from "./1.9.0.md";
import v1100 from "./1.10.0.md";
import v1110 from "./1.11.0.md";
import v1120 from "./1.12.0.md";
import v1130 from "./1.13.0.md";
import v1140 from "./1.14.0.md";
import v1150 from "./1.15.0.md";
import v1160 from "./1.16.0.md";
import v1170 from "./1.17.0.md";
import v1180 from "./1.18.0.md";
import v1190 from "./1.19.0.md";
import v1200 from "./1.20.0.md";
import v1210 from "./1.21.0.md";
import v1220 from "./1.22.0.md";

// Newest first. Every user-visible change bumps
// internal/build/stashbooru-version.txt and adds an entry here.
export const stashBooruChangelog: IStashBooruChange[] = [
  { version: "1.22.0", date: "2026-11-23", content: v1220 },
  { version: "1.21.0", date: "2026-11-23", content: v1210 },
  { version: "1.20.0", date: "2026-11-23", content: v1200 },
  { version: "1.19.0", date: "2026-11-23", content: v1190 },
  { version: "1.18.0", date: "2026-11-23", content: v1180 },
  { version: "1.17.0", date: "2026-11-23", content: v1170 },
  { version: "1.16.0", date: "2026-11-23", content: v1160 },
  { version: "1.15.0", date: "2026-11-23", content: v1150 },
  { version: "1.14.0", date: "2026-11-23", content: v1140 },
  { version: "1.13.0", date: "2026-11-23", content: v1130 },
  { version: "1.12.0", date: "2026-11-23", content: v1120 },
  { version: "1.11.0", date: "2026-11-23", content: v1110 },
  { version: "1.10.0", date: "2026-11-22", content: v1100 },
  { version: "1.9.0", date: "2026-11-22", content: v190 },
  { version: "1.8.0", date: "2026-11-21", content: v180 },
  { version: "1.7.0", date: "2026-11-21", content: v170 },
  { version: "1.6.2", date: "2026-11-20", content: v162 },
  { version: "1.6.1", date: "2026-11-20", content: v161 },
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
