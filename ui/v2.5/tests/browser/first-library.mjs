import assert from "node:assert/strict";
import { spawn, execFile } from "node:child_process";
import { once } from "node:events";
import { mkdtemp, mkdir, writeFile, rm } from "node:fs/promises";
import { createWriteStream } from "node:fs";
import { createServer } from "node:net";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { promisify } from "node:util";

// This suite owns its server and fresh folders. It never accepts a server URL or
// an existing configuration/library. No GraphQL or processing responses are mocked.
assert.ok(
  process.env.STASH_BROWSER_BINARY,
  "Set STASH_BROWSER_BINARY to a built StashBooru binary"
);
const binary = resolve(process.env.STASH_BROWSER_BINARY);
const { chromium } = await import(
  process.env.PLAYWRIGHT_MODULE ?? "playwright"
);
const run = promisify(execFile);
const root = await mkdtemp(join(tmpdir(), "stashbooru-first-library-"));
const artifacts =
  process.env.STASH_BROWSER_SCREENSHOTS ?? join(root, "artifacts");
await mkdir(artifacts, { recursive: true });
const browser = await chromium.launch({
  executablePath: process.env.CHROMIUM_EXECUTABLE,
  args: ["--no-sandbox", "--disable-dev-shm-usage"],
});
let server;
let output;
let passed = false;

async function stopServer() {
  if (server && server.exitCode === null && server.signalCode === null) {
    const exited = once(server, "exit");
    server.kill("SIGTERM");
    await exited;
  }
  output?.end();
  server = undefined;
}

async function startServer(directory, label) {
  const listener = createServer();
  listener.listen(0, "127.0.0.1");
  await once(listener, "listening");
  const port = listener.address().port;
  await new Promise((done) => listener.close(done));
  const url = `http://127.0.0.1:${port}`;
  // Do not inherit overrides that could point the test at a real database.
  const env = Object.fromEntries(
    Object.entries(process.env).filter(([key]) => !key.startsWith("STASH_"))
  );
  env.STASH_LOCAL_HTTPS_ENABLED = "false";
  output = createWriteStream(join(artifacts, `${label}-server.log`));
  server = spawn(
    binary,
    [
      "--config",
      join(directory, "config.yml"),
      "--host",
      "127.0.0.1",
      "--port",
      String(port),
      "--nobrowser",
    ],
    { cwd: directory, env }
  );
  server.stdout.pipe(output, { end: false });
  server.stderr.pipe(output, { end: false });
  let spawnError;
  server.on("error", (error) => {
    spawnError = error;
  });
  for (let attempt = 0; attempt < 120; attempt++) {
    if (spawnError) throw spawnError;
    assert.equal(
      server.exitCode,
      null,
      "Isolated server exited before becoming ready"
    );
    try {
      const response = await fetch(`${url}/graphql`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ query: "{ systemStatus { status } }" }),
        signal: AbortSignal.timeout(1000),
      });
      const payload = await response.json();
      if (payload.data?.systemStatus) return url;
    } catch {
      /* Wait for this new process to bind and initialize. */
    }
    await new Promise((done) => setTimeout(done, 250));
  }
  throw new Error("Isolated server did not become ready");
}

async function mutation(page, name, action) {
  const response = page.waitForResponse(
    (r) =>
      r.request().method() === "POST" &&
      r.url().endsWith("/graphql") &&
      r.request().postDataJSON()?.operationName === name
  );
  await action();
  const payload = await (await response).json();
  assert.equal(
    payload.errors,
    undefined,
    `${name}: ${JSON.stringify(payload.errors)}`
  );
  return payload.data;
}

async function check(viewport, label) {
  const directory = join(root, label);
  const media = join(directory, "media");
  await mkdir(media, { recursive: true });
  const page = await browser.newPage({ viewport });
  const errors = [];
  page.on("pageerror", (error) => errors.push(String(error)));
  // Draw a neutral, deterministic PNG; ffmpeg encodes a real, tiny H.264 Video.
  const png = await page.evaluate(() => {
    const canvas = document.createElement("canvas");
    canvas.width = 640;
    canvas.height = 480;
    const ctx = canvas.getContext("2d");
    ctx.fillStyle = "#243550";
    ctx.fillRect(0, 0, 640, 480);
    ctx.fillStyle = "#57c4c8";
    ctx.fillRect(80, 90, 220, 250);
    return canvas.toDataURL("image/png").split(",")[1];
  });
  await writeFile(join(media, "sample.png"), Buffer.from(png, "base64"));
  await run(process.env.FFMPEG ?? "ffmpeg", [
    "-v",
    "error",
    "-f",
    "lavfi",
    "-i",
    "testsrc2=size=640x360:rate=12",
    "-t",
    "2",
    "-c:v",
    "libx264",
    "-pix_fmt",
    "yuv420p",
    join(media, "sample.mp4"),
  ]);
  let url = await startServer(directory, label);
  try {
    await page.goto(url, { waitUntil: "domcontentloaded" });
    await page.getByRole("button", { name: "Next", exact: true }).click();
    await page.getByRole("button", { name: /^Add directory$/i }).click();
    const dialog = page.getByRole("dialog");
    await dialog.locator("input").fill(media);
    await dialog.getByRole("button", { name: "Confirm", exact: true }).click();
    for (const [id, value] of Object.entries({
      database: join(directory, "stash-go.sqlite"),
      generated: join(directory, "generated"),
      cache: join(directory, "cache"),
      blobs: join(directory, "blobs"),
    })) {
      await page.locator(`#${id} input:not([type=checkbox])`).fill(value);
    }
    await page.getByRole("button", { name: "Next", exact: true }).click();
    await page.getByRole("button", { name: "Next", exact: true }).click();
    await page.getByRole("button", { name: "Confirm", exact: true }).click();
    await page.waitForURL(`${url}/welcome`);
    await page.getByRole("button", { name: "Finish", exact: true }).waitFor();
    assert.equal(
      await page.locator("#download-ffmpeg").count(),
      0,
      "Install ffmpeg/ffprobe on PATH; this test must not download them"
    );
    const saved = await mutation(page, "ConfigureUI", () =>
      page.getByRole("button", { name: "Finish", exact: true }).click()
    );
    assert.ok(
      Number.isInteger(saved.configureUI.lastNoteSeen) &&
        saved.configureUI.lastNoteSeen > 0
    );
    await page.waitForURL(`${url}/settings?tab=library`);
    await page.locator("#stash-table").waitFor();
    assert.ok((await page.locator("#stash-table").innerText()).includes(media));
    await page.reload({ waitUntil: "networkidle" });
    assert.equal(
      await page.getByRole("dialog").count(),
      0,
      "Finish acknowledgement did not survive refresh"
    );
    await page.screenshot({
      path: join(artifacts, `${label}-library.png`),
      fullPage: true,
    });
    if (viewport.width < 768) {
      await page
        .getByLabel("Settings section", { exact: true })
        .selectOption("tasks");
    } else {
      await page.getByRole("tab", { name: "Tasks", exact: true }).click();
    }
    await page.locator("#tasks-library > summary").click();
    await mutation(page, "MetadataScan", () =>
      page.getByRole("button", { name: "Scan", exact: true }).click()
    );
    // Observe actual indexed results, allowing the bounded scan to complete.
    await page.waitForFunction(
      async () => {
        const response = await fetch("graphql", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            query: "{ findImages { count } findScenes { count } }",
          }),
        });
        const { data } = await response.json();
        return data?.findImages.count === 1 && data?.findScenes.count === 1;
      },
      null,
      { timeout: 30000, polling: 250 }
    );
    if (viewport.width < 768) await page.locator(".nav-menu-toggle").click();
    await page.getByRole("link", { name: "All", exact: true }).click();
    const image = page.locator(".media-list .image-card");
    const video = page.locator(".media-list .scene-card");
    await image.waitFor();
    await video.waitFor();
    for (const card of [image, video]) {
      assert.equal(await card.count(), 1);
      await card.locator("img").first().waitFor();
      await card
        .locator("img")
        .first()
        .evaluate((img) => img.decode());
    }
    await page.screenshot({
      path: join(artifacts, `${label}-all.png`),
      fullPage: true,
    });
    const href = await image
      .locator('a[href^="/images/"]')
      .first()
      .getAttribute("href");
    const imageID = new URL(href, url).pathname.split("/").pop();
    await image.locator('a[href^="/images/"]').first().click();
    await page.locator(".Lightbox-footer-center .image-link").click();
    await page.getByRole("tab", { name: "Edit", exact: true }).click();
    const title = `First library ${label}`;
    await page.locator("#image-edit-details #title").fill(title);
    await mutation(page, "ImageUpdate", () =>
      page
        .locator("#image-edit-details")
        .getByRole("button", { name: "Save", exact: true })
        .click()
    );
    await page.reload({ waitUntil: "networkidle" });
    await page.getByRole("tab", { name: "Edit", exact: true }).click();
    assert.equal(
      await page.locator("#image-edit-details #title").inputValue(),
      title
    );
    await page.screenshot({
      path: join(artifacts, `${label}-edit.png`),
      fullPage: true,
    });
    assert.ok(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth
      ),
      "Page overflows viewport"
    );
    await stopServer();
    url = await startServer(directory, `${label}-restart`);
    await page.goto(`${url}/images/${imageID}`, { waitUntil: "networkidle" });
    assert.equal(
      await page.getByRole("dialog").count(),
      0,
      "Finish acknowledgement did not survive restart"
    );
    await page.getByRole("tab", { name: "Edit", exact: true }).click();
    assert.equal(
      await page.locator("#image-edit-details #title").inputValue(),
      title
    );
    assert.deepEqual(errors, []);
    console.log(
      `${label}: fresh setup, scan, typed All cards/thumbnails, Image title save, refresh and server restart passed`
    );
  } catch (error) {
    await page
      .screenshot({
        path: join(artifacts, `${label}-failure.png`),
        fullPage: true,
      })
      .catch(() => {});
    throw error;
  } finally {
    await page.close();
    await stopServer();
  }
}

try {
  await check({ width: 1440, height: 1000 }, "desktop");
  await check({ width: 390, height: 844 }, "mobile");
  passed = true;
} finally {
  await stopServer();
  await browser.close();
  if (passed) await rm(root, { recursive: true, force: true });
  else console.error(`Failed isolated instance retained at ${root}`);
}
