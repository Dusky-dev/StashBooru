import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";

// Mutates grouping only in an explicitly authorized isolated fixture. Native
// playback activity writes are intercepted and grouping is cleaned up on exit.
assert.equal(
  process.env.STASH_BROWSER_ALLOW_STACK_WRITES,
  "1",
  "Use an isolated fixture and set STASH_BROWSER_ALLOW_STACK_WRITES=1"
);
const engines = await import(process.env.PLAYWRIGHT_MODULE ?? "playwright");
const engineName = process.env.STASH_BROWSER_ENGINE ?? "chromium";
const baseURL = process.env.STASH_BROWSER_URL ?? "http://127.0.0.1:9999";
const shots = process.env.STASH_BROWSER_SCREENSHOTS;
if (shots) fs.mkdirSync(shots, { recursive: true });
const browser = await engines[engineName].launch({
  headless: true,
  executablePath:
    engineName === "chromium"
      ? process.env.CHROMIUM_EXECUTABLE
      : process.env.FIREFOX_EXECUTABLE,
  args:
    engineName === "chromium"
      ? ["--no-sandbox", "--disable-dev-shm-usage"]
      : [],
  firefoxUserPrefs: process.env.STASH_BROWSER_FIREFOX_SINGLE_PROCESS
    ? {
        "fission.autostart": false,
        "browser.tabs.remote.autostart": false,
        "security.sandbox.content.level": 0,
      }
    : undefined,
});

const created = new Set();
async function gql(query, variables = {}) {
  const response = await fetch(`${baseURL}/graphql`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ query, variables }),
  });
  const payload = await response.json();
  assert.equal(payload.errors, undefined, JSON.stringify(payload.errors));
  return payload.data;
}
const stackFields =
  "id title version representative member_count members{id media{kind id} label position representative}";
async function stackFor(kind, id) {
  return (
    await gql(
      `query($media:MediaReferenceInput!){findVisualStackForMedia(media:$media){${stackFields}}}`,
      { media: { kind, id } }
    )
  ).findVisualStackForMedia;
}
async function getStack(id) {
  return (
    await gql(`query($id:ID!){findVisualStack(id:$id){${stackFields}}}`, { id })
  ).findVisualStack;
}

// Only mutate fingerprints in the explicit isolated fixture. Restore every
// previous value even after a failed assertion, so no persistent test data changes.
async function setPHash(fileID, value) {
  const result = await gql(
    "mutation($input:FileSetFingerprintsInput!){fileSetFingerprints(input:$input)}",
    { input: { id: String(fileID), fingerprints: [{ type: "phash", value }] } }
  );
  assert.equal(result.fileSetFingerprints, true);
}
async function snapshot() {
  return gql(`query {
    findImages(filter:{per_page:100,sort:"id"}){count images{id title details date rating100 organized urls galleries{id} visual_files{
      ... on ImageFile {id path size fingerprints{type value}}
      ... on VideoFile {id path size fingerprints{type value}}
    }}}
    findScenes(filter:{per_page:100,sort:"id"}){count scenes{id title details date rating100 organized urls galleries{id} files{id path size fingerprints{type value}}}}
  }`);
}
async function operation(page, name) {
  await page.locator(".media-list #more-menu").click();
  await page.getByText(name, { exact: true }).click();
}
function modal(page) {
  return page.locator(".modal.show");
}
function row(page, key) {
  return modal(page).locator(
    `.visual-stack-editor-member[data-member-id="${key}"]`
  );
}
function card(page, kind, id) {
  return page
    .locator(`.media-list .${kind === "IMAGE" ? "image" : "scene"}-card`)
    .filter({
      has: page.locator(
        `a[href^="/${kind === "IMAGE" ? "images" : "scenes"}/${id}"]`
      ),
    })
    .first();
}
async function save(page) {
  await modal(page)
    .getByRole("button", { name: "Save stack", exact: true })
    .click();
  await modal(page).waitFor({ state: "hidden" });
}
async function selected(page, key) {
  await page
    .locator(
      `.visual-stack-thumbnail[data-member-id="${key}"][aria-pressed="true"]`
    )
    .waitFor();
}
async function player(page, id) {
  await page.waitForFunction((id) => {
    const players = Array.from(
      document.querySelectorAll(".unified-media-native-scene-player")
    );
    return (
      players.length === 1 &&
      players[0].dataset.sceneId === id &&
      players[0].querySelector("video")?.readyState >= 2
    );
  }, id);
  const poster = await page
    .locator(".unified-media-native-scene-player .vjs-poster")
    .evaluate((node) => node.style.backgroundImage);
  assert.ok(
    !poster || poster === "none",
    "Autoplay retained a screenshot poster"
  );
}

try {
  for (const width of [1440, 390]) {
    const page = await browser.newPage({
      viewport: { width, height: width < 500 ? 844 : 1000 },
    });
    const errors = [];
    const originalPHashes = new Map();
    async function restorePHashes() {
      for (const [fileID, value] of originalPHashes) {
        await setPHash(fileID, value);
        originalPHashes.delete(fileID);
      }
    }
    page.on("pageerror", (e) => errors.push(e.message));
    await page.route("**/graphql", async (route) => {
      const name = route.request().postDataJSON()?.operationName;
      if (name === "SceneSaveActivity")
        return route.fulfill({ json: { data: { sceneSaveActivity: true } } });
      if (name === "SceneAddPlay")
        return route.fulfill({
          json: { data: { sceneAddPlay: { count: 1, history: [] } } },
        });
      return route.continue();
    });
    try {
      const before = await snapshot();
      await page.goto(`${baseURL}/media`, { waitUntil: "domcontentloaded" });
      await page.locator(".media-list .image-card").first().waitFor();
      const grouping = page.getByRole("button", {
        name: "Group stacks",
        exact: true,
      });
      // No browser-local setting exists in the fresh Playwright context.
      // Representative-only mode must now be the default.
      await grouping.waitFor();
      assert.equal(
        await grouping.getAttribute("aria-pressed"),
        "true",
        "Fresh installs must show only stack representatives by default"
      );
      if (
        await modal(page)
          .getByText("Release Notes", { exact: true })
          .isVisible()
      ) {
        await modal(page)
          .getByRole("button", { name: "Close", exact: true })
          .click();
        await modal(page).waitFor({ state: "hidden" });
      }
      if (shots && engineName === "chromium" && width === 1440)
        await page.screenshot({
          path: path.join(shots, "before.png"),
          fullPage: true,
        });
      // Equal numeric IDs are deliberately selected independently.
      for (const [kind, id] of [
        ["IMAGE", "1"],
        ["IMAGE", "2"],
        ["VIDEO", "1"],
        ["VIDEO", "2"],
      ]) {
        const target = card(page, kind, id);
        await target.hover();
        await target.locator('input[type="checkbox"]').check();
      }
      await operation(page, "Create stack from selection…");
      await row(page, "scene:2").waitFor();
      assert.equal(
        await modal(page).locator(".visual-stack-editor-member").count(),
        4
      );
      await modal(page)
        .getByLabel("Stack title (optional)")
        .fill("Original and variants");
      await row(page, "image:1").getByRole("combobox").selectOption("Original");
      await row(page, "scene:1")
        .getByRole("combobox")
        .selectOption("Converted copy");
      // Selection follows native sort order. Make the expected variant order explicit.
      await row(page, "scene:1")
        .getByRole("button", { name: "Move scene:1 up", exact: true })
        .click();
      await row(page, "scene:1")
        .getByLabel("Representative scene:1", { exact: true })
        .check();
      await modal(page)
        .getByRole("button", { name: "Create stack", exact: true })
        .click();
      await modal(page).waitFor({ state: "hidden" });
      let stack = await stackFor("IMAGE", "1");
      created.add(stack.id);
      assert.deepEqual(
        stack.members.map((m) => m.id),
        ["image:1", "scene:1", "image:2", "scene:2"]
      );
      // An explicit false must override the default, and toggling back
      // must restore one card per stack without changing selected members.
      await grouping.click();
      await page.waitForFunction(
        () =>
          document.querySelectorAll(
            ".media-list .image-card,.media-list .scene-card"
          ).length === 8
      );
      await grouping.click();
      await page.waitForFunction(
        () =>
          document.querySelectorAll(
            ".media-list .image-card,.media-list .scene-card"
          ).length === 5
      );
      assert.match(
        await page.locator(".media-list").innerText(),
        /matching members.*hidden member/s
      );
      assert.match(
        await page.locator(".media-list").innerText(),
        /3 selected members are hidden/
      );
      await card(page, "VIDEO", "1")
        .getByRole("button", { name: "Stack · 4", exact: true })
        .click();
      const expanded = page.locator(".visual-stack-expanded");
      await expanded.getByLabel("Select image:1", { exact: true }).uncheck();
      await expanded.getByLabel("Select image:1", { exact: true }).check();
      await expanded
        .getByRole("button", { name: "Preview scene:1", exact: true })
        .click();
      await page.locator(".Lightbox").waitFor();
      await selected(page, "scene:1");
      await player(page, "1");
      await page
        .getByRole("button", { name: "Next variant", exact: true })
        .click();
      await selected(page, "image:2");
      await page.waitForFunction(
        () =>
          document.querySelectorAll(".unified-media-native-scene-player")
            .length === 0
      );
      await page.keyboard.press("ArrowRight");
      await selected(page, "scene:2");
      await player(page, "2");
      await page.waitForTimeout(4300);
      await selected(page, "scene:2");
      assert.equal(
        await page.locator(".unified-media-native-scene-player").count(),
        1,
        "Video completion advanced the stack"
      );
      const strip = page.locator(".visual-stack-filmstrip");
      await strip
        .getByRole("button", { name: "Manage stack", exact: true })
        .click();
      await row(page, "image:2").waitFor();
      // The stack manager's pHash search is an actual native Images query,
      // with the first Image anchoring mixed stacks represented by a Video.
      const similarRequest = page.waitForRequest((request) => {
        const payload = request.postDataJSON?.();
        return (
          request.url().endsWith("/graphql") &&
          payload?.operationName === "FindImages" &&
          payload?.variables?.filter?.sort === "perceptual_similarity:5:1:phash"
        );
      });
      await modal(page)
        .getByRole("button", {
          name: "Find similar Images (pHash)",
          exact: true,
        })
        .click();
      await similarRequest;
      await modal(page).getByLabel("Maximum pHash distance").selectOption("3");
      await modal(page).getByLabel("Stack title (optional)").focus();
      await page.keyboard.press("ArrowLeft");
      await selected(page, "scene:2");
      await page.keyboard.press("Escape");
      await modal(page).waitFor({ state: "hidden" });
      await selected(page, "scene:2");
      await strip
        .getByRole("button", { name: "Manage stack", exact: true })
        .click();
      await row(page, "image:2").waitFor();
      await row(page, "image:2")
        .getByLabel("Representative image:2", { exact: true })
        .check();
      await save(page);
      await selected(page, "scene:2");
      await player(page, "2");
      assert.equal(
        await strip
          .getByRole("link", { name: "Open this member", exact: true })
          .getAttribute("href"),
        "/scenes/2"
      );
      const bounds = await strip.boundingBox();
      assert.ok(
        bounds &&
          bounds.x >= -1 &&
          bounds.x + bounds.width <= width + 1 &&
          bounds.y + bounds.height <= (width < 500 ? 844 : 1000) + 1,
        "Filmstrip extends beyond viewport"
      );
      if (shots)
        await page.screenshot({
          path: path.join(
            shots,
            `${engineName}-${width === 1440 ? "desktop" : "mobile"}.png`
          ),
        });
      await page.getByTitle("Close Lightbox", { exact: true }).click();
      await page.locator(".Lightbox").waitFor({ state: "hidden" });
      assert.equal(
        await page.locator(".unified-media-native-scene-player").count(),
        0
      );

      // A collapsed stack must remain expandable in Wall mode, just as it is
      // in Grid mode. Opening its badge must not navigate away or toggle the
      // representative. Individual hidden members still use typed IDs.
      await page
        .locator(".media-list button")
        .filter({ has: page.locator("svg[data-icon='square']") })
        .click();
      await page.locator(".media-wall .wall-item").first().waitFor();
      const wallBadge = page
        .locator(".media-wall .visual-stack-wall-control")
        .getByRole("button", { name: "Stack · 4", exact: true });
      await wallBadge.waitFor();
      const wallURL = page.url();
      await wallBadge.click();
      const wallMembers = page.locator(".visual-stack-expanded");
      await wallMembers.getByLabel("Select image:1", { exact: true }).uncheck();
      assert.equal(await page.locator(".selected-count").innerText(), "3");
      await wallMembers.getByLabel("Select image:1", { exact: true }).check();
      assert.equal(await page.locator(".selected-count").innerText(), "4");
      assert.equal(page.url(), wallURL, "Stack badge click navigated away");
      assert.equal(
        await page.locator(".media-wall .wall-item").count(),
        5,
        "Expanding a stack unexpectedly changed the grouped Wall results"
      );
      await wallBadge.click();

      // Images-only Wall: the true Image representative must be present
      // exactly once. Opening its stack expands hidden Image/Video variants
      // without navigating away or changing the number of Wall cards.
      await page.goto(`${baseURL}/images`, { waitUntil: "domcontentloaded" });
      await page.locator(".image-list .image-card").first().waitFor();
      await page
        .locator(".image-list button")
        .filter({ has: page.locator("svg[data-icon='square']") })
        .click();
      await page.locator(".image-list .gallery .wall-item").first().waitFor();
      const imagesWall = page.locator(".image-list .gallery .wall-item");
      assert.equal(await imagesWall.count(), 3);
      const imageWallBadge = page
        .locator(".image-list .visual-stack-wall-control")
        .getByRole("button", { name: "Stack · 4", exact: true });
      await imageWallBadge.waitFor();
      const imageWallURL = page.url();
      await imageWallBadge.click();
      await page
        .locator('.visual-stack-expanded a[href="/images/1"]')
        .waitFor();
      assert.equal(page.url(), imageWallURL);
      assert.equal(await imagesWall.count(), 3);
      await imageWallBadge.click();

      await page.goto(`${baseURL}/images/1`);
      await selected(page, "image:1");
      await page
        .getByRole("button", { name: "Next variant", exact: true })
        .click();
      await page.waitForURL("**/scenes/1");
      await selected(page, "scene:1");
      await page
        .locator(".visual-stack-filmstrip")
        .getByRole("button", { name: "Manage stack", exact: true })
        .click();
      await row(page, "scene:2")
        .getByRole("button", { name: "Move scene:2 up", exact: true })
        .click();
      await save(page);
      stack = await getStack(stack.id);
      assert.deepEqual(
        stack.members.map((m) => m.id),
        ["image:1", "scene:1", "scene:2", "image:2"]
      );
      // Seed an actual pHash collision in two existing fixture Images.
      // The search below goes through SQLite and the real GraphQL endpoint,
      // and Save persists membership only after the user reviews the match.
      const imageFiles = [2, 3].map((id) => {
        const image = before.findImages.images.find(
          (item) => Number(item.id) === id
        );
        const file = image?.visual_files?.[0];
        assert.ok(file?.id, `Image ${id} must have a primary file`);
        return file;
      });
      assert.notEqual(imageFiles[0].id, imageFiles[1].id);
      for (const file of imageFiles) {
        const original = file.fingerprints.find(
          (fingerprint) => fingerprint.type === "phash"
        );
        originalPHashes.set(String(file.id), original?.value ?? null);
        await setPHash(file.id, "1a2b3c4d5e6f7081");
      }
      const nativeMatches = await gql(
        'query { findImages(filter:{per_page:20,sort:"perceptual_similarity:0:2:phash"}) { images { id } } }'
      );
      assert.ok(
        nativeMatches.findImages.images.some((image) => String(image.id) === "3"),
        "SQLite pHash similarity must return the seeded Image 3"
      );

      await page
        .locator(".visual-stack-filmstrip")
        .getByRole("button", { name: "Manage stack", exact: true })
        .click();
      await modal(page)
        .getByRole("button", {
          name: "Find similar Images (pHash)",
          exact: true,
        })
        .click();
      await modal(page).getByLabel("Maximum pHash distance").selectOption("0");
      const similarRow = modal(page)
        .locator(".visual-stack-similar-row")
        .filter({ has: page.locator('a[href="/images/3"]') });
      await similarRow.waitFor();
      assert.equal((await getStack(stack.id)).member_count, 4);
      await similarRow.getByRole("button", { name: "Add to stack" }).click();
      await row(page, "image:3").waitFor();
      assert.equal(
        (await getStack(stack.id)).member_count,
        4,
        "Suggested membership must not be committed before Save"
      );
      await save(page);
      assert.equal((await getStack(stack.id)).member_count, 5);
      await page
        .locator(".visual-stack-filmstrip")
        .getByRole("button", { name: "Manage stack", exact: true })
        .click();
      await row(page, "scene:2")
        .getByLabel("Split scene:2", { exact: true })
        .check();
      await row(page, "image:3")
        .getByLabel("Split image:3", { exact: true })
        .check();
      await modal(page)
        .getByRole("button", {
          name: "Split selected members into a new stack",
          exact: true,
        })
        .click();
      await modal(page).waitFor({ state: "hidden" });
      const other = await stackFor("IMAGE", "3");
      created.add(other.id);
      assert.equal(other.member_count, 2);
      assert.equal((await getStack(stack.id)).member_count, 3);
      await page
        .locator(".visual-stack-filmstrip")
        .getByRole("button", { name: "Manage stack", exact: true })
        .click();
      await modal(page)
        .getByLabel("Add member by Image or Video URL")
        .fill("/images/3");
      await modal(page)
        .getByRole("button", { name: "Review member", exact: true })
        .click();
      await modal(page)
        .getByRole("button", {
          name: "Merge stacks (keep all members)",
          exact: true,
        })
        .click();
      await row(page, "scene:2").waitFor();
      assert.equal((await getStack(stack.id)).member_count, 5);
      assert.equal(await getStack(other.id), null);
      await row(page, "image:2")
        .getByRole("button", { name: "Remove image:2 from stack", exact: true })
        .click();
      await save(page);
      stack = await getStack(stack.id);
      assert.equal(stack.representative, "image:1");
      assert.equal(stack.member_count, 4);
      await page
        .locator(".visual-stack-filmstrip")
        .getByRole("button", { name: "Manage stack", exact: true })
        .click();
      await modal(page)
        .getByRole("button", { name: "Unstack all (keep media)", exact: true })
        .click();
      await modal(page).waitFor({ state: "hidden" });
      await page
        .locator(".visual-stack-filmstrip")
        .waitFor({ state: "hidden" });
      assert.equal(await getStack(stack.id), null);
      await restorePHashes();
      assert.deepEqual(
        await snapshot(),
        before,
        "Grouping changed native metadata, URLs, files, fingerprints or gallery membership"
      );
      assert.deepEqual(errors, []);
      console.log(
        `${engineName} ${width}px: default representative grouping, Grid and both Wall expansion controls, native pHash suggestion and staged Save, create, reorder, split, merge and unstack passed`
      );
    } catch (error) {
      if (shots)
        await page.screenshot({
          path: path.join("/tmp", `p08-${engineName}-${width}-failure.png`),
        });
      throw error;
    } finally {
      await restorePHashes();
      await page.close();
    }
  }
} finally {
  for (const id of created) {
    const stack = await getStack(id);
    if (stack)
      await gql(
        "mutation($input:VisualStackVersionInput!){visualStackDestroy(input:$input)}",
        { input: { id, version: stack.version } }
      );
  }
  await browser.close();
}
