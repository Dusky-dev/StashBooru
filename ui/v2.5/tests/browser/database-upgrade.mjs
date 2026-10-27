import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { cp, mkdtemp, mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { startIsolatedServer } from "./helpers/isolated-server.mjs";

// Both apps own fresh paths. The old app creates a genuine schema-99 database;
// never simulate an upgrade by changing a current database's version label.
assert.ok(process.env.STASH_BROWSER_BINARY, "Set STASH_BROWSER_BINARY");
assert.ok(
  process.env.STASH_UPGRADE_FROM_BINARY,
  "Set STASH_UPGRADE_FROM_BINARY (schema 99)"
);
const currentBinary = resolve(process.env.STASH_BROWSER_BINARY);
const oldBinary = resolve(process.env.STASH_UPGRADE_FROM_BINARY);
const { chromium } = await import(
  process.env.PLAYWRIGHT_MODULE ?? "playwright"
);
const root = await mkdtemp(join(tmpdir(), "stashbooru-upgrade-"));
const artifacts =
  process.env.STASH_BROWSER_SCREENSHOTS ?? join(root, "artifacts");
await mkdir(artifacts, { recursive: true });
const browser = await chromium.launch({
  executablePath: process.env.CHROMIUM_EXECUTABLE,
  args: ["--no-sandbox", "--disable-dev-shm-usage"],
});
let instance;
let successful = false;
const snapshotQuery = `query($artist: ID!, $character: ID!, $copyright: ID!, $scene: ID!) {
  findStudio(id: $artist) {id name aliases urls custom_fields}
  findPerformer(id: $character) {id name alias_list}
  findCopyright(id: $copyright) {id name aliases}
  findScene(id: $scene) {id title details custom_fields studio {id} artists {id} performers {id} copyrights {id} tags {id} groups {group {id} scene_index} files {id}}
}`;
async function call(page, url, query, variables = {}) {
  const response = await page.request.post(url + "/graphql", {
    data: { query, variables },
  });
  const result = await response.json();
  assert.equal(result.errors, undefined, JSON.stringify(result.errors));
  return result.data;
}
async function status(page, url) {
  return (
    await call(page, url, "{systemStatus {status databaseSchema appSchema}}")
  ).systemStatus;
}
async function stop() {
  await instance?.stop();
  instance = undefined;
}
async function start(directory, label, old = false) {
  instance = await startIsolatedServer({
    binary: old ? oldBinary : currentBinary,
    directory,
    artifacts,
    label,
    ui: old ? undefined : process.env.STASH_BROWSER_UI,
  });
  return instance.url;
}
async function configure(directory, backup) {
  for (const name of ["", "media", "generated", "cache", "metadata"])
    await mkdir(join(directory, name), { recursive: true });
  await writeFile(
    join(directory, "config.yml"),
    JSON.stringify({
      stash: [{ path: join(directory, "media") }],
      database: join(directory, "library.sqlite"),
      generated: join(directory, "generated"),
      cache: join(directory, "cache"),
      metadata: join(directory, "metadata"),
      blobs_path: join(directory, "blobs"),
      blobs_storage: "DATABASE",
      backup_directory_path: backup,
    })
  );
}
function schema(file, version, identities) {
  execFileSync("python3", [
    "-c",
    `
import pathlib, sqlite3, sys
with sqlite3.connect(pathlib.Path(sys.argv[1]).as_uri() + '?mode=ro', uri=True) as db:
    assert db.execute('PRAGMA integrity_check').fetchall() == [('ok',)]
    assert db.execute('PRAGMA foreign_key_check').fetchall() == []
    assert db.execute('SELECT version, dirty FROM schema_migrations').fetchall() == [(int(sys.argv[2]), 0)]
    assert bool(db.execute("SELECT name FROM sqlite_master WHERE name='provider_identities'").fetchall()) == (sys.argv[3] == 'true')
    assert any(col[1] == 'hide_thumbnail' for col in db.execute('PRAGMA table_info(tags)')) == (int(sys.argv[2]) >= 101)
`,
    file,
    String(version),
    String(identities),
  ]);
}
async function imageBytes(page, url, id) {
  const { findStudio } = await call(
    page,
    url,
    "query($id:ID!) {findStudio(id:$id) {image_path}}",
    { id }
  );
  const result = await page.request.get(findStudio.image_path);
  assert.equal(result.status(), 200);
  return result.body();
}
async function check(viewport, label) {
  const directory = join(root, label);
  const blocked = join(directory, "blocked-default");
  await configure(directory, blocked);
  await writeFile(blocked, "Unavailable default backup directory");
  let page = await browser.newPage({ viewport });
  const errors = [];
  const track = () => page.on("pageerror", (e) => errors.push(String(e)));
  track();
  const png = await page.evaluate(() => {
    const c = document.createElement("canvas");
    c.width = 64;
    c.height = 48;
    const ctx = c.getContext("2d");
    ctx.fillStyle = "#57c4c8";
    ctx.fillRect(0, 0, 64, 48);
    return c.toDataURL("image/png");
  });
  let url = await start(directory, label + "-old", true);
  assert.deepEqual(await status(page, url), {
    status: "OK",
    databaseSchema: 99,
    appSchema: 99,
  });
  const artist = (
    await call(
      page,
      url,
      "mutation($input:StudioCreateInput!) {studioCreate(input:$input) {id}}",
      {
        input: {
          name: "Upgrade Artist",
          aliases: ["waligner_(artist)"],
          urls: ["https://example.invalid/artist"],
          image: png,
          custom_fields: { note: "Retain artist value" },
        },
      }
    )
  ).studioCreate;
  const character = (
    await call(
      page,
      url,
      'mutation {performerCreate(input:{name:"Upgrade Character",alias_list:["Character alias"]}) {id}}'
    )
  ).performerCreate;
  const copyright = (
    await call(
      page,
      url,
      'mutation {copyrightCreate(input:{name:"Upgrade Copyright",aliases:["Series alias"]}) {id}}'
    )
  ).copyrightCreate;
  const tag = (
    await call(
      page,
      url,
      'mutation {tagCreate(input:{name:"Upgrade tag"}) {id}}'
    )
  ).tagCreate;
  const group = (
    await call(
      page,
      url,
      'mutation {groupCreate(input:{name:"Upgrade Collection"}) {id}}'
    )
  ).groupCreate;
  const scene = (
    await call(
      page,
      url,
      "mutation($input:SceneCreateInput!) {sceneCreate(input:$input) {id}}",
      {
        input: {
          title: "Upgrade Video",
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
    "mutation($id:ID!,$ids:[ID!]!) {sceneArtistsUpdate(scene_id:$id,artist_ids:$ids) {id}}",
    { id: scene.id, ids: [artist.id] }
  );
  await call(
    page,
    url,
    "mutation($id:ID!,$ids:[ID!]!) {sceneCopyrightsUpdate(scene_id:$id,copyright_ids:$ids) {id}}",
    { id: scene.id, ids: [copyright.id] }
  );
  const ids = {
    artist: artist.id,
    character: character.id,
    copyright: copyright.id,
    scene: scene.id,
  };
  const before = await call(page, url, snapshotQuery, ids);
  const blob = await imageBytes(page, url, artist.id);
  await page.close();
  await stop();
  schema(join(directory, "library.sqlite"), 99, false);
  url = await start(directory, label + "-upgrade");
  page = await browser.newPage({ viewport });
  track();
  assert.deepEqual(await status(page, url), {
    status: "NEEDS_MIGRATION",
    databaseSchema: 99,
    appSchema: 101,
  });
  await page.goto(url, { waitUntil: "networkidle" });
  await page
    .getByRole("heading", { name: "Migration required", exact: true })
    .waitFor();
  const input = page.locator('input[name="backupPath"]');
  const migrate = page.getByRole("button", {
    name: "Perform schema migration",
    exact: true,
  });
  const rejected = join(blocked, "cannot-save.sqlite");
  await input.fill(rejected);
  await migrate.click();
  await page
    .getByRole("heading", { name: "Migration failed", exact: true })
    .waitFor();
  assert.equal(await input.inputValue(), rejected);
  assert.equal((await status(page, url)).databaseSchema, 99);
  schema(join(directory, "library.sqlite"), 99, false);
  await page.screenshot({ path: join(artifacts, label + "-rejected.png") });
  const custom = join(directory, "custom-backups");
  await mkdir(custom);
  const selected = join(custom, "upgrade.sqlite");
  await input.fill(selected);
  const response = page.waitForResponse(
    (r) => r.request().postDataJSON()?.operationName === "Migrate"
  );
  await migrate.click();
  const submitted = await (await response).json();
  assert.equal(
    submitted.errors,
    undefined,
    "Full backup paths must bypass an unavailable default: " +
      JSON.stringify(submitted.errors)
  );
  for (let attempt = 0; attempt < 120; attempt++) {
    if ((await status(page, url)).status === "OK") break;
    await new Promise((done) => setTimeout(done, 250));
  }
  assert.deepEqual(await status(page, url), {
    status: "OK",
    databaseSchema: 101,
    appSchema: 101,
  });
  await page.waitForURL(
    (location) =>
      location.pathname !== "/migrate" && location.pathname !== "/setup"
  );
  await page
    .getByRole("heading", { name: "Migration required", exact: true })
    .waitFor({ state: "detached" });
  assert.deepEqual(await call(page, url, snapshotQuery, ids), before);
  schema(join(directory, "library.sqlite"), 101, true);
  schema(selected, 99, false);
  assert.equal(
    await readFile(blocked, "utf8"),
    "Unavailable default backup directory"
  );
  assert.deepEqual(await imageBytes(page, url, artist.id), blob);
  await page.goto(url + "/scenes/" + scene.id, { waitUntil: "networkidle" });
  const notes = page
    .locator(".modal.show")
    .filter({ hasText: "StashBooru: getting started" });
  if (await notes.count()) {
    await notes.getByRole("button", { name: "Close", exact: true }).click();
    await notes.waitFor({ state: "detached" });
  }
  await page.getByText("Upgrade Video", { exact: true }).first().waitFor();
  await page.reload({ waitUntil: "networkidle" });
  assert.deepEqual(await call(page, url, snapshotQuery, ids), before);
  assert.equal(
    await page.evaluate(
      () => document.documentElement.scrollWidth > innerWidth
    ),
    false
  );
  await page.screenshot({ path: join(artifacts, label + "-upgraded.png") });
  await page.close();
  await stop();
  // Restore into a separate fresh directory and reopen with the actual old app.
  const rollback = join(root, label + "-rollback");
  await configure(rollback, join(rollback, "backups"));
  await cp(selected, join(rollback, "library.sqlite"));
  url = await start(rollback, label + "-rollback", true);
  page = await browser.newPage({ viewport });
  track();
  assert.deepEqual(await status(page, url), {
    status: "OK",
    databaseSchema: 99,
    appSchema: 99,
  });
  assert.deepEqual(await call(page, url, snapshotQuery, ids), before);
  assert.deepEqual(await imageBytes(page, url, artist.id), blob);
  await page.close();
  await stop();
  url = await start(directory, label + "-restart");
  page = await browser.newPage({ viewport });
  track();
  assert.deepEqual(await status(page, url), {
    status: "OK",
    databaseSchema: 101,
    appSchema: 101,
  });
  assert.deepEqual(await call(page, url, snapshotQuery, ids), before);
  await page.close();
  await stop();
  assert.deepEqual(errors, []);
  console.log(
    label +
      ": actual 99→101 upgrade, failed backup/draft, custom-path retry, old-schema backup/old-app restore, metadata/aliases/blob bytes, refresh and restart passed."
  );
}
try {
  await check({ width: 1440, height: 1000 }, "desktop");
  await check({ width: 390, height: 844 }, "mobile");
  successful = true;
} catch (error) {
  for (const page of browser.contexts().flatMap((c) => c.pages()))
    await page
      .screenshot({
        path: join(artifacts, "upgrade-failure.png"),
        fullPage: true,
      })
      .catch(() => {});
  throw error;
} finally {
  await browser.close();
  await stop();
  if (successful && process.env.STASH_BROWSER_SCREENSHOTS)
    await rm(root, { recursive: true, force: true });
  else console.log("Isolated upgrade fixture: " + root);
}
