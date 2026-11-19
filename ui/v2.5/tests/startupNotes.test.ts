import assert from "node:assert/strict";
import test from "node:test";
import {
  acknowledgement,
  startupNotes,
} from "../src/components/Dialogs/startupNotes.ts";

const changelog = [
  { version: "1.3.0", date: "2026-12-03", content: "three" },
  { version: "1.2.0", date: "2026-12-01", content: "two" },
  { version: "1.1.0", date: "2026-11-19", content: "one" },
];
const versions = (notes: ReturnType<typeof startupNotes>) =>
  notes?.kind === "changes" ? notes.entries.map((e) => e.version) : notes;

test("a fresh install gets the introduction, not the changelog", () => {
  assert.deepEqual(startupNotes({}, "1.3.0", changelog), {
    kind: "introduction",
  });
});

test("an update shows every version since the last one seen", () => {
  assert.deepEqual(
    versions(startupNotes({ lastVersionSeen: "1.1.0" }, "1.3.0", changelog)),
    ["1.3.0", "1.2.0"]
  );
  assert.equal(
    startupNotes({ lastVersionSeen: "1.3.0" }, "1.3.0", changelog),
    undefined
  );
});

test("installs from before versioned notes count as 1.0.0", () => {
  assert.deepEqual(
    versions(startupNotes({ lastNoteSeen: 20261023 }, "1.2.0", changelog)),
    ["1.2.0", "1.1.0"]
  );
  // An unreadable marker is treated the same way, never as "seen everything".
  assert.deepEqual(
    versions(startupNotes({ lastVersionSeen: "garbage" }, "1.1.0", changelog)),
    ["1.1.0"]
  );
});

test("notes for versions newer than the running build are not shown", () => {
  assert.deepEqual(
    versions(startupNotes({ lastVersionSeen: "1.1.0" }, "1.2.0", changelog)),
    ["1.2.0"]
  );
});

test("acknowledging records the running version and the newest note date", () => {
  assert.deepEqual(acknowledgement("1.3.0", 20261023), {
    lastNoteSeen: 20261023,
    lastVersionSeen: "1.3.0",
  });
});
