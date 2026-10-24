import assert from "node:assert/strict";
import { mkdtemp, mkdir, writeFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { startIsolatedServer } from "./helpers/isolated-server.mjs";

// This suite owns every configuration/database and server it writes to. All
// responses are real; failure injection never fabricates GraphQL response data.
assert.ok(process.env.STASH_BROWSER_BINARY, "Set STASH_BROWSER_BINARY");
const binary = resolve(process.env.STASH_BROWSER_BINARY);
const ui = process.env.STASH_BROWSER_UI;
const { chromium } = await import(
  process.env.PLAYWRIGHT_MODULE ?? "playwright"
);
const root = await mkdtemp(join(tmpdir(), "stashbooru-collections-"));
const artifacts =
  process.env.STASH_BROWSER_SCREENSHOTS ?? join(root, "artifacts");
await mkdir(artifacts, { recursive: true });
const browser = await chromium.launch({
  executablePath: process.env.CHROMIUM_EXECUTABLE,
  args: ["--no-sandbox", "--disable-dev-shm-usage"],
});
let instance;
let successful = false;
async function call(page, url, query, variables = {}) {
  const response = await page.request.post(url + "/graphql", {
    data: { query, variables },
  });
  const payload = await response.json();
  assert.equal(payload.errors, undefined, JSON.stringify(payload.errors));
  return payload.data;
}
async function mutation(page, operation, click, rejected = false) {
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
  return payload;
}
async function check(viewport, label) {
  const directory = join(root, label);
  const media = join(directory, "media");
  await mkdir(media, { recursive: true });
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
  instance = await startIsolatedServer({
    binary,
    directory,
    artifacts,
    label,
    ui,
  });
  const { url } = instance;
  const page = await browser.newPage({ viewport });
  const errors = [];
  page.on("pageerror", (error) => errors.push(String(error)));
  await call(
    page,
    url,
    "mutation { configureUI(input: {lastNoteSeen: 99999999}) }"
  );
  const artist = (
    await call(
      page,
      url,
      'mutation { studioCreate(input: {name: "Fixture artist"}) {id} }'
    )
  ).studioCreate;
  const tag = (
    await call(
      page,
      url,
      'mutation { tagCreate(input: {name: "Fixture tag"}) {id} }'
    )
  ).tagCreate;
  const create = async (name, extra = {}) =>
    (
      await call(
        page,
        url,
        "mutation($input: GroupCreateInput!) {groupCreate(input: $input) {id name}}",
        { input: { name, ...extra } }
      )
    ).groupCreate;
  const a = await create("Part A");
  const b = await create("Part B");
  const c = await create("Part C", {
    synopsis: "Keep this native metadata",
    urls: ["https://example.org/collection"],
    studio_id: artist.id,
    tag_ids: [tag.id],
  });
  const parent = await create("Acceptance collection", {
    sub_groups: [
      { group_id: a.id, description: "Opening" },
      { group_id: b.id, description: "Ending" },
    ],
  });
  const other = await create("Other collection", {
    sub_groups: [{ group_id: c.id, description: "Other placement" }],
  });
  const group = async (id) =>
    (
      await call(
        page,
        url,
        "query($id: ID!) {findGroup(id: $id) {id name synopsis urls studio {id} tags {id} containing_groups {group {id} description} sub_groups {group {id} description} } }",
        { id }
      )
    ).findGroup;
  const originalC = await group(c.id);
  const originalOther = await group(other.id);
  const children = async () => (await group(parent.id)).sub_groups;
  const childIDs = async () => (await children()).map((item) => item.group.id);
  const cards = page.locator(".group-card:visible");
  const card = (id) =>
    cards.filter({ has: page.locator('a[href="/groups/' + id + '"]') });
  const order = () =>
    cards.evaluateAll((items) =>
      items.map(
        (item) =>
          item
            .querySelector('a[href^="/groups/"]')
            .getAttribute("href")
            .split("/")[2]
      )
    );
  async function expectOrder(expected) {
    await page.waitForFunction((ids) => {
      const actual = [...document.querySelectorAll(".group-card")].map(
        (item) =>
          item
            .querySelector('a[href^="/groups/"]')
            .getAttribute("href")
            .split("/")[2]
      );
      return JSON.stringify(actual) === JSON.stringify(ids);
    }, expected);
    assert.deepEqual(await order(), expected);
    assert.deepEqual(await childIDs(), expected);
    await page
      .getByRole("tab", { name: /Sub-Groups/ })
      .locator('.badge[data-value="' + expected.length + '"]')
      .waitFor({ timeout: 5000 });
  }
  await page.goto(
    url +
      "/groups/" +
      parent.id +
      "/subgroups?sortby=sub_group_order&sortdir=asc&perPage=40",
    { waitUntil: "networkidle" }
  );
  await expectOrder([a.id, b.id]);
  const add = page.getByTitle("Add Sub-Groups", { exact: true });
  await add.click();
  const dialog = page.getByRole("dialog");
  const select = dialog.getByRole("combobox");
  await select.fill("Part C");
  await page.getByRole("option").filter({ hasText: "Part C" }).click();
  const description = dialog.locator("input.text-input");
  await description.fill("Middle placement");
  // A concurrent edit makes the chosen child an ancestor after the dialog was
  // opened. The real backend must reject the now-cyclic draft, then allow retry.
  await call(
    page,
    url,
    "mutation($input: GroupUpdateInput!) {groupUpdate(input: $input) {id}}",
    { input: { id: c.id, sub_groups: [{ group_id: parent.id }] } }
  );
  await mutation(
    page,
    "AddGroupSubGroups",
    () => dialog.getByRole("button", { name: "Add", exact: true }).click(),
    true
  );
  await page.locator(".toast.danger").waitFor();
  assert.equal(await dialog.isVisible(), true);
  assert.equal(await description.inputValue(), "Middle placement");
  assert.deepEqual(await childIDs(), [a.id, b.id]);
  await call(
    page,
    url,
    "mutation($input: GroupUpdateInput!) {groupUpdate(input: $input) {id}}",
    { input: { id: c.id, sub_groups: [] } }
  );
  let release;
  const gate = new Promise((done) => {
    release = done;
  });
  const holdAdd = async (route) => {
    const request = route.request();
    if (
      request.method() === "POST" &&
      request.postDataJSON()?.operationName === "AddGroupSubGroups"
    )
      await gate;
    await route.continue();
  };
  await page.route("**/graphql", holdAdd);
  try {
    const request = page.waitForRequest(
      (r) =>
        r.method() === "POST" &&
        r.postDataJSON()?.operationName === "AddGroupSubGroups"
    );
    await Promise.all([
      request,
      dialog.getByRole("button", { name: "Add", exact: true }).click(),
    ]);
    await page.waitForFunction(() =>
      document.querySelector(".modal button .spinner-border")
    );
    assert.equal(await description.isDisabled(), true);
    const controls = dialog.locator(".group-select .react-select__control");
    assert.equal(await controls.count(), 2);
    assert.equal(
      await controls.evaluateAll((items) =>
        items.every((item) => item.getAttribute("aria-disabled") === "true")
      ),
      true
    );
    assert.equal(
      await dialog
        .getByRole("button", { name: "Cancel", exact: true })
        .isDisabled(),
      true
    );
    const response = page.waitForResponse(
      (r) =>
        r.request().method() === "POST" &&
        r.request().postDataJSON()?.operationName === "AddGroupSubGroups"
    );
    release();
    assert.equal((await (await response).json()).errors, undefined);
    await dialog.waitFor({ state: "hidden" });
  } finally {
    release();
    await page.unrouteAll({ behavior: "wait" });
  }
  await expectOrder([a.id, b.id, c.id]);
  assert.equal(
    (await children()).find((item) => item.group.id === c.id).description,
    "Middle placement"
  );
  const handle = card(c.id).locator(".card-drag-handle");
  await handle.hover();
  await mutation(page, "ReorderSubGroups", async () => {
    const source = await handle.boundingBox();
    await page.mouse.move(
      source.x + source.width / 2,
      source.y + source.height / 2
    );
    await page.mouse.down();
    await page.mouse.move(
      source.x + source.width / 2,
      source.y + source.height / 2 + 5,
      { steps: 5 }
    );
    // The narrow layout stacks cards across screens. Keep the native drag
    // active while scrolling its destination into the viewport.
    await card(a.id).scrollIntoViewIfNeeded();
    const target = await card(a.id).boundingBox();
    await page.mouse.move(target.x + 5, target.y + target.height / 2, {
      steps: 20,
    });
    await page.mouse.move(target.x + 6, target.y + target.height / 2, {
      steps: 5,
    });
    await page.mouse.up();
  });
  await expectOrder([c.id, a.id, b.id]);
  assert.deepEqual(
    (await children()).map((item) => item.description),
    ["Middle placement", "Opening", "Ending"]
  );
  await page.reload({ waitUntil: "networkidle" });
  await expectOrder([c.id, a.id, b.id]);
  const returnURL = page.url();
  await card(c.id).locator("a.group-card-header").last().click();
  await page.waitForURL(new RegExp("/groups/" + c.id + "/scenes"));
  await page.goBack({ waitUntil: "networkidle" });
  assert.equal(page.url(), returnURL);
  await expectOrder([c.id, a.id, b.id]);
  await card(c.id).locator("input.card-check").check();
  const remove = page.getByTitle("Remove from Group", { exact: true });
  const beforeRemoval = await children();
  const rejectRemove = async (route) => {
    const request = route.request();
    const payload =
      request.method() === "POST" ? request.postDataJSON() : undefined;
    if (payload?.operationName === "RemoveGroupSubGroups") {
      payload.variables.input.containing_group_id =
        "invalid-collection-fixture";
      await route.continue({ postData: JSON.stringify(payload) });
    } else await route.continue();
  };
  await page.route("**/graphql", rejectRemove);
  try {
    await mutation(page, "RemoveGroupSubGroups", () => remove.click(), true);
  } finally {
    await page.unroute("**/graphql", rejectRemove);
  }
  await page.locator(".toast.danger").waitFor();
  assert.equal(await card(c.id).locator("input.card-check").isChecked(), true);
  assert.deepEqual(await children(), beforeRemoval);
  await mutation(page, "RemoveGroupSubGroups", () => remove.click());
  await card(c.id).waitFor({ state: "hidden" });
  await page
    .locator(".selected-count:visible")
    .waitFor({ state: "hidden", timeout: 5000 });
  await expectOrder([a.id, b.id]);
  assert.deepEqual(
    await group(c.id),
    originalC,
    "Unlink changed the child record or its other memberships"
  );
  assert.deepEqual(
    await group(other.id),
    originalOther,
    "Unlink changed another parent"
  );
  await add.waitFor();
  await add.click();
  await dialog.waitFor();
  await dialog.getByRole("button", { name: "Cancel", exact: true }).click();
  await page.reload({ waitUntil: "networkidle" });
  await expectOrder([a.id, b.id]);
  await page.screenshot({
    path: join(artifacts, label + "-collection.png"),
    fullPage: true,
  });
  assert.equal(
    await page.evaluate(
      () => document.documentElement.scrollWidth > window.innerWidth + 1
    ),
    false
  );
  assert.deepEqual(errors, []);
  await page.close();
  await instance.stop();
  instance = undefined;
  console.log(
    "Collection " +
      label +
      ": stale-cycle rejection/retry, description/order, Back/reload, rejected removal/selection, unlink and other-parent preservation passed."
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
        path: join(artifacts, "collection-failure.png"),
        fullPage: true,
      })
      .catch(() => {});
  throw error;
} finally {
  await browser.close();
  await instance?.stop();
  if (successful && process.env.STASH_BROWSER_SCREENSHOTS)
    await rm(root, { recursive: true, force: true });
  else console.log("Isolated Collection fixture: " + root);
}
