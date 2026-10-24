import assert from "node:assert/strict";
import fs from "node:fs";

// Use an explicitly supplied isolated library. Only an outside Image's temporary
// Gallery membership is written; cleanup also runs when an assertion fails.
const { chromium } = await import(
  process.env.PLAYWRIGHT_MODULE ?? "playwright"
);
const origin = process.env.STASH_BROWSER_URL;
const galleryID = process.env.STASH_BROWSER_GALLERY_ID;
const groupID = process.env.STASH_BROWSER_COLLECTION_ID;
assert.ok(
  origin && galleryID && groupID,
  "Set an isolated URL and Gallery/Collection IDs"
);
const shots = process.env.STASH_BROWSER_SCREENSHOTS;
if (shots) fs.mkdirSync(shots, { recursive: true });
const browser = await chromium.launch({
  executablePath: process.env.CHROMIUM_EXECUTABLE,
  args: ["--no-sandbox", "--disable-dev-shm-usage"],
});
async function call(page, query, variables = {}) {
  const response = await page.request.post(`${origin}/graphql`, {
    data: { query, variables },
  });
  const payload = await response.json();
  assert.equal(payload.errors, undefined, JSON.stringify(payload.errors));
  return payload.data;
}
async function capture(page, name) {
  if (shots)
    await page.screenshot({ path: `${shots}/${name}.png`, fullPage: true });
}
async function mutation(page, operation, click) {
  const [response] = await Promise.all([
    page.waitForResponse(
      (r) =>
        r.request().method() === "POST" &&
        r.request().postDataJSON()?.operationName === operation
    ),
    click(),
  ]);
  assert.equal((await response.json()).errors, undefined);
  await page.waitForLoadState("networkidle");
}

async function rejectedMutation(page, operation, click) {
  // Send an invalid Gallery ID to the actual server. Preserve its real error
  // response so the native catch/toast/retry path is exercised without mocks.
  const before = await memberIDs(page);
  let rejected = false;
  const handler = async (route) => {
    const request = route.request();
    if (request.method() === "POST") {
      const body = request.postDataJSON();
      if (body?.operationName === operation) {
        rejected = true;
        body.variables.gallery_id = "invalid-gallery-fixture";
        await route.continue({ postData: JSON.stringify(body) });
        return;
      }
    }
    await route.continue();
  };
  await page.route("**/graphql", handler);
  try {
    const [response] = await Promise.all([
      page.waitForResponse(
        (r) =>
          r.request().method() === "POST" &&
          r.request().postDataJSON()?.operationName === operation
      ),
      click(),
    ]);
    assert.ok(
      (await response.json()).errors?.length,
      "Server must reject the invalid Gallery ID"
    );
    assert.equal(rejected, true);
    await page.waitForLoadState("networkidle");
    assert.equal(
      await page.locator(".selected-count:visible").innerText(),
      "1",
      "Failed action lost the retry selection"
    );
    assert.deepEqual(
      await memberIDs(page),
      before,
      "Rejected action changed membership"
    );
  } finally {
    await page.unroute("**/graphql", handler);
  }
}
async function ids(page, kind) {
  await page.locator(`.${kind}-card:visible`).first().waitFor();
  return page
    .locator(`.${kind}-card:visible a.${kind}-card-link`)
    .evaluateAll((links) => [
      ...new Set(
        links.map((link) => new URL(link.href).pathname.split("/").pop())
      ),
    ]);
}
const galleryImages = `query($id:ID!){findImages(image_filter:{galleries:{value:[$id],modifier:INCLUDES}},filter:{per_page:-1,sort:"title",direction:DESC}){images{id}}}`;
const allImages = `query{findImages(filter:{per_page:-1,sort:"title",direction:ASC}){images{id galleries{id}}}}`;
const nativeImage = `query($id:ID!){findImage(id:$id){id title artists{id} performers{id} copyrights{id} tags{id} galleries{id}}}`;
const removeImage = `mutation($id:ID!,$images:[ID!]!){removeGalleryImages(input:{gallery_id:$id,image_ids:$images})}`;
const allVideos = `query{findScenes(filter:{per_page:-1}){scenes{id groups{group{id} scene_index}}}}`;
async function memberIDs(page) {
  return (
    await call(page, galleryImages, { id: galleryID })
  ).findImages.images.map((i) => i.id);
}
const failures = [];
try {
  for (const [name, viewport] of [
    ["desktop", { width: 1440, height: 1000 }],
    ["mobile", { width: 390, height: 844 }],
  ]) {
    for (const kind of ["gallery", "collection"]) {
      const page = await browser.newPage({ viewport });
      const errors = [];
      page.on("pageerror", (e) => errors.push(String(e)));
      let candidate;
      try {
        if (kind === "gallery") {
          const members = await memberIDs(page);
          assert.ok(members.length >= 2, "Give the Gallery two member Images");
          const candidates = (
            await call(page, allImages)
          ).findImages.images.filter(
            (i) => !i.galleries.some((g) => g.id === galleryID)
          );
          assert.ok(
            candidates.length >= 2,
            "Give the fixture two outside Images"
          );
          candidate = candidates[0].id;
          const before = (await call(page, nativeImage, { id: candidate }))
            .findImage;
          await page.goto(
            `${origin}/galleries/${galleryID}?perPage=1&sortby=title&sortdir=desc&p=2`,
            { waitUntil: "networkidle" }
          );
          assert.deepEqual(await ids(page, "image"), [members[1]]);
          const search = new URL(page.url()).search;
          await page.getByRole("tab", { name: "Add", exact: true }).click();
          await page.waitForLoadState("networkidle");
          assert.equal(
            new URL(page.url()).search,
            search,
            "Gallery Add discarded list options"
          );
          assert.equal(
            await page
              .locator(".gallery-container .sidebar-pane:visible")
              .count(),
            0,
            "Gallery Add mounted the general Images sidebar"
          );
          await capture(page, `gallery-add-layout-${name}`);
          const outside = await ids(page, "image");
          assert.equal(outside.length, 1);
          assert.ok(
            outside.every((id) => !members.includes(id)),
            "Add included existing members"
          );
          await page.getByRole("tab", { name: "Images", exact: true }).click();
          await page.waitForLoadState("networkidle");
          assert.equal(new URL(page.url()).search, search);
          assert.deepEqual(await ids(page, "image"), [members[1]]);
          const returnURL = page.url();
          await page
            .locator(".image-card:visible a.image-card-link")
            .first()
            .click();
          await page.locator(".Lightbox-footer-center .image-link").click();
          await page.waitForURL(
            (url) => url.pathname === `/images/${members[1]}`
          );
          await page.goBack();
          await page.waitForURL(returnURL);
          await page.locator(".image-card:visible").first().waitFor();
          assert.deepEqual(await ids(page, "image"), [members[1]]);
          await page.reload({ waitUntil: "networkidle" });
          assert.equal(page.url(), returnURL);
          assert.deepEqual(await ids(page, "image"), [members[1]]);
          await page.goto(
            `${origin}/galleries/${galleryID}/add?perPage=40&sortby=title&sortdir=asc`,
            { waitUntil: "networkidle" }
          );
          const card = page.locator(".image-card:visible").filter({
            has: page.locator(`a.image-card-link[href="/images/${candidate}"]`),
          });
          await card.locator("input.card-check").check();
          await rejectedMutation(page, "AddGalleryImages", () =>
            page.getByTitle("Add to Gallery", { exact: true }).click()
          );
          assert.equal(
            await card.locator("input.card-check").isChecked(),
            true
          );
          await mutation(page, "AddGalleryImages", () =>
            page.getByTitle("Add to Gallery", { exact: true }).click()
          );
          assert.ok(
            (
              await call(page, nativeImage, { id: candidate })
            ).findImage.galleries.some((g) => g.id === galleryID)
          );
          await card.waitFor({ state: "hidden", timeout: 10000 });
          await page
            .locator(".selected-count:visible")
            .waitFor({ state: "hidden", timeout: 10000 });
          assert.deepEqual(
            (await memberIDs(page)).sort(),
            [...members, candidate].sort(),
            "Adding membership changed other members"
          );
          await page.getByRole("tab", { name: "Images", exact: true }).click();
          await page.waitForLoadState("networkidle");
          await card.locator("input.card-check").check();
          await rejectedMutation(page, "RemoveGalleryImages", () =>
            page.getByTitle("Remove from Gallery", { exact: true }).click()
          );
          assert.equal(
            await card.locator("input.card-check").isChecked(),
            true
          );
          await mutation(page, "RemoveGalleryImages", () =>
            page.getByTitle("Remove from Gallery", { exact: true }).click()
          );
          await card.waitFor({ state: "hidden", timeout: 10000 });
          await page
            .locator(".selected-count:visible")
            .waitFor({ state: "hidden", timeout: 10000 });
          assert.deepEqual(
            (await memberIDs(page)).sort(),
            [...members].sort(),
            "Removing membership changed other members"
          );
          assert.deepEqual(
            (await call(page, nativeImage, { id: candidate })).findImage,
            before,
            "Removing membership changed native Image metadata"
          );
        } else {
          const scenes = (await call(page, allVideos)).findScenes.scenes;
          const members = scenes.filter((s) =>
            s.groups.some((g) => g.group.id === groupID)
          );
          assert.ok(
            members.length >= 2 && scenes.some((s) => !members.includes(s)),
            "Give the Collection two numbered Videos and an outside Video"
          );
          const number = (scene) =>
            scene.groups.find((g) => g.group.id === groupID).scene_index;
          assert.ok(members.every((s) => number(s) !== null));
          assert.equal(
            new Set(members.map(number)).size,
            members.length,
            "Use distinct scene numbers"
          );
          members.sort((a, b) => number(b) - number(a));
          await page.goto(
            `${origin}/groups/${groupID}?sortby=group_scene_number&sortdir=desc&perPage=1&p=2`,
            { waitUntil: "networkidle" }
          );
          await page.waitForURL(
            (url) => url.pathname === `/groups/${groupID}/scenes`
          );
          const location = new URL(page.url());
          assert.equal(location.pathname, `/groups/${groupID}/scenes`);
          for (const [key, value] of Object.entries({
            sortby: "group_scene_number",
            sortdir: "desc",
            perPage: "1",
            p: "2",
          }))
            assert.equal(
              location.searchParams.get(key),
              value,
              `Collection redirect discarded ${key}`
            );
          assert.deepEqual(await ids(page, "scene"), [members[1].id]);
          const returnURL = page.url();
          await page
            .locator(".scene-card:visible a.scene-card-link")
            .first()
            .click();
          await page.waitForURL(
            (url) => url.pathname === `/scenes/${members[1].id}`
          );
          await page.goBack();
          await page.waitForURL(returnURL);
          await page.locator(".scene-card:visible").first().waitFor();
          assert.deepEqual(await ids(page, "scene"), [members[1].id]);
          await page.reload({ waitUntil: "networkidle" });
          assert.equal(page.url(), returnURL);
          assert.deepEqual(await ids(page, "scene"), [members[1].id]);
          await capture(page, `membership-collection-order-${name}`);
          await page.goto(
            `${origin}/groups/${groupID}?q=__membership_no_match__&perPage=1`,
            { waitUntil: "networkidle" }
          );
          assert.equal(
            new URL(page.url()).searchParams.get("q"),
            "__membership_no_match__"
          );
          assert.equal(await page.locator(".scene-card:visible").count(), 0);
          await page.getByRole("tab", { name: /^Characters/ }).click();
          await page.waitForURL(
            (url) => url.pathname === `/groups/${groupID}/performers`
          );
          await page.waitForLoadState("networkidle");
          assert.equal(
            new URL(page.url()).searchParams.get("q"),
            null,
            "An explicit media-tab switch must retain its existing filter reset"
          );
        }
        assert.deepEqual(errors, []);
        await capture(page, `membership-${kind}-${name}`);
        console.log(
          `${name}/${kind}: scope, options, member Back, refresh and native membership passed`
        );
      } catch (error) {
        failures.push(`${name}/${kind}: ${error.stack}`);
        await capture(page, `membership-${kind}-${name}-failure`).catch(
          () => {}
        );
      } finally {
        try {
          if (candidate)
            await call(page, removeImage, {
              id: galleryID,
              images: [candidate],
            });
        } finally {
          await page.close();
        }
      }
    }
  }
} finally {
  await browser.close();
}
assert.deepEqual(failures, []);
