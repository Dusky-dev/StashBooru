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
      name: "tools-settings-fixture",
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
const summary = (page, group) => page.locator(`#tools-${group} > summary`);
const isOpen = (page, group) =>
  page.locator(`#tools-${group}`).evaluate((el) => el.open);
const fit = async (page) =>
  assert.equal(
    await page.evaluate(
      () => document.documentElement.scrollWidth > innerWidth + 1
    ),
    false
  );
let currentPage;
try {
  for (const [name, viewport] of [
    ["desktop", { width: 1440, height: 1000 }],
    ["mobile", { width: 390, height: 844 }],
  ]) {
    const page = await browser.newPage({ viewport });
    currentPage = page;
    const errors = [];
    page.on("pageerror", (error) => errors.push(error.message));
    const requests = [];
    let failHealth = true;
    let releaseHealth;
    let healthStarted;
    await page.route("**/image/**", async (route) => {
      const request = route.request();
      const url = new URL(request.url());
      const body =
        request.method() === "POST" ? request.postDataJSON() : undefined;
      requests.push({ path: url.pathname, search: url.search, body });
      let data = {};
      if (url.pathname.endsWith("/metadata-health")) {
        if (failHealth)
          return route.fulfill({
            status: 503,
            body: "Metadata check unavailable",
          });
        healthStarted?.();
        await new Promise((resolve) => {
          releaseHealth = resolve;
        });
        data = {
          counts: { fixture: 1 },
          findings: [
            {
              code: "fixture",
              kind: "artist",
              entityIDs: [7],
              detail: "Review this Artist explicitly",
            },
          ],
        };
      } else if (url.pathname.endsWith("/alias-collisions/inspect"))
        data = {
          input: url.searchParams.get("value"),
          normalized: "same_name",
          ambiguous: true,
          matches: [
            {
              kind: "artist",
              entityID: 7,
              name: "Artist A",
              matchKinds: ["canonical"],
            },
            {
              kind: "artist",
              entityID: 8,
              name: "Artist B",
              matchKinds: ["alias"],
            },
          ],
        };
      else if (url.pathname.endsWith("/alias-collisions"))
        data = { counts: {}, collisions: [] };
      else if (url.pathname.endsWith("/clusters/copy-metadata"))
        data = {
          sourceImageID: 1,
          results: [
            {
              imageID: 2,
              added: { characters: 1, artists: 0, copyrights: 0, tags: 2 },
            },
          ],
        };
      else if (url.pathname.endsWith("/clusters"))
        data = {
          threshold: Number(url.searchParams.get("threshold")),
          neighbors: 10,
          indexedImages: 2,
          singletonsShown: true,
          clusters: [
            {
              ids: [1, 2],
              preferredID: 1,
              members: [
                {
                  id: 1,
                  width: 1000,
                  height: 1000,
                  fileSize: 2000,
                  preferred: true,
                },
                {
                  id: 2,
                  width: 500,
                  height: 500,
                  fileSize: 1000,
                  preferred: false,
                },
              ],
            },
          ],
        };
      else
        return route.fulfill({
          contentType: "image/svg+xml",
          body: '<svg xmlns="http://www.w3.org/2000/svg" width="1" height="1"/>',
        });
      return route.fulfill({ json: data });
    });
    await page.goto(`${origin}/settings?tab=tools&return=images%2F42`);
    await summary(page, "health").waitFor();
    for (const group of ["health", "similarity", "aliases", "developer"])
      assert.equal(await isOpen(page, group), false);
    assert.equal(
      requests.length,
      0,
      "Entering Tools must not start scans or model/provider work"
    );
    assert.equal(
      await page
        .getByRole("link", { name: "GraphQL playground", exact: true })
        .count(),
      0
    );
    assert.equal(
      await page
        .getByRole("link", { name: "Video filename parser", exact: true })
        .getAttribute("href"),
      "/sceneFilenameParser"
    );
    assert.equal(
      await page
        .getByRole("link", { name: "Video duplicate checker", exact: true })
        .getAttribute("href"),
      "/sceneDuplicateChecker"
    );
    await page.waitForFunction(
      () =>
        getComputedStyle(document.querySelector(".tab-pane.active")).opacity ===
        "1"
    );
    await fit(page);
    if (shots)
      await page.screenshot({
        path: `${shots}/tools-overview-${name}.png`,
        fullPage: true,
      });
    await summary(page, "health").focus();
    await summary(page, "health").press("Enter");
    assert.equal(requests.length, 0);
    await page
      .getByRole("button", { name: "Scan metadata", exact: true })
      .click();
    await page
      .getByText("Metadata check unavailable", { exact: true })
      .waitFor();
    await summary(page, "health").click();
    await summary(page, "health").click();
    await page
      .getByText("Metadata check unavailable", { exact: true })
      .waitFor();
    failHealth = false;
    const started = new Promise((resolve) => {
      healthStarted = resolve;
    });
    await page
      .getByRole("button", { name: "Scan metadata", exact: true })
      .click();
    await started;
    await summary(page, "health").click();
    releaseHealth();
    await page
      .getByText("Review this Artist explicitly", { exact: true })
      .waitFor({ state: "attached" });
    await summary(page, "health").click();
    await page
      .getByText("Review this Artist explicitly", { exact: true })
      .waitFor();
    assert.equal(
      requests.filter((r) => r.path.endsWith("/metadata-health")).length,
      2
    );

    await summary(page, "aliases").click();
    const alias = page.getByRole("textbox", {
      name: "Canonical name or alias to inspect",
      exact: true,
    });
    assert.notEqual(
      await alias.evaluate((el) => getComputedStyle(el).backgroundColor),
      "rgb(255, 255, 255)"
    );
    await alias.fill("same_name");
    await page
      .getByRole("button", { name: "Inspect value", exact: true })
      .click();
    await page.getByText("Artist A #7", { exact: true }).waitFor();
    await page.getByText("Artist B #8", { exact: true }).waitFor();
    await alias.fill("unsaved_draft");
    await summary(page, "aliases").click();
    await summary(page, "aliases").click();
    assert.equal(await alias.inputValue(), "unsaved_draft");
    await page.getByText("Artist A #7", { exact: true }).waitFor();
    if (await page.locator("#settings-section").isVisible())
      await page.locator("#settings-section").selectOption("library");
    else
      await page
        .locator('#settings-menu-container a[href*="tab=library"]')
        .click();
    await page.goBack();
    await page.waitForURL(/tab=tools/);
    assert.equal(await alias.inputValue(), "unsaved_draft");
    assert.equal(await isOpen(page, "aliases"), true);
    await page.getByText("Artist A #7", { exact: true }).waitFor();
    await page
      .getByRole("button", { name: "Scan all collisions", exact: true })
      .click();
    await page
      .getByText("No alias collisions found.", { exact: true })
      .waitFor();

    await summary(page, "similarity").click();
    await page.getByLabel("Similarity threshold", { exact: true }).fill("0.85");
    await page.getByLabel("Show singletons", { exact: true }).focus();
    await page.getByLabel("Show singletons", { exact: true }).press("Space");
    await page
      .getByRole("button", { name: "Find clusters", exact: true })
      .click();
    await page.getByText("Preferred Image #1", { exact: true }).waitFor();
    const clusterRequest = requests.find((r) => r.path.endsWith("/clusters"));
    assert.equal(
      new URLSearchParams(clusterRequest.search).get("threshold"),
      "0.85"
    );
    assert.equal(
      new URLSearchParams(clusterRequest.search).get("includeSingletons"),
      "true"
    );
    await page
      .getByRole("button", { name: "Select other Images", exact: true })
      .click();
    await summary(page, "similarity").click();
    await summary(page, "similarity").click();
    assert.equal(
      await page
        .getByLabel("Similarity threshold", { exact: true })
        .inputValue(),
      "0.85"
    );
    assert.equal(
      await page
        .getByLabel("Add preferred metadata", { exact: true })
        .isChecked(),
      true
    );
    page.once("dialog", (dialog) => dialog.dismiss());
    await page
      .getByRole("button", { name: "Add metadata to 1 selected", exact: true })
      .click();
    assert.equal(requests.filter((r) => r.body).length, 0);
    page.once("dialog", (dialog) => dialog.accept());
    await page
      .getByRole("button", { name: "Add metadata to 1 selected", exact: true })
      .click();
    await page.getByText("Added 1 Character", { exact: false }).waitFor();
    assert.deepEqual(
      requests.filter((r) => r.body).map((r) => r.body),
      [{ sourceImageID: 1, targetImageIDs: [2] }]
    );
    assert.equal(
      (await page.locator('#tools-similarity a[href="/images/1"]').count()) > 0,
      true
    );
    await fit(page);
    if (shots)
      await page.screenshot({ path: `${shots}/tools-results-${name}.png` });

    await summary(page, "developer").focus();
    await summary(page, "developer").press("Enter");
    const playground = page.getByRole("link", {
      name: "GraphQL playground",
      exact: true,
    });
    assert.equal(await playground.getAttribute("href"), "/playground");
    assert.equal(await playground.getAttribute("target"), "_blank");
    for (const group of ["aliases", "developer"]) {
      await page.goto(
        `${origin}/settings?tab=tools&return=images%2F42#tools-${group}`
      );
      await page.waitForFunction(
        (key) => document.querySelector(`#tools-${key}`)?.open,
        group
      );
      assert.equal(new URL(page.url()).searchParams.get("return"), "images/42");
      await fit(page);
    }
    await page.reload();
    await page.waitForFunction(
      () => document.querySelector("#tools-developer")?.open
    );
    await summary(page, "developer").focus();
    await summary(page, "developer").press("Space");
    assert.equal(await isOpen(page, "developer"), false);
    assert.deepEqual(errors, [], errors.join("\n"));
    await page.close();
    console.log(
      `${name}: Tools grouping, explicit reads/copy confirmation, retained results/drafts, links, bookmarks, refresh and layout passed`
    );
  }
} catch (error) {
  if (currentPage && !currentPage.isClosed()) {
    console.error(await currentPage.locator("body").innerText());
    if (shots)
      await currentPage.screenshot({ path: `${shots}/tools-failure.png` });
  }
  throw error;
} finally {
  await browser.close();
  await server.close();
}
