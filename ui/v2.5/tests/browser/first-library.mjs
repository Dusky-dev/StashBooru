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

async function graphql(page, url, query, variables = {}) {
  const payload = await (
    await page.request.post(`${url}/graphql`, { data: { query, variables } })
  ).json();
  assert.equal(payload.errors, undefined, JSON.stringify(payload.errors));
  return payload.data;
}

async function checkVideoList(page, url, videoID) {
  await page.goto(`${url}/scenes?sortby=date`);
  const list = page.locator(".scene-list");
  const link = list.locator(`a.scene-card-link[href^="/scenes/${videoID}"]`);
  await link.waitFor();

  // The seeded library is small. Make its real list tall enough to scroll
  // without scanning a second library or simulating the page contents.
  const tallList = await page.addStyleTag({
    content: ".scene-list { padding-top: 1500px; min-height: 3000px; }",
  });
  await page.evaluate(() => {
    history.scrollRestoration = "manual";
  });
  // Go Back once while the Video page's lazy code is still loading, so the
  // list was only hidden behind Suspense, then once after the page loads.
  for (const holdScripts of [true, false]) {
    await link.scrollIntoViewIfNeeded();
    const top = await page.evaluate(() => scrollY);
    assert.ok(top > 300, "Videos fixture must scroll before opening a detail");
    let held = [];
    const hold = (route) => (held ? held.push(route) : route.continue());
    if (holdScripts) await page.route("**/*.js", hold);
    await link.click();
    await page.waitForURL(
      (location) => location.pathname === `/scenes/${videoID}`
    );
    await page.evaluate(() => window.scrollTo(0, 0));
    await page.goBack();
    await page.waitForURL((location) => location.pathname === "/scenes");
    if (holdScripts) {
      const routes = held;
      held = undefined;
      await Promise.all(routes.map((route) => route.continue()));
      await page.unroute("**/*.js", hold);
    }
    await link.waitFor();
    await page.waitForFunction((saved) => Math.abs(scrollY - saved) < 2, top);
    await page.evaluate(() => window.scrollTo(0, 0));
  }
  await tallList.evaluate((style) => style.remove());

  const more = list.locator("#more-menu");
  const menu = page.locator(".scene-list-operations-dropdown");
  await more.click();
  await menu.getByText("Select All", { exact: true }).click();
  const checked = list.locator("input.card-check:checked");
  assert.equal(await checked.count(), 2);
  await more.click();
  await menu.getByText("Create stack from selection…", { exact: true }).click();
  const dialog = page.getByRole("dialog");
  await dialog
    .getByRole("radio", {
      name: `Representative scene:${videoID}`,
      exact: true,
    })
    .check();
  await dialog
    .getByRole("button", { name: "Create stack", exact: true })
    .click();
  await dialog.waitFor({ state: "hidden" });
  await link.waitFor();
  assert.equal(await checked.count(), 0);
  const result = await graphql(
    page,
    url,
    "query ($media: MediaReferenceInput!) { findVisualStackForMedia(media: $media) { member_count } }",
    { media: { kind: "VIDEO", id: videoID } }
  );
  assert.equal(result.findVisualStackForMedia.member_count, 2);
}

async function checkProfileAll(page, url, imageID, label) {
  const { findScenes } = await graphql(
    page,
    url,
    "{ findScenes { scenes { id } } }"
  );
  assert.equal(findScenes.scenes.length, 1);
  const videoID = findScenes.scenes[0].id;
  // Independent Image and Video ID spaces can reuse the same numeric ID.
  assert.equal(imageID, videoID);
  const { performer, studio, tag, copyright, outside } = await graphql(
    page,
    url,
    `mutation {
      performer: performerCreate(input: { name: "Fixture Character" }) { id }
      studio: studioCreate(input: { name: "Fixture Artist" }) { id }
      tag: tagCreate(input: { name: "Fixture Tag" }) { id }
      copyright: copyrightCreate(input: { name: "Fixture Copyright" }) { id }
      outside: sceneCreate(input: { title: "Unrelated Video" }) { id }
    }`
  );
  const { child } = await graphql(
    page,
    url,
    `mutation ($parent: ID!) {
      child: copyrightCreate(input: { name: "Fixture Sub-Copyright", parent_ids: [$parent] }) { id }
    }`,
    { parent: copyright.id }
  );
  const links = {
    performer_ids: [performer.id],
    studio_id: studio.id,
    tag_ids: [tag.id],
  };
  await graphql(
    page,
    url,
    `mutation (
      $image: ImageUpdateInput!, $video: SceneUpdateInput!,
      $imageID: ID!, $videoID: ID!, $child: ID!
    ) {
      imageUpdate(input: $image) { id }
      sceneUpdate(input: $video) { id }
      imageCopyrightsUpdate(image_id: $imageID, copyright_ids: [$child]) { id }
      sceneCopyrightsUpdate(scene_id: $videoID, copyright_ids: [$child]) { id }
    }`,
    {
      image: { id: imageID, ...links },
      video: { id: videoID, ...links },
      imageID,
      videoID,
      child: child.id,
    }
  );

  const imagePath = `/images/${imageID}`;
  const videoPath = `/scenes/${videoID}`;
  const mixed = [imagePath, videoPath];
  const global = [...mixed, `/scenes/${outside.id}`];
  const cardLinkSelector =
    ".media-list .grid-card :is(a.image-card-link, a.scene-card-link)";
  async function expectGrid(paths) {
    await page.waitForFunction(
      ({ selector, expected }) => {
        const rendered = Array.from(
          document.querySelectorAll(selector),
          (link) => new URL(link.href).pathname
        ).sort();
        return (
          rendered.length === expected.length &&
          expected.every((path, i) => rendered[i] === path)
        );
      },
      { selector: cardLinkSelector, expected: [...paths].sort() }
    );
  }

  await page.goto(`${url}/media`);
  await expectGrid(global);
  for (const [route, id] of [
    ["performers", performer.id],
    ["studios", studio.id],
    ["tags", tag.id],
    ["copyrights", copyright.id],
  ]) {
    await page.goto(`${url}/${route}/${id}/all`);
    await expectGrid(mixed);
  }

  const profilePath = `/copyrights/${copyright.id}/all`;
  const profileURL = page.url();
  const firstLink = await page
    .locator(cardLinkSelector)
    .first()
    .getAttribute("href");
  const imageFirst = new URL(firstLink, url).pathname === imagePath;
  const lightboxLink = (path) =>
    page.locator(`.Lightbox-footer-center .image-link[href="${path}"]`);
  await page
    .locator(`.media-list .image-card-link[href="${imagePath}"]`)
    .click();
  await lightboxLink(imagePath).waitFor();
  const steps = page.locator(".Lightbox-display > .Lightbox-navbutton");
  const compact = page.locator(".Lightbox-compact-navigation");
  const mobile = label === "mobile";
  assert.equal(await compact.isVisible(), mobile);
  function step(forward) {
    if (mobile) {
      return compact
        .getByRole("button", {
          name: forward ? "Next" : "Previous",
          exact: true,
        })
        .click();
    }
    return (forward ? steps.last() : steps.first()).click();
  }
  await step(imageFirst);
  await lightboxLink(videoPath).waitFor();
  await page.locator(".unified-media-native-scene-player .video-js").waitFor();
  await step(!imageFirst);
  await lightboxLink(imagePath).waitFor();
  await page
    .locator(".unified-media-native-scene-player")
    .waitFor({ state: "hidden" });
  await page.getByTitle("Close Lightbox", { exact: true }).click();
  await page.locator(".Lightbox").waitFor({ state: "hidden" });
  assert.equal(page.url(), profileURL);

  for (const [tab, selector, path] of [
    ["images", "a.image-card-link", imagePath],
    ["videos", "a.scene-card-link", videoPath],
  ]) {
    await page.locator(`#copyright-media-tabs-tab-${tab}`).click();
    const link = page.locator(
      `#copyright-media-tabs-tabpane-${tab} ${selector}`
    );
    await link.waitFor();
    assert.equal(new URL(await link.getAttribute("href"), url).pathname, path);
  }
  await page.locator("#copyright-media-tabs-tab-all").click();
  await expectGrid(mixed);

  await page.goto(`${url}/media`);
  await expectGrid(global);
  await page.goBack();
  await page.waitForURL((u) => u.pathname === profilePath);
  await expectGrid(mixed);
  await page.reload();
  await expectGrid(mixed);
  await page.screenshot({
    path: join(artifacts, `${label}-profile-all.png`),
    fullPage: true,
  });
  await checkVideoList(page, url, videoID);
  await checkImpliedTags(page, url, imageID, videoID, tag.id);
}

// Card tag popovers list direct Tags, then the Tags they imply (parents),
// loaded when the popover opens so a parent edit shows on the next open.
async function checkImpliedTags(page, url, imageID, videoID, directTagID) {
  const { franchise } = await graphql(
    page,
    url,
    `mutation { franchise: tagCreate(input: { name: "My Hero Academia" }) { id } }`
  );
  const { series } = await graphql(
    page,
    url,
    `mutation ($parent: ID!) {
      series: tagCreate(input: { name: "Vigilantes", parent_ids: [$parent] }) { id }
    }`,
    { parent: franchise.id }
  );
  const tagIDs = [directTagID, series.id];
  await graphql(
    page,
    url,
    `mutation ($image: ImageUpdateInput!, $video: SceneUpdateInput!) {
      imageUpdate(input: $image) { id }
      sceneUpdate(input: $video) { id }
    }`,
    {
      image: { id: imageID, tag_ids: tagIDs },
      video: { id: videoID, tag_ids: tagIDs },
    }
  );
  const popoverTags = async (button) => {
    await page.mouse.move(0, 0);
    await page.locator(".hover-popover-content").waitFor({ state: "detached" });
    await button.hover();
    const popover = page.locator(".hover-popover-content");
    await popover.locator(".implied-links").waitFor();
    return {
      direct: await popover
        .locator(".tag-item:not(.implied-links .tag-item)")
        .allInnerTexts(),
      implied: await popover
        .locator(".implied-links .tag-item")
        .allInnerTexts(),
      heading: await popover.locator(".implied-links-heading").innerText(),
    };
  };
  const cards = [
    ["/images", `.image-card-link[href="/images/${imageID}"]`],
    ["/scenes", `.scene-card-link[href^="/scenes/${videoID}"]`],
  ];
  // Park the pointer before reloading: a cursor left over a card button can
  // open that button's popover on the new page after the "detached" check.
  const parkAndGo = async (path) => {
    await page.mouse.move(0, 0);
    await page.goto(`${url}${path}`);
  };
  const tagButton = async ([path, card]) => {
    await parkAndGo(path);
    const button = page
      .locator(".grid-card", { has: page.locator(card) })
      .locator(".tag-count button");
    await button.waitFor();
    assert.equal((await button.innerText()).trim(), "2", "count stays direct");
    return button;
  };
  for (const entry of cards) {
    // The fixture's direct Sub-Copyright implies its parent Copyright.
    const copyrights = page
      .locator(".grid-card", { has: page.locator(entry[1]) })
      .locator(".copyright-count button");
    await parkAndGo(entry[0]);
    await copyrights.waitFor();
    assert.equal((await copyrights.innerText()).trim(), "1");
    const copyrightShown = await popoverTags(copyrights);
    assert.deepEqual(copyrightShown.direct, ["Fixture Sub-Copyright"]);
    assert.deepEqual(copyrightShown.implied, ["Fixture Copyright"]);

    const shown = await popoverTags(await tagButton(entry));
    assert.equal(shown.heading, "Implied");
    assert.deepEqual(shown.implied, ["My Hero Academia"]);
    assert.ok(shown.direct.includes("Vigilantes"), shown.direct.join());
    assert.ok(!shown.direct.includes("My Hero Academia"));
  }

  // A new grandparent shows on the next open without reloading the list.
  const button = await tagButton(cards[0]);
  await popoverTags(button);
  await graphql(
    page,
    url,
    `mutation ($id: ID!) {
      tagCreate(input: { name: "Shonen", child_ids: [$id] }) { id }
    }`,
    { id: franchise.id }
  );
  const shown = await popoverTags(button);
  assert.deepEqual(shown.implied.sort(), ["My Hero Academia", "Shonen"]);
}

// Fork destinations use the native utility-button styling: every navbar
// control fits the viewport, icon-only links keep a tooltip, and the phone
// menu lists them with labels.
async function checkNavbar(page, mobile) {
  const paths = ["/following", "/identities", "/image-duplicates", "/booru"];
  if (mobile) {
    await page.locator(".nav-menu-toggle").click();
    await page.locator(".navbar-collapse.show").waitFor();
  }
  const scope = mobile ? ".navbar-collapse" : ".navbar-buttons";
  const links = await page.evaluate(
    ({ paths, scope }) =>
      paths.map((path) => {
        const link = document.querySelector(
          `${scope} a.nav-utility[href="${path}"]`
        );
        const bounds = link?.getBoundingClientRect();
        const label = link?.querySelector(".nav-utility-label");
        return {
          path,
          title: link?.querySelector(".btn")?.title ?? "",
          visible: !!bounds && bounds.width > 0,
          inside: !!bounds && bounds.left >= 0 && bounds.right <= innerWidth,
          label: label && label.getBoundingClientRect().width > 0,
        };
      }),
    { paths, scope }
  );
  for (const link of links) {
    assert.ok(link.visible && link.inside, JSON.stringify(link));
    assert.ok(link.title, `${link.path} needs a tooltip`);
    if (mobile) assert.ok(link.label, `${link.path} needs a menu label`);
  }
  if (mobile) {
    await page.locator(".nav-menu-toggle").click();
    await page.locator(".navbar-collapse.show").waitFor({ state: "detached" });
  } else {
    const overflow = await page.evaluate(() =>
      [...document.querySelectorAll(".top-nav .btn")]
        .map((node) => node.getBoundingClientRect())
        .filter((bounds) => bounds.width > 0 && bounds.right > innerWidth)
        .map((bounds) => Math.round(bounds.right))
    );
    assert.deepEqual(overflow, [], "Navbar controls overflow the viewport");
  }
}

async function check(viewport, label) {
  const directory = join(root, label);
  const media = join(directory, "media");
  await mkdir(media, { recursive: true });
  let page = await browser.newPage({ viewport });
  const errors = [];
  const recordError = (error) => errors.push(String(error));
  page.on("pageerror", recordError);
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
    // Finishing setup counts as having seen this version's notes.
    assert.match(saved.configureUI.lastVersionSeen ?? "", /^\d+\.\d+\.\d+$/);
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
    // Wait until the scan job has finished and both items are indexed.
    // waitForFunction does not await an async predicate (its Promise is
    // truthy), so poll with page.evaluate, which resolves the Promise.
    const deadline = Date.now() + 30000;
    for (;;) {
      const ready = await page.evaluate(async () => {
        const response = await fetch("graphql", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            query:
              "{ findImages { count } findScenes { count } jobQueue { id } }",
          }),
        });
        const { data } = await response.json();
        return (
          data?.findImages.count === 1 &&
          data?.findScenes.count === 1 &&
          !data?.jobQueue?.length
        );
      });
      if (ready) break;
      assert.ok(Date.now() < deadline, "Scan did not index both items");
      await page.waitForTimeout(250);
    }
    if (viewport.width < 768) await page.locator(".nav-menu-toggle").click();
    await page.getByRole("link", { name: "All", exact: true }).click();
    const image = page.locator(".media-list .image-card");
    const video = page.locator(".media-list .scene-card");
    await image.waitFor();
    await video.waitFor();
    await checkNavbar(page, viewport.width < 768);
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
    // A real built instance must leave ordinary native media byte-for-byte
    // unchanged. Malformed/unknown opt-in requests fail closed, never
    // returning a different archived image or replacing the original.
    const nativeImageURL = new URL(`image/${imageID}/image`, url + "/");
    const expectedNative = Buffer.from(png, "base64");
    const ordinaryImageResponse = await fetch(nativeImageURL);
    assert.equal(ordinaryImageResponse.status, 200);
    assert.deepEqual(
      Buffer.from(await ordinaryImageResponse.arrayBuffer()),
      expectedNative
    );
    const ordinaryThumbnailResponse = await fetch(
      new URL(`image/${imageID}/thumbnail`, url + "/")
    );
    assert.equal(ordinaryThumbnailResponse.status, 200);
    assert.ok((await ordinaryThumbnailResponse.arrayBuffer()).byteLength > 0);
    const invalidNative = await fetch(
      `${nativeImageURL}?storageShadowCandidate=stack-51-delta-0123456789abcdef01234567`
    );
    assert.equal(invalidNative.status, 400);
    const missingCandidate = await fetch(
      `${nativeImageURL}?storageShadowCandidate=stack-51-delta-0123456789abcdef01234567&storageShadowStack=51`
    );
    assert.equal(missingCandidate.status, 409);
    const stillOriginal = await fetch(nativeImageURL);
    assert.deepEqual(
      Buffer.from(await stillOriginal.arrayBuffer()),
      expectedNative
    );
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
    assert.deepEqual(
      errors,
      [],
      "Page errors before deliberate server shutdown"
    );
    // Unmount the client before stopping its server. A live Apollo client can
    // otherwise report expected in-flight fetch failures during this test step.
    // Use a fresh page after restart so persisted state cannot come from cache.
    await page.close();
    await stopServer();
    url = await startServer(directory, `${label}-restart`);
    page = await browser.newPage({ viewport });
    page.on("pageerror", recordError);
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
    await checkProfileAll(page, url, imageID, label);
    assert.deepEqual(errors, []);
    console.log(
      `${label}: setup, scan, Image title, All navigation, and Videos scroll/stack selection passed`
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
