import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { createServer } from "vite";

// Mount native providers, viewer and Bootstrap controls against an isolated
// Apollo transport. No catalogue writes, server, workers or real media required.
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
const job = (id, status = "FAILED") => ({
  __typename: "Job",
  id,
  status,
  description: `Fixture job ${id}`,
  addTime: `2026-10-04T12:00:${id.padStart(2, "0")}Z`,
  startTime: null,
  endTime: null,
  subTasks: [],
  progress: null,
  error: status === "FAILED" ? `Failure ${id}\nDetail line` : null,
});
const emit = (page, value, type = "REMOVE") =>
  page.evaluate(({ value, type }) => window.qolFixture.emitJob(value, type), {
    value,
    type,
  });
const modal = (page) => page.locator(".modal.show");
const detail = (page) => page.locator("main .visual-stack-filmstrip");
const strip = (page) => page.locator(".Lightbox .visual-stack-filmstrip");
const row = (page, key) =>
  modal(page).locator(`.visual-stack-editor-member[data-member-id="${key}"]`);
const order = (page) =>
  modal(page)
    .locator(".visual-stack-editor-member")
    .evaluateAll((rows) => rows.map((r) => r.dataset.memberId));
const selected = (page, key) =>
  strip(page)
    .locator(`[data-member-id="${key}"][aria-pressed="true"]`)
    .waitFor();

async function drag(page, source, target, cancel = false) {
  const handle = row(page, source).getByRole("button", {
    name: `Drag ${source} to reorder`,
    exact: true,
  });
  await handle.scrollIntoViewIfNeeded();
  const from = await handle.boundingBox();
  const to = await row(page, target).boundingBox();
  if (page.viewportSize().width < 500) {
    const session = await page.context().newCDPSession(page);
    const x = from.x + from.width / 2;
    const y = from.y + from.height / 2;
    const endX = to.x + to.width / 2;
    const endY = to.y + to.height / 2;
    await session.send("Input.dispatchTouchEvent", {
      type: "touchStart",
      touchPoints: [{ x, y }],
    });
    for (let step = 1; step <= 8; step++) {
      await session.send("Input.dispatchTouchEvent", {
        type: "touchMove",
        touchPoints: [
          { x: x + ((endX - x) * step) / 8, y: y + ((endY - y) * step) / 8 },
        ],
      });
    }
    if (cancel) await page.keyboard.press("Escape");
    await session.send("Input.dispatchTouchEvent", {
      type: "touchEnd",
      touchPoints: [],
    });
    await session.detach();
    return;
  }
  await page.mouse.move(from.x + from.width / 2, from.y + from.height / 2);
  await page.mouse.down();
  await page.mouse.move(to.x + to.width / 2, to.y + to.height / 2, {
    steps: 8,
  });
  if (cancel) await page.keyboard.press("Escape");
  await page.mouse.up();
}

let activePage;
try {
  console.log(`Browser: ${browser.version()}`);
  for (const [name, viewport] of [
    ["desktop", { width: 1440, height: 1000 }],
    ["mobile", { width: 390, height: 844 }],
  ]) {
    const page = await browser.newPage({
      viewport,
      isMobile: name === "mobile",
      hasTouch: name === "mobile",
    });
    activePage = page;
    const errors = [];
    page.on("pageerror", (error) => errors.push(error.message));
    page.on("console", (message) => {
      if (/unmounted component|Maximum update depth/i.test(message.text()))
        errors.push(message.text());
    });
    await page.addInitScript(() => {
      Object.defineProperty(navigator, "clipboard", {
        configurable: true,
        value: {
          writeText: async (text) => {
            window.copiedError = text;
          },
        },
      });
    });
    await page.goto(`${baseURL}/tests/browser/fixtures/jobs-stacks.html`);
    await detail(page).waitFor();
    console.log(`${name}: jobs`);
    // A late snapshot and a burst of subscription events must not lose failures.
    await page.evaluate(
      (value) => {
        window.qolFixture.holdQueue = true;
        window.qolFixture.setQueue([value]);
      },
      job("1", "RUNNING")
    );
    await page
      .getByRole("button", { name: "Refresh jobs", exact: true })
      .click();
    await emit(page, job("1"));
    await emit(page, job("2"));
    await page.evaluate(() => window.qolFixture.releaseQueue());
    await page.getByText("Failure 1\nDetail line", { exact: true }).waitFor();
    await page.getByText("Failure 2\nDetail line", { exact: true }).waitFor();
    await page
      .locator(".job")
      .first()
      .getByRole("button", { name: "Copy error", exact: true })
      .click();
    assert.equal(await page.evaluate(() => window.copiedError), job("1").error);
    await page
      .getByRole("button", { name: "Toggle tasks", exact: true })
      .click();
    await emit(page, job("3"));
    await page
      .getByRole("button", { name: "Toggle tasks", exact: true })
      .click();
    assert.equal(await page.locator(".job").count(), 3);
    await page.reload();
    await page.getByText("Failure 3\nDetail line", { exact: true }).waitFor();
    await page
      .locator(".job")
      .first()
      .getByRole("button", { name: "Dismiss", exact: true })
      .click();
    await emit(page, job("1"));
    assert.equal(await page.locator(".job").count(), 2);
    await page
      .getByRole("button", { name: "Dismiss all failures", exact: true })
      .click();
    await page.getByText("No jobs queued.", { exact: true }).waitFor();
    await page.reload();
    await page.getByText("No jobs queued.", { exact: true }).waitFor();
    await emit(page, job("4", "RUNNING"), "ADD");
    await emit(page, job("5", "FINISHED"));
    await emit(page, job("6", "CANCELLED"));
    await emit(page, job("7"));
    await page
      .getByText("Fixture job 5", { exact: true })
      .waitFor({ state: "hidden", timeout: 15000 });
    assert.equal(await page.locator(".job").count(), 2);
    assert.equal(
      await page
        .getByText("Failure 7\nDetail line", { exact: true })
        .isVisible(),
      true
    );
    // Clipboard fallback reports success/failure and restores focus without leaking a textarea.
    await page.evaluate(() => {
      navigator.clipboard.writeText = async () => {
        throw new Error("Denied");
      };
      document.execCommand = () => {
        window.copiedError = document.querySelector("textarea").value;
        return true;
      };
    });
    const copy = page
      .locator(".job")
      .filter({ hasText: "Fixture job 7" })
      .getByRole("button", { name: "Copy error", exact: true });
    await copy.click();
    assert.equal(await page.evaluate(() => window.copiedError), job("7").error);
    assert.equal(await page.locator("textarea").count(), 0);
    assert.equal(
      await copy.evaluate((button) => button === document.activeElement),
      true
    );
    await page.evaluate(() => {
      document.execCommand = () => false;
    });
    await copy.click();
    await page.getByText(/Copy failed\. Select the error text/).waitFor();
    if (shots)
      await page.screenshot({
        path: path.join(shots, `jobs-${name}.png`),
        fullPage: true,
      });

    console.log(`${name}: draft reordering`);
    await page
      .getByRole("button", { name: "Open editor", exact: true })
      .click();
    await row(page, "image:1").waitFor();
    const initial = ["image:1", "scene:1", "image:2", "image:3"];
    assert.deepEqual(await order(page), initial);
    await drag(page, "image:1", "image:2", true);
    assert.deepEqual(await order(page), initial);
    assert.equal(
      await modal(page).isVisible(),
      true,
      "Escape cancelled a drag without closing the editor"
    );
    await drag(page, "image:1", "image:2");
    assert.deepEqual(await order(page), [
      "scene:1",
      "image:2",
      "image:1",
      "image:3",
    ]);
    assert.deepEqual(
      await page.evaluate(() =>
        window.qolFixture.snapshot().members.map((m) => m.id)
      ),
      initial,
      "Draft drag wrote the stack before Save"
    );
    assert.equal(
      await row(page, "image:1")
        .getByLabel("Representative image:1", { exact: true })
        .isChecked(),
      true
    );
    assert.equal(
      await row(page, "image:2").getByRole("combobox").inputValue(),
      "Restoration"
    );
    // The existing keyboard controls remain usable after a drag.
    const up = row(page, "image:1").getByRole("button", {
      name: "Move image:1 up",
      exact: true,
    });
    await up.focus();
    await page.keyboard.press("Enter");
    assert.deepEqual(await order(page), [
      "scene:1",
      "image:1",
      "image:2",
      "image:3",
    ]);
    if (shots)
      await page.screenshot({
        path: path.join(shots, `stack-order-${name}.png`),
        fullPage: true,
      });
    await modal(page)
      .getByRole("button", { name: "Save stack", exact: true })
      .click();
    await modal(page).waitFor({ state: "hidden" });
    assert.deepEqual(
      await page.evaluate(() =>
        window.qolFixture.snapshot().members.map((m) => m.id)
      ),
      ["scene:1", "image:1", "image:2", "image:3"]
    );
    // Cancel/drop outside cannot reorder or save a draft.
    await page
      .getByRole("button", { name: "Open editor", exact: true })
      .click();
    await row(page, "image:1").waitFor();
    const beforeOutside = await order(page);
    const handle = await row(page, "image:1")
      .getByRole("button", { name: "Drag image:1 to reorder", exact: true })
      .boundingBox();
    await page.mouse.move(handle.x + 10, handle.y + 10);
    await page.mouse.down();
    await page.mouse.move(1, 1);
    await page.mouse.up();
    assert.deepEqual(await order(page), beforeOutside);
    await modal(page)
      .getByRole("button", { name: "Cancel", exact: true })
      .click();
    await modal(page).waitFor({ state: "hidden" });

    console.log(`${name}: representative shortcuts and viewer cleanup`);
    await page.evaluate(() => {
      window.qolFixture.conflict = true;
    });
    await detail(page)
      .getByRole("button", { name: "Make this representative", exact: true })
      .click();
    await detail(page)
      .getByText(/Stack changed\. Reload/)
      .waitFor();
    assert.equal(
      await page.evaluate(() => window.qolFixture.snapshot().representative),
      "image:1"
    );
    await page.evaluate(() => {
      window.qolFixture.conflict = false;
    });
    await detail(page)
      .getByRole("button", { name: "Reload stack", exact: true })
      .click();
    await detail(page)
      .getByRole("button", { name: "Make this representative", exact: true })
      .click();
    await page.waitForFunction(
      () => window.qolFixture.snapshot().representative === "image:2"
    );
    const saved = await page.evaluate(() => window.qolFixture.mutations.at(-1));
    assert.deepEqual(
      saved.members.map((m) => `${m.media.kind}:${m.media.id}`),
      ["VIDEO:1", "IMAGE:1", "IMAGE:2", "IMAGE:3"]
    );
    assert.deepEqual(
      saved.members.map((m) => m.label),
      ["Converted copy", "Original", "Restoration", "Edited variant"]
    );
    assert.equal(
      await detail(page)
        .getByRole("button", { name: "Make this representative", exact: true })
        .isDisabled(),
      true
    );
    await page
      .getByRole("button", { name: "Switch detail kind", exact: true })
      .click();
    assert.equal(
      await detail(page)
        .getByRole("button", {
          name: "Compare with representative",
          exact: true,
        })
        .isDisabled(),
      true
    );
    await detail(page)
      .getByRole("button", { name: "Make this representative", exact: true })
      .click();
    await page.waitForFunction(
      () => window.qolFixture.snapshot().representative === "scene:1"
    );
    await page
      .getByRole("button", { name: "Switch detail kind", exact: true })
      .click();
    await detail(page)
      .getByRole("button", { name: "Make this representative", exact: true })
      .click();
    await page.waitForFunction(
      () => window.qolFixture.snapshot().representative === "image:2"
    );

    await page
      .getByRole("button", { name: "Open preview", exact: true })
      .click();
    await selected(page, "image:2");
    await strip(page).locator('[data-member-id="image:1"]').click();
    await selected(page, "image:1");
    await strip(page)
      .getByRole("button", { name: "Compare with representative", exact: true })
      .click();
    await page.locator(".Lightbox-reference-comparison").waitFor();
    assert.equal(await page.locator(".Lightbox").count(), 1);
    assert.equal(await page.locator(".Lightbox-carousel").count(), 0);
    await page.evaluate(() => window.qolFixture.refreshMetadata());
    await selected(page, "image:1");
    if (shots)
      await page.screenshot({
        path: path.join(shots, `stack-compare-${name}.png`),
        fullPage: true,
      });
    await strip(page)
      .getByRole("button", { name: "Stop comparing", exact: true })
      .click();
    await page
      .locator(".Lightbox-reference-comparison")
      .waitFor({ state: "hidden" });
    await strip(page)
      .getByRole("button", { name: "Compare with representative", exact: true })
      .click();
    await page.evaluate(() => window.qolFixture.openOther());
    await selected(page, "image:3");
    assert.equal(
      await page.locator(".Lightbox-reference-comparison").count(),
      0,
      "Another caller inherited the previous stack comparison"
    );
    await page.keyboard.press("Escape");
    await page.locator(".Lightbox").waitFor({ state: "hidden" });
    // Detail comparison opens the same shared viewer with the representative.
    await page
      .getByRole("button", { name: "Open preview", exact: true })
      .click();
    await selected(page, "image:2");
    await strip(page).locator('[data-member-id="image:1"]').click();
    await strip(page)
      .getByRole("button", { name: "Make this representative", exact: true })
      .click();
    await page.waitForFunction(
      () => window.qolFixture.snapshot().representative === "image:1"
    );
    await selected(page, "image:1");
    await page.keyboard.press("Escape");
    await page.locator(".Lightbox").waitFor({ state: "hidden" });
    await detail(page)
      .getByRole("button", { name: "Compare with representative", exact: true })
      .click();
    await selected(page, "image:2");
    await page.locator(".Lightbox-reference-comparison").waitFor();
    await page.evaluate(() => {
      window.qolFixture.holdMutation = true;
    });
    await strip(page)
      .getByRole("button", { name: "Make this representative", exact: true })
      .click();
    const count = await page.evaluate(() => window.qolFixture.mutations.length);
    assert.equal(
      await strip(page)
        .getByRole("button", { name: "Make this representative", exact: true })
        .isDisabled(),
      true
    );
    await page.keyboard.press("Escape");
    await page.locator(".Lightbox").waitFor({ state: "hidden" });
    await page.evaluate(() => window.qolFixture.releaseMutation());
    await page.waitForFunction(
      () => window.qolFixture.snapshot().representative === "image:2"
    );
    assert.equal(
      await page.evaluate(() => window.qolFixture.mutations.length),
      count
    );
    assert.equal(
      await page.locator(".Lightbox").count(),
      0,
      "Late mutation reopened the viewer"
    );
    assert.deepEqual(errors, []);
    assert.equal(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth
      ),
      true,
      `${name} page overflowed horizontally`
    );
    console.log(`${name}: passed`);
    await page.close();
  }
} catch (error) {
  if (activePage && !activePage.isClosed()) {
    console.error(await activePage.locator("body").innerText());
    if (shots)
      await activePage.screenshot({
        path: path.join(shots, "failure.png"),
        fullPage: true,
      });
  }
  throw error;
} finally {
  await browser.close();
  await server.close();
}
