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
  "unifiedMedia.copyright.part",
  "unifiedMedia.06.part",
];

const expectedUnifiedSha256 =
  "dbfd1ea8487f8fe17dc6701572d984733aa33d6be54192d4b1e2f1d47c299336";

rmSync(outputDir, { recursive: true, force: true });
mkdirSync(outputDir, { recursive: true });

const unifiedMedia = Buffer.concat(
  unifiedParts.map((part) => readFileSync(resolve(sourceDir, part)))
);
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
