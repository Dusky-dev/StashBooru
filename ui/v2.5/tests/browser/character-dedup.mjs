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
    const allGroups = Array.from({ length: 26 }, (_, i) => ({
      destination: { id: i * 2 + 1, name: "Hatsune Miku" },
      sources: [{ id: i * 2 + 2, name: "Miku Hatsune" }],
      copyrights: [{ id: 1, name: "Series" }],
      reason: "same Copyrights and full-name identity",
    }));
    let version = 0;
    const plan = {
      fingerprint: "a".repeat(64),
      scanned: 60,
      groups: [...allGroups],
      ignored: [],
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
        const input = route.request().postDataJSON();
        if (input.action === "merge") {
          applies.push(input);
          await route.fulfill({ json: { jobID: 42 } });
          return;
        }
        if (input.action === "exclude") {
          const rejected = plan.groups.filter((g) =>
            input.selected.includes(g.destination.id)
          );
          plan.groups = plan.groups.filter(
            (g) => !input.selected.includes(g.destination.id)
          );
          plan.ignored.push(
            ...rejected.map((g) => ({
              left: g.destination,
              right: g.sources[0],
            }))
          );
        } else if (input.action === "restore") {
          for (const pair of input.pairs) {
            const index = plan.ignored.findIndex(
              (v) => v.left.id === pair.leftId && v.right.id === pair.rightId
            );
            assert.notEqual(index, -1);
            plan.ignored.splice(index, 1);
            plan.groups.push(
              allGroups.find((g) => g.destination.id === pair.leftId)
            );
          }
          plan.groups.sort((a, b) => a.destination.id - b.destination.id);
        } else {
          throw new Error(`Unexpected action ${input.action}`);
        }
        version++;
        plan.fingerprint = String.fromCharCode(97 + version).repeat(64);
      }
      await route.fulfill({ json: plan });
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
    await page.locator("#tasks-profiles > summary").click();
    const section = page.locator("#character-dedup-task");
    assert.equal(
      await section.locator('h2 a[href="/help/Tasks.md"]').count(),
      1
    );
    await section
      .getByRole("button", { name: "Preview Character merges" })
      .click();
    await section
      .getByText("60 Characters checked", { exact: false })
      .waitFor();
    assert.equal(await section.locator("tbody tr").first().count(), 1);
    assert.equal(
      await section.locator("table").first().locator("tbody tr").count(),
      25
    );
    assert.equal(
      await section
        .getByRole("button", { name: "Merge 0 selected groups" })
        .isDisabled(),
      true
    );

    await section.locator("#character-merge-1").check();
    await section
      .getByRole("button", { name: "Next", exact: true })
      .first()
      .click();
    assert.equal(
      await section.locator("table").first().locator("tbody tr").count(),
      1
    );
    await section.locator("#character-merge-51").check();
    await section
      .getByRole("button", { name: "Mark selected as not duplicates" })
      .click();
    await section.getByText("Not duplicates (2)").waitFor();
    assert.equal(plan.groups.length, 24);
    await section.getByText("Not duplicates (2)").click();
    await section
      .getByRole("button", { name: "Suggest again" })
      .first()
      .click();
    await section.getByText("Not duplicates (1)").waitFor();
    assert.equal(plan.groups.length, 25);

    await section.getByText("Skipped Characters (1)").click();
    await section
      .getByText("short name is ambiguous", { exact: false })
      .waitFor();
    assert.equal(await section.locator('a[href="/copyrights/1"]').count(), 25);
    await section.scrollIntoViewIfNeeded();
    assert.equal(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth + 1
      ),
      true
    );
    if (shots) {
      await section.screenshot({
        path: `${shots}/character-merges-${name}.png`,
      });
    }

    // The restored pair can be selected; only that group is submitted.
    await section.locator("#character-merge-1").check();
    const mergeFingerprint = plan.fingerprint;
    await section
      .getByRole("button", { name: "Merge 1 selected groups" })
      .click();
    await section
      .getByText("Merge job #42 queued.", { exact: false })
      .waitFor();
    assert.deepEqual(applies, [
      {
        fingerprint: mergeFingerprint,
        action: "merge",
        selected: [1],
        pairs: [],
      },
    ]);
    failure = true;
    await section
      .getByRole("button", { name: "Preview Character merges" })
      .click();
    await section.getByRole("alert").waitFor();
    assert.equal(
      await section.getByRole("button", { name: /Merge .*selected/ }).count(),
      0
    );
    failure = false;
    await section
      .getByRole("button", { name: "Preview Character merges" })
      .click();
    await section
      .getByRole("button", { name: "Merge 0 selected groups" })
      .waitFor();
    assert.deepEqual(errors, []);
    await page.evaluate(() => window.settingsFixture.dispose());
    await page.close();
    console.log(
      `${name}: Character preview, cross-page selection, persistent rejections, restore, partial apply, help, error recovery and layout passed`
    );
  }
} finally {
  await browser.close();
  await server.close();
}
