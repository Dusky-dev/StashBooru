import assert from "node:assert/strict";
import fs from "node:fs";

// Mutates an explicitly identified Artist in an isolated fixture, then restores
// its original parent. The parents must each supply a distinct general Tag.
const { chromium } = await import(
  process.env.PLAYWRIGHT_MODULE ?? "playwright"
);
const origin = process.env.STASH_BROWSER_URL;
const artist = process.env.STASH_BROWSER_ARTIST_ID;
const oldParent = process.env.STASH_BROWSER_OLD_PARENT_ID;
const newParent = process.env.STASH_BROWSER_NEW_PARENT_ID;
const image = process.env.STASH_BROWSER_IMAGE_ID;
const scene = process.env.STASH_BROWSER_VIDEO_ID;
assert.ok(
  origin && artist && oldParent && newParent && image && scene,
  "Set URL, Artist, old/new parent and Image/Video IDs for an isolated fixture"
);
assert.notEqual(oldParent, newParent);
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
const profileQuery = `query Profiles($artist:ID!,$old:ID!,$next:ID!) {
  artist:findStudio(id:$artist){id parent_studio{id}}
  old:findStudio(id:$old){id name tags{id name}}
  next:findStudio(id:$next){id name tags{id name}}
}`;
const parentMutation = `mutation Parent($input:StudioUpdateInput!){studioUpdate(input:$input){id}}`;
const mediaQuery = (kind) =>
  `query Media($id:ID!){media:find${kind}(id:$id){id artists{id} performers{id} copyrights{id} tags{id} effective_associations{artists{id name} tags{id name}}}}`;
const direct = ({ effective_associations: _derived, ...native }) => native;
try {
  for (const [name, viewport] of [
    ["desktop", { width: 1440, height: 1000 }],
    ["mobile", { width: 390, height: 844 }],
  ]) {
    for (const [kind, path, id] of [
      ["Image", "images", image],
      ["Scene", "scenes", scene],
    ]) {
      const page = await browser.newPage({ viewport });
      const errors = [];
      page.on("pageerror", (error) => errors.push(String(error)));
      let originalParent;
      let haveOriginal = false;
      try {
        const profiles = await call(page, profileQuery, {
          artist,
          old: oldParent,
          next: newParent,
        });
        assert.ok(profiles.artist && profiles.old && profiles.next);
        originalParent = profiles.artist.parent_studio?.id ?? null;
        haveOriginal = true;
        const oldTag = profiles.old.tags.find(
          (t) => !profiles.next.tags.some((n) => n.id === t.id)
        );
        const newTag = profiles.next.tags.find(
          (t) => !profiles.old.tags.some((n) => n.id === t.id)
        );
        assert.ok(oldTag && newTag, "Give each parent a distinct general Tag");
        await call(page, parentMutation, {
          input: { id: artist, parent_id: oldParent },
        });
        const before = (await call(page, mediaQuery(kind), { id })).media;
        assert.ok(
          before.artists.some((a) => a.id === artist),
          "Fixture must directly select the edited Artist"
        );
        assert.ok(
          !before.tags.some((t) => t.id === oldTag.id || t.id === newTag.id),
          "Parent Tags must be inherited, not directly selected"
        );
        await page.goto(`${origin}/${path}/${id}`, {
          waitUntil: "networkidle",
        });
        const tags = page.locator(".booru-tag-section-general");
        await tags
          .getByRole("link", { name: oldTag.name, exact: true })
          .waitFor();
        await page
          .locator(`.booru-entity-card-name[href="/studios/${artist}"]`)
          .click();
        await page
          .getByRole("button", { name: "Edit", exact: true })
          .first()
          .click();
        await page
          .locator('[data-field="parent_id"] input')
          .fill(profiles.next.name);
        await page
          .getByText(profiles.next.name, { exact: true })
          .last()
          .click();
        const saved = page.waitForResponse(
          (r) =>
            r.request().method() === "POST" &&
            r.request().postDataJSON()?.operationName === "StudioUpdate"
        );
        await page
          .getByRole("button", { name: "Save", exact: true })
          .first()
          .click();
        assert.equal((await (await saved).json()).errors, undefined);
        await page
          .getByRole("button", { name: "Edit", exact: true })
          .first()
          .waitFor();
        await page.goBack();
        await page.waitForURL((url) => url.pathname === `/${path}/${id}`);
        await tags
          .getByRole("link", { name: newTag.name, exact: true })
          .waitFor({ timeout: 10000 });
        assert.equal(
          await tags
            .getByRole("link", { name: oldTag.name, exact: true })
            .count(),
          0
        );
        assert.equal(
          await page
            .locator(`.booru-entity-card-name[href="/studios/${oldParent}"]`)
            .count(),
          0
        );
        await page
          .locator(`.booru-entity-card-name[href="/studios/${newParent}"]`)
          .waitFor();
        await page.locator(".booru-association-sources > summary").click();
        assert.ok(
          (
            await page.locator(".booru-association-sources").innerText()
          ).includes(profiles.next.name)
        );
        const after = (await call(page, mediaQuery(kind), { id })).media;
        assert.deepEqual(
          direct(after),
          direct(before),
          "Reparenting changed native media relationships"
        );
        if (shots)
          await page.screenshot({
            path: `${shots}/profile-return-${kind}-${name}.png`,
            fullPage: true,
          });
        await page.reload({ waitUntil: "networkidle" });
        await tags
          .getByRole("link", { name: newTag.name, exact: true })
          .waitFor();
        assert.deepEqual(
          direct((await call(page, mediaQuery(kind), { id })).media),
          direct(before)
        );
        assert.deepEqual(errors, []);
        console.log(
          `${name}/${kind}: Artist reparent, Back without refresh, inherited Tags/provenance and native relationships passed`
        );
      } catch (error) {
        if (shots)
          await page
            .screenshot({
              path: `${shots}/profile-return-${kind}-${name}-failure.png`,
              fullPage: true,
            })
            .catch(() => {});
        throw error;
      } finally {
        try {
          if (haveOriginal)
            await call(page, parentMutation, {
              input: { id: artist, parent_id: originalParent },
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
