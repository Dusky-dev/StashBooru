import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { createServer } from "vite";

// Real Welcome, release-note selection/dialog and Changelog; configuration is isolated.
const { chromium } = await import(
  process.env.PLAYWRIGHT_MODULE ?? "playwright"
);
const server = await createServer({
  root: path.resolve(import.meta.dirname, "../.."),
  server: { host: "127.0.0.1", port: 0 },
  plugins: [
    {
      name: "first-use-fixture",
      configureServer(vite) {
        vite.middlewares.use((req, _res, next) => {
          if (
            /^\/(?:stash\/)?(?:notes|welcome|settings)(?:\?|$)/.test(
              req.url ?? ""
            )
          )
            req.url = "/tests/browser/fixtures/first-use-notes.html";
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
const introduction = "StashBooru: getting started";
const guide =
  "https://github.com/Dusky-dev/StashBooru/blob/develop/docs/features.md";
try {
  for (const [name, viewport, prefix] of [
    ["desktop", { width: 1440, height: 1000 }, ""],
    ["mobile", { width: 390, height: 844 }, "/stash"],
  ]) {
    const context = await browser.newContext({ viewport });
    const page = await context.newPage();
    const errors = [];
    page.on("pageerror", (error) => errors.push(error.message));
    const writes = () =>
      page.evaluate(() =>
        window.firstUseFixture.calls.filter(
          (call) => call.name === "ConfigureUI"
        )
      );
    await page.goto(`${origin}${prefix}/notes`);
    const dialog = page.getByRole("dialog");
    await dialog
      .getByRole("heading", { name: introduction, exact: true })
      .waitFor();
    await dialog
      .getByRole("link", { name: "StashBooru feature guide", exact: true })
      .waitFor();
    assert.equal(await dialog.locator("h3").first().innerText(), introduction);
    await dialog
      .getByRole("heading", { name: "v0.29.0", exact: true })
      .waitFor();
    assert.equal(
      await dialog
        .getByRole("link", { name: "StashBooru feature guide", exact: true })
        .getAttribute("href"),
      guide
    );
    await page.keyboard.press("Escape");
    assert.equal(await dialog.isVisible(), true);
    assert.equal(
      (await writes()).length,
      0,
      "Opening or Escape cannot acknowledge notes"
    );
    await dialog
      .getByRole("heading", { name: introduction, exact: true })
      .scrollIntoViewIfNeeded();
    if (shots)
      await page.screenshot({
        path: path.join(shots, `first-use-notes-${name}.png`),
      });
    await dialog.getByRole("button", { name: "Close", exact: true }).click();
    await page
      .getByRole("heading", { name: "All notes acknowledged", exact: true })
      .waitFor();
    const acknowledged = (await writes()).at(-1).input;
    assert.equal(acknowledged.lastNoteSeen, 20261023);
    assert.equal(acknowledged.title, "Retained title");
    assert.equal(acknowledged.customCSS, "retained preference");
    await page.reload();
    await page
      .getByRole("heading", { name: "All notes acknowledged", exact: true })
      .waitFor();
    assert.equal(
      (await writes()).length,
      0,
      "Refresh keeps the saved marker without another write"
    );
    await page
      .getByRole("link", { name: "Changelog fixture", exact: true })
      .click();
    await page
      .getByRole("heading", { name: introduction, exact: true })
      .waitFor();
    await page
      .getByRole("link", { name: "StashBooru feature guide", exact: true })
      .waitFor();
    const oldVersion = page.getByRole("button", { name: /v0.29.3/ });
    await oldVersion.click();
    await page
      .locator(".changelog-version")
      .filter({ has: oldVersion })
      .getByText("The Scenes page and related scene list views", {
        exact: false,
      })
      .waitFor();
    await oldVersion.click();
    assert.equal(
      (await writes()).length,
      0,
      "Revisiting history does not change acknowledgements"
    );
    await page
      .getByRole("link", { name: "Welcome fixture", exact: true })
      .click();
    await page
      .getByRole("heading", { name: introduction, exact: true })
      .waitFor();
    await page
      .getByText("Finish opens Library settings.", { exact: false })
      .waitFor();
    assert.equal(
      await page
        .getByRole("link", { name: "StashBooru feature guide", exact: true })
        .getAttribute("href"),
      guide
    );
    const issueLink = page.locator(
      'a[href="https://github.com/Dusky-dev/StashBooru/issues"]'
    );
    assert.equal(await issueLink.count(), 1);
    if (shots) {
      await page
        .getByRole("heading", { name: introduction, exact: true })
        .scrollIntoViewIfNeeded();
      await page.screenshot({
        path: path.join(shots, `first-use-welcome-${name}.png`),
      });
    }
    await page.getByRole("button", { name: "Finish", exact: true }).click();
    await page.waitForURL(`${origin}${prefix}/settings?tab=library`);
    await page
      .getByRole("heading", {
        name: "Library destination fixture",
        exact: true,
      })
      .waitFor();
    assert.equal((await writes()).at(-1).input.lastNoteSeen, 20261023);
    assert.equal(
      await page.evaluate(() =>
        window.firstUseFixture.calls.some(
          (call) => call.name === "DownloadFFMpeg"
        )
      ),
      false
    );
    assert.equal(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth + 1
      ),
      true
    );
    assert.deepEqual(errors, []);
    await context.close();

    // Existing users who saw upstream notes receive only the newer fork entry.
    const upgrade = await browser.newContext({ viewport });
    await upgrade.addInitScript(() =>
      localStorage.setItem(
        "first-use-fixture-ui",
        JSON.stringify({ title: "Existing title", lastNoteSeen: 20251026 })
      )
    );
    const upgradePage = await upgrade.newPage();
    await upgradePage.goto(`${origin}${prefix}/notes`);
    await upgradePage
      .getByRole("heading", { name: introduction, exact: true })
      .waitFor();
    assert.equal(
      await upgradePage.getByRole("dialog").locator("h3").count(),
      1
    );
    await upgrade.close();
    console.log(
      `${name}: fork-first notes, explicit acknowledgement/refresh, retained UI settings, upstream history, Welcome and Finish destination passed`
    );
  }
} finally {
  await browser.close();
  await server.close();
}
