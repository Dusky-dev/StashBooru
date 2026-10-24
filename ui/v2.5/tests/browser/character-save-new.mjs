import assert from "node:assert/strict";
import { mkdtemp, mkdir, writeFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { startIsolatedServer } from "./helpers/isolated-server.mjs";

// Own every database, config and generated portrait. Never accept an existing
// library URL. Rejections modify an outgoing fixture ID, not server responses.
assert.ok(process.env.STASH_BROWSER_BINARY, "Set STASH_BROWSER_BINARY");
const binary = resolve(process.env.STASH_BROWSER_BINARY);
const ui = process.env.STASH_BROWSER_UI;
const { chromium } = await import(
  process.env.PLAYWRIGHT_MODULE ?? "playwright"
);
const root = await mkdtemp(join(tmpdir(), "stashbooru-character-save-"));
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
  const data = await (
    await page.request.post(url + "/graphql", { data: { query, variables } })
  ).json();
  assert.equal(data.errors, undefined, JSON.stringify(data.errors));
  return data.data;
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
    "mutation {configureUI(input: {lastNoteSeen: 99999999})}"
  );
  const copyright = (
    await call(
      page,
      url,
      'mutation {copyrightCreate(input: {name: "Fixture Copyright"}) {id name}}'
    )
  ).copyrightCreate;
  const artist = (
    await call(
      page,
      url,
      'mutation {studioCreate(input: {name: "Fixture Artist"}) {id name}}'
    )
  ).studioCreate;
  const contextKind = label === "desktop" ? "copyright" : "artist";
  const contextTarget = contextKind === "copyright" ? copyright : artist;
  const tag = (
    await call(
      page,
      url,
      'mutation {tagCreate(input: {name: "Fixture tag"}) {id name}}'
    )
  ).tagCreate;
  const all = async () =>
    (
      await call(
        page,
        url,
        '{findPerformers(filter: {per_page: -1, sort: "id", direction: ASC}) {performers {id name disambiguation details alias_list tags {id} custom_fields disambiguation_context {copyright {id} artist {id}}}}}'
      )
    ).findPerformers.performers;
  let writes = 0;
  const inputs = [];
  page.on("request", (request) => {
    const data =
      request.method() === "POST" ? request.postDataJSON() : undefined;
    if (data?.operationName === "PerformerCreate") {
      writes++;
      inputs.push(data.variables.input);
    }
  });
  const form = () => page.locator("#performer-edit");
  const name = () => form().locator('input[name="name"]');
  const actions = () => page.locator(".details-edit").first();
  async function saveNew() {
    await actions().locator("button.dropdown-toggle-split").click();
    await page
      .locator(".dropdown-menu.show")
      .getByText("Save & New", { exact: true })
      .click();
  }
  async function action(operation, click, rejected = false) {
    const [response] = await Promise.all([
      page.waitForResponse(
        (r) =>
          r.request().method() === "POST" &&
          r.request().postDataJSON()?.operationName === operation
      ),
      click(),
    ]);
    const payload = await response.json();
    assert.equal(!!payload.errors?.length, rejected, JSON.stringify(payload));
    if (rejected)
      assert.match(payload.errors[0].message, /invalid-character-fixture/);
    return payload.data;
  }
  await page.goto(url + "/performers/new", { waitUntil: "networkidle" });
  await saveNew();
  await page
    .waitForFunction(
      () =>
        document
          .querySelector('#performer-edit input[name="name"]')
          ?.classList.contains("is-invalid"),
      undefined,
      { timeout: 5000 }
    )
    .catch(() => {});
  assert.equal(
    writes,
    0,
    "Save & New must validate a blank name before sending a create mutation"
  );
  assert.equal(
    await name().evaluate((input) => input.classList.contains("is-invalid")),
    true
  );
  assert.deepEqual(await all(), []);

  await name().fill("First Character");
  await form()
    .locator('textarea[name="details"]')
    .fill("Keep the draft details");
  await form()
    .locator('[data-field="alias_list"] input')
    .first()
    .fill("Fixture alias");
  const png = await page.evaluate(() => {
    const canvas = document.createElement("canvas");
    canvas.width = 64;
    canvas.height = 96;
    const c = canvas.getContext("2d");
    c.fillStyle = "#243550";
    c.fillRect(0, 0, 64, 96);
    c.fillStyle = "#57c4c8";
    c.fillRect(12, 16, 24, 48);
    return canvas.toDataURL("image/png").split(",")[1];
  });
  const portrait = join(directory, "portrait.png");
  await writeFile(portrait, Buffer.from(png, "base64"));
  await actions()
    .getByRole("button", { name: /^Set image/ })
    .click();
  await page
    .locator('#set-image-popover input[type="file"]')
    .setInputFiles(portrait);
  await page.locator(".performer-image-container img").waitFor();
  await page.locator("#disambiguation-context-type").selectOption(contextKind);
  const context = form()
    .locator(".form-group")
    .filter({ has: page.locator("#disambiguation-context-type") });
  await context.locator('input[role="combobox"]').fill(contextTarget.name);
  await page
    .getByRole("option")
    .filter({ hasText: contextTarget.name })
    .click();
  await form()
    .locator('[data-field="tag_ids"] input[role="combobox"]')
    .fill(tag.name);
  await page.getByRole("option").filter({ hasText: tag.name }).click();
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
  const reject = async (route) => {
    const request = route.request();
    const data =
      request.method() === "POST" ? request.postDataJSON() : undefined;
    if (data?.operationName === "PerformerCreate") {
      data.variables.input.tag_ids = ["invalid-character-fixture"];
      await route.continue({ postData: JSON.stringify(data) });
    } else await route.continue();
  };
  await page.route("**/graphql", reject);
  try {
    await action("PerformerCreate", saveNew, true);
  } finally {
    await page.unroute("**/graphql", reject);
  }
  await form().waitFor();
  assert.equal(await name().inputValue(), "First Character");
  assert.equal(
    await form().locator('textarea[name="details"]').inputValue(),
    "Keep the draft details"
  );
  assert.equal(
    await form().getByPlaceholder("full", { exact: true }).inputValue(),
    "User field named full"
  );
  assert.equal(
    await page.locator("#disambiguation-context-type").inputValue(),
    contextKind
  );
  assert.deepEqual(await all(), []);
  const first = (await action("PerformerCreate", saveNew)).performerCreate;
  await form().waitFor();
  await page.waitForFunction(
    () =>
      document.querySelector('#performer-edit input[name="name"]')?.value === ""
  );
  assert.equal(new URL(page.url()).pathname, "/performers/new");
  assert.equal(
    await form().locator('textarea[name="details"]').inputValue(),
    ""
  );
  assert.equal(
    await page.locator("#disambiguation-context-type").inputValue(),
    "custom"
  );
  assert.equal(
    await form().locator(".custom-fields-row:not(.custom-fields-new)").count(),
    0
  );
  await page
    .locator(".performer-image-container img")
    .waitFor({ state: "detached" });
  const records = await all();
  assert.equal(records.length, 1);
  assert.equal(records[0].id, first.id);
  assert.equal(records[0].name, "First Character");
  assert.equal(records[0].details, "Keep the draft details");
  assert.deepEqual(records[0].alias_list, ["Fixture alias"]);
  assert.deepEqual(records[0].tags, [{ id: tag.id }]);
  assert.deepEqual(records[0].custom_fields, { full: "User field named full" });
  assert.deepEqual(records[0].disambiguation_context, {
    copyright: contextKind === "copyright" ? { id: copyright.id } : null,
    artist: contextKind === "artist" ? { id: artist.id } : null,
  });

  const beforeBlank = writes;
  await saveNew();
  await page.waitForFunction(() =>
    document
      .querySelector('#performer-edit input[name="name"]')
      ?.classList.contains("is-invalid")
  );
  assert.equal(
    writes,
    beforeBlank,
    "The fresh next form must also validate before writing"
  );
  assert.deepEqual(await all(), records);
  const portraitBytes = async () => {
    const data = await call(
      page,
      url,
      "query($id: ID!) {findPerformer(id: $id) {image_path}}",
      { id: first.id }
    );
    const response = await page.request.get(data.findPerformer.image_path);
    assert.equal(response.ok(), true);
    assert.match(response.headers()["content-type"], /^image\//);
    return response.body();
  };
  const savedPortrait = await portraitBytes();
  assert.ok(savedPortrait.length > 0);

  await name().fill("Second Character");
  const second = (
    await action("PerformerCreate", () =>
      actions().getByRole("button", { name: "Save", exact: true }).click()
    )
  ).performerCreate;
  await page.waitForURL(
    (location) =>
      location.pathname === "/performers/" + second.id ||
      location.pathname.startsWith("/performers/" + second.id + "/")
  );
  assert.equal(inputs.at(-1).details, "");
  assert.deepEqual(inputs.at(-1).tag_ids, []);
  assert.deepEqual(inputs.at(-1).custom_fields, {});
  assert.equal(inputs.at(-1).image, undefined);
  const saved = await all();
  assert.equal(saved.length, 2);
  assert.deepEqual(
    saved.find((item) => item.id === first.id),
    records[0]
  );
  const next = saved.find((item) => item.id === second.id);
  assert.equal(next.name, "Second Character");
  assert.deepEqual(next.alias_list, []);
  assert.deepEqual(next.tags, []);
  assert.equal(next.disambiguation_context, null);
  await page.goto(url + "/performers?perPage=40&sortby=name&sortdir=asc", {
    waitUntil: "networkidle",
  });
  const listURL = page.url();
  await page.goto(url + "/performers/" + first.id, {
    waitUntil: "networkidle",
  });
  await page.locator("#performer-page button.edit").first().click();
  await name().fill("Updated Character");
  const rejectUpdate = async (route) => {
    const request = route.request();
    const data =
      request.method() === "POST" ? request.postDataJSON() : undefined;
    if (data?.operationName === "PerformerUpdate") {
      data.variables.input.id = "invalid-character-fixture";
      await route.continue({ postData: JSON.stringify(data) });
    } else await route.continue();
  };
  await page.route("**/graphql", rejectUpdate);
  try {
    await action(
      "PerformerUpdate",
      () =>
        actions().getByRole("button", { name: "Save", exact: true }).click(),
      true
    );
  } finally {
    await page.unroute("**/graphql", rejectUpdate);
  }
  await form().waitFor();
  assert.equal(await name().inputValue(), "Updated Character");
  assert.deepEqual(await all(), saved);
  await action("PerformerUpdate", () =>
    actions().getByRole("button", { name: "Save", exact: true }).click()
  );
  await form().waitFor({ state: "detached" });
  const updated = saved.map((item) =>
    item.id === first.id ? { ...item, name: "Updated Character" } : item
  );
  assert.deepEqual(await all(), updated);
  assert.deepEqual(await portraitBytes(), savedPortrait);
  await page.locator("#performer-page button.edit").first().click();
  await name().fill("Unsaved name");
  await actions().getByRole("button", { name: "Cancel", exact: true }).click();
  await form().waitFor({ state: "detached" });
  assert.deepEqual(await all(), updated);
  await page.reload({ waitUntil: "networkidle" });
  assert.deepEqual(await all(), updated);
  await page.screenshot({
    path: join(artifacts, label + "-created.png"),
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
  await page.getByText("Updated Character", { exact: true }).first().waitFor();
  await page.close();
  await instance.stop();
  instance = undefined;
  url = await start("-restart");
  page = await browser.newPage({ viewport });
  page.on("pageerror", recordError);
  await page.goto(url + "/performers/" + first.id, {
    waitUntil: "networkidle",
  });
  assert.deepEqual(await all(), updated);
  assert.deepEqual(await portraitBytes(), savedPortrait);
  assert.deepEqual(errors, []);
  await page.close();
  await instance.stop();
  instance = undefined;
  console.log(
    "Character Save & New " +
      label +
      ": required-name validation, actual rejection/draft retry, separate records, clean next form, refresh/restart and native context/tag/custom fields passed."
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
        path: join(artifacts, "character-save-failure.png"),
        fullPage: true,
      })
      .catch(() => {});
  throw error;
} finally {
  await browser.close();
  await instance?.stop();
  if (passed && process.env.STASH_BROWSER_SCREENSHOTS)
    await rm(root, { recursive: true, force: true });
  else console.log("Isolated Character fixture: " + root);
}
