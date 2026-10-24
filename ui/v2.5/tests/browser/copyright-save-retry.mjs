import assert from "node:assert/strict";
import { mkdtemp, mkdir, writeFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { startIsolatedServer } from "./helpers/isolated-server.mjs";

// Real local mutations, including a Character removed after selection. Own every
// path and process; never intercept requests or connect to an existing library.
assert.ok(process.env.STASH_BROWSER_BINARY, "Set STASH_BROWSER_BINARY");
const binary = resolve(process.env.STASH_BROWSER_BINARY);
const { chromium } = await import(
  process.env.PLAYWRIGHT_MODULE ?? "playwright"
);
const root = await mkdtemp(join(tmpdir(), "stashbooru-copyright-save-"));
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
  const result = await (
    await page.request.post(url + "/graphql", {
      data: { query, variables },
    })
  ).json();
  assert.equal(result.errors, undefined, JSON.stringify(result.errors));
  return result.data;
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
      ui: process.env.STASH_BROWSER_UI,
    });
    return instance.url;
  }
  let url = await start("");
  let page = await browser.newPage({ viewport });
  const errors = [];
  page.on("pageerror", (error) => errors.push(String(error)));
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
        "mutation($input:CopyrightCreateInput!){copyrightCreate(input:$input){id name}}",
        { input }
      )
    ).copyrightCreate;
  const parent = await create({ name: "Root Copyright" });
  const otherParent = await create({ name: "Other Root Copyright" });
  const child = await create({ name: "Child Copyright" });
  const descendant = await create({
    name: "Descendant Copyright",
    parent_ids: [child.id],
  });
  const character = async (name) =>
    (
      await call(
        page,
        url,
        "mutation($input:PerformerCreateInput!){performerCreate(input:$input){id}}",
        { input: { name } }
      )
    ).performerCreate;
  const keep = await character("Keep Character");
  const removed = await character("Removed Character");
  const tag = (
    await call(
      page,
      url,
      'mutation {tagCreate(input:{name:"Root Copyright",aliases:["Keep ordinary Tag"]}){id}}'
    )
  ).tagCreate;
  const others = async () =>
    await call(
      page,
      url,
      "query($tag:ID!,$character:ID!){findTag(id:$tag){id name aliases parents{id} children{id}} findPerformer(id:$character){id name}}",
      { tag: tag.id, character: keep.id }
    );
  const originalOthers = await others();
  const all = async () =>
    (
      await call(
        page,
        url,
        '{findCopyrights(filter:{per_page:-1,sort:"id",direction:ASC}){copyrights{id name sort_name description aliases parents{id} children{id} ordered_children{id} performers{id} tags{id}}}}'
      )
    ).findCopyrights.copyrights;
  const original = await all();
  const form = () => page.locator("#copyright-edit");
  const control = (label) =>
    form()
      .locator(".form-group")
      .filter({ has: page.locator("label").getByText(label, { exact: true }) })
      .locator("input,textarea")
      .first();
  const name = () => control("Name");
  const actions = () => page.locator(".details-edit").first();
  async function select(label, text) {
    const box =
      label === "Characters"
        ? page.locator("#copyright-edit-tabs-tabpane-characters")
        : form()
            .locator(".form-group")
            .filter({
              has: page.locator("label").getByText(label, { exact: true }),
            });
    const input = box.getByRole("combobox");
    await input.fill(text);
    await page
      .getByRole("option")
      .filter({ has: page.getByText(text, { exact: true }) })
      .click();
    await input.press("Escape");
  }
  async function remove(label, text) {
    const box =
      label === "Characters"
        ? page.locator("#copyright-edit-tabs-tabpane-characters")
        : form()
            .locator(".form-group")
            .filter({
              has: page.locator("label").getByText(label, { exact: true }),
            });
    await box
      .locator(".react-select__multi-value")
      .filter({ hasText: text })
      .getByRole("button")
      .click();
  }
  async function action(operation, click, pattern) {
    const [response] = await Promise.all([
      page.waitForResponse(
        (r) =>
          r.request().method() === "POST" &&
          r.request().postDataJSON()?.operationName === operation
      ),
      click(),
    ]);
    const payload = await response.json();
    if (pattern) assert.match(JSON.stringify(payload.errors), pattern);
    else
      assert.equal(payload.errors, undefined, JSON.stringify(payload.errors));
    await actions().locator(".save").waitFor({ state: "visible" });
    await page.waitForFunction(
      () => !document.querySelector(".details-edit .save:disabled")
    );
    return payload;
  }
  async function profile(id) {
    await page.waitForURL(
      (u) =>
        u.pathname === `/copyrights/${id}` ||
        u.pathname.startsWith(`/copyrights/${id}/`)
    );
    await actions()
      .getByRole("button", { name: "Edit", exact: true })
      .waitFor();
  }
  try {
    await page.goto(url + "/copyrights/new?parent_id=" + parent.id);
    await name().waitFor();
    assert.equal(await actions().locator(".save").isDisabled(), true);
    await name().fill("  ");
    assert.equal(await actions().locator(".save").isDisabled(), true);
    await name().fill("Root Copyright");
    await control("Sort alphabetically as (optional)").fill("First sort value");
    await control("Aliases").fill("First alias, Second alias\nThird alias");
    await control("Details").fill("Keep Copyright description");
    await select("Main Copyrights", otherParent.name);
    await select("Sub-Copyrights", child.name);
    await select("Characters", "Keep Character");
    await select("Characters", "Removed Character");
    const artwork = await page.evaluate(() => {
      const canvas = document.createElement("canvas");
      canvas.width = 144;
      canvas.height = 81;
      canvas.getContext("2d").fillRect(0, 0, 144, 81);
      return canvas.toDataURL("image/png").split(",")[1];
    });
    const imageFile = join(directory, "copyright.png");
    await writeFile(imageFile, Buffer.from(artwork, "base64"));
    await actions()
      .getByRole("button", { name: "Set image…", exact: true })
      .click();
    await page
      .locator('#set-image-popover input[type="file"]')
      .setInputFiles(imageFile);
    await name().click();
    await action(
      "CopyrightCreate",
      () => actions().locator(".save").click(),
      /unique|already|exists/i
    );
    assert.equal(await name().inputValue(), "Root Copyright");
    assert.deepEqual(await all(), original);
    // A selected Character can disappear in another tab before this save.
    await call(
      page,
      url,
      "mutation($input:PerformerDestroyInput!){performerDestroy(input:$input)}",
      { input: { id: removed.id } }
    );
    await name().fill("First Copyright");
    await action(
      "CopyrightPerformersUpdate",
      () => actions().locator(".save").click(),
      /foreign key/i
    );
    assert.equal(await name().inputValue(), "First Copyright");
    assert.equal(
      await control("Details").inputValue(),
      "Keep Copyright description"
    );
    const partial = await all();
    assert.equal(partial.length, original.length + 1);
    const first = partial.find((c) => c.name === "First Copyright");
    assert.ok(
      first,
      "Metadata save creates exactly one Copyright before link failure"
    );
    await page.screenshot({
      path: join(artifacts, label + "-rejected.png"),
      fullPage: true,
    });
    await remove("Characters", "Removed Character");
    // Correcting the stale selection must update that same native record.
    const success = await action("CopyrightPerformersUpdate", () =>
      actions().locator(".save").click()
    );
    assert.equal(success.data.copyrightPerformersUpdate.id, first.id);
    await profile(first.id);
    const saved = await all();
    assert.equal(saved.length, original.length + 1);
    const stored = saved.find((c) => c.id === first.id);
    assert.equal(stored.sort_name, "First sort value");
    assert.equal(stored.description, "Keep Copyright description");
    assert.deepEqual(stored.aliases, [
      "First alias",
      "Second alias",
      "Third alias",
    ]);
    assert.deepEqual(
      stored.parents.map((x) => x.id).sort(),
      [parent.id, otherParent.id].sort()
    );
    assert.deepEqual(
      stored.children.map((x) => x.id),
      [child.id]
    );
    assert.deepEqual(
      stored.performers.map((x) => x.id),
      [keep.id]
    );
    const imageURL = (
      await call(
        page,
        url,
        "query($id:ID!){findCopyright(id:$id){image_path}}",
        { id: first.id }
      )
    ).findCopyright.image_path;
    const imageBytes = await (await page.request.get(imageURL)).body();
    assert.ok(imageBytes.length);
    await actions().getByRole("button", { name: "Edit", exact: true }).click();
    await name().fill("Updated Copyright");
    await select("Main Copyrights", descendant.name);
    await action(
      "CopyrightUpdate",
      () => actions().locator(".save").click(),
      /ancestor|cycle|descendant/i
    );
    assert.equal(await name().inputValue(), "Updated Copyright");
    assert.deepEqual(await all(), saved);
    await remove("Main Copyrights", descendant.name);
    await actions().locator(".save").click();
    await profile(first.id);
    const final = await all();
    assert.deepEqual(
      final.map((c) => (c.id === first.id ? { ...c, name: stored.name } : c)),
      saved
    );
    await actions().getByRole("button", { name: "Edit", exact: true }).click();
    await name().fill("Discard this draft");
    await actions()
      .getByRole("button", { name: "Cancel", exact: true })
      .click();
    assert.deepEqual(await all(), final);
    await page.reload();
    await profile(first.id);
    await page.goto(url + "/copyrights");
    const listURL = page.url();
    await page
      .locator(`a.booru-entity-card-name[href="/copyrights/${first.id}"]`)
      .click();
    await profile(first.id);
    await page.goBack();
    assert.equal(page.url(), listURL);
    assert.deepEqual(await all(), final);
    assert.deepEqual(await others(), originalOthers);
    await page.close();
    await instance.stop();
    instance = undefined;
    url = await start("-restart");
    page = await browser.newPage({ viewport });
    page.on("pageerror", (error) => errors.push(String(error)));
    await page.goto(url + "/copyrights/" + first.id);
    await profile(first.id);
    assert.deepEqual(await all(), final);
    assert.deepEqual(await others(), originalOthers);
    const after = (
      await call(
        page,
        url,
        "query($id:ID!){findCopyright(id:$id){image_path}}",
        { id: first.id }
      )
    ).findCopyright.image_path;
    assert.deepEqual(await (await page.request.get(after)).body(), imageBytes);
    assert.equal(
      await page.evaluate(
        () => document.documentElement.scrollWidth > innerWidth
      ),
      false
    );
    assert.deepEqual(errors, []);
    await page.screenshot({
      path: join(artifacts, label + "-saved.png"),
      fullPage: true,
    });
    console.log(
      label +
        ": Copyright rejected create, partial-link retry, hierarchy retry, Cancel, Back, refresh and restart passed"
    );
  } catch (error) {
    await page
      .screenshot({
        path: join(artifacts, label + "-failure.png"),
        fullPage: true,
      })
      .catch(() => {});
    throw error;
  } finally {
    await page.close();
    await instance?.stop();
    instance = undefined;
  }
}
try {
  await check({ width: 1440, height: 1000 }, "desktop");
  await check({ width: 390, height: 844 }, "mobile");
  passed = true;
} finally {
  await instance?.stop();
  await browser.close();
  if (passed) await rm(root, { recursive: true, force: true });
  else console.error("Failed fixture retained:", root);
}
