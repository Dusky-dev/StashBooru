import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { createServer } from "vite";

// Main page "On this day": the row asks for the viewer's own date, shows
// native Image cards newest first, and is hidden on days with nothing.
const engines = await import(process.env.PLAYWRIGHT_MODULE ?? "playwright");
const root = path.resolve(import.meta.dirname, "../..");
const server = await createServer({
  root,
  server: { host: "127.0.0.1", port: 0 },
});
await server.listen();
const baseURL = `http://127.0.0.1:${server.httpServer.address().port}`;
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
    const errors = [];
    page.on("pageerror", (e) => errors.push(e.message));
    const asked = [];
    let items = [
      { id: "3", date: "2023-07-14" },
      { id: "1", date: "2019-07-14" },
    ];
    await page.route("**/on-this-day?*", (route) => {
      asked.push(new URL(route.request().url()).searchParams);
      return route.fulfill({ json: { items } });
    });
    await page.goto(`${baseURL}/tests/browser/fixtures/on-this-day.html`);
    const row = page.locator(".recommendation-row");
    await row.getByRole("heading", { name: "On this day: Images" }).waitFor();
    const today = await page.evaluate(() => {
      const now = new Date();
      return [now.getFullYear(), now.getMonth() + 1, now.getDate()].map(String);
    });
    const params = asked[0];
    assert.deepEqual(
      [params.get("year"), params.get("month"), params.get("day")],
      today,
      "the viewer's own date is sent"
    );
    assert.equal(params.get("kind"), "image");
    await page.waitForFunction(
      () =>
        document.querySelectorAll(".recommendation-row .image-card").length >= 2
    );
    const titles = await row
      .locator(
        ".slick-slide:not(.slick-cloned) .image-card .card-section-title"
      )
      .allTextContents();
    assert.deepEqual(
      titles.map((t) => t.trim()),
      ["Image 3", "Image 1"],
      "newest first, as the server ordered them"
    );
    assert.equal(await row.getByRole("link", { name: "View all" }).count(), 0);
    if (shots)
      await page.screenshot({ path: `${shots}/on-this-day-${name}.png` });
    // The card strip stays inside the viewport (the slider's own arrows are
    // shared by every main-page row and sit at its edge).
    assert.ok(
      await page.evaluate(
        () =>
          document
            .querySelector(".recommendation-row .slick-list")
            .getBoundingClientRect().right <= window.innerWidth
      ),
      "cards overflow the viewport"
    );
    // Nothing on this day: no row at all.
    items = [];
    await page.reload();
    await page.getByRole("heading", { name: "On this day fixture" }).waitFor();
    for (let i = 0; i < 20 && asked.length < 2; i++)
      await page.waitForTimeout(100);
    await page.waitForTimeout(300);
    assert.equal(await page.locator(".recommendation-row").count(), 0);
    assert.deepEqual(errors, []);
    console.log(`${name}: On this day row order, empty day and layout passed`);
    await page.close();
  }
} finally {
  await browser.close();
  await server.close();
}
