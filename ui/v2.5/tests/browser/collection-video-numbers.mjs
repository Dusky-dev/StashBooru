import assert from "node:assert/strict";
import { mkdtemp, mkdir, writeFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { startIsolatedServer } from "./helpers/isolated-server.mjs";

// Metadata-only Videos exercise native edit/order contracts, not codec playback.
// Every server/database is created here; existing libraries are never accepted.
assert.ok(process.env.STASH_BROWSER_BINARY, "Set STASH_BROWSER_BINARY");
const binary = resolve(process.env.STASH_BROWSER_BINARY);
const ui = process.env.STASH_BROWSER_UI;
const { chromium } = await import(
  process.env.PLAYWRIGHT_MODULE ?? "playwright"
);
const root = await mkdtemp(join(tmpdir(), "stashbooru-video-numbers-"));
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
  const createGroup = async (name) =>
    (
      await call(
        page,
        url,
        "mutation($input: GroupCreateInput!) {groupCreate(input: $input) {id name}}",
        { input: { name } }
      )
    ).groupCreate;
  const main = await createGroup("Numbered collection");
  const other = await createGroup("Other collection");
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
  const tag = (
    await call(
      page,
      url,
      'mutation {tagCreate(input: {name: "Fixture tag"}) {id}}'
    )
  ).tagCreate;
  const copyright = (
    await call(
      page,
      url,
      'mutation {copyrightCreate(input: {name: "Fixture Copyright"}) {id}}'
    )
  ).copyrightCreate;
  const gallery = (
    await call(
      page,
      url,
      'mutation {galleryCreate(input: {title: "Fixture Gallery"}) {id}}'
    )
  ).galleryCreate;
  const createScene = async (input) =>
    (
      await call(
        page,
        url,
        "mutation($input: SceneCreateInput!) {sceneCreate(input: $input) {id title}}",
        { input }
      )
    ).sceneCreate;
  const subject = await createScene({
    title: "Numbered Video",
    code: "keep-code",
    custom_fields: {
      note: "Keep the custom value",
      full: "User field named full",
    },
    details: "Keep the native metadata",
    urls: ["https://example.org/video"],
    studio_id: artist.id,
    performer_ids: [character.id],
    tag_ids: [tag.id],
    gallery_ids: [gallery.id],
    groups: [
      { group_id: main.id, scene_index: 2 },
      { group_id: other.id, scene_index: 7 },
    ],
  });
  await call(
    page,
    url,
    "mutation($id: ID!, $artists: [ID!]!) {sceneArtistsUpdate(scene_id: $id, artist_ids: $artists) {id}}",
    { id: subject.id, artists: [artist.id] }
  );
  await call(
    page,
    url,
    "mutation($id: ID!, $copyrights: [ID!]!) {sceneCopyrightsUpdate(scene_id: $id, copyright_ids: $copyrights) {id}}",
    { id: subject.id, copyrights: [copyright.id] }
  );
  const following = await createScene({
    title: "Following Video",
    groups: [{ group_id: main.id, scene_index: 20 }],
  });
  const snapshot = async () =>
    (
      await call(
        page,
        url,
        "query($id: ID!) {findScene(id: $id) {id title code details urls studio {id} artists {id} copyrights {id} performers {id} tags {id} galleries {id} files {id} custom_fields groups {group {id} scene_index} }}",
        { id: subject.id }
      )
    ).findScene;
  const original = await snapshot();
  const unchangedMetadata = ({ groups: _groups, ...rest }) => rest;
  let writes = 0;
  page.on("request", (request) => {
    if (
      request.method() === "POST" &&
      request.postDataJSON()?.operationName === "SceneUpdate"
    )
      writes++;
  });
  const collectionURL =
    url +
    "/groups/" +
    main.id +
    "/scenes?sortby=group_scene_number&sortdir=asc&perPage=40";
  const cards = page.locator(".scene-card:visible");
  const card = (id) =>
    cards.filter({
      has: page.locator(
        'a.scene-card-link[href="/scenes/' +
          id +
          '"], a.scene-card-link[href^="/scenes/' +
          id +
          '?"]'
      ),
    });
  async function expectOrder(ids) {
    await page.waitForFunction((expected) => {
      const actual = [...document.querySelectorAll(".scene-card")].map((item) =>
        new URL(item.querySelector("a.scene-card-link").href).pathname
          .split("/")
          .pop()
      );
      return JSON.stringify(actual) === JSON.stringify(expected);
    }, ids);
  }
  async function openSubject() {
    await card(subject.id)
      .locator(".card-section-title-row a")
      .filter({ hasText: subject.title })
      .last()
      .click();
    await page.waitForURL(
      (location) => location.pathname === "/scenes/" + subject.id
    );
    await page.getByRole("tab", { name: "Edit", exact: true }).click();
  }
  const editor = page.locator("#scene-edit-details:visible");
  const number = editor
    .locator(".group-row")
    .filter({ hasText: main.name })
    .locator('input[type="number"]');
  const otherNumber = editor
    .locator(".group-row")
    .filter({ hasText: other.name })
    .locator('input[type="number"]');
  const save = editor.getByRole("button", { name: "Save", exact: true });
  async function savedValue(expected, rejected = false) {
    const [response] = await Promise.all([
      page.waitForResponse(
        (r) =>
          r.request().method() === "POST" &&
          r.request().postDataJSON()?.operationName === "SceneUpdate"
      ),
      save.click(),
    ]);
    const payload = await response.json();
    assert.equal(!!payload.errors?.length, rejected, JSON.stringify(payload));
    if (rejected)
      assert.match(payload.errors[0].message, /invalid-video-fixture/);
    // Scene save also awaits its native Artist/Copyright writes before restoring
    // the form. Do not assert against an intermediate mutation/cache result.
    await editor.waitFor();
    assert.equal(
      await number.inputValue(),
      expected === null ? "" : String(expected)
    );
    assert.equal(await otherNumber.inputValue(), "7");
    if (!rejected) {
      const saved = await snapshot();
      assert.deepEqual(
        saved.groups.map((item) => item.group.id).sort(),
        original.groups.map((item) => item.group.id).sort()
      );
      assert.equal(
        saved.groups.find((item) => item.group.id === main.id).scene_index,
        expected
      );
      assert.equal(
        saved.groups.find((item) => item.group.id === other.id).scene_index,
        7
      );
      assert.deepEqual(unchangedMetadata(saved), unchangedMetadata(original));
    }
  }
  await page.goto(collectionURL, { waitUntil: "networkidle" });
  await expectOrder([subject.id, following.id]);
  const returnURL = page.url();
  await openSubject();
  assert.equal(await number.inputValue(), "2");
  await number.fill("1.5");
  assert.equal(
    await number.inputValue(),
    "1.5",
    "Fractional input must not silently truncate to a different whole number"
  );
  await page.waitForFunction(
    () =>
      [...document.querySelectorAll("#scene-edit-details button")].find(
        (item) => item.textContent.trim() === "Save"
      )?.disabled === true
  );
  assert.equal(writes, 0, "Invalid input must not issue a SceneUpdate");
  await number.fill("1e2");
  assert.equal(
    Number(await number.inputValue()),
    100,
    "A complete HTML exponent value must be parsed as a whole number"
  );
  await savedValue(100);
  await page.reload({ waitUntil: "networkidle" });
  await page.getByRole("tab", { name: "Edit", exact: true }).click();
  assert.equal(await number.inputValue(), "100");
  await page.goBack({ waitUntil: "networkidle" });
  assert.equal(page.url(), returnURL);
  await expectOrder([following.id, subject.id]);
  await openSubject();
  await number.fill("0");
  await savedValue(0);
  await page.getByRole("tab", { name: /Collections/ }).click();
  const mainCard = page
    .locator(".group-card:visible")
    .filter({ has: page.locator('a[href="/groups/' + main.id + '"]') });
  await mainCard
    .locator(".group-scene-number")
    .filter({ hasText: /#0$/ })
    .waitFor({ timeout: 5000 });
  await page.screenshot({
    path: join(artifacts, label + "-zero.png"),
    fullPage: true,
  });
  await page.goBack({ waitUntil: "networkidle" });
  assert.equal(page.url(), returnURL);
  await expectOrder([subject.id, following.id]);
  await card(subject.id)
    .locator(".scene-group-scene-number")
    .filter({ hasText: /#0$/ })
    .waitFor({ timeout: 5000 });
  await openSubject();
  await number.fill("");
  await savedValue(null);
  await page.getByRole("tab", { name: /Collections/ }).click();
  await mainCard.waitFor();
  assert.equal(
    await mainCard.locator(".group-scene-number").count(),
    0,
    "Clearing a number must not render a fake zero"
  );
  await page.getByRole("tab", { name: "Edit", exact: true }).click();
  await number.fill("-1");
  const beforeRejected = await snapshot();
  const reject = async (route) => {
    const request = route.request();
    const payload =
      request.method() === "POST" ? request.postDataJSON() : undefined;
    if (payload?.operationName === "SceneUpdate") {
      payload.variables.input.id = "invalid-video-fixture";
      await route.continue({ postData: JSON.stringify(payload) });
    } else await route.continue();
  };
  await page.route("**/graphql", reject);
  try {
    await savedValue(-1, true);
  } finally {
    await page.unroute("**/graphql", reject);
  }
  await page.locator(".toast.danger").waitFor();
  assert.deepEqual(
    await snapshot(),
    beforeRejected,
    "Rejected save changed the native Video"
  );
  await savedValue(-1);
  await page.reload({ waitUntil: "networkidle" });
  await page.getByRole("tab", { name: "Edit", exact: true }).click();
  assert.equal(await number.inputValue(), "-1");
  assert.equal(await otherNumber.inputValue(), "7");
  await page.goBack({ waitUntil: "networkidle" });
  assert.equal(page.url(), returnURL);
  await expectOrder([subject.id, following.id]);
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
    "Collection Video numbers " +
      label +
      ": fraction validation, full numeric parsing, zero/null/signed values, rejection/retry, Back/order/refresh and metadata preservation passed."
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
        path: join(artifacts, "video-number-failure.png"),
        fullPage: true,
      })
      .catch(() => {});
  throw error;
} finally {
  await browser.close();
  await instance?.stop();
  if (successful && process.env.STASH_BROWSER_SCREENSHOTS)
    await rm(root, { recursive: true, force: true });
  else console.log("Isolated Video numbering fixture: " + root);
}
