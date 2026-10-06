import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { createServer } from "vite";
const engines = await import(process.env.PLAYWRIGHT_MODULE ?? "playwright");
const root = path.resolve(import.meta.dirname, "../..");
const server = await createServer({
  root,
  server: { host: "127.0.0.1", port: 0 },
  plugins: [
    {
      name: "character-task-fixture",
      configureServer(vite) {
        vite.middlewares.use((req, _res, next) => {
          if (req.url?.split("?")[0] === "/settings")
            req.url =
              "/tests/browser/fixtures/settings-maintenance.html?tab=tasks";
          next();
        });
      },
    },
  ],
});
await server.listen();
const url = `http://127.0.0.1:${server.httpServer.address().port}/settings?tab=tasks`;
const browser = await engines.chromium.launch({
  headless: true,
  executablePath: process.env.CHROMIUM_EXECUTABLE,
  args: ["--no-sandbox", "--disable-dev-shm-usage", "--disable-gpu"],
});
const shots = process.env.STASH_BROWSER_SCREENSHOTS;
if (shots) fs.mkdirSync(shots, { recursive: true });
try {
  for (const [name, viewport] of [
    ["desktop", { width: 1440, height: 1000 }],
    ["mobile", { width: 390, height: 844 }],
  ]) {
    const page = await browser.newPage({ viewport });
    page.setDefaultTimeout(60000);
    page.setDefaultNavigationTimeout(90000);
    const errors = [];
    page.on("pageerror", (e) => errors.push(e.message));
    let failure = false;
    const applies = [];
    const plan = {
      fingerprint: "a".repeat(64),
      scanned: 60,
      groups: Array.from({ length: 26 }, (_, i) => ({
        destination: { id: i * 2 + 1, name: "Hatsune Miku" },
        sources: [{ id: i * 2 + 2, name: "Miku Hatsune" }],
        copyrights: [{ id: 1, name: "Series" }],
        reason: "same Copyrights and full-name identity",
      })),
      skipped: [
        {
          character: { id: 99, name: "Miku" },
          reason: "short name is ambiguous or has no unique full-name match",
        },
      ],
    };
    await page.route("**/character-dedup", async (route) => {
      if (failure) {
        await route.fulfill({
          status: 409,
          body: "Character catalogue changed; preview again before merging",
        });
        return;
      }
      if (route.request().method() === "POST") {
        applies.push(route.request().postDataJSON());
        await route.fulfill({ json: { jobID: 42 } });
      } else await route.fulfill({ json: plan });
    });
    await page.route("**/association-inheritance*", (route) =>
      route.fulfill({
        json: {
          current: {
            characters: true,
            artists: true,
            copyrights: true,
            tags: true,
          },
        },
      })
    );
    await page.goto(url);
    const section = page.locator("#character-dedup-task");
    await section
      .getByRole("button", { name: "Preview Character merges" })
      .click();
    await section
      .getByText("60 Characters checked", { exact: false })
      .waitFor();
    assert.equal(await section.locator("tbody tr").count(), 25);
    await section.getByRole("button", { name: "Next", exact: true }).click();
    assert.equal(await section.locator("tbody tr").count(), 1);
    await section.getByText("Skipped Characters (1)", { exact: true }).click();
    await section
      .getByText("short name is ambiguous", { exact: false })
      .waitFor();
    assert.equal(await section.locator('a[href="/copyrights/1"]').count(), 1);
    await section.scrollIntoViewIfNeeded();
    assert.equal(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth + 1
      ),
      true
    );
    if (shots)
      await section.screenshot({
        path: `${shots}/character-merges-${name}.png`,
      });
    await section
      .getByRole("button", { name: "Merge 26 reviewed groups" })
      .click();
    await section
      .getByText("Merge job #42 queued.", { exact: false })
      .waitFor();
    assert.deepEqual(applies, [{ fingerprint: plan.fingerprint }]);
    failure = true;
    await section
      .getByRole("button", { name: "Preview Character merges" })
      .click();
    await section.getByRole("alert").waitFor();
    assert.equal(
      await section.getByRole("button", { name: /Merge .*reviewed/ }).count(),
      0
    );
    failure = false;
    await section
      .getByRole("button", { name: "Preview Character merges" })
      .click();
    await section
      .getByRole("button", { name: "Merge 26 reviewed groups" })
      .waitFor();
    assert.deepEqual(errors, []);
    await page.evaluate(() => window.settingsFixture.dispose());
    await page.close();
    console.log(
      `${name}: Character preview, pagination, apply, error recovery, layout and Copyright task input passed`
    );
  }
} finally {
  await browser.close();
  await server.close();
}
