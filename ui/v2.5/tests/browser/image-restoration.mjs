import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { createServer } from "vite";

// Native mounted components/styles. HTTP processing is mocked; no models or user catalogue.
const engines = await import(process.env.PLAYWRIGHT_MODULE ?? "playwright");
const root = path.resolve(import.meta.dirname, "../..");
const source = fs.readFileSync(process.env.STASH_RESTORATION_FIXTURE_SOURCE);
const output = fs.readFileSync(process.env.STASH_RESTORATION_FIXTURE_OUTPUT);
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
    page.on("pageerror", (error) => errors.push(error.message));
    const requests = [];
    let session;
    let installed = true;
    let held = false;
    let holdPreparation = false;
    let releasePreparation;
    let preparationStarted;
    let nextID = 0;
    await page.route("**/image/restoration**", async (route) => {
      const request = route.request();
      const url = new URL(request.url());
      if (request.method() === "GET" && url.searchParams.has("file")) {
        return route.fulfill({
          status: 200,
          contentType: "image/png",
          body: url.searchParams.get("file") === "output" ? output : source,
        });
      }
      if (request.method() === "POST") {
        const payload = request.postDataJSON();
        requests.push(payload);
        if (payload.action === "prepare") {
          session = {
            id: String(++nextID).padStart(32, "0"),
            status: "queued",
            backend: payload.backend,
            capabilities: {
              available: installed,
              model: "SD1.5 inpainting fixture",
              revision: "fixture-revision",
              cpu: false,
              gpu: true,
              notice: installed
                ? ""
                : "Set STASH_RESTORATION_MODEL_DIR and install the inpainting snapshot. No automatic download.",
            },
            receipt: {
              width: 256,
              height: 192,
              outputSHA256: "source",
              normalization: "8-bit fixture",
            },
          };
          if (holdPreparation) {
            preparationStarted();
            await new Promise((resolve) => {
              releasePreparation = resolve;
            });
          }
        } else if (payload.action === "generate") {
          assert.equal(payload.options.hardware, "gpu");
          assert.ok(payload.mask.length > 100);
          session.status = "running";
          session.pendingGeneration = true;
        } else if (payload.action === "save") {
          session.status = "saved";
          session.derivedImageID = 22;
        } else if (payload.action === "cancel") session.status = "cancelled";
        else if (payload.action === "discard")
          session = { ...session, status: "discarded" };
      } else if (session?.status === "queued") session.status = "ready";
      else if (session?.status === "running" && !held) {
        session.status = "preview";
        session.receipt.outputSHA256 = "generated-fixture";
      }
      return route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(session),
      });
    });
    await page.goto(`${baseURL}/tests/browser/fixtures/image-restoration.html`);
    await page.getByRole("combobox", { name: "Run on" }).selectOption("remote");
    await page
      .getByRole("button", { name: "Prepare source", exact: true })
      .click();
    const canvas = page.getByLabel("Paint restoration mask");
    await canvas.waitFor({ state: "visible" });
    await page.waitForFunction(
      () =>
        document.querySelector("button") &&
        Array.from(document.querySelectorAll("button")).some(
          (b) => b.textContent === "Brush" && !b.disabled
        )
    );
    assert.equal(requests.at(-1).backend, "remote");
    assert.equal(
      await page
        .getByRole("button", { name: "Generate preview", exact: true })
        .isEnabled(),
      false
    );
    const paint = async () => {
      await canvas.scrollIntoViewIfNeeded();
      const box = await canvas.boundingBox();
      const x = box.x + box.width * 0.3;
      const y = box.y + Math.min(120, box.height * 0.25);
      await page.mouse.move(x, y);
      await page.mouse.down();
      await page.mouse.move(x + 30, y + 20, { steps: 4 });
      await page.mouse.up();
    };
    await paint();
    await page.getByRole("button", { name: "Undo", exact: true }).click();
    assert.equal(
      await page
        .getByRole("button", { name: "Generate preview", exact: true })
        .isEnabled(),
      false
    );
    await page.getByRole("button", { name: "Box", exact: true }).click();
    await paint();
    await page.getByRole("button", { name: "Erase", exact: true }).click();
    await paint();
    await page.getByRole("button", { name: "Undo", exact: true }).click();
    await page
      .getByRole("spinbutton", { name: "Feather (source pixels)" })
      .fill("2");
    await page
      .getByRole("combobox", { name: "Zoom", exact: true })
      .selectOption("2");
    await page
      .getByRole("combobox", { name: "Zoom", exact: true })
      .selectOption("1");
    await page
      .getByRole("combobox", { name: "Hardware", exact: true })
      .selectOption("gpu");
    if (shots) {
      await canvas.scrollIntoViewIfNeeded();
      await page.screenshot({ path: path.join(shots, `p11-mask-${name}.png`) });
    }
    await page
      .getByRole("button", { name: "Generate preview", exact: true })
      .click();
    const save = page.getByRole("button", {
      name: "Save generated derivative",
      exact: true,
    });
    await save.waitFor({ state: "visible" });
    await page.waitForFunction(() =>
      [...document.querySelectorAll("button")].some(
        (b) => b.textContent === "Save generated derivative" && !b.disabled
      )
    );
    assert.equal(requests.find((r) => r.action === "generate").feather, 2);
    await page
      .getByRole("combobox", { name: "Comparison", exact: true })
      .selectOption("slider");
    await page
      .getByRole("combobox", { name: "Comparison", exact: true })
      .selectOption("difference");
    await page
      .getByRole("combobox", { name: "Comparison", exact: true })
      .selectOption("both");
    await page
      .getByRole("spinbutton", { name: "Seed", exact: true })
      .fill("99");
    assert.equal(await save.isEnabled(), false);
    await page
      .getByLabel("Optional starting-pixel reference (PNG, JPEG or WebP)")
      .setInputFiles({
        name: "guide.png",
        mimeType: "image/png",
        buffer: source,
      });
    await page.getByText("guide.png", { exact: false }).waitFor();
    assert.equal(
      await page
        .getByRole("spinbutton", { name: "Strength", exact: true })
        .inputValue(),
      "0.75"
    );
    await page
      .getByRole("button", { name: "Generate preview", exact: true })
      .click();
    await page.waitForFunction(() =>
      [...document.querySelectorAll("button")].some(
        (b) => b.textContent === "Save generated derivative" && !b.disabled
      )
    );
    await page
      .getByRole("checkbox", {
        name: "Add derivative to the source's visual stack",
      })
      .check();
    assert.ok(
      requests.filter((r) => r.action === "generate").at(-1).reference.length >
        100
    );
    await save.scrollIntoViewIfNeeded();
    if (shots)
      await page.screenshot({
        path: path.join(shots, `p11-review-${name}.png`),
      });
    await save.click();
    await page.getByRole("link", { name: "Open Image", exact: true }).waitFor();
    assert.equal(requests.at(-1).stack, true);
    assert.equal(
      await page
        .getByRole("link", { name: "Open Image", exact: true })
        .getAttribute("href"),
      "/images/22"
    );
    await page
      .getByRole("button", { name: "Close", exact: true })
      .last()
      .click();
    await page
      .getByRole("button", { name: "Open restoration", exact: true })
      .click();
    installed = false;
    await page
      .getByRole("button", { name: "Prepare source", exact: true })
      .click();
    await page
      .getByText("Set STASH_RESTORATION_MODEL_DIR", { exact: false })
      .waitFor();
    assert.equal(
      await page
        .getByRole("button", { name: "Generate preview", exact: true })
        .isEnabled(),
      false
    );
    await page
      .getByRole("button", { name: "Close", exact: true })
      .last()
      .click();
    await page.waitForFunction(() => !document.querySelector(".modal"));
    await page
      .getByRole("button", { name: "Open restoration", exact: true })
      .click();
    installed = true;
    await page
      .getByRole("button", { name: "Prepare source", exact: true })
      .click();
    await canvas.waitFor({ state: "visible" });
    await page.waitForFunction(() =>
      [...document.querySelectorAll("button")].some(
        (b) => b.textContent === "Brush" && !b.disabled
      )
    );
    await paint();
    await page
      .getByRole("combobox", { name: "Hardware", exact: true })
      .selectOption("gpu");
    held = true;
    await page
      .getByRole("button", { name: "Generate preview", exact: true })
      .click();
    await page
      .getByRole("button", { name: "Cancel processing", exact: true })
      .click();
    await page
      .getByRole("button", { name: "Close", exact: true })
      .last()
      .click();
    assert.ok(requests.some((r) => r.action === "cancel"));
    await page.waitForFunction(() => !document.querySelector(".modal"));
    await page
      .getByRole("button", { name: "Open restoration", exact: true })
      .click();
    holdPreparation = true;
    const pendingPrepare = new Promise((resolve) => {
      preparationStarted = resolve;
    });
    await page
      .getByRole("button", { name: "Prepare source", exact: true })
      .click();
    await pendingPrepare;
    await page
      .getByRole("button", { name: "Close", exact: true })
      .last()
      .click();
    const lateCancel = page.waitForResponse((r) => {
      if (r.request().method() !== "POST") return false;
      const payload = r.request().postDataJSON();
      return (
        payload.action === "cancel" &&
        payload.sessionID === String(nextID).padStart(32, "0")
      );
    });
    releasePreparation();
    await lateCancel;
    assert.deepEqual(errors, []);
    await page.close();
    console.log(
      `${name}: mask tools, undo/feather/zoom, remote/GPU choices, P03 comparison, invalidation, exact reviewed save, missing model, reopen/cancellation passed`
    );
  }
} finally {
  await browser.close();
  await server.close();
}
