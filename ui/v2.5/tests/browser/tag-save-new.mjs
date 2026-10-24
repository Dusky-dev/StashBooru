import assert from "node:assert/strict";
import { mkdtemp, mkdir, writeFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { startIsolatedServer } from "./helpers/isolated-server.mjs";

// Own every server, database and artwork file. Actual duplicate names and a
// descendant parent exercise rejection; no requests/responses are intercepted.
assert.ok(process.env.STASH_BROWSER_BINARY, "Set STASH_BROWSER_BINARY");
const binary = resolve(process.env.STASH_BROWSER_BINARY);
const ui = process.env.STASH_BROWSER_UI;
const { chromium } = await import(
  process.env.PLAYWRIGHT_MODULE ?? "playwright"
);
const root = await mkdtemp(join(tmpdir(), "stashbooru-tag-save-"));
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
  for (const folder of ["media", "generated", "cache", "metadata"])
    await mkdir(join(directory, folder), { recursive: true });
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
        "mutation($input:TagCreateInput!){tagCreate(input:$input){id name}}",
        { input }
      )
    ).tagCreate;
  const parent = await create({ name: "Root Tag" });
  const secondParent = await create({ name: "Other Root Tag" });
  const child = await create({ name: "Child Tag" });
  const descendant = await create({
    name: "Descendant Tag",
    parent_ids: [child.id],
  });
  const copyright = (
    await call(
      page,
      url,
      'mutation {copyrightCreate(input:{name:"Root Tag",aliases:["Keep copyright alias"]}){id}}'
    )
  ).copyrightCreate;
  await call(
    page,
    url,
    "mutation($id:ID!,$tags:[ID!]!){copyrightTagsUpdate(copyright_id:$id,tag_ids:$tags){id}}",
    { id: copyright.id, tags: [parent.id] }
  );
  const video = (
    await call(
      page,
      url,
      'mutation($tags:[ID!]!){sceneCreate(input:{title:"Keep native Video",tag_ids:$tags}){id}}',
      { tags: [child.id] }
    )
  ).sceneCreate;
  const unchanged = async () =>
    await call(
      page,
      url,
      "query($copyright:ID!,$video:ID!){findCopyright(id:$copyright){id name aliases parents{id} children{id} tags{id}} findScene(id:$video){id title tags{id} files{id}}}",
      { copyright: copyright.id, video: video.id }
    );
  const originalOthers = await unchanged();
  const all = async () =>
    (
      await call(
        page,
        url,
        '{findTags(filter:{per_page:-1,sort:"id",direction:ASC}){tags{id name sort_name description aliases parents{id} children{id} custom_fields}}}'
      )
    ).findTags.tags;
  const original = await all();
  const inputs = [];
  page.on("request", (request) => {
    const data =
      request.method() === "POST" ? request.postDataJSON() : undefined;
    if (data?.operationName === "TagCreate") inputs.push(data.variables.input);
  });
  const form = () => page.locator("#tag-edit");
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
    await form()
      .locator('[data-field="' + field + '"] input[role="combobox"]')
      .fill(text);
    await page
      .getByRole("option")
      .filter({ has: page.getByText(text, { exact: true }) })
      .click();
    // Native multi-selects stay open for more choices; close them before the
    // next control so a menu option cannot intercept that click on mobile.
    await form()
      .locator('[data-field="' + field + '"] input[role="combobox"]')
      .press("Escape");
  }
  await page.goto(url + "/tags/new", { waitUntil: "networkidle" });
  await saveNew();
  await page
    .waitForFunction(
      () =>
        document
          .querySelector('#tag-edit input[name="name"]')
          ?.classList.contains("is-invalid"),
      undefined,
      { timeout: 5000 }
    )
    .catch(() => {});
  assert.equal(
    inputs.length,
    0,
    "Save & New must validate a blank name before sending TagCreate"
  );
  assert.equal(
    await name().evaluate((input) => input.classList.contains("is-invalid")),
    true
  );
  assert.deepEqual(await all(), original);
  await name().fill(parent.name);
  await form().locator('input[name="sort_name"]').fill("First sort value");
  await form()
    .locator('textarea[name="description"]')
    .fill("Keep tag description");
  await form()
    .locator('[data-field="aliases"] input')
    .first()
    .fill("Fixture alias");
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
  await page.locator(".tag-details img.logo").waitFor();
  await select("parent_ids", parent.name);
  await select("parent_ids", secondParent.name);
  await select("child_ids", child.name);
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
  await action("TagCreate", saveNew, /already exists/);
  await form().waitFor();
  assert.equal(await name().inputValue(), parent.name);
  assert.equal(
    await form().locator('textarea[name="description"]').inputValue(),
    "Keep tag description"
  );
  assert.equal(
    await form().getByPlaceholder("full", { exact: true }).inputValue(),
    "User field named full"
  );
  assert.deepEqual(await all(), original);
  await name().fill("First Tag");
  const first = (await action("TagCreate", saveNew)).tagCreate;
  await form().waitFor();
  await page.waitForFunction(
    () => document.querySelector('#tag-edit input[name="name"]')?.value === ""
  );
  assert.equal(new URL(page.url()).pathname, "/tags/new");
  assert.equal(
    await form().locator('input[name="sort_name"]').inputValue(),
    ""
  );
  assert.equal(
    await form().locator('textarea[name="description"]').inputValue(),
    ""
  );
  assert.equal(
    await form().locator(".custom-fields-row:not(.custom-fields-new)").count(),
    0
  );
  await page.locator(".tag-details img.logo").waitFor({ state: "detached" });
  const records = await all();
  assert.equal(records.length, 5);
  const subject = records.find((item) => item.id === first.id);
  assert.equal(subject.name, "First Tag");
  assert.equal(subject.sort_name, "First sort value");
  assert.equal(subject.description, "Keep tag description");
  assert.deepEqual(subject.aliases, ["Fixture alias"]);
  assert.deepEqual(
    subject.parents.map((item) => item.id).sort(),
    [parent.id, secondParent.id].sort()
  );
  assert.deepEqual(subject.children, [{ id: child.id }]);
  assert.deepEqual(subject.custom_fields, { full: "User field named full" });
  assert.deepEqual(records.find((item) => item.id === parent.id).children, [
    { id: first.id },
  ]);
  assert.deepEqual(
    records.find((item) => item.id === secondParent.id).children,
    [{ id: first.id }]
  );
  assert.deepEqual(records.find((item) => item.id === child.id).parents, [
    { id: first.id },
  ]);
  assert.deepEqual(
    records.find((item) => item.id === descendant.id),
    original.find((item) => item.id === descendant.id)
  );
  const beforeBlank = inputs.length;
  await saveNew();
  await page.waitForFunction(() =>
    document
      .querySelector('#tag-edit input[name="name"]')
      ?.classList.contains("is-invalid")
  );
  assert.equal(inputs.length, beforeBlank);
  const artworkBytes = async () => {
    const data = await call(
      page,
      url,
      "query($id:ID!){findTag(id:$id){image_path}}",
      { id: first.id }
    );
    const response = await page.request.get(data.findTag.image_path);
    assert.equal(response.ok(), true);
    assert.match(response.headers()["content-type"], /^image\//);
    return response.body();
  };
  const savedArtwork = await artworkBytes();
  assert.ok(savedArtwork.length > 0);
  await name().fill("Second Tag");
  const second = (
    await action("TagCreate", () =>
      actions().getByRole("button", { name: "Save", exact: true }).click()
    )
  ).tagCreate;
  await page.waitForURL(
    (location) =>
      location.pathname === "/tags/" + second.id ||
      location.pathname.startsWith("/tags/" + second.id + "/")
  );
  assert.deepEqual(inputs.at(-1).parent_ids, []);
  assert.deepEqual(inputs.at(-1).child_ids, []);
  assert.deepEqual(inputs.at(-1).aliases, []);
  assert.equal(inputs.at(-1).sort_name, "");
  assert.equal(inputs.at(-1).description, "");
  assert.deepEqual(inputs.at(-1).custom_fields, {});
  assert.equal(inputs.at(-1).image, undefined);
  const saved = await all();
  assert.equal(saved.length, 6);
  assert.deepEqual(
    saved.filter((item) => item.id !== second.id),
    records
  );
  await page.goto(url + "/tags?perPage=40&sortby=name&sortdir=asc", {
    waitUntil: "networkidle",
  });
  const listURL = page.url();
  await page.goto(url + "/tags/" + first.id, { waitUntil: "networkidle" });
  await page.locator("#tag-page button.edit").first().click();
  await name().fill("Updated Tag");
  await select("parent_ids", descendant.name);
  await action(
    "TagUpdate",
    () => actions().getByRole("button", { name: "Save", exact: true }).click(),
    /descendant/
  );
  await form().waitFor();
  assert.equal(await name().inputValue(), "Updated Tag");
  assert.deepEqual(
    await all(),
    saved,
    "Rejected Tag cycle changed saved records"
  );
  const badParent = form()
    .locator('[data-field="parent_ids"] .react-select__multi-value')
    .filter({ hasText: descendant.name });
  await badParent.locator('[role="button"]').click();
  await action("TagUpdate", () =>
    actions().getByRole("button", { name: "Save", exact: true }).click()
  );
  await form().waitFor({ state: "detached" });
  const updated = saved.map((item) =>
    item.id === first.id ? { ...item, name: "Updated Tag" } : item
  );
  assert.deepEqual(await all(), updated);
  assert.deepEqual(await artworkBytes(), savedArtwork);
  await page.locator("#tag-page button.edit").first().click();
  await name().fill("Unsaved tag name");
  await actions().getByRole("button", { name: "Cancel", exact: true }).click();
  await form().waitFor({ state: "detached" });
  assert.deepEqual(await all(), updated);
  await page.reload({ waitUntil: "networkidle" });
  assert.deepEqual(await all(), updated);
  assert.deepEqual(await unchanged(), originalOthers);
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
  await page.getByText("Updated Tag", { exact: true }).first().waitFor();
  await page.close();
  await instance.stop();
  instance = undefined;
  url = await start("-restart");
  page = await browser.newPage({ viewport });
  page.on("pageerror", recordError);
  await page.goto(url + "/tags/" + first.id, { waitUntil: "networkidle" });
  assert.deepEqual(await all(), updated);
  assert.deepEqual(await unchanged(), originalOthers);
  assert.deepEqual(await artworkBytes(), savedArtwork);
  assert.deepEqual(errors, []);
  await page.close();
  await instance.stop();
  instance = undefined;
  console.log(
    "Tag Save & New " +
      label +
      ": required-name validation, duplicate create/draft retry, clean next form, multiple parents and rejected cycle/retry, Cancel, Back, refresh/restart, artwork and separate native Copyright/Video metadata passed."
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
        path: join(artifacts, "tag-save-failure.png"),
        fullPage: true,
      })
      .catch(() => {});
  throw error;
} finally {
  await browser.close();
  await instance?.stop();
  if (passed && process.env.STASH_BROWSER_SCREENSHOTS)
    await rm(root, { recursive: true, force: true });
  else console.log("Isolated Tag fixture: " + root);
}
