import assert from "node:assert/strict";
import { mkdtemp, mkdir, writeFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { startIsolatedServer } from "./helpers/isolated-server.mjs";

// Image details panel: filename layout tokens, resizing, hiding and the
// remembered layout, on an isolated server and library.
assert.ok(process.env.STASH_BROWSER_BINARY, "Set STASH_BROWSER_BINARY");
const binary = resolve(process.env.STASH_BROWSER_BINARY);
const ui = process.env.STASH_BROWSER_UI;
const { chromium } = await import(
  process.env.PLAYWRIGHT_MODULE ?? "playwright"
);
const root = await mkdtemp(join(tmpdir(), "stashbooru-image-details-"));
const artifacts =
  process.env.STASH_BROWSER_SCREENSHOTS ?? join(root, "artifacts");
await mkdir(artifacts, { recursive: true });
const browser = await chromium.launch({
  executablePath: process.env.CHROMIUM_EXECUTABLE,
  args: ["--no-sandbox", "--disable-dev-shm-usage"],
});
let instance;
let passed = false;
async function call(page, url, query, variables = {}) {
  const payload = await (
    await page.request.post(url + "/graphql", { data: { query, variables } })
  ).json();
  assert.equal(payload.errors, undefined, JSON.stringify(payload.errors));
  return payload.data;
}
const name =
  "[afrobull](hyrule_warriors+the_legend_of_zelda).link+linkle_0123456789abcdef0123456789abcdef.png";

async function check(viewport, label) {
  const directory = join(root, label);
  const media = join(directory, "media");
  for (const folder of [
    media,
    ...["generated", "cache", "metadata"].map((part) => join(directory, part)),
  ])
    await mkdir(folder, { recursive: true });
  const page = await browser.newPage({ viewport });
  const errors = [];
  page.on("pageerror", (error) => errors.push(String(error)));
  const [png, other] = await page.evaluate(() =>
    ["#3a2550", "#25503a"].map((colour) => {
      const canvas = document.createElement("canvas");
      canvas.width = 160;
      canvas.height = 120;
      const context = canvas.getContext("2d");
      context.fillStyle = colour;
      context.fillRect(0, 0, 160, 120);
      return canvas.toDataURL("image/png").split(",")[1];
    })
  );
  await writeFile(join(media, name), Buffer.from(png, "base64"));
  await writeFile(join(media, "holiday.png"), Buffer.from(other, "base64"));
  await writeFile(
    join(directory, "config.yml"),
    JSON.stringify({
      stash: [{ path: media }],
      database: join(directory, "library.sqlite"),
      generated: join(directory, "generated"),
      cache: join(directory, "cache"),
      metadata: join(directory, "metadata"),
      blobs_path: join(directory, "blobs"),
    })
  );
  instance = await startIsolatedServer({
    binary,
    directory,
    artifacts,
    label,
    ui,
  });
  const url = instance.url;
  await call(
    page,
    url,
    'mutation { configureUI(input: {lastNoteSeen: 99999999, lastVersionSeen: "999.0.0"}) }'
  );
  await call(
    page,
    url,
    "mutation($path: String!) {metadataScan(input: {paths: [$path]})}",
    { path: media }
  );
  let images = [];
  for (let attempt = 0; attempt < 120 && images.length < 2; attempt++) {
    images = (
      await call(
        page,
        url,
        '{findImages(filter: {per_page: -1, sort: "path"}) {images {id files {path}}}}'
      )
    ).findImages.images;
    if (images.length < 2) await new Promise((done) => setTimeout(done, 250));
  }
  assert.equal(images.length, 2);
  const tagged = images.find((image) => image.files[0].path.endsWith(name));
  const plain = images.find((image) => image !== tagged);

  await page.goto(url + "/images/" + tagged.id, { waitUntil: "networkidle" });
  // Tokens from the default layout appear with the file details.
  const tokens = page.locator(".image-file-info .filename-token-name");
  await tokens.first().waitFor();
  assert.deepEqual(await tokens.allInnerTexts(), [
    "%artist%",
    "%copyright%",
    "%character%",
    "%md5%",
    "%ext%",
  ]);
  assert.deepEqual(
    await page
      .locator(".image-file-info .filename-token-value")
      .allInnerTexts(),
    [
      "afrobull",
      "hyrule_warriors+the_legend_of_zelda",
      "link+linkle",
      "0123456789abcdef0123456789abcdef",
      "png",
    ]
  );
  // Long values wrap instead of being cut off.
  const clipped = await page.evaluate(() =>
    [...document.querySelectorAll(".image-file-info dd, .image-file-info dd *")]
      .filter(
        (cell) =>
          cell.scrollWidth > cell.clientWidth + 1 ||
          cell.scrollHeight > cell.clientHeight + 1
      )
      .map((cell) => cell.textContent)
  );
  assert.deepEqual(clipped, [], "no details value is cut off");
  await page.screenshot({
    path: join(artifacts, `image-details-${label}.png`),
  });

  const tabs = page.locator(".image-tabs");
  const toggle = page.locator(".image-details-toggle");
  const separator = page.getByRole("separator", {
    name: "Resize details panel",
  });
  if (viewport.width >= 1200) {
    const before = (await tabs.boundingBox()).width;
    await separator.focus();
    await page.keyboard.press("ArrowRight");
    await page.keyboard.press("Shift+ArrowRight");
    const wider = (await tabs.boundingBox()).width;
    assert.ok(wider >= before + 95, `resized from ${before} to ${wider}`);
    const box = await separator.boundingBox();
    await page.mouse.move(box.x + box.width / 2, box.y + 40);
    await page.mouse.down();
    await page.mouse.move(box.x + box.width / 2 - 60, box.y + 40, {
      steps: 4,
    });
    await page.mouse.up();
    const dragged = (await tabs.boundingBox()).width;
    assert.ok(
      Math.abs(dragged - (wider - 60)) <= 4,
      `dragged to ${dragged}, expected ${wider - 60}`
    );
    await page.reload({ waitUntil: "networkidle" });
    assert.ok(
      Math.abs((await tabs.boundingBox()).width - dragged) <= 2,
      "the width is remembered"
    );
  } else {
    assert.equal(await separator.isVisible(), false, "no resize on mobile");
  }
  await toggle.click();
  assert.equal(await tabs.isVisible(), false, "Hide details hides the panel");
  await page.screenshot({
    path: join(artifacts, `image-details-${label}-hidden.png`),
  });
  await page.reload({ waitUntil: "networkidle" });
  assert.equal(await tabs.isVisible(), false, "hidden stays hidden");
  await page.getByRole("button", { name: "Show details" }).click();
  await tabs.waitFor();

  // A filename outside the layout shows no tokens.
  await page.goto(url + "/images/" + plain.id, { waitUntil: "networkidle" });
  await page.locator(".image-file-info").first().waitFor();
  assert.equal(
    await page.locator(".image-file-info .filename-token-name").count(),
    0
  );
  assert.deepEqual(errors, []);
  await page.close();
  await instance.stop();
  instance = undefined;
  console.log(
    `Image details ${label}: filename tokens, no cut-off values, resize, hide and remembered layout passed.`
  );
}
try {
  await check({ width: 1440, height: 1000 }, "desktop");
  await check({ width: 390, height: 844 }, "mobile");
  passed = true;
} finally {
  await browser.close();
  await instance?.stop();
  if (passed && process.env.STASH_BROWSER_SCREENSHOTS)
    await rm(root, { recursive: true, force: true });
  else console.log("Isolated Image details fixture: " + root);
}
