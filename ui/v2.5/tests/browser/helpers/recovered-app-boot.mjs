// Exercise a restored StashBooru database and media using only throwaway
// fixtures. This helper is deliberately NOT a general-purpose restore command.
import assert from "node:assert/strict";
import { spawn, execFile } from "node:child_process";
import { once } from "node:events";
import { createHash } from "node:crypto";
import { createWriteStream } from "node:fs";
import { copyFile, mkdir, readFile, realpath, writeFile } from "node:fs/promises";
import { createServer } from "node:net";
import { join, relative, sep } from "node:path";
import { promisify } from "node:util";

const exec = promisify(execFile);
const hash = (value) => createHash("sha256").update(value).digest("hex");
const delay = (milliseconds) => new Promise((done) => setTimeout(done, milliseconds));

function inside(path, parent) {
  const rel = relative(parent, path);
  return rel !== "" && rel !== ".." && !rel.startsWith(".." + sep) && !rel.includes("\0");
}

async function python(repository, script, args) {
  const { stdout } = await exec(
    "python3",
    [join(repository, "scripts", script), ...args],
    { cwd: repository, timeout: 180000, maxBuffer: 1024 * 1024 }
  );
  const value = JSON.parse(stdout);
  if (Object.hasOwn(value, "verified"))
    assert.equal(value.verified, true, script + " did not verify");
  return value;
}

async function portNumber() {
  const listener = createServer();
  listener.listen(0, "127.0.0.1");
  await once(listener, "listening");
  const port = listener.address().port;
  await new Promise((done) => listener.close(done));
  return port;
}

async function bootRecovered({ binary, root, repository, workspace, media, backups, snapshot, members, originals }) {
  assert.match(root, /stash-packed-native-e2e-[^/]+$/, "Only generated acceptance fixtures are supported");
  assert.equal(await realpath(root), root, "Scratch root must be canonical");
  for (const path of [workspace, media, backups]) {
    assert.ok(inside(path, root), "Fixture escaped temporary root");
    assert.equal(await realpath(path), path, "Fixture must not use symbolic links");
  }
  // A saved, real SQLite catalogue with both original image IDs is required.
  // Call only after the source StashBooru instance has shut down.
  const privateRoot = join(root, "recovery-private");
  await mkdir(privateRoot, { mode: 0o700 });
  const savedDB = join(privateRoot, "backup.sqlite");
  const savedConfig = join(privateRoot, "saved-config.yml");
  const originalDB = join(workspace, "stash.sqlite");
  await exec("python3", [
    "-c",
    "import sqlite3,sys; source=sqlite3.connect('file:'+sys.argv[1]+'?mode=ro',uri=True); destination=sqlite3.connect(sys.argv[2]); source.backup(destination); destination.close(); source.close()",
    originalDB, savedDB,
  ], { timeout: 30000 });
  await copyFile(join(workspace, "config.yml"), savedConfig);
  const initialBackupDigest = hash(await readFile(savedDB));
  const initialConfigDigest = hash(await readFile(savedConfig));
  const savedOriginals = new Map();
  for (const member of members) {
    const bytes = await readFile(join(media, member.name));
    assert.deepEqual(bytes, originals.get(member.name));
    savedOriginals.set(member.name, hash(bytes));
  }

  const receipt = join(privateRoot, "backup-set.json");
  const restored = join(privateRoot, "recovered");
  const imageRebase = join(privateRoot, "image-path-rebase");
  const allRebase = join(privateRoot, "all-file-path-rebase");
  const checkpoint = join(privateRoot, "rollback-checkpoint");
  const pair = ["--backup-set-receipt", receipt];
  await python(repository, "image_storage_backup_set.py", [
    "create", "--database", savedDB, "--config", savedConfig,
    "--store", backups, "--snapshot", snapshot, "--output", receipt,
  ]);
  for (const [script, action, args] of [
    ["image_storage_disposable_snapshot_restore.py", "rehearse",
      [...pair, "--output", restored]],
    ["image_storage_disposable_path_rebase.py", "rebase",
      [...pair, "--recovered-output", restored, "--output", imageRebase]],
    ["image_storage_disposable_all_file_rebase.py", "rebase",
      [...pair, "--recovered-output", restored,
        "--image-rebase-output", imageRebase, "--output", allRebase]],
    ["image_storage_disposable_rollback_checkpoint.py", "prepare",
      [...pair, "--recovered-output", restored,
        "--image-rebase-output", imageRebase,
        "--all-file-rebase-output", allRebase, "--output", checkpoint]],
    ["image_storage_disposable_rollback_checkpoint.py", "audit",
      [...pair, "--recovered-output", restored,
        "--image-rebase-output", imageRebase,
        "--all-file-rebase-output", allRebase, "--output", checkpoint]],
  ]) {
    const proof = await python(repository, script, [action, ...args]);
    if (action !== "audit" && action !== "prepare") {
      const audited = await python(repository, script, ["audit", ...args]);
      assert.equal(audited.status, "complete_verified");
    }
    for (const flag of ["canActivate", "originalEvictionEnabled", "transactionalRollbackVerified"])
      assert.equal(proof[flag], false, flag + " must remain disabled");
  }

  // NEVER reuse the saved configuration: it could contain production keys,
  // paths, plugin hooks, remote sources, or public bind addresses.
  // Generate a separate allowlisted YAML config and copy the fully rebased
  // catalogue again so application migrations can only change that clone.
  const boot = join(privateRoot, "isolated-app");
  await mkdir(boot, { mode: 0o700 });
  for (const sub of ["generated", "cache", "blobs", "metadata", "downloads", "plugins", "scrapers", "backups", "home", "xdg"]) {
    await mkdir(join(boot, sub), { mode: 0o700 });
  }
  const bootDB = join(boot, "catalog.sqlite");
  await copyFile(join(allRebase, "all-file-paths.sqlite"), bootDB);
  const port = await portNumber();
  const yaml = [
    "database: " + JSON.stringify(bootDB),
    "stash:",
    "  - path: " + JSON.stringify(join(restored, "media")),
    ...["generated", "cache", "metadata", "downloads"].map(
      (key) => key + ": " + JSON.stringify(join(boot, key))
    ),
    "blobs_path: " + JSON.stringify(join(boot, "blobs")),
    "backup_directory_path: " + JSON.stringify(join(boot, "backups")),
    "plugins_path: " + JSON.stringify(join(boot, "plugins")),
    "scrapers_path: " + JSON.stringify(join(boot, "scrapers")),
    "host: 127.0.0.1",
    "port: " + port,
    "nobrowser: true",
    "logout: false",
    "local_https_enabled: false",
    "external_host: \"\"",
    "stash_boxes: []",
    "custom_served_folders: {}",
    "",
  ].join("\n");
  const bootConfig = join(boot, "config.yml");
  await writeFile(bootConfig, yaml, { flag: "wx", mode: 0o600 });
  assert.ok(!yaml.includes(workspace), "Restored config must not contain historic source paths");

  const env = {
    PATH: process.env.PATH ?? "/usr/bin:/bin",
    HOME: join(boot, "home"),
    XDG_CONFIG_HOME: join(boot, "xdg"),
    XDG_CACHE_HOME: join(boot, "cache"),
    XDG_DATA_HOME: join(boot, "xdg"),
    LANG: "C.UTF-8",
    STASH_LOCAL_HTTPS_ENABLED: "false",
    HTTP_PROXY: "http://127.0.0.1:9",
    HTTPS_PROXY: "http://127.0.0.1:9",
    NO_PROXY: "127.0.0.1,localhost",
  };
  const log = createWriteStream(join(boot, "boot.log"), { flags: "wx", mode: 0o600 });
  const child = spawn(binary, [
    "--config", bootConfig, "--host", "127.0.0.1",
    "--port", String(port), "--nobrowser",
  ], { cwd: boot, env });
  child.stdout.pipe(log, { end: false });
  child.stderr.pipe(log, { end: false });
  let spawnError;
  child.on("error", (error) => { spawnError = error; });
  const url = "http://127.0.0.1:" + port;
  try {
    let ready = false;
    for (let i = 0; i < 160; i++) {
      if (spawnError) throw spawnError;
      assert.equal(child.exitCode, null, "Restored app exited before startup");
      try {
        const response = await fetch(url + "/graphql", {
          method: "POST",
          headers: { "content-type": "application/json" },
          body: JSON.stringify({ query: "{systemStatus{status}}" }),
          signal: AbortSignal.timeout(800),
        });
        if ((await response.json()).data?.systemStatus) { ready = true; break; }
      } catch { /* isolated application is starting */ }
      await delay(250);
    }
    assert.ok(ready, "Isolated restored application never became ready");
    const response = await fetch(url + "/graphql", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ query: "{findImages{images{id}}}" }),
    });
    assert.equal(response.status, 200);
    const data = await response.json();
    assert.equal(data.errors, undefined, JSON.stringify(data.errors));
    const ids = new Set(data.data.findImages.images.map((image) => String(image.id)));
    assert.deepEqual(ids, new Set(members.map((member) => String(member.id))));
    for (const member of members) {
      const image = await fetch(url + "/image/" + member.id + "/image");
      assert.equal(image.status, 200, "Restored image route failed");
      assert.deepEqual(Buffer.from(await image.arrayBuffer()), originals.get(member.name));
    }
  } finally {
    if (child.exitCode === null && child.signalCode === null) {
      const exited = once(child, "exit");
      child.kill("SIGTERM");
      await exited;
    }
    log.end();
  }
  for (const member of members) {
    assert.equal(hash(await readFile(join(media, member.name))), savedOriginals.get(member.name));
  }
  assert.equal(hash(await readFile(savedDB)), initialBackupDigest);
  assert.equal(hash(await readFile(savedConfig)), initialConfigDigest);
  const finalProof = await python(repository, "image_storage_disposable_rollback_checkpoint.py", [
    "audit", ...pair, "--recovered-output", restored,
    "--image-rebase-output", imageRebase,
    "--all-file-rebase-output", allRebase, "--output", checkpoint,
  ]);
  assert.equal(finalProof.status, "complete_verified");
  console.log("Restored real StashBooru boot and native image reads succeeded in disposable scratch");
}

export { bootRecovered };
