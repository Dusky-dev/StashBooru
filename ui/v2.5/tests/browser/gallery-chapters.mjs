import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { once } from "node:events";
import { mkdtemp, mkdir, writeFile, rm } from "node:fs/promises";
import { createWriteStream } from "node:fs";
import { createServer } from "node:net";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";

// Own the configuration, database, media and loopback process. Never point this
// suite at an existing library. GraphQL responses always come from the server.
assert.ok(process.env.STASH_BROWSER_BINARY, "Set STASH_BROWSER_BINARY");
const binary = resolve(process.env.STASH_BROWSER_BINARY);
const ui = process.env.STASH_BROWSER_UI;
const { chromium } = await import(
  process.env.PLAYWRIGHT_MODULE ?? "playwright"
);
const root = await mkdtemp(join(tmpdir(), "stashbooru-chapters-"));
const artifacts =
  process.env.STASH_BROWSER_SCREENSHOTS ?? join(root, "artifacts");
await mkdir(artifacts, { recursive: true });
const browser = await chromium.launch({
  executablePath: process.env.CHROMIUM_EXECUTABLE,
  args: ["--no-sandbox", "--disable-dev-shm-usage"],
});
let server;
let output;
let successful = false;
async function stop() {
  if (server && server.exitCode === null && server.signalCode === null) {
    const exited = once(server, "exit");
    server.kill("SIGTERM");
    await exited;
  }
  output?.end();
  server = undefined;
}
async function call(page, url, query, variables = {}) {
  const response = await page.request.post(url + "/graphql", {
    data: { query, variables },
  });
  const payload = await response.json();
  assert.equal(payload.errors, undefined, JSON.stringify(payload.errors));
  return payload.data;
}
async function start(directory, label) {
  const listener = createServer();
  listener.listen(0, "127.0.0.1");
  await once(listener, "listening");
  const port = listener.address().port;
  await new Promise((done) => listener.close(done));
  const url = "http://127.0.0.1:" + port;
  const env = Object.fromEntries(
    Object.entries(process.env).filter(([key]) => !key.startsWith("STASH_"))
  );
  env.STASH_LOCAL_HTTPS_ENABLED = "false";
  if (ui) env.STASH_UI = resolve(ui);
  output = createWriteStream(join(artifacts, label + "-server.log"));
  server = spawn(
    binary,
    [
      "--config",
      join(directory, "config.yml"),
      "--host",
      "127.0.0.1",
      "--port",
      String(port),
      "--nobrowser",
    ],
    { cwd: directory, env }
  );
  server.stdout.pipe(output, { end: false });
  server.stderr.pipe(output, { end: false });
  let spawnError;
  server.on("error", (error) => {
    spawnError = error;
  });
  for (let attempt = 0; attempt < 120; attempt++) {
    if (spawnError) throw spawnError;
    assert.equal(server.exitCode, null, "Isolated server exited");
    try {
      const response = await fetch(url + "/graphql", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ query: "{ systemStatus { status } }" }),
        signal: AbortSignal.timeout(1000),
      });
      if ((await response.json()).data?.systemStatus) return url;
    } catch {
      /* The new process has not bound its port yet. */
    }
    await new Promise((done) => setTimeout(done, 250));
  }
  throw new Error("Isolated server did not become ready");
}
async function check(viewport, label) {
  const directory = join(root, label);
  const media = join(directory, "media");
  await mkdir(media, { recursive: true });
  const page = await browser.newPage({ viewport });
  const errors = [];
  page.on("pageerror", (error) => errors.push(String(error)));
  for (const [index, color] of ["#243550", "#57c4c8"].entries()) {
    const png = await page.evaluate((fill) => {
      const canvas = document.createElement("canvas");
      canvas.width = 128;
      canvas.height = 96;
      const ctx = canvas.getContext("2d");
      ctx.fillStyle = fill;
      ctx.fillRect(0, 0, 128, 96);
      return canvas.toDataURL("image/png").split(",")[1];
    }, color);
    await writeFile(
      join(media, String(index) + ".png"),
      Buffer.from(png, "base64")
    );
  }
  await writeFile(
    join(directory, "config.yml"),
    JSON.stringify({
      stash: [{ path: media }],
      database: join(directory, "library.sqlite"),
      generated: join(directory, "generated"),
      cache: join(directory, "cache"),
      metadata: join(directory, "metadata"),
      blobs_path: join(directory, "blobs"),
    })
  );
  const url = await start(directory, label);
  await call(
    page,
    url,
    "mutation { configureUI(input: {lastNoteSeen: 99999999}) }"
  );
  await call(
    page,
    url,
    "mutation($path: String!) { metadataScan(input: {paths: [$path]}) }",
    { path: media }
  );
  let images = [];
  for (let attempt = 0; attempt < 120; attempt++) {
    images = (
      await call(
        page,
        url,
        "{ findImages(filter: {per_page: -1}) { images { id title galleries {id} } } }"
      )
    ).findImages.images;
    if (images.length === 2) break;
    await new Promise((done) => setTimeout(done, 250));
  }
  assert.equal(images.length, 2, "Scan must index both generated PNGs");
  const gallery = (
    await call(
      page,
      url,
      'mutation { galleryCreate(input: {title: "Chapter acceptance"}) {id} }'
    )
  ).galleryCreate;
  await call(
    page,
    url,
    "mutation($gallery: ID!, $images: [ID!]!) { addGalleryImages(input: {gallery_id: $gallery, image_ids: $images}) }",
    { gallery: gallery.id, images: images.map((image) => image.id) }
  );
  const snapshot = async () =>
    await call(
      page,
      url,
      "query($id: ID!) { findGallery(id: $id) {id title image_count chapters {id title image_index} } findImages(filter: {per_page: -1}) {images {id title galleries {id} } } }",
      { id: gallery.id }
    );
  const before = await snapshot();
  await page.goto(url + "/galleries/" + gallery.id, {
    waitUntil: "networkidle",
  });
  await page.getByRole("tab", { name: "Chapters", exact: true }).click();
  await page
    .getByRole("button", { name: "Create Chapter", exact: true })
    .click();
  const form = page
    .locator("form")
    .filter({ has: page.locator('input[name="image_index"]') });
  const title = form.locator('input[name="title"]');
  const index = form.locator('input[name="image_index"]');
  const save = form.getByRole("button", { name: "Save", exact: true });
  const cancel = form.getByRole("button", { name: "Cancel", exact: true });
  const remove = form.getByRole("button", { name: "Delete", exact: true });
  async function action(
    operation,
    button,
    { rejected = false, invalidID = false } = {}
  ) {
    let release;
    const gate = new Promise((done) => {
      release = done;
    });
    const requestArrived = page.waitForRequest(
      (request) =>
        request.method() === "POST" &&
        request.postDataJSON()?.operationName === operation
    );
    let count = 0;
    const handler = async (route) => {
      const request = route.request();
      const body =
        request.method() === "POST" ? request.postDataJSON() : undefined;
      if (body?.operationName !== operation) return route.continue();
      count++;
      await gate;
      if (invalidID) body.variables.id = "invalid-chapter-fixture";
      await route.continue({ postData: JSON.stringify(body) });
    };
    await page.route("**/graphql", handler);
    try {
      await button.click();
      await requestArrived;
      await page.waitForFunction(() =>
        document.querySelector('form[aria-busy="true"]')
      );
      assert.equal(await title.isDisabled(), true);
      assert.equal(await index.isDisabled(), true);
      assert.equal(await save.isDisabled(), true);
      assert.equal(await cancel.isDisabled(), true);
      if (await remove.count()) assert.equal(await remove.isDisabled(), true);
      const response = page.waitForResponse(
        (r) =>
          r.request().method() === "POST" &&
          r.request().postDataJSON()?.operationName === operation
      );
      release();
      const payload = await (await response).json();
      assert.equal(!!payload.errors?.length, rejected, JSON.stringify(payload));
      assert.equal(count, 1, "One action must issue one mutation");
      if (rejected) {
        await page.locator(".toast.danger").waitFor();
        await form.waitFor({ state: "visible", timeout: 5000 });
        await page.waitForFunction(() =>
          document.querySelector('form[aria-busy="false"]')
        );
        assert.equal(await title.isEnabled(), true);
        assert.equal(await cancel.isEnabled(), true);
      } else {
        await form.waitFor({ state: "hidden" });
      }
    } finally {
      release();
      await page.unroute("**/graphql", handler);
    }
  }
  await title.fill("Opening");
  await index.fill("3");
  await action("GalleryChapterCreate", save, { rejected: true });
  assert.equal(await title.inputValue(), "Opening");
  assert.equal(await index.inputValue(), "3");
  assert.deepEqual((await snapshot()).findGallery.chapters, []);
  await index.fill("2");
  await action("GalleryChapterCreate", save);
  const created = (await snapshot()).findGallery.chapters[0];
  assert.equal(created.title, "Opening");
  assert.equal(created.image_index, 2);
  await page
    .locator(".tab-pane.active")
    .getByRole("button", { name: "Edit", exact: true })
    .click();
  await title.fill("Retained edit");
  await index.fill("3");
  await action("GalleryChapterUpdate", save, { rejected: true });
  assert.equal(await title.inputValue(), "Retained edit");
  assert.equal(await index.inputValue(), "3");
  assert.deepEqual((await snapshot()).findGallery.chapters, [created]);
  await index.fill("1");
  await action("GalleryChapterUpdate", save);
  const updated = (await snapshot()).findGallery.chapters[0];
  assert.deepEqual(updated, {
    id: created.id,
    title: "Retained edit",
    image_index: 1,
  });
  await page.reload({ waitUntil: "networkidle" });
  await page.getByRole("tab", { name: "Chapters", exact: true }).click();
  await page
    .locator(".tab-pane.active")
    .getByRole("button", { name: "Edit", exact: true })
    .click();
  assert.equal(await title.inputValue(), "Retained edit");
  assert.equal(await index.inputValue(), "1");
  await title.fill("Unsaved delete draft");
  await action("GalleryChapterDestroy", remove, {
    rejected: true,
    invalidID: true,
  });
  assert.equal(await title.inputValue(), "Unsaved delete draft");
  assert.deepEqual((await snapshot()).findGallery.chapters, [updated]);
  await page.screenshot({
    path: join(artifacts, label + "-retry.png"),
    fullPage: true,
  });
  assert.equal(
    await form.evaluate(
      (element) => element.scrollWidth > window.innerWidth + 1
    ),
    false
  );
  await action("GalleryChapterDestroy", remove);
  const after = await snapshot();
  assert.deepEqual(after.findGallery.chapters, []);
  assert.deepEqual(
    after.findImages,
    before.findImages,
    "Chapter editing changed native Images or membership"
  );
  assert.equal(after.findGallery.title, before.findGallery.title);
  assert.equal(after.findGallery.image_count, 2);
  await page.reload({ waitUntil: "networkidle" });
  await page.getByRole("tab", { name: "Chapters", exact: true }).click();
  await page
    .getByRole("button", { name: "Create Chapter", exact: true })
    .waitFor();
  assert.equal(
    await page
      .locator(".tab-pane.active")
      .getByRole("button", { name: "Edit", exact: true })
      .count(),
    0
  );
  assert.deepEqual(errors, []);
  await page.close();
  await stop();
  console.log(
    "Gallery chapter " +
      label +
      ": create/update/delete rejection, retained drafts, pending controls, retry, reload and native membership passed."
  );
}
try {
  await check({ width: 1440, height: 1000 }, "desktop");
  await check({ width: 390, height: 844 }, "mobile");
  successful = true;
} catch (error) {
  const page = browser.contexts()[0]?.pages()[0];
  if (page)
    await page
      .screenshot({
        path: join(artifacts, "chapter-failure.png"),
        fullPage: true,
      })
      .catch(() => {});
  throw error;
} finally {
  await browser.close();
  await stop();
  if (successful && process.env.STASH_BROWSER_SCREENSHOTS)
    await rm(root, { recursive: true, force: true });
  else console.log("Isolated chapter fixture: " + root);
}
