import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { createServer } from "vite";
const { chromium } = await import(
  process.env.PLAYWRIGHT_MODULE ?? "playwright"
);
const server = await createServer({
  root: path.resolve(import.meta.dirname, "../.."),
  server: { host: "127.0.0.1", port: 0 },
  plugins: [
    {
      name: "tasks-settings-fixture",
      configureServer(vite) {
        vite.middlewares.use((req, _res, next) => {
          if (req.url?.split("?")[0] === "/settings")
            req.url = `/tests/browser/fixtures/settings-maintenance.html${req.url.includes("?") ? req.url.slice(req.url.indexOf("?")) : ""}`;
          next();
        });
      },
    },
  ],
});
await server.listen();
const origin = `http://127.0.0.1:${server.httpServer.address().port}`;
const browser = await chromium.launch({
  headless: true,
  executablePath: process.env.CHROMIUM_EXECUTABLE,
  args: ["--no-sandbox", "--disable-dev-shm-usage", "--disable-gpu"],
});
const shots = process.env.STASH_BROWSER_SCREENSHOTS;
if (shots) fs.mkdirSync(shots, { recursive: true });
const groups = ["library", "profiles", "media", "maintenance", "plugins"];
const summary = (page, group) => page.locator(`#tasks-${group} > summary`);
const isOpen = (page, group) =>
  page.locator(`#tasks-${group}`).evaluate((el) => el.open);
const fit = async (page) =>
  assert.equal(
    await page.evaluate(
      () => document.documentElement.scrollWidth > innerWidth + 1
    ),
    false
  );
async function chooseSection(page, tab) {
  if (await page.locator("#settings-section").isVisible())
    await page.locator("#settings-section").selectOption(tab);
  else
    await page
      .locator(`#settings-menu-container a[href*="tab=${tab}"]`)
      .click();
}
let currentPage;
try {
  for (const [name, viewport] of [
    ["desktop", { width: 1440, height: 1000 }],
    ["mobile", { width: 390, height: 844 }],
  ]) {
    const page = await browser.newPage({ viewport });
    currentPage = page;
    const errors = [];
    page.on("pageerror", (e) => errors.push(e.message));
    page.on("console", (msg) => {
      if (/unmounted component|Maximum update depth/i.test(msg.text()))
        errors.push(msg.text());
    });
    let releasePreview;
    let previewStarted;
    let previews = 0;
    const posts = [];
    const defaults = {
      characters: true,
      artists: true,
      copyrights: true,
      tags: true,
    };
    await page.route("**/association-inheritance*", (route) =>
      route.fulfill({ json: { current: defaults } })
    );
    await page.route("**/character-dedup", async (route) => {
      if (route.request().method() !== "GET")
        throw new Error("Unexpected Character mutation");
      previews++;
      previewStarted?.();
      await new Promise((resolve) => {
        releasePreview = resolve;
      });
      await route.fulfill({
        json: {
          fingerprint: "a".repeat(64),
          scanned: 2,
          groups: [
            {
              destination: { id: 1, name: "Character A" },
              sources: [{ id: 2, name: "Alias A" }],
              copyrights: [{ id: 1, name: "Series A" }],
              reason: "Fixture review",
            },
          ],
          skipped: [],
        },
      });
    });
    await page.route("**/image/converter", (route) => {
      posts.push(route.request().postDataJSON());
      return route.fulfill({
        status: 503,
        body: "Animation worker unavailable",
      });
    });
    await page.goto(`${origin}/settings?tab=tasks&return=images%2F42`);
    await page
      .locator("#job-queue")
      .getByRole("heading", { name: "Task Queue", exact: true })
      .waitFor();
    assert.equal(
      await page
        .locator("#job-queue")
        .evaluate((el) =>
          el.nextElementSibling.classList.contains("tasks-panel-tasks")
        ),
      true
    );
    for (const group of groups) {
      assert.equal(await isOpen(page, group), false);
      assert.equal(await summary(page, group).isVisible(), true);
    }
    assert.equal(previews, 0);
    assert.deepEqual(posts, []);
    await fit(page);
    if (shots)
      await page.screenshot({
        path: `${shots}/tasks-overview-${name}.png`,
        fullPage: true,
      });

    // Native keyboard disclosure; closing during a request keeps its result.
    await summary(page, "profiles").focus();
    await summary(page, "profiles").press("Enter");
    assert.equal(await isOpen(page, "profiles"), true);
    const started = new Promise((resolve) => {
      previewStarted = resolve;
    });
    await page
      .getByRole("button", { name: "Preview Character merges", exact: true })
      .click();
    await started;
    await summary(page, "profiles").click();
    releasePreview();
    await page
      .locator("#character-dedup-task tbody tr")
      .waitFor({ state: "attached" });
    await summary(page, "profiles").click();
    await page.getByText("2 Characters checked", { exact: false }).waitFor();
    assert.equal(previews, 1);
    const flag = page.locator("#review-inherit-artists");
    await flag.focus();
    await flag.press("Space");
    assert.equal(await flag.isChecked(), false);
    await summary(page, "profiles").focus();
    await summary(page, "profiles").press("Space");
    assert.equal(await isOpen(page, "profiles"), false);
    await summary(page, "profiles").press("Enter");
    assert.equal(await flag.isChecked(), false);
    await chooseSection(page, "library");
    await page.goBack();
    await page.waitForURL(/tab=tasks/);
    assert.equal(await isOpen(page, "profiles"), true);
    assert.equal(await flag.isChecked(), false);
    assert.equal(previews, 1);
    assert.equal(new URL(page.url()).searchParams.get("return"), "images/42");
    await fit(page);
    await page.waitForFunction(
      () =>
        getComputedStyle(document.querySelector(".tab-pane.active")).opacity ===
        "1"
    );
    if (shots)
      await page.screenshot({ path: `${shots}/tasks-review-${name}.png` });

    // Failed submissions remain visible after closing and reopening a group.
    await summary(page, "media").click();
    await page
      .getByRole("button", { name: "Inspect existing images", exact: true })
      .click();
    await page
      .getByText("Animation worker unavailable", { exact: true })
      .waitFor();
    await summary(page, "media").click();
    await summary(page, "media").click();
    await page
      .getByText("Animation worker unavailable", { exact: true })
      .waitFor();
    assert.deepEqual(posts, [{ action: "inspect-animations" }]);
    await summary(page, "maintenance").click();
    await page.locator("#backup").click();
    await page.locator(".modal.show").waitFor();
    await page
      .locator(".modal.show")
      .getByRole("button", { name: "Cancel", exact: true })
      .click();
    await page.locator(".modal.show").waitFor({ state: "detached" });
    await summary(page, "plugins").click();
    await page
      .getByText("Tasks from enabled plugins appear here when available.", {
        exact: true,
      })
      .waitFor();
    await fit(page);

    // Existing target IDs reveal the containing group and stay above the fold.
    for (const [hash, group] of [
      ["character-dedup-task", "profiles"],
      ["animation-inspection-task", "media"],
      ["backup", "maintenance"],
    ]) {
      await page.goto(
        `${origin}/settings?tab=tasks&return=images%2F42#${hash}`
      );
      await page.waitForFunction(
        (id) => document.getElementById(id)?.closest("details")?.open,
        hash
      );
      assert.equal(await isOpen(page, group), true);
      await page.waitForFunction((id) => {
        const y = document.getElementById(id)?.getBoundingClientRect().top;
        return y >= 0 && y < innerHeight - 30;
      }, hash);
      assert.equal(new URL(page.url()).searchParams.get("return"), "images/42");
      await fit(page);
    }
    await page.reload();
    await page.waitForFunction(
      () => document.querySelector("#tasks-maintenance")?.open
    );
    await page.goto(`${origin}/settings?tab=tasks#job-queue`);
    await page.locator("#job-queue").waitFor();
    for (const group of groups) assert.equal(await isOpen(page, group), false);
    await page.goto(`${origin}/settings?tab=tasks#path-metadata-matching`);
    await page.locator("#advanced-settings").focus();
    await page.locator("#advanced-settings").press("Space");
    await page.waitForFunction(
      () => document.querySelector("#tasks-library")?.open
    );
    await page.locator("#path-metadata-matching").waitFor();
    await fit(page);
    assert.deepEqual(errors, [], errors.join("\n"));
    await page.close();
    console.log(
      `${name}: Tasks grouping, pending responses/drafts, errors, Back, bookmarks, refresh and layout passed`
    );
  }
} catch (error) {
  if (currentPage && !currentPage.isClosed()) {
    console.error(await currentPage.locator("body").innerText());
    if (shots)
      await currentPage.screenshot({ path: `${shots}/tasks-failure.png` });
  }
  throw error;
} finally {
  await browser.close();
  await server.close();
}
