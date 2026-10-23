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
            /^\/(?:stash\/)?(?:settings|performers\/7|studios\/7|copyrights\/7)$/.test(
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
const profileName = "Name & alias/タグ #7?";
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
      await context.route(
        /\/(?:image\/alias-collisions\/inspect|association-inheritance|character-dedup)(?:\?|$)/,
        async (route) => {
          const request = route.request();
          const url = new URL(request.url());
          requests.push({
            method: request.method(),
            path: url.pathname,
            value: url.searchParams.get("value"),
          });
          if (url.pathname.endsWith("/inspect"))
            return route.fulfill({
              json: {
                input: url.searchParams.get("value"),
                normalized: "fixture-name",
                ambiguous: true,
                matches: [
                  {
                    kind,
                    entityID: 7,
                    name: "Profile A",
                    matchKinds: ["canonical"],
                  },
                  {
                    kind,
                    entityID: 8,
                    name: "Profile B",
                    matchKinds: ["alias"],
                  },
                ],
              },
            });
          return route.fulfill({
            json: {
              current: {
                characters: true,
                artists: true,
                copyrights: true,
                tags: true,
              },
            },
          });
        }
      );
      const page = await context.newPage();
      currentPage = page;
      const originalURL = `${origin}${prefix}/${route}/7?view=retained&page=2`;
      await page.goto(originalURL);
      const group = page.locator(".profile-library-review");
      await group.locator("summary").focus();
      await group.locator("summary").press("Enter");
      await page
        .getByLabel("Fixture draft", { exact: true })
        .fill(`Retain ${kind} note`);
      assert.equal(
        await group.getByRole("link").count(),
        kind === "character" ? 3 : 2
      );
      const sourceBox = await page.locator(".identity-sources").boundingBox();
      const reviewBox = await group.boundingBox();
      assert.ok(
        reviewBox.y >= sourceBox.y + sourceBox.height - 1,
        "Library review must occupy its own row below Sources & aliases"
      );
      const inspect = group.getByRole("link", {
        name: "Inspect this name",
        exact: true,
      });
      const destination = new URL(await inspect.getAttribute("href"), origin);
      assert.equal(destination.pathname, `${prefix}/settings`);
      assert.equal(destination.searchParams.get("alias"), profileName);
      assert.equal(destination.hash, "#tools-aliases");
      assert.equal(await inspect.getAttribute("target"), "_blank");
      assert.equal(
        requests.length,
        0,
        "Opening profile review must not run a task"
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
      let opened = context.waitForEvent("page");
      await inspect.click();
      let review = await opened;
      const alias = review.getByRole("textbox", {
        name: "Canonical name or alias to inspect",
        exact: true,
      });
      await alias.waitFor();
      assert.equal(await alias.inputValue(), profileName);
      assert.equal(
        requests.length,
        0,
        "Prefilling must not inspect or resolve identities"
      );
      await review
        .getByRole("button", { name: "Inspect value", exact: true })
        .click();
      await review.getByText("Profile A #7", { exact: true }).waitFor();
      await review.getByText("Profile B #8", { exact: true }).waitFor();
      assert.equal(requests[0].value, profileName);
      assert.equal(
        requests[0].path,
        `${prefix}/image/alias-collisions/inspect`
      );
      await review.close();
      assert.equal(page.url(), originalURL);
      assert.equal(
        await page.getByLabel("Fixture draft", { exact: true }).inputValue(),
        `Retain ${kind} note`
      );
      for (const [label, target] of [
        [
          "Review inherited associations (whole library)",
          "association-inheritance-task",
        ],
        ...(kind === "character"
          ? [
              [
                "Review duplicate Characters (whole library)",
                "character-dedup-task",
              ],
            ]
          : []),
      ]) {
        const link = group.getByRole("link", { name: label, exact: true });
        assert.equal(
          await link.getAttribute("href"),
          `${prefix}/settings?tab=tasks#${target}`
        );
        assert.equal(await link.getAttribute("target"), "_blank");
        opened = context.waitForEvent("page");
        await link.click();
        review = await opened;
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
      }
      assert.equal(
        requests.some((request) => request.method !== "GET"),
        false
      );
      assert.equal(
        requests.filter((request) => request.path.endsWith("/character-dedup"))
          .length,
        0
      );
      assert.deepEqual(errors, [], errors.join("\n"));
      await context.close();
      console.log(
        `${layout} ${kind}: native profile links, encoded name, explicit inspection, URL prefix, task anchors and origin retention passed`
      );
    }
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
