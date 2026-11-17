import assert from "node:assert/strict";
import { startIsolatedServer } from "./helpers/isolated-server.mjs";
import { mkdtemp, mkdir, writeFile, rm } from "node:fs/promises";
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
async function imageCardPaths(page) {
  const links = page.locator(".image-list a.image-card-link");
  await links.first().waitFor();
  return links.evaluateAll((cards) => cards.map((card) => card.pathname));
}
async function checkImageStackBrowse(page, url, images, representative) {
  await page.goto(url + "/images", { waitUntil: "networkidle" });
  const representativePath = `/images/${representative}`;
  assert.deepEqual(await imageCardPaths(page), [representativePath]);

  const list = page.locator(".image-list");
  const viewOptions = list.getByRole("button", {
    name: "View options",
    exact: true,
  });
  const variants = list.getByRole("menuitemcheckbox", {
    name: "Show individual variants",
    exact: true,
  });
  await viewOptions.click();
  assert.equal(await variants.getAttribute("aria-checked"), "false");
  await variants.click();
  await page.waitForURL(
    (location) => location.searchParams.get("variants") === "true"
  );
  const variantPath = `/images/${images[0].id}`;
  const variantLink = list.locator(`a.image-card-link[href="${variantPath}"]`);
  await variantLink.waitFor();
  const paths = images.map(({ id }) => `/images/${id}`);
  assert.deepEqual(await imageCardPaths(page), paths);

  await page.reload({ waitUntil: "networkidle" });
  await variantLink.waitFor();
  assert.deepEqual(await imageCardPaths(page), paths);

  const card = list.locator(".image-card").filter({
    has: page.locator(`a.image-card-link[href="${variantPath}"]`),
  });
  await card.hover();
  await card.locator("input.card-check").check();
  await viewOptions.click();
  assert.equal(await variants.getAttribute("aria-checked"), "true");
  await variants.click();
  await page.waitForURL((location) => !location.searchParams.has("variants"));
  const notice = list.locator(".image-hidden-selection");
  await notice.waitFor();
  assert.match(
    await notice.innerText(),
    /1 selected Image is not on this page/
  );
  assert.deepEqual(await imageCardPaths(page), [representativePath]);
  await notice
    .getByRole("button", { name: "Select None", exact: true })
    .click();
  await notice.waitFor({ state: "hidden" });

  // Only the representative matches the grouped list, so both Random
  // entry points must open it even though it is last in the native sort.
  const groupedURL = page.url();
  await list.locator("#more-menu").click();
  await page
    .locator(".image-list-operations-dropdown")
    .getByText("View Random", { exact: true })
    .click();
  await page.waitForURL(url + representativePath);
  await page.goBack();
  await page.waitForURL(groupedURL);
  await viewOptions.waitFor();
  assert.deepEqual(await imageCardPaths(page), [representativePath]);
  await page.keyboard.press("p");
  await page.keyboard.press("r");
  await page.waitForURL(url + representativePath);
}
async function check(viewport, label) {
  const directory = join(root, label);
  const media = join(directory, "media");
  await mkdir(media, { recursive: true });
  const page = await browser.newPage({ viewport });
  const errors = [];
  page.on("pageerror", (error) => errors.push(String(error)));
  for (const [index, color] of ["#243550", "#57c4c8", "#e0a27f"].entries()) {
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
        '{ findImages(filter: {per_page: -1, sort: "path", direction: ASC}) { images { id } } }'
      )
    ).findImages.images;
    if (images.length === 3) break;
    await new Promise((done) => setTimeout(done, 250));
  }
  assert.equal(images.length, 3, "Scan must index all three generated PNGs");
  const [first, second, outside] = images;
  const inGallery = [first.id, second.id];
  await page.goto(url + "/images", { waitUntil: "networkidle" });
  const list = page.locator(".image-list");
  const selected = list.locator("input.card-check:checked");
  for (const { id } of images) {
    const card = list.locator(".image-card").filter({
      has: page.locator(`a.image-card-link[href="/images/${id}"]`),
    });
    await card.hover();
    await card.locator("input.card-check").check();
  }
  const dialog = page.getByRole("dialog");
  const createFromSelection = async () => {
    await list.locator("#more-menu").click();
    await page
      .locator(".image-list-operations-dropdown")
      .getByText("Create stack from selection…", { exact: true })
      .click();
    await dialog
      .getByRole("radio", { name: `Representative image:${outside.id}` })
      .waitFor();
  };
  await createFromSelection();
  await dialog.getByRole("button", { name: "Cancel", exact: true }).click();
  await dialog.waitFor({ state: "hidden" });
  assert.equal(await selected.count(), images.length);
  await createFromSelection();
  await dialog
    .getByRole("radio", { name: `Representative image:${outside.id}` })
    .check();
  await dialog
    .getByRole("button", { name: "Create stack", exact: true })
    .click();
  await dialog.waitFor({ state: "hidden" });
  assert.equal(await selected.count(), 0);
  assert.equal(await list.locator(".image-hidden-selection").count(), 0);
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
    { gallery: gallery.id, images: inGallery }
  );
  const snapshot = async () =>
    await call(
      page,
      url,
      "query($id: ID!) { findGallery(id: $id) {id title image_count chapters {id title image_index} } findImages(filter: {per_page: -1}) {images {id title galleries {id} } } }",
      { id: gallery.id }
    );
  const before = await snapshot();
  await checkImageStackBrowse(page, url, images, outside.id);
  await page.goto(url + "/galleries/" + gallery.id, {
    waitUntil: "networkidle",
  });
  assert.deepEqual(
    await imageCardPaths(page),
    inGallery.map((id) => `/images/${id}`),
    "The gallery must show its own Images in path order"
  );
  assert.equal(
    await page
      .getByRole("button", { name: "View options", exact: true })
      .count(),
    0
  );
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
      ": Images variants and Random, gallery membership order, chapter rejection/retry and reload passed."
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
