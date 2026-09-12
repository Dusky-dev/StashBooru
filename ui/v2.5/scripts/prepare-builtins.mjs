import { createHash } from "node:crypto";
import { mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const uiRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const sourceDir = resolve(uiRoot, "builtin-source");
const outputDir = resolve(uiRoot, "public", "builtin");

const unifiedParts = [
  "unifiedMedia.00.part",
  "unifiedMedia.01.part",
  "unifiedMedia.02.part",
  "unifiedMedia.03.part",
  "unifiedMedia.04.part",
  "unifiedMedia.05.part",
  "unifiedMedia.06.part",
];
const unifiedExtensions = [
  "unifiedMedia.copyright.part",
  "unifiedMedia.copyright.guard.part",
];
const unifiedClose = Buffer.from("})();\n");

const expectedUnifiedSha256 =
  "ec98261b6de21d9de51c30265682fdfaf61240cba90a76666f8a47f9dc580e3b";

rmSync(outputDir, { recursive: true, force: true });
mkdirSync(outputDir, { recursive: true });

const unifiedCore = Buffer.concat(
  unifiedParts.map((part) => readFileSync(resolve(sourceDir, part)))
);
if (!unifiedCore.subarray(-unifiedClose.length).equals(unifiedClose)) {
  throw new Error(
    "Unified Media core no longer ends at the expected IIFE boundary"
  );
}

const unifiedMedia = Buffer.concat([
  unifiedCore.subarray(0, -unifiedClose.length),
  ...unifiedExtensions.map((part) => readFileSync(resolve(sourceDir, part))),
  unifiedClose,
]);
const actualUnifiedSha256 = createHash("sha256")
  .update(unifiedMedia)
  .digest("hex");

if (actualUnifiedSha256 !== expectedUnifiedSha256) {
  throw new Error(
    `Unified Media source checksum mismatch: expected ${expectedUnifiedSha256}, got ${actualUnifiedSha256}`
  );
}

writeFileSync(resolve(outputDir, "unifiedMedia.js"), unifiedMedia);

for (const file of [
  "unifiedMedia.css",
  "universal-file-info.js",
  "bundle-extra.js",
]) {
  writeFileSync(
    resolve(outputDir, file),
    readFileSync(resolve(sourceDir, file))
  );
}

console.log(
  `[built-in-ui] prepared Unified Media (${unifiedMedia.length} bytes) + File Info + Advanced fields`
);
