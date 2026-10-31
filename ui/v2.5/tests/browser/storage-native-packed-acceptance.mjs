import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { once } from "node:events";
import { createWriteStream } from "node:fs";
import {
  copyFile,
  mkdir,
  mkdtemp,
  readFile,
  rm,
  writeFile,
} from "node:fs/promises";
import { createServer } from "node:net";
import { tmpdir } from "node:os";
import { basename, join, resolve } from "node:path";
import { bootRecovered } from "./helpers/recovered-app-boot.mjs";

// The only accepted server is a freshly created fixture instance. Never
// request or accept a user-supplied URL, media path, database or credentials.
assert.ok(process.env.STASH_BROWSER_BINARY, "Built isolated server required");
const binary = resolve(process.env.STASH_BROWSER_BINARY);
const { chromium } = await import(
  process.env.PLAYWRIGHT_MODULE ?? "playwright"
);
const repository = resolve(import.meta.dirname, "../../../..");
const root = await mkdtemp(join(tmpdir(), "stash-packed-native-e2e-"));
const workspace = join(root, "instance");
const media = join(workspace, "media");
const backups = join(root, "backups");
const artifacts =
  process.env.STASH_BROWSER_SCREENSHOTS || join(root, "artifacts");
for (const directory of [
  workspace,
  media,
  backups,
  artifacts,
  join(workspace, "scripts"),
])
  await mkdir(directory, { recursive: true });
for (const script of [
  "media_library_backup.py",
  "image_storage_backup_preflight.py",
  "image_storage_native_candidate.py",
  "image_storage_pack_proof.py",
  "image_storage_shadow_member.py",
  "image_delta_storage.py",
  "gallery_block_storage.py",
]) {
  await copyFile(
    join(repository, "scripts", script),
    join(workspace, "scripts", script)
  );
}

const browser = await chromium.launch({
  args: ["--no-sandbox", "--disable-dev-shm-usage"],
});
const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
let server,
  logs,
  passed = false;

async function start() {
  const listener = createServer();
  listener.listen(0, "127.0.0.1");
  await once(listener, "listening");
  const port = listener.address().port;
  await new Promise((finish) => listener.close(finish));
  const url = "http://127.0.0.1:" + port;
  const env = Object.fromEntries(
    Object.entries(process.env).filter(([k]) => !k.startsWith("STASH_"))
  );
  env.STASH_LOCAL_HTTPS_ENABLED = "false";
  logs = createWriteStream(join(artifacts, "server.log"));
  server = spawn(
    binary,
    [
      "--config",
      join(workspace, "config.yml"),
      "--host",
      "127.0.0.1",
      "--port",
      String(port),
      "--nobrowser",
    ],
    { cwd: workspace, env }
  );
  server.stdout.pipe(logs, { end: false });
  server.stderr.pipe(logs, { end: false });
  let spawnFailure;
  server.on("error", (error) => {
    spawnFailure = error;
  });
  for (let i = 0; i < 120; i++) {
    if (spawnFailure) throw spawnFailure;
    assert.equal(server.exitCode, null, "Isolated app exited before startup");
    try {
      const r = await fetch(url + "/graphql", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ query: "{systemStatus{status}}" }),
        signal: AbortSignal.timeout(1000),
      });
      if ((await r.json()).data?.systemStatus) return url;
    } catch {
      /* startup pending */
    }
    await new Promise((done) => setTimeout(done, 250));
  }
  throw new Error("Isolated app failed to become ready");
}

async function post(url, path, data) {
  const response = await fetch(url + path, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(data),
  });
  const text = await response.text();
  assert.equal(response.status, 200, path + " failed: " + text.slice(0, 1500));
  return JSON.parse(text);
}
async function get(url, path) {
  const response = await fetch(url + path);
  assert.equal(
    response.status,
    200,
    path + " failed with HTTP " + response.status
  );
  return response.json();
}
async function gql(url, query, variables = {}) {
  const answer = await post(url, "/graphql", { query, variables });
  assert.equal(answer.errors, undefined, JSON.stringify(answer.errors));
  return answer.data;
}
async function poll(task, accept, label, timeout = 150000) {
  const deadline = Date.now() + timeout;
  let current;
  while (Date.now() < deadline) {
    current = await task();
    if (accept(current)) return current;
    await new Promise((done) => setTimeout(done, 350));
  }
  throw new Error(
    label + " timed out: " + JSON.stringify(current).slice(0, 900)
  );
}
function makeImages() {
  const canvas = document.createElement("canvas");
  canvas.width = 192;
  canvas.height = 160;
  const ctx = canvas.getContext("2d");
  const pixels = ctx.createImageData(192, 160);
  let seed = 19791;
  for (let i = 0; i < pixels.data.length; i += 4) {
    seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0;
    pixels.data[i] = seed & 255;
    pixels.data[i + 1] = (seed >>> 8) & 255;
    pixels.data[i + 2] = (seed >>> 16) & 255;
    pixels.data[i + 3] = 255;
  }
  ctx.putImageData(pixels, 0, 0);
  const first = canvas.toDataURL("image/png").split(",")[1];
  for (let y = 45; y < 50; y++)
    for (let x = 18; x < 24; x++) pixels.data[(y * 192 + x) * 4 + 1] ^= 28;
  ctx.putImageData(pixels, 0, 0);
  return { first, second: canvas.toDataURL("image/png").split(",")[1] };
}
async function onboarding(url) {
  await page.goto(url, { waitUntil: "domcontentloaded" });
  await page.getByRole("button", { name: "Next", exact: true }).click();
  await page.getByRole("button", { name: /^Add directory$/i }).click();
  const dialog = page.getByRole("dialog");
  await dialog.locator("input").fill(media);
  await dialog.getByRole("button", { name: "Confirm", exact: true }).click();
  for (const [id, value] of Object.entries({
    database: join(workspace, "stash.sqlite"),
    generated: join(workspace, "generated"),
    cache: join(workspace, "cache"),
    blobs: join(workspace, "blobs"),
  }))
    await page.locator("#" + id + " input:not([type=checkbox])").fill(value);
  await page.getByRole("button", { name: "Next", exact: true }).click();
  await page.getByRole("button", { name: "Next", exact: true }).click();
  await page.getByRole("button", { name: "Confirm", exact: true }).click();
  await page.waitForURL(url + "/welcome");
  await page.getByRole("button", { name: "Finish", exact: true }).click();
  await page.waitForURL(url + "/settings?tab=library");
  await page.getByRole("tab", { name: "Tasks", exact: true }).click();
  await page.locator("#tasks-library > summary").click();
  const response = page.waitForResponse(
    (r) =>
      r.url().endsWith("/graphql") &&
      r.request().postDataJSON()?.operationName === "MetadataScan"
  );
  await page.getByRole("button", { name: "Scan", exact: true }).click();
  assert.equal((await (await response).json()).errors, undefined);
  await poll(
    () => gql(url, "{findImages{count}}"),
    (x) => x.findImages.count === 2,
    "Two synthetic images were not scanned"
  );
}
async function verifyNative(url, item, original, candidate, stackID) {
  const path = "/image/" + item.id + "/image";
  const direct = await fetch(url + path);
  assert.equal(direct.status, 200);
  assert.deepEqual(Buffer.from(await direct.arrayBuffer()), original);
  const params = new URLSearchParams({
    storageShadowCandidate: candidate,
    storageShadowStack: String(stackID),
  });
  const target = url + path + "?" + params;
  const packed = await fetch(target);
  const packedBytes = Buffer.from(await packed.arrayBuffer());
  assert.equal(
    packed.status,
    200,
    "packed GET rejected for " +
      item.id +
      ": " +
      packedBytes.toString("utf8").slice(0, 200)
  );
  assert.deepEqual(packedBytes, original);
  assert.equal(packed.headers.get("cache-control"), "no-store");
  assert.equal(packed.headers.get("x-content-type-options"), "nosniff");
  const range = await fetch(target, { headers: { Range: "bytes=5-79" } });
  assert.equal(range.status, 206);
  assert.deepEqual(
    Buffer.from(await range.arrayBuffer()),
    original.subarray(5, 80)
  );
  const preview = await fetch(
    url + "/image/" + item.id + "/thumbnail?" + params
  );
  const previewBytes = Buffer.from(await preview.arrayBuffer());
  assert.equal(
    preview.status,
    200,
    "packed thumbnail rejected: " + previewBytes.toString("utf8").slice(0, 200)
  );
  assert.equal(preview.headers.get("content-type"), "image/jpeg");
  assert.ok(
    previewBytes.length > 128 &&
      previewBytes[0] === 255 &&
      previewBytes[1] === 216
  );
}

async function verifyBrowserPreview(url, member, candidate, stackID) {
  const prefix = "/image/" + member.id + "/image?";
  const query = new URLSearchParams({
    storageShadowCandidate: candidate,
    storageShadowStack: String(stackID),
  });
  const expected = url + prefix + query;
  const packedResponse = page.waitForResponse(
    (response) => response.url() === expected,
    { timeout: 20000 }
  );
  await page.goto(url + "/images/" + member.id + "?" + query, {
    waitUntil: "domcontentloaded",
  });
  assert.equal((await packedResponse).status(), 200);
  const picture = page.locator("img.image-image").first();
  await picture.waitFor();
  await picture.evaluate(async (img) => img.decode());
  const rendered = await picture.evaluate((img) => ({
    source: img.getAttribute("src"),
    width: img.naturalWidth,
    height: img.naturalHeight,
    complete: img.complete,
  }));
  assert.equal(rendered.source, prefix + query);
  assert.equal(rendered.width, 192);
  assert.equal(rendered.height, 160);
  assert.equal(rendered.complete, true);
}

async function verifyBrowserFallback(url, member, stackID) {
  // The browser opts in with a structurally valid but unknown candidate.
  // A 409 must trigger the image-detail viewer's original-file fallback.
  const missing = "stack-" + stackID + "-delta-" + "0".repeat(24);
  const query = new URLSearchParams({
    storageShadowCandidate: missing,
    storageShadowStack: String(stackID),
  });
  const failedRequest = page.waitForResponse(
    (response) =>
      response.url() === url + "/image/" + member.id + "/image?" + query,
    { timeout: 20000 }
  );
  await page.goto(url + "/images/" + member.id + "?" + query, {
    waitUntil: "domcontentloaded",
  });
  assert.equal((await failedRequest).status(), 409);
  await page.waitForFunction(
    () => {
      const img = document.querySelector("img.image-image");
      return (
        img?.complete === true &&
        img.naturalWidth === 192 &&
        img.naturalHeight === 160 &&
        !img.getAttribute("src")?.includes("storageShadowCandidate")
      );
    },
    null,
    { timeout: 15000 }
  );
  const path = await page
    .locator("img.image-image")
    .first()
    .getAttribute("src");
  assert.match(path, new RegExp("/image/" + member.id + "/image"));
}

try {
  const images = await page.evaluate(makeImages);
  const originals = new Map([
    ["source.png", Buffer.from(images.first, "base64")],
    ["variant.png", Buffer.from(images.second, "base64")],
  ]);
  for (const [name, data] of originals)
    await writeFile(join(media, name), data);
  let url = await start();
  await onboarding(url);
  const all = await gql(
    url,
    "{findImages{images{id visual_files{... on ImageFile{path}}}}}"
  );
  const members = all.findImages.images.map((item) => ({
    id: item.id,
    name: basename(item.visual_files[0].path),
  }));
  assert.equal(members.length, 2);
  for (const member of members) assert.ok(originals.has(member.name));
  const stack = await gql(
    url,
    "mutation($input:VisualStackCreateInput!){visualStackCreate(input:$input){id member_count}}",
    {
      input: {
        title: "Isolated P12/P13 acceptance",
        members: members.map((m) => ({ media: { kind: "IMAGE", id: m.id } })),
        representative: { kind: "IMAGE", id: members[0].id },
      },
    }
  );
  assert.equal(stack.visualStackCreate.member_count, 2);
  const stackID = Number(stack.visualStackCreate.id);
  assert.ok(Number.isSafeInteger(stackID) && stackID > 0);
  await post(url, "/image/media-backup", {
    action: "config",
    destination: backups,
  });
  await post(url, "/image/media-backup", { action: "backup" });
  const state = await poll(
    () => get(url, "/image/media-backup"),
    (s) => !s.busy && s.snapshots?.some((entry) => entry.files === 2),
    "Verified media snapshot"
  );
  const snapshot = state.snapshots.find((entry) => entry.files === 2).snapshot;
  const created = [];
  for (const method of ["delta", "shared-tiles"]) {
    await post(url, "/image/storage-candidate", {
      stackID,
      snapshot,
      method,
      acknowledged: true,
    });
    const list = await poll(
      () => get(url, "/image/storage-candidate"),
      (s) =>
        !s.busy &&
        s.candidates?.some(
          (entry) =>
            entry.group === "stack:" + stackID &&
            entry.method === method &&
            entry.snapshot === snapshot
        ),
      method + " native candidate"
    );
    const candidate = list.candidates.find(
      (entry) =>
        entry.group === "stack:" + stackID &&
        entry.method === method &&
        entry.snapshot === snapshot
    );
    assert.match(
      candidate.id,
      /^stack-[1-9]\d*-(delta|shared-tiles)-[a-f0-9]{24}$/
    );
    created.push(candidate);
    await verifyBrowserPreview(url, members[0], candidate.id, stackID);
    for (const member of members)
      await verifyNative(
        url,
        member,
        originals.get(member.name),
        candidate.id,
        stackID
      );
    const invalid = await fetch(
      url +
        "/image/" +
        members[0].id +
        "/image?" +
        new URLSearchParams({
          storageShadowCandidate: candidate.id,
          storageShadowStack: String(stackID),
          archive: "/etc/passwd",
        })
    );
    assert.equal(invalid.status, 400);
    const wrongStack = await fetch(
      url +
        "/image/" +
        members[0].id +
        "/image?" +
        new URLSearchParams({
          storageShadowCandidate: candidate.id,
          storageShadowStack: "999999",
        })
    );
    assert.equal(wrongStack.status, 409);
    console.log(
      method +
        " real-server image GET, Range, JPEG thumbnail and source verification passed"
    );
  }
  await verifyBrowserFallback(url, members[0], stackID);
  // Exercise persisted non-activating trial selection through the real
  // Settings interface; normal media routes must still return originals.
  await page.goto(url + "/settings?tab=library", {
    waitUntil: "domcontentloaded",
  });
  await page.getByRole("tab", { name: "Tasks", exact: true }).click();
  // The storage tools are inside a native <details> disclosure.  Playwright
  // must expand it before interacting with the candidate selector; bypassing
  // visibility here would hide a real Settings navigation regression.
  await page.locator("#tasks-media > summary").click();
  await page.locator("#shadow-candidate").selectOption(created[0].id);
  await page.locator("#shadow-intent-acknowledge").check();
  await page
    .getByRole("button", {
      name: "Prepare candidate (no activation)",
    })
    .click();
  await page.getByText("Saved intent: prepared", { exact: false }).waitFor();
  const prepared = await get(url, "/image/storage-shadow-selection");
  assert.equal(prepared.State, "prepared");
  assert.equal(prepared.CandidateID, created[0].id);
  assert.equal(prepared.EvidenceVerifiedNow, true);
  assert.equal(prepared.OriginalsAuthoritative, true);
  // The new endpoint does not accept paths or arbitrary member IDs. It
  // verifies every saved member against the retained original and backup.
  const withoutAcknowledgement = await fetch(
    url + "/image/storage-shadow-preflight",
    {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ Acknowledged: false }),
    }
  );
  assert.equal(withoutAcknowledgement.status, 400);
  await page.locator("#shadow-intent-acknowledge").check();
  await page
    .getByRole("button", {
      name: "Verify prepared candidate bytes (read-only)",
    })
    .click();
  await page
    .getByText("Verified 2 packed images byte-for-byte", { exact: false })
    .waitFor({ timeout: 90000 });
  const verifiedAgain = await post(url, "/image/storage-shadow-preflight", {
    Acknowledged: true,
  });
  assert.equal(verifiedAgain.ReadOnly, true);
  assert.equal(verifiedAgain.MembersVerified, 2);
  assert.equal(verifiedAgain.MediaBackupVerified, true);
  assert.equal(verifiedAgain.AllOriginalBytesVerified, true);
  assert.equal(verifiedAgain.OriginalsAuthoritative, true);
  for (const flag of [
    "NativeRoutingEnabled",
    "CanActivate",
    "OriginalEvictionEnabled",
    "TransactionalRollbackVerified",
  ])
    assert.equal(verifiedAgain[flag], false, flag + " must remain false");

  // Canary uses only the saved prepared state; it is never automatic.
  // A readable original remains the fallback for an invalid candidate.
  const canaryURL = (member) => url + "/image/" + member.id + "/storage-canary";
  for (const member of members) {
    const response = await fetch(canaryURL(member));
    assert.equal(response.status, 200);
    assert.equal(
      response.headers.get("x-stash-shadow-source"),
      "packed-verified"
    );
    assert.equal(response.headers.get("cache-control"), "no-store");
    assert.deepEqual(
      Buffer.from(await response.arrayBuffer()),
      originals.get(member.name)
    );
  }
  const partial = await fetch(canaryURL(members[0]), {
    headers: { Range: "bytes=5-79" },
  });
  assert.equal(partial.status, 206);
  assert.deepEqual(
    Buffer.from(await partial.arrayBuffer()),
    originals.get(members[0].name).subarray(5, 80)
  );
  const injection = await fetch(canaryURL(members[0]) + "?archive=/etc/shadow");
  assert.equal(injection.status, 400);
  const modifiedOriginal = Buffer.from(
    "isolated source no longer matches pack"
  );
  await writeFile(join(media, members[0].name), modifiedOriginal);
  const fallback = await fetch(canaryURL(members[0]));
  assert.equal(fallback.status, 200);
  assert.equal(
    fallback.headers.get("x-stash-shadow-source"),
    "original-fallback"
  );
  assert.deepEqual(Buffer.from(await fallback.arrayBuffer()), modifiedOriginal);
  await writeFile(join(media, members[0].name), originals.get(members[0].name));
  assert.equal(
    (await fetch(canaryURL(members[0]))).headers.get("x-stash-shadow-source"),
    "packed-verified"
  );
  // Comparison images themselves use the shared verification slot, so verify
  // the canary API before opening these concurrent UI previews.
  await page
    .getByRole("button", {
      name: "Compare original and packed reconstruction",
    })
    .click();
  const canaryLink = page.getByRole("link", {
    name: "Open prepared canary (verified archive or original fallback)",
  });
  assert.equal(
    await canaryLink.getAttribute("href"),
    "image/" + members[0].id + "/storage-canary"
  );
  // A normal server restart must not activate archived routing. The saved
  // prepared selection is reloaded and freshly verified after restart.
  const stopped = once(server, "exit");
  server.kill("SIGTERM");
  await stopped;
  logs?.end();
  url = await start();
  const restarted = await get(url, "/image/storage-shadow-selection");
  assert.equal(restarted.State, "prepared");
  assert.equal(restarted.Revision, prepared.Revision);
  assert.equal(restarted.NativeRoutingEnabled, false);
  assert.equal(
    (await fetch(canaryURL(members[0]))).headers.get("x-stash-shadow-source"),
    "packed-verified"
  );
  // Reopen Settings on the replacement isolated process for disarm coverage.
  await page.goto(url + "/settings?tab=library", {
    waitUntil: "domcontentloaded",
  });
  await page.getByRole("tab", { name: "Tasks", exact: true }).click();
  await page.locator("#tasks-media > summary").click();

  for (const key of [
    "NativeRoutingEnabled",
    "CanActivate",
    "OriginalEvictionEnabled",
    "TransactionalRollbackVerified",
  ])
    assert.equal(prepared[key], false, key + " must remain disabled");
  for (const member of members) {
    const normal = await fetch(url + "/image/" + member.id + "/image");
    assert.equal(normal.status, 200);
    assert.deepEqual(
      Buffer.from(await normal.arrayBuffer()),
      originals.get(member.name)
    );
  }
  await page.locator("#shadow-intent-acknowledge").check();
  await page.getByRole("button", { name: "Disarm saved trial" }).click();
  await page.getByText("Saved intent: disarmed", { exact: false }).waitFor();
  const disarmed = await get(url, "/image/storage-shadow-selection");
  assert.equal(disarmed.State, "disarmed");
  assert.equal(disarmed.NativeRoutingEnabled, false);
  assert.equal(disarmed.OriginalEvictionEnabled, false);
  assert.equal(disarmed.Revision, prepared.Revision + 1);
  const revokedCanary = await fetch(canaryURL(members[0]));
  assert.equal(revokedCanary.status, 409);

  const disarmedPreflight = await fetch(
    url + "/image/storage-shadow-preflight",
    {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ Acknowledged: true }),
    }
  );
  assert.equal(disarmedPreflight.status, 409);

  const selected = members[0];
  const tamperURL =
    url +
    "/image/" +
    selected.id +
    "/image?" +
    new URLSearchParams({
      storageShadowCandidate: created[0].id,
      storageShadowStack: String(stackID),
    });
  await writeFile(
    join(media, selected.name),
    Buffer.from("synthetic original changed")
  );
  assert.equal((await fetch(tamperURL)).status, 409);
  await writeFile(join(media, selected.name), originals.get(selected.name));
  assert.equal((await fetch(tamperURL)).status, 200);
  for (const member of members)
    assert.deepEqual(
      await readFile(join(media, member.name)),
      originals.get(member.name)
    );

  // End the original fixture process before taking its SQLite backup.
  // The recovered process must use a separately rebased and writable clone.
  const stoppedForRecovery = once(server, "exit");
  server.kill("SIGTERM");
  await stoppedForRecovery;
  logs?.end();
  await bootRecovered({
    binary,
    root,
    repository,
    workspace,
    media,
    backups,
    snapshot,
    members,
    originals,
  });
  passed = true;
  console.log(
    "Isolated native-route P12/P13 acceptance completed; all originals intact"
  );
} finally {
  await page.close();
  await browser.close();
  if (server && server.exitCode === null && server.signalCode === null) {
    const exited = once(server, "exit");
    server.kill("SIGTERM");
    await exited;
  }
  logs?.end();
  if (passed) await rm(root, { recursive: true, force: true });
  else
    console.error("Isolated failure files and server log retained at " + root);
}
