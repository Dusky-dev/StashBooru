import assert from "node:assert/strict";
import { mkdtemp, mkdir, writeFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { startIsolatedServer } from "./helpers/isolated-server.mjs";

// Own all servers, paths and square artwork. Every response is real; duplicate
// names and a descendant parent exercise native rejection without interception.
assert.ok(process.env.STASH_BROWSER_BINARY, "Set STASH_BROWSER_BINARY");
const binary = resolve(process.env.STASH_BROWSER_BINARY);
const ui = process.env.STASH_BROWSER_UI;
const { chromium } = await import(
  process.env.PLAYWRIGHT_MODULE ?? "playwright"
);
const root = await mkdtemp(join(tmpdir(), "stashbooru-artist-save-"));
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
  for (const name of ["media", "generated", "cache", "metadata"])
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
  let page = await browser.newPage({ viewport });
  const errors = [];
  const recordError = (error) => errors.push(String(error));
  page.on("pageerror", recordError);
  await call(
    page,
    url,
    "mutation {configureUI(input:{lastNoteSeen:99999999})}"
  );
  const create = async (input) =>
    (
      await call(
        page,
        url,
        "mutation($input:StudioCreateInput!) {studioCreate(input:$input) {id name}}",
        { input }
      )
    ).studioCreate;
  const parent = await create({ name: "Root Artist" });
  const child = await create({ name: "Child Artist" });
  const descendant = await create({
    name: "Descendant Artist",
    parent_id: child.id,
  });
  const tag = (
    await call(
      page,
      url,
      'mutation {tagCreate(input:{name:"Fixture tag"}) {id name}}'
    )
  ).tagCreate;
  const all = async () =>
    (
      await call(
        page,
        url,
        '{findStudios(filter:{per_page:-1,sort:"id",direction:ASC}) {studios {id name aliases urls details parent_studio {id} child_studios {id} tags {id} custom_fields}}}'
      )
    ).findStudios.studios;
  const original = await all();
  const inputs = [];
  page.on("request", (request) => {
    const data =
      request.method() === "POST" ? request.postDataJSON() : undefined;
    if (data?.operationName === "StudioCreate")
      inputs.push(data.variables.input);
  });
  const form = () => page.locator("#studio-edit");
  const name = () => form().locator('input[name="name"]');
  const actions = () => page.locator(".details-edit").first();
  async function saveNew() {
    await actions().locator("button.dropdown-toggle-split").click();
    await page
      .locator(".dropdown-menu.show")
      .getByText("Save & New", { exact: true })
      .click();
  }
  async function action(operation, click, errorPattern) {
    const [response] = await Promise.all([
      page.waitForResponse(
        (r) =>
          r.request().method() === "POST" &&
          r.request().postDataJSON()?.operationName === operation
      ),
      click(),
    ]);
    const payload = await response.json();
    assert.equal(
      !!payload.errors?.length,
      !!errorPattern,
      JSON.stringify(payload)
    );
    if (errorPattern) assert.match(payload.errors[0].message, errorPattern);
    return payload.data;
  }
  async function select(field, text) {
    const input = form().locator(
      '[data-field="' + field + '"] input[role="combobox"]'
    );
    await input.fill(text);
    await page.getByRole("option").filter({ hasText: text }).click();
  }
  await page.goto(url + "/studios/new", { waitUntil: "networkidle" });
  await saveNew();
  await page
    .waitForFunction(
      () =>
        document
          .querySelector('#studio-edit input[name="name"]')
          ?.classList.contains("is-invalid"),
      undefined,
      { timeout: 5000 }
    )
    .catch(() => {});
  assert.equal(
    inputs.length,
    0,
    "Save & New must validate a blank name before sending StudioCreate"
  );
  assert.equal(
    await name().evaluate((input) => input.classList.contains("is-invalid")),
    true
  );
  assert.deepEqual(await all(), original);

  await name().fill(parent.name);
  await form().locator('textarea[name="details"]').fill("Keep artist details");
  await form()
    .locator('[data-field="aliases"] input')
    .first()
    .fill("Fixture alias");
  await form()
    .locator('[data-field="urls"] input')
    .first()
    .fill("https://example.org/artist");
  const png = await page.evaluate(() => {
    const canvas = document.createElement("canvas");
    canvas.width = 96;
    canvas.height = 96;
    const c = canvas.getContext("2d");
    c.fillStyle = "#243550";
    c.fillRect(0, 0, 96, 96);
    c.fillStyle = "#57c4c8";
    c.fillRect(20, 20, 40, 40);
    return canvas.toDataURL("image/png").split(",")[1];
  });
  const artwork = join(directory, "artwork.png");
  await writeFile(artwork, Buffer.from(png, "base64"));
  await actions()
    .getByRole("button", { name: /^Set image/ })
    .click();
  await page
    .locator('#set-image-popover input[type="file"]')
    .setInputFiles(artwork);
  await page.locator(".studio-details img.logo").waitFor();
  await select("parent_id", parent.name);
  await select("child_ids", child.name);
  await select("tag_ids", tag.name);
  await form()
    .getByRole("button", { name: "Custom Fields", exact: true })
    .click();
  await form()
    .locator(".custom-fields-new .custom-fields-field input")
    .fill("full");
  await form().locator(".custom-fields-new .custom-fields-field input").blur();
  await form()
    .locator(".custom-fields-new .custom-fields-value input")
    .fill("User field named full");
  await form().locator(".custom-fields-new .custom-fields-value input").blur();
  await action("StudioCreate", saveNew, /already exists/);
  await form().waitFor();
  assert.equal(await name().inputValue(), parent.name);
  assert.equal(
    await form().locator('textarea[name="details"]').inputValue(),
    "Keep artist details"
  );
  assert.equal(
    await form().getByPlaceholder("full", { exact: true }).inputValue(),
    "User field named full"
  );
  assert.deepEqual(
    await all(),
    original,
    "Rejected create changed the original hierarchy"
  );
  await name().fill("First Artist");
  const first = (await action("StudioCreate", saveNew)).studioCreate;
  await form().waitFor();
  await page.waitForFunction(
    () =>
      document.querySelector('#studio-edit input[name="name"]')?.value === ""
  );
  assert.equal(new URL(page.url()).pathname, "/studios/new");
  assert.equal(
    await form().locator('textarea[name="details"]').inputValue(),
    ""
  );
  assert.equal(
    await form().locator(".custom-fields-row:not(.custom-fields-new)").count(),
    0
  );
  await page.locator(".studio-details img.logo").waitFor({ state: "detached" });
  const records = await all();
  assert.equal(records.length, 4);
  const firstRecord = records.find((item) => item.id === first.id);
  assert.equal(firstRecord.name, "First Artist");
  assert.equal(firstRecord.details, "Keep artist details");
  assert.deepEqual(firstRecord.aliases, ["Fixture alias"]);
  assert.deepEqual(firstRecord.urls, ["https://example.org/artist"]);
  assert.deepEqual(firstRecord.parent_studio, { id: parent.id });
  assert.deepEqual(firstRecord.child_studios, [{ id: child.id }]);
  assert.deepEqual(firstRecord.tags, [{ id: tag.id }]);
  assert.deepEqual(firstRecord.custom_fields, {
    full: "User field named full",
  });
  assert.deepEqual(
    records.find((item) => item.id === parent.id).child_studios,
    [{ id: first.id }]
  );
  assert.deepEqual(records.find((item) => item.id === child.id).parent_studio, {
    id: first.id,
  });
  assert.deepEqual(
    records.find((item) => item.id === descendant.id),
    original.find((item) => item.id === descendant.id)
  );
  const beforeBlank = inputs.length;
  await saveNew();
  await page.waitForFunction(() =>
    document
      .querySelector('#studio-edit input[name="name"]')
      ?.classList.contains("is-invalid")
  );
  assert.equal(inputs.length, beforeBlank);
  const artworkBytes = async () => {
    const data = await call(
      page,
      url,
      "query($id:ID!){findStudio(id:$id){image_path}}",
      { id: first.id }
    );
    const response = await page.request.get(data.findStudio.image_path);
    assert.equal(response.ok(), true);
    assert.match(response.headers()["content-type"], /^image\//);
    return response.body();
  };
  const savedArtwork = await artworkBytes();
  assert.ok(savedArtwork.length > 0);
  await name().fill("Second Artist");
  const second = (
    await action("StudioCreate", () =>
      actions().getByRole("button", { name: "Save", exact: true }).click()
    )
  ).studioCreate;
  await page.waitForURL(
    (location) =>
      location.pathname === "/studios/" + second.id ||
      location.pathname.startsWith("/studios/" + second.id + "/")
  );
  assert.equal(inputs.at(-1).parent_id, null);
  assert.deepEqual(inputs.at(-1).child_ids, []);
  assert.deepEqual(inputs.at(-1).tag_ids, []);
  assert.deepEqual(inputs.at(-1).aliases, []);
  assert.deepEqual(inputs.at(-1).urls, []);
  assert.deepEqual(inputs.at(-1).custom_fields, {});
  assert.equal(inputs.at(-1).image, undefined);
  const saved = await all();
  assert.equal(saved.length, 5);
  assert.deepEqual(
    saved.filter((item) => item.id !== second.id),
    records
  );
  assert.equal(saved.find((item) => item.id === second.id).parent_studio, null);

  await page.goto(url + "/studios?perPage=40&sortby=name&sortdir=asc", {
    waitUntil: "networkidle",
  });
  const listURL = page.url();
  await page.goto(url + "/studios/" + first.id, { waitUntil: "networkidle" });
  await page.locator("#studio-page button.edit").first().click();
  await name().fill("Updated Artist");
  await select("parent_id", descendant.name);
  await action(
    "StudioUpdate",
    () => actions().getByRole("button", { name: "Save", exact: true }).click(),
    /ancestor of itself/
  );
  await form().waitFor();
  assert.equal(await name().inputValue(), "Updated Artist");
  assert.deepEqual(
    await all(),
    saved,
    "Rejected cycle must roll back the whole Artist update"
  );
  await select("parent_id", parent.name);
  await action("StudioUpdate", () =>
    actions().getByRole("button", { name: "Save", exact: true }).click()
  );
  await form().waitFor({ state: "detached" });
  const updated = saved.map((item) =>
    item.id === first.id ? { ...item, name: "Updated Artist" } : item
  );
  assert.deepEqual(await all(), updated);
  assert.deepEqual(await artworkBytes(), savedArtwork);
  await page.locator("#studio-page button.edit").first().click();
  await name().fill("Unsaved artist name");
  await actions().getByRole("button", { name: "Cancel", exact: true }).click();
  await form().waitFor({ state: "detached" });
  assert.deepEqual(await all(), updated);
  await page.reload({ waitUntil: "networkidle" });
  assert.deepEqual(await all(), updated);
  await page.screenshot({
    path: join(artifacts, label + "-saved.png"),
    fullPage: true,
  });
  assert.equal(
    await page.evaluate(
      () => document.documentElement.scrollWidth > innerWidth + 1
    ),
    false
  );
  await page.goBack({ waitUntil: "networkidle" });
  assert.equal(page.url(), listURL);
  await page.getByText("Updated Artist", { exact: true }).first().waitFor();
  await page.close();
  await instance.stop();
  instance = undefined;
  url = await start("-restart");
  page = await browser.newPage({ viewport });
  page.on("pageerror", recordError);
  await page.goto(url + "/studios/" + first.id, { waitUntil: "networkidle" });
  assert.deepEqual(await all(), updated);
  assert.deepEqual(await artworkBytes(), savedArtwork);
  assert.deepEqual(errors, []);
  await page.close();
  await instance.stop();
  instance = undefined;
  console.log(
    "Artist Save & New " +
      label +
      ": required-name validation, duplicate create/draft retry, clean next form, rejected hierarchy cycle/retry, Cancel, Back, refresh/restart and native metadata/artwork passed."
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
        path: join(artifacts, "artist-save-failure.png"),
        fullPage: true,
      })
      .catch(() => {});
  throw error;
} finally {
  await browser.close();
  await instance?.stop();
  if (passed && process.env.STASH_BROWSER_SCREENSHOTS)
    await rm(root, { recursive: true, force: true });
  else console.log("Isolated Artist fixture: " + root);
}
