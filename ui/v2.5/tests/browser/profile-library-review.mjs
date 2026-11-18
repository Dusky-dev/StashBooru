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
      name: "profile-library-review-fixture",
      configureServer(vite) {
        vite.middlewares.use((req, _res, next) => {
          if (
            /^\/(?:stash\/)?(?:settings|performers\/7|studios\/7(?:\/edit)?|copyrights\/7)$/.test(
              req.url?.split("?")[0] ?? ""
            )
          )
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
let currentPage;
try {
  for (const [layout, viewport, prefix] of [
    ["desktop", { width: 1440, height: 1000 }, ""],
    ["mobile", { width: 390, height: 844 }, "/stash"],
  ]) {
    for (const [kind, route] of [
      ["character", "performers"],
      ["artist", "studios"],
      ["copyright", "copyrights"],
    ]) {
      const context = await browser.newContext({ viewport });
      const errors = [];
      context.on("page", (page) =>
        page.on("pageerror", (error) => errors.push(error.message))
      );
      const requests = [];
      await context.route(/\/character-dedup(?:\?|$)/, async (route) => {
        requests.push(route.request().method());
        return route.fulfill({ json: {} });
      });
      const page = await context.newPage();
      currentPage = page;
      const originalURL = `${origin}${prefix}/${route}/7?view=retained&page=2`;
      await page.goto(originalURL);
      const sources = page.locator(".identity-sources");
      await sources
        .locator("summary")
        .getByText("Identities", { exact: true })
        .waitFor();
      const group = page.locator(".profile-library-review");
      if (kind !== "character") {
        assert.equal(
          await group.count(),
          0,
          "Only Character profiles link to a library-wide review"
        );
        assert.deepEqual(errors, [], errors.join("\n"));
        await context.close();
        console.log(`${layout} ${kind}: Identities panel, no library review`);
        continue;
      }
      await group.locator("summary").focus();
      await group.locator("summary").press("Enter");
      await page
        .getByLabel("Fixture draft", { exact: true })
        .fill(`Retain ${kind} note`);
      assert.equal(await group.getByRole("link").count(), 1);
      const sourceBox = await sources.boundingBox();
      const reviewBox = await group.boundingBox();
      assert.ok(
        reviewBox.y >= sourceBox.y + sourceBox.height - 1,
        "Library review must occupy its own row below Identities"
      );
      assert.equal(
        await page.evaluate(
          () => document.documentElement.scrollWidth > innerWidth + 1
        ),
        false
      );
      if (shots)
        await page.screenshot({
          path: `${shots}/profile-review-${kind}-${layout}.png`,
          fullPage: true,
        });
      const target = "character-dedup-task";
      const link = group.getByRole("link", {
        name: "Review duplicate Characters (whole library)",
        exact: true,
      });
      assert.equal(
        await link.getAttribute("href"),
        `${prefix}/settings?tab=tasks#${target}`
      );
      assert.equal(await link.getAttribute("target"), "_blank");
      const opened = context.waitForEvent("page");
      await link.click();
      const review = await opened;
      await review.waitForFunction(
        (id) => document.getElementById(id)?.closest("details")?.open,
        target
      );
      await review.locator(`#${target}`).waitFor();
      assert.equal(
        await review.evaluate(
          () => document.documentElement.scrollWidth > innerWidth + 1
        ),
        false
      );
      await review.close();
      assert.equal(page.url(), originalURL);
      assert.equal(
        await page.getByLabel("Fixture draft", { exact: true }).inputValue(),
        `Retain ${kind} note`
      );
      assert.equal(
        requests.some((method) => method !== "GET"),
        false,
        "Opening the review must not run a task"
      );
      assert.deepEqual(errors, [], errors.join("\n"));
      await context.close();
      console.log(
        `${layout} ${kind}: Identities panel, duplicate review link, URL prefix, task anchor and origin retention passed`
      );
    }
    const context = await browser.newContext({ viewport });
    await context.route("**/graphql", async (route) => {
      const request = route.request();
      const headers = {
        "Access-Control-Allow-Origin": request.headers().origin ?? origin,
        "Access-Control-Allow-Credentials": "true",
        "Access-Control-Allow-Headers": "content-type",
        "Access-Control-Allow-Methods": "POST, OPTIONS",
      };
      if (request.method() === "OPTIONS") {
        await route.fulfill({ status: 204, headers });
        return;
      }
      const { operationName } = request.postDataJSON();
      assert.ok(
        ["FindStudiosForSelect", "FindTagsForSelect"].includes(operationName),
        `Unexpected editor operation: ${operationName}`
      );
      await route.fulfill({
        headers,
        json: {
          data:
            operationName === "FindStudiosForSelect"
              ? { findStudios: { count: 0, studios: [] } }
              : { findTags: { count: 0, tags: [] } },
        },
      });
    });
    const page = await context.newPage();
    currentPage = page;
    const errors = [];
    page.on("pageerror", (error) => {
      errors.push(error.message);
      console.error(error.stack);
    });
    await page.goto(`${origin}${prefix}/studios/7/edit`);
    const parent = page.locator('[data-field="parent_id"]');
    const children = page.locator('[data-field="child_ids"]');
    await parent.getByText("Parent Artist", { exact: true }).waitFor();
    await children.getByText("Child Artists", { exact: true }).waitFor();
    await page.addScriptTag({ url: `${origin}/builtin/bundle-extra.js` });
    await page.locator("#bundle-extra").waitFor();
    for (const field of [parent, children]) {
      assert.equal(await field.isVisible(), true);
      assert.equal(
        await field.evaluate((node) => node.closest("#bundle-extra") === null),
        true
      );
    }
    await parent.getByText("Parent profile", { exact: true }).waitFor();
    await children.getByText("Child profile", { exact: true }).waitFor();
    if (shots) {
      await parent.scrollIntoViewIfNeeded();
      await page.screenshot({
        path: path.join(shots, `artist-hierarchy-${layout}.png`),
      });
    }
    await page.locator("#name").fill("Edited Artist");
    await page.getByRole("button", { name: "Save", exact: true }).click();
    await page.waitForFunction(() =>
      window.settingsFixture.changes.some((change) => change.artistInput)
    );
    const saved = await page.evaluate(
      () =>
        window.settingsFixture.changes.find((change) => change.artistInput)
          .artistInput
    );
    assert.equal(saved.name, "Edited Artist");
    assert.equal(saved.parent_id, "2");
    assert.deepEqual(saved.child_ids, ["3"]);
    assert.equal(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth + 1
      ),
      true
    );
    assert.deepEqual(errors, []);
    await context.close();
    console.log(
      `${layout}: native Artist hierarchy stays visible with built-ins loaded; save retains parent/child IDs`
    );
  }
} catch (error) {
  if (currentPage && !currentPage.isClosed()) {
    console.error(await currentPage.locator("body").innerText());
    if (shots)
      await currentPage.screenshot({
        path: `${shots}/profile-review-failure.png`,
      });
  }
  throw error;
} finally {
  await browser.close();
  await server.close();
}
