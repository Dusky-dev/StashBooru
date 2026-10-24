import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import {
  cp,
  mkdtemp,
  mkdir,
  readdir,
  readFile,
  rm,
  writeFile,
} from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { startIsolatedServer } from "./helpers/isolated-server.mjs";

// Own every database, path and server. Python's standard library checks SQLite
// integrity and extracts our own backup ZIP; no existing library is accepted.
assert.ok(process.env.STASH_BROWSER_BINARY, "Set STASH_BROWSER_BINARY");
const binary = resolve(process.env.STASH_BROWSER_BINARY);
const ui = process.env.STASH_BROWSER_UI;
const { chromium } = await import(
  process.env.PLAYWRIGHT_MODULE ?? "playwright"
);
const root = await mkdtemp(join(tmpdir(), "stashbooru-backups-"));
const artifacts =
  process.env.STASH_BROWSER_SCREENSHOTS ?? join(root, "artifacts");
await mkdir(artifacts, { recursive: true });
const browser = await chromium.launch({
  executablePath: process.env.CHROMIUM_EXECUTABLE,
  args: ["--no-sandbox", "--disable-dev-shm-usage"],
});
let instance;
let successful = false;
async function stop() {
  await instance?.stop();
  instance = undefined;
}
async function call(page, url, query, variables = {}) {
  const response = await page.request.post(url + "/graphql", {
    data: { query, variables },
  });
  const payload = await response.json();
  assert.equal(payload.errors, undefined, JSON.stringify(payload.errors));
  return payload.data;
}
async function configure(directory, storage, backup) {
  // Native setup creates the configured data roots. This fixture writes its
  // configuration directly, so create those same roots before starting it.
  for (const folder of ["", "media", "generated", "cache", "metadata"]) {
    await mkdir(join(directory, folder), { recursive: true });
  }
  await writeFile(
    join(directory, "config.yml"),
    JSON.stringify({
      stash: [{ path: join(directory, "media") }],
      database: join(directory, "library.sqlite"),
      generated: join(directory, "generated"),
      cache: join(directory, "cache"),
      metadata: join(directory, "metadata"),
      blobs_path: join(directory, "blobs"),
      blobs_storage: storage,
      backup_directory_path: backup,
    })
  );
}
async function start(directory, label) {
  instance = await startIsolatedServer({
    binary,
    directory,
    artifacts,
    label,
    ui,
  });
  return instance.url;
}
function integrity(database) {
  execFileSync("python3", [
    "-c",
    `
import pathlib, sqlite3, sys
with sqlite3.connect(pathlib.Path(sys.argv[1]).as_uri() + '?mode=ro', uri=True) as db:
    assert db.execute('PRAGMA integrity_check').fetchall() == [('ok',)]
    assert db.execute('PRAGMA foreign_key_check').fetchall() == []
`,
    database,
  ]);
}
async function check(viewport, label, storage) {
  const directory = join(root, label);
  const backups = join(directory, "backups");
  await configure(directory, storage, backups);
  // A regular file in place of the directory gives a real, portable server
  // failure even when the test process runs as root.
  await writeFile(backups, "Blocked backup destination");
  const page = await browser.newPage({ viewport });
  const errors = [];
  page.on("pageerror", (error) => errors.push(String(error)));
  const png = await page.evaluate(() => {
    const canvas = document.createElement("canvas");
    canvas.width = 64;
    canvas.height = 48;
    const ctx = canvas.getContext("2d");
    ctx.fillStyle = "#57c4c8";
    ctx.fillRect(0, 0, 64, 48);
    return canvas.toDataURL("image/png");
  });
  // Unindexed media bytes are deliberately separate from database/blob backups.
  const mediaBytes = Buffer.from(png.split(",")[1], "base64");
  await writeFile(join(directory, "media", "original.png"), mediaBytes);
  const url = await start(directory, label);
  await call(
    page,
    url,
    "mutation { configureUI(input: {lastNoteSeen: 99999999}) }"
  );
  const artist = (
    await call(
      page,
      url,
      "mutation($input: StudioCreateInput!) {studioCreate(input: $input) {id image_path}}",
      {
        input: {
          name: "Backup Artist",
          image: png,
          urls: ["https://example.invalid/artist"],
          custom_fields: { note: "Preserve artist" },
        },
      }
    )
  ).studioCreate;
  const character = (
    await call(
      page,
      url,
      'mutation {performerCreate(input: {name: "Backup Character"}) {id}}'
    )
  ).performerCreate;
  const copyright = (
    await call(
      page,
      url,
      'mutation {copyrightCreate(input: {name: "Backup Copyright"}) {id}}'
    )
  ).copyrightCreate;
  const tag = (
    await call(
      page,
      url,
      'mutation {tagCreate(input: {name: "Backup tag"}) {id}}'
    )
  ).tagCreate;
  const group = (
    await call(
      page,
      url,
      'mutation {groupCreate(input: {name: "Backup Collection"}) {id}}'
    )
  ).groupCreate;
  const scene = (
    await call(
      page,
      url,
      "mutation($input: SceneCreateInput!) {sceneCreate(input: $input) {id}}",
      {
        input: {
          title: "Backup Video",
          details: "Retain native metadata",
          studio_id: artist.id,
          performer_ids: [character.id],
          tag_ids: [tag.id],
          groups: [{ group_id: group.id, scene_index: 0 }],
          custom_fields: { note: "Video value", full: "Legitimate field" },
        },
      }
    )
  ).sceneCreate;
  await call(
    page,
    url,
    "mutation($id: ID!, $copyrights: [ID!]!) {sceneCopyrightsUpdate(scene_id: $id, copyright_ids: $copyrights) {id}}",
    { id: scene.id, copyrights: [copyright.id] }
  );
  const snapshot = async (origin) =>
    await call(
      page,
      origin,
      "query($artist: ID!, $scene: ID!) {findStudio(id: $artist) {id name urls custom_fields} findScene(id: $scene) {id title details custom_fields studio {id} artists {id} performers {id} copyrights {id} tags {id} groups {group {id} scene_index} files {id}}}",
      { artist: artist.id, scene: scene.id }
    );
  const before = await snapshot(url);
  const imageResponse = await page.request.get(artist.image_path);
  assert.equal(imageResponse.status(), 200);
  const blobBytes = await imageResponse.body();
  assert.ok(blobBytes.length > 0);
  await page.goto(url + "/settings?tab=tasks#backup", {
    waitUntil: "networkidle",
  });
  await page.locator("#tasks-profiles > summary").click();
  const otherDraft = page.locator("#review-inherit-artists");
  await otherDraft.focus();
  await otherDraft.press("Space");
  await page.locator("#backup").click();
  const dialog = page.locator(".modal.show");
  await dialog.waitFor();
  const include = dialog.locator("#backup-include-blobs");
  if (storage === "FILESYSTEM") {
    await include.focus();
    await include.press("Space");
  } else assert.equal(await include.isDisabled(), true);
  const expectedInclude = storage !== "FILESYSTEM";
  // Delay the request, but never fabricate a response. The actual server will
  // reject the blocked path after the pending-state assertions.
  let release;
  const gate = new Promise((done) => {
    release = done;
  });
  let count = 0;
  const route = async (request) => {
    const data = request.request().postDataJSON();
    if (data?.operationName !== "BackupDatabase") return request.continue();
    count++;
    await gate;
    await request.continue();
  };
  await page.route("**/graphql", route);
  const arrived = page.waitForRequest(
    (request) => request.postDataJSON()?.operationName === "BackupDatabase"
  );
  const response = page.waitForResponse(
    (result) =>
      result.request().postDataJSON()?.operationName === "BackupDatabase"
  );
  await dialog.getByRole("button", { name: "Backup", exact: true }).click();
  await arrived;
  try {
    assert.equal(
      await dialog.count(),
      1,
      "Keep backup choices mounted while saving"
    );
    assert.equal(await dialog.locator("#backup-directory").isDisabled(), true);
    assert.equal(await dialog.locator("#backup-download").isDisabled(), true);
    assert.equal(await include.isDisabled(), true);
    assert.equal(
      await dialog
        .getByRole("button", { name: "Cancel", exact: true })
        .isDisabled(),
      true
    );
    assert.equal(
      await dialog.locator(".ModalFooter button").last().isDisabled(),
      true
    );
    await dialog
      .locator(".ModalFooter button")
      .last()
      .evaluate((button) => button.click());
    assert.equal(count, 1);
  } finally {
    release();
  }
  const rejected = await (await response).json();
  assert.match(
    JSON.stringify(rejected.errors),
    /could not create backup directory/
  );
  const failureNotice = dialog.locator(".alert-danger");
  await failureNotice.waitFor();
  assert.match(
    await failureNotice.textContent(),
    /could not create backup directory/
  );
  await dialog.getByRole("button", { name: "Backup", exact: true }).waitFor();
  assert.equal(await include.isChecked(), expectedInclude);
  assert.equal(await dialog.locator("#backup-directory").isChecked(), true);
  assert.deepEqual(await snapshot(url), before);
  assert.equal(
    await page.evaluate(
      () => document.documentElement.scrollWidth > innerWidth
    ),
    false
  );
  await page.screenshot({
    path: join(artifacts, label + "-rejected.png"),
    fullPage: false,
  });
  await page.unroute("**/graphql", route);
  await rm(backups);
  await mkdir(backups);
  await dialog.getByRole("button", { name: "Backup", exact: true }).click();
  await dialog.waitFor({ state: "detached" });
  const savedFiles = await readdir(backups);
  assert.equal(savedFiles.length, 1);
  const saved = join(backups, savedFiles[0]);
  integrity(saved);
  assert.deepEqual(await snapshot(url), before);
  assert.equal(
    await otherDraft.isChecked(),
    false,
    "Backup must retain other Tasks drafts"
  );
  assert.equal(
    await page.locator("#tasks-profiles").evaluate((element) => element.open),
    true
  );
  // Download the default complete backup: SQLite for database blobs, a ZIP for
  // filesystem blobs. The browser must stay on Tasks rather than navigate away.
  await page.locator("#backup").click();
  await dialog.waitFor();
  assert.equal(
    await dialog.locator(".alert-danger").count(),
    0,
    "Successful retry clears the old error"
  );
  await dialog.locator("#backup-download").check();
  const downloaded = page.waitForEvent("download");
  await dialog
    .getByRole("button", { name: "Download backup", exact: true })
    .click();
  const download = await downloaded;
  assert.equal(await download.failure(), null);
  const target = join(
    directory,
    "downloaded-backup" + (storage === "FILESYSTEM" ? ".zip" : ".sqlite")
  );
  await download.saveAs(target);
  await dialog.waitFor({ state: "detached" });
  assert.equal(new URL(page.url()).pathname, "/settings");
  assert.equal(new URL(page.url()).searchParams.get("tab"), "tasks");
  assert.deepEqual(
    await readFile(join(directory, "media", "original.png")),
    mediaBytes
  );
  assert.deepEqual(await snapshot(url), before);
  await page.reload({ waitUntil: "networkidle" });
  await page.locator("#backup").click();
  await dialog.getByRole("button", { name: "Cancel", exact: true }).click();
  await dialog.waitFor({ state: "detached" });
  await page.close();
  await stop();

  // Restore into two NEW stopped instances; no source database is overwritten.
  for (const [kind, source] of [
    ["saved", saved],
    ["downloaded", target],
  ]) {
    const restored = join(root, label + "-" + kind);
    await configure(restored, storage, join(restored, "backups"));
    if (source.endsWith(".zip")) {
      const names = JSON.parse(
        execFileSync(
          "python3",
          [
            "-c",
            `
import json, pathlib, sys, zipfile
with zipfile.ZipFile(sys.argv[1]) as archive:
    assert archive.testzip() is None
    names = archive.namelist()
    assert 'library.sqlite' in names
    assert any(n.startswith('blobs/') for n in names)
    assert all(n == 'library.sqlite' or (n.startswith('blobs/') and '..' not in pathlib.PurePosixPath(n).parts) for n in names)
    archive.extractall(sys.argv[2])
    print(json.dumps(names))
`,
            source,
            restored,
          ],
          { encoding: "utf8" }
        )
      );
      assert.ok(names.length > 1);
    } else {
      await cp(source, join(restored, "library.sqlite"));
      if (storage === "FILESYSTEM") {
        // A database-only backup requires the separately preserved blobs.
        await cp(join(directory, "blobs"), join(restored, "blobs"), {
          recursive: true,
        });
      }
    }
    integrity(join(restored, "library.sqlite"));
    const restoredURL = await start(restored, label + "-" + kind);
    const restoredPage = await browser.newPage({ viewport });
    restoredPage.on("pageerror", (error) => errors.push(String(error)));
    // Use the restored instance for every API read after the source was stopped.
    const data = await call(
      restoredPage,
      restoredURL,
      "query($artist: ID!, $scene: ID!) {findStudio(id: $artist) {id name urls custom_fields image_path} findScene(id: $scene) {id title details custom_fields studio {id} artists {id} performers {id} copyrights {id} tags {id} groups {group {id} scene_index} files {id}}}",
      { artist: artist.id, scene: scene.id }
    );
    const { image_path: imagePath, ...studio } = data.findStudio;
    assert.deepEqual({ ...data, findStudio: studio }, before);
    const restoredBlob = await restoredPage.request.get(imagePath);
    assert.equal(restoredBlob.status(), 200);
    assert.deepEqual(await restoredBlob.body(), blobBytes);
    assert.deepEqual(
      await readdir(join(restored, "media")),
      [],
      "Backup does not contain media bytes"
    );
    await restoredPage.goto(restoredURL + "/studios/" + artist.id, {
      waitUntil: "networkidle",
    });
    const notes = restoredPage
      .locator(".modal.show")
      .filter({ hasText: "StashBooru: getting started" });
    if (await notes.count()) {
      await notes.getByRole("button", { name: "Close", exact: true }).click();
      await notes.waitFor({ state: "detached" });
    }
    await restoredPage.locator(".studio-head .studio-name").waitFor();
    assert.equal(
      await restoredPage.locator(".studio-head .studio-name").textContent(),
      "Backup Artist"
    );
    assert.equal(
      await restoredPage.evaluate(
        () => document.documentElement.scrollWidth > innerWidth
      ),
      false
    );
    await restoredPage.screenshot({
      path: join(artifacts, label + "-" + kind + "-restored.png"),
      fullPage: false,
    });
    await restoredPage.close();
    await stop();
  }
  assert.deepEqual(errors, []);
  console.log(
    label +
      ": actual backup rejection/pending/retry, saved and downloaded restore, native metadata/blob bytes and media exclusion passed."
  );
}
try {
  for (const [label, viewport] of [
    ["desktop", { width: 1440, height: 1000 }],
    ["mobile", { width: 390, height: 844 }],
  ]) {
    for (const storage of ["DATABASE", "FILESYSTEM"])
      await check(viewport, label + "-" + storage.toLowerCase(), storage);
  }
  successful = true;
} catch (error) {
  for (const page of browser.contexts().flatMap((context) => context.pages())) {
    await page
      .screenshot({
        path: join(artifacts, "backup-failure.png"),
        fullPage: true,
      })
      .catch(() => {});
  }
  throw error;
} finally {
  await browser.close();
  await stop();
  if (successful && process.env.STASH_BROWSER_SCREENSHOTS)
    await rm(root, { recursive: true, force: true });
  else console.log("Isolated backup fixture: " + root);
}
