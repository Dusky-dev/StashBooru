import assert from "node:assert/strict";
import { mkdtemp, mkdir, writeFile, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { startIsolatedServer } from "./helpers/isolated-server.mjs";

// Own every server, path and generated PNG. No existing library URL or mocked
// response is accepted. Failure injection alters only an outgoing fixture ID.
assert.ok(process.env.STASH_BROWSER_BINARY, "Set STASH_BROWSER_BINARY");
const binary = resolve(process.env.STASH_BROWSER_BINARY);
const ui = process.env.STASH_BROWSER_UI;
const { chromium } = await import(
  process.env.PLAYWRIGHT_MODULE ?? "playwright"
);
const root = await mkdtemp(join(tmpdir(), "stashbooru-image-save-"));
const artifacts =
  process.env.STASH_BROWSER_SCREENSHOTS ?? join(root, "artifacts");
await mkdir(artifacts, { recursive: true });
const browser = await chromium.launch({
  executablePath: process.env.CHROMIUM_EXECUTABLE,
  args: ["--no-sandbox", "--disable-dev-shm-usage"],
});
let instance;
let passed = false;
async function call(page, url, query, variables = {}) {
  const payload = await (
    await page.request.post(url + "/graphql", { data: { query, variables } })
  ).json();
  assert.equal(payload.errors, undefined, JSON.stringify(payload.errors));
  return payload.data;
}
async function check(viewport, label) {
  const directory = join(root, label);
  const media = join(directory, "media");
  for (const folder of [
    media,
    ...["generated", "cache", "metadata"].map((name) => join(directory, name)),
  ])
    await mkdir(folder, { recursive: true });
  let page = await browser.newPage({ viewport });
  const errors = [];
  const recordError = (error) => errors.push(String(error));
  page.on("pageerror", recordError);
  const png = await page.evaluate(() => {
    const canvas = document.createElement("canvas");
    canvas.width = 128;
    canvas.height = 96;
    const context = canvas.getContext("2d");
    context.fillStyle = "#243550";
    context.fillRect(0, 0, 128, 96);
    context.fillStyle = "#57c4c8";
    context.fillRect(12, 16, 40, 48);
    return canvas.toDataURL("image/png").split(",")[1];
  });
  const filename = join(media, "fixture.png");
  const bytes = Buffer.from(png, "base64");
  await writeFile(filename, bytes);
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
  async function start(suffix) {
    instance = await startIsolatedServer({
      binary,
      directory,
      artifacts,
      label: label + suffix,
      ui,
    });
    return instance.url;
  }
  let url = await start("");
  await call(
    page,
    url,
    "mutation { configureUI(input: {lastNoteSeen: 99999999}) }"
  );
  await call(
    page,
    url,
    "mutation($path: String!) {metadataScan(input: {paths: [$path]})}",
    { path: media }
  );
  let images = [];
  for (let attempt = 0; attempt < 120; attempt++) {
    images = (
      await call(
        page,
        url,
        "{findImages(filter: {per_page: -1}) {images {id}}}"
      )
    ).findImages.images;
    if (images.length === 1) break;
    await new Promise((done) => setTimeout(done, 250));
  }
  assert.equal(images.length, 1);
  const id = images[0].id;
  const artist = (
    await call(
      page,
      url,
      'mutation {studioCreate(input: {name: "Fixture Artist"}) {id}}'
    )
  ).studioCreate;
  const character = (
    await call(
      page,
      url,
      'mutation {performerCreate(input: {name: "Fixture Character"}) {id}}'
    )
  ).performerCreate;
  const copyright = (
    await call(
      page,
      url,
      'mutation {copyrightCreate(input: {name: "Fixture Copyright"}) {id}}'
    )
  ).copyrightCreate;
  const tag = (
    await call(
      page,
      url,
      'mutation {tagCreate(input: {name: "Fixture tag"}) {id}}'
    )
  ).tagCreate;
  const gallery = (
    await call(
      page,
      url,
      'mutation {galleryCreate(input: {title: "Fixture Gallery"}) {id}}'
    )
  ).galleryCreate;
  await call(
    page,
    url,
    "mutation($input: ImageUpdateInput!) {imageUpdate(input: $input) {id}}",
    {
      input: {
        id,
        title: "Initial Image",
        code: "keep-code",
        details: "Keep details",
        date: "2026-01-02",
        photographer: "Keep photographer",
        urls: ["https://example.org/image"],
        studio_id: artist.id,
        performer_ids: [character.id],
        tag_ids: [tag.id],
        gallery_ids: [gallery.id],
        custom_fields: {
          full: { note: "Keep note", full: "User field named full" },
        },
      },
    }
  );
  await call(
    page,
    url,
    "mutation($id: ID!, $ids: [ID!]!) {imageArtistsUpdate(image_id: $id, artist_ids: $ids) {id}}",
    { id, ids: [artist.id] }
  );
  await call(
    page,
    url,
    "mutation($id: ID!, $ids: [ID!]!) {imageCopyrightsUpdate(image_id: $id, copyright_ids: $ids) {id}}",
    { id, ids: [copyright.id] }
  );
  const snapshot = async () =>
    (
      await call(
        page,
        url,
        "query($id: ID!) {findImage(id: $id) {id title code details date photographer urls studio {id} artists {id} copyrights {id} performers {id} tags {id} galleries {id} visual_files {... on ImageFile {id}} custom_fields}}",
        { id }
      )
    ).findImage;
  const original = await snapshot();
  assert.equal(original.visual_files.length, 1);
  let note = "Keep note";
  const expected = (title) => ({
    ...original,
    title,
    custom_fields: { ...original.custom_fields, note },
  });
  const editor = () => page.locator("#image-edit-details:visible");
  const title = () => editor().locator('input[name="title"]');
  const save = () =>
    editor().getByRole("button", { name: "Save", exact: true });
  async function open() {
    await page.getByRole("tab", { name: "Edit", exact: true }).click();
    await editor().waitFor();
  }
  async function expandCustomFields() {
    const toggle = editor().locator(".custom-fields-input .collapse-button");
    // Center lower controls explicitly so the fixed mobile navbar does not
    // intercept Playwright's minimal automatic scroll. Keep a real click.
    await toggle.evaluate((element) =>
      element.scrollIntoView({ block: "center" })
    );
    await toggle.click();
  }
  async function saveTitle(value, rejected = false, holdAssociations = false) {
    let release;
    const gate = new Promise((done) => {
      release = done;
    });
    const active = [];
    const hold = (route) => {
      const pending = (async () => {
        const request = route.request();
        const operation =
          request.method() === "POST"
            ? request.postDataJSON()?.operationName
            : undefined;
        if (["ImageArtistsUpdate", "ImageCopyrightsUpdate"].includes(operation))
          await gate;
        await route.continue();
      })();
      active.push(pending);
      return pending;
    };
    if (holdAssociations) await page.route("**/graphql", hold);
    await title().fill(value);
    let response;
    try {
      [response] = await Promise.all([
        page.waitForResponse(
          (r) =>
            r.request().method() === "POST" &&
            r.request().postDataJSON()?.operationName === "ImageUpdate"
        ),
        save().click(),
      ]);
      if (holdAssociations) {
        // Allow the main mutation/cache reinitialization to render before the
        // separately saved associations finish. All responses remain real.
        await page.evaluate(
          () =>
            new Promise((done) =>
              requestAnimationFrame(() => requestAnimationFrame(done))
            )
        );
      }
    } finally {
      release();
      await Promise.all(active);
      if (holdAssociations) await page.unroute("**/graphql", hold);
    }
    const payload = await response.json();
    assert.equal(!!payload.errors?.length, rejected, JSON.stringify(payload));
    if (rejected)
      assert.match(payload.errors[0].message, /invalid-image-fixture/);
    await editor().waitFor();
    assert.equal(await title().inputValue(), value);
    if (!rejected) {
      await page.waitForFunction(
        () =>
          [...document.querySelectorAll("#image-edit-details button")].find(
            (button) => button.textContent.trim() === "Save"
          )?.disabled === true
      );
      assert.deepEqual(await snapshot(), expected(value));
      // The legitimate user key "full" must retain its scalar value in the form.
      assert.equal(
        await editor().getByPlaceholder("full", { exact: true }).inputValue(),
        "User field named full"
      );
      assert.equal(
        await editor().getByPlaceholder("note", { exact: true }).inputValue(),
        note
      );
    }
  }
  await page.goto(url + "/images?perPage=40&sortby=title&sortdir=asc", {
    waitUntil: "networkidle",
  });
  const returnURL = page.url();
  // Exercise the native detail deep link; thumbnail/title clicks open the viewer.
  await page.goto(url + "/images/" + id, { waitUntil: "networkidle" });
  await open();
  await saveTitle("First save", false, true);
  await expandCustomFields();
  note = "Edited note";
  await editor().getByPlaceholder("note", { exact: true }).fill(note);
  await editor().getByPlaceholder("note", { exact: true }).blur();
  await saveTitle("Second save");
  await page.reload({ waitUntil: "networkidle" });
  await open();
  assert.equal(await title().inputValue(), "Second save");
  await saveTitle("Third save after refresh");
  const beforeRejected = await snapshot();
  await expandCustomFields();
  note = "Retry note";
  await editor().getByPlaceholder("note", { exact: true }).fill(note);
  await editor().getByPlaceholder("note", { exact: true }).blur();
  const reject = async (route) => {
    const request = route.request();
    const payload =
      request.method() === "POST" ? request.postDataJSON() : undefined;
    if (payload?.operationName === "ImageUpdate") {
      payload.variables.input.id = "invalid-image-fixture";
      await route.continue({ postData: JSON.stringify(payload) });
    } else await route.continue();
  };
  await page.route("**/graphql", reject);
  try {
    await saveTitle("Retry draft", true);
  } finally {
    await page.unroute("**/graphql", reject);
  }
  await page.locator(".toast.danger").waitFor();
  assert.deepEqual(await snapshot(), beforeRejected);
  assert.equal(await save().isEnabled(), true);
  assert.equal(
    await editor().getByPlaceholder("note", { exact: true }).inputValue(),
    note
  );
  // Retrying the retained draft must succeed without leaving or reloading.
  const [retryResponse] = await Promise.all([
    page.waitForResponse(
      (r) =>
        r.request().method() === "POST" &&
        r.request().postDataJSON()?.operationName === "ImageUpdate"
    ),
    save().click(),
  ]);
  assert.equal((await retryResponse.json()).errors, undefined);
  await editor().waitFor();
  await page.waitForFunction(
    () =>
      [...document.querySelectorAll("#image-edit-details button")].find(
        (button) => button.textContent.trim() === "Save"
      )?.disabled === true
  );
  assert.deepEqual(await snapshot(), expected("Retry draft"));
  await saveTitle("Final save");
  await page.screenshot({
    path: join(artifacts, label + "-saved.png"),
    fullPage: true,
  });
  await page.goBack({ waitUntil: "networkidle" });
  assert.equal(page.url(), returnURL);
  await page.getByText("Final save", { exact: true }).first().waitFor();
  assert.equal(
    await page.evaluate(
      () => document.documentElement.scrollWidth > innerWidth + 1
    ),
    false
  );
  assert.deepEqual(await readFile(filename), bytes);
  await page.close();
  await instance.stop();
  instance = undefined;
  url = await start("-restart");
  page = await browser.newPage({ viewport });
  page.on("pageerror", recordError);
  await page.goto(url + "/images/" + id, { waitUntil: "networkidle" });
  await open();
  assert.equal(await title().inputValue(), "Final save");
  assert.deepEqual(await snapshot(), expected("Final save"));
  assert.deepEqual(await readFile(filename), bytes);
  assert.deepEqual(errors, []);
  await page.close();
  await instance.stop();
  instance = undefined;
  console.log(
    "Image editor " +
      label +
      ": repeated saves, scalar full field, real rejection/draft retry, Back, refresh/restart, native relationships and PNG bytes passed."
  );
}
try {
  await check({ width: 1440, height: 1000 }, "desktop");
  await check({ width: 390, height: 844 }, "mobile");
  passed = true;
} catch (error) {
  const page = browser.contexts()[0]?.pages()[0];
  if (page)
    await page
      .screenshot({
        path: join(artifacts, "image-save-failure.png"),
        fullPage: true,
      })
      .catch(() => {});
  throw error;
} finally {
  await browser.close();
  await instance?.stop();
  if (passed && process.env.STASH_BROWSER_SCREENSHOTS)
    await rm(root, { recursive: true, force: true });
  else console.log("Isolated Image editor fixture: " + root);
}
