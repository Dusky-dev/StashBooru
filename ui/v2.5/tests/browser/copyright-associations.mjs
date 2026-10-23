import assert from "node:assert/strict";
import fs from "node:fs";

// Mutates an explicitly identified Copyright in an isolated fixture, then restores
// its original parents and shared inheritance defaults. Each parent must supply
// a distinct general Tag, separate from the fixture's other relationships.
const { chromium } = await import(
  process.env.PLAYWRIGHT_MODULE ?? "playwright"
);
const origin = process.env.STASH_BROWSER_URL;
const copyright = process.env.STASH_BROWSER_COPYRIGHT_ID;
const oldParent = process.env.STASH_BROWSER_OLD_PARENT_ID;
const newParent = process.env.STASH_BROWSER_NEW_PARENT_ID;
const image = process.env.STASH_BROWSER_IMAGE_ID;
const scene = process.env.STASH_BROWSER_VIDEO_ID;
assert.ok(
  origin && copyright && oldParent && newParent && image && scene,
  "Set URL, Copyright, old/new parent and Image/Video IDs for an isolated fixture"
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
const profileQuery = `query Profiles($copyright:ID!,$old:ID!,$next:ID!) {
  copyright:findCopyright(id:$copyright){id parents{id}}
  old:findCopyright(id:$old){id name tags{id name}}
  next:findCopyright(id:$next){id name tags{id name}}
}`;
const parentMutation = `mutation Parent($input:CopyrightUpdateInput!){copyrightUpdate(input:$input){id}}`;
const mediaQuery = (kind) =>
  `query Media($id:ID!){media:find${kind}(id:$id){id artists{id} performers{id} copyrights{id} tags{id} effective_associations{copyrights{id name} tags{id name}}}}`;
const direct = ({ effective_associations: _derived, ...native }) => native;
const defaultsQuery = `query Defaults{configuration{defaults{associationInheritance{characters artists copyrights tags}}}}`;
const defaultsMutation = `mutation Restore($input:ConfigDefaultSettingsInput!){configureDefaults(input:$input){associationInheritance{characters artists copyrights tags}}}`;
async function toggleInheritance(page, enabled, mediaPath) {
  await page.keyboard.press("g");
  await page.keyboard.press("z");
  await page.waitForURL((url) => url.pathname === "/settings");
  const chooser = page.getByLabel("Settings section", { exact: true });
  await chooser.waitFor({ state: "attached" });
  if (await chooser.isVisible()) await chooser.selectOption("library");
  else await page.getByRole("tab", { name: "Library", exact: true }).click();
  const toggle = page.locator("#inherit-copyright-ancestors");
  assert.equal(await toggle.isChecked(), !enabled);
  const [response] = await Promise.all([
    page.waitForResponse(
      (r) =>
        r.request().method() === "POST" &&
        r.request().postDataJSON()?.operationName === "ConfigureDefaults"
    ),
    // Bootstrap's input is transparent; click the visible switch track. Let
    // Playwright check hit targets rather than forcing a click through the nav.
    toggle.locator("..").click({ position: { x: 12, y: 12 } }),
  ]);
  const result = await response.json();
  assert.equal(result.errors, undefined);
  assert.equal(
    result.data.configureDefaults.associationInheritance.copyrights,
    enabled
  );
  for (let n = 0; n < 3 && new URL(page.url()).pathname === "/settings"; n++)
    await page.goBack();
  await page.waitForURL((url) => url.pathname === mediaPath);
}

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
      let originalDefaults;
      let haveOriginal = false;
      try {
        originalDefaults = (await call(page, defaultsQuery)).configuration
          .defaults.associationInheritance;
        await call(page, defaultsMutation, {
          input: {
            associationInheritance: { ...originalDefaults, copyrights: true },
          },
        });
        const profiles = await call(page, profileQuery, {
          copyright,
          old: oldParent,
          next: newParent,
        });
        assert.ok(profiles.copyright && profiles.old && profiles.next);
        originalParent = profiles.copyright.parents.map((p) => p.id);
        haveOriginal = true;
        const oldTag = profiles.old.tags.find(
          (t) => !profiles.next.tags.some((n) => n.id === t.id)
        );
        const newTag = profiles.next.tags.find(
          (t) => !profiles.old.tags.some((n) => n.id === t.id)
        );
        assert.ok(oldTag && newTag, "Give each parent a distinct general Tag");
        await call(page, parentMutation, {
          input: { id: copyright, parent_ids: [oldParent] },
        });
        const before = (await call(page, mediaQuery(kind), { id })).media;
        assert.ok(
          before.copyrights.some((a) => a.id === copyright),
          "Fixture must directly select the edited Copyright"
        );
        assert.ok(
          !before.effective_associations.tags.some((t) => t.id === newTag.id),
          "The new Copyright Tag must not come from another fixture relationship"
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
        await toggleInheritance(page, false, `/${path}/${id}`);
        await tags
          .getByRole("link", { name: oldTag.name, exact: true })
          .waitFor({ state: "hidden", timeout: 10000 });
        const disabled = (await call(page, mediaQuery(kind), { id })).media;
        assert.ok(
          !disabled.effective_associations.tags.some((t) => t.id === oldTag.id)
        );
        assert.deepEqual(direct(disabled), direct(before));
        await page
          .locator(`.booru-entity-card-name[href="/copyrights/${copyright}"]`)
          .waitFor();
        await toggleInheritance(page, true, `/${path}/${id}`);
        await tags
          .getByRole("link", { name: oldTag.name, exact: true })
          .waitFor();
        await page
          .locator(`.booru-entity-card-name[href="/copyrights/${copyright}"]`)
          .click();
        await page
          .getByRole("button", { name: "Edit", exact: true })
          .first()
          .click();
        const parents = page.locator("#copyright-edit .form-group").filter({
          has: page.locator("label").filter({ hasText: /^Main Copyrights$/ }),
        });
        for (const remove of await parents
          .getByRole("button", { name: /^Remove / })
          .all())
          await remove.click();
        await parents.locator("input").fill(profiles.next.name);
        await page
          .getByText(profiles.next.name, { exact: true })
          .last()
          .click();
        const saved = page.waitForResponse(
          (r) =>
            r.request().method() === "POST" &&
            r.request().postDataJSON()?.operationName === "CopyrightUpdate"
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
            .locator(`.booru-entity-card-name[href="/copyrights/${oldParent}"]`)
            .count(),
          0
        );
        await page
          .locator(`.booru-entity-card-name[href="/copyrights/${newParent}"]`)
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
            path: `${shots}/copyright-return-${kind}-${name}.png`,
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
          `${name}/${kind}: Copyright reparent and inheritance toggles, Back without refresh, Tags/provenance and native relationships passed`
        );
      } catch (error) {
        if (shots)
          await page
            .screenshot({
              path: `${shots}/copyright-return-${kind}-${name}-failure.png`,
              fullPage: true,
            })
            .catch(() => {});
        throw error;
      } finally {
        try {
          if (haveOriginal)
            await call(page, parentMutation, {
              input: { id: copyright, parent_ids: originalParent },
            });
        } finally {
          try {
            if (originalDefaults)
              await call(page, defaultsMutation, {
                input: { associationInheritance: originalDefaults },
              });
          } finally {
            await page.close();
          }
        }
      }
    }
  }
} finally {
  await browser.close();
}
