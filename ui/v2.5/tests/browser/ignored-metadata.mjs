import assert from "node:assert/strict";
import { mkdtemp, mkdir, writeFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { startIsolatedServer } from "./helpers/isolated-server.mjs";

// Ignored Tags and Copyrights on a real local server: tick Ignored on the edit
// page, the record leaves its list, the Ignored filter brings it back, and
// unticking restores it. Own every path and process; never intercept requests.
assert.ok(process.env.STASH_BROWSER_BINARY, "Set STASH_BROWSER_BINARY");
const binary = resolve(process.env.STASH_BROWSER_BINARY);
const { chromium } = await import(
  process.env.PLAYWRIGHT_MODULE ?? "playwright"
);
const root = await mkdtemp(join(tmpdir(), "stashbooru-ignored-"));
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
  const result = await (
    await page.request.post(url + "/graphql", {
      data: { query, variables },
    })
  ).json();
  assert.equal(result.errors, undefined, JSON.stringify(result.errors));
  return result.data;
}
const ignoredFilter = encodeURIComponent(
  JSON.stringify({ type: "ignored", modifier: "EQUALS", value: "true" })
);

async function check(viewport, label) {
  const directory = join(root, label);
  for (const folder of ["media", "generated", "cache", "metadata"])
    await mkdir(join(directory, folder), { recursive: true });
  await writeFile(
    join(directory, "config.yml"),
    JSON.stringify({
      stash: [{ path: join(directory, "media") }],
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
    ui: process.env.STASH_BROWSER_UI,
  });
  const url = instance.url;
  const page = await browser.newPage({ viewport });
  const errors = [];
  page.on("pageerror", (error) => errors.push(String(error)));
  try {
    await call(
      page,
      url,
      'mutation {configureUI(input:{lastNoteSeen:99999999, lastVersionSeen: "999.0.0"})}'
    );
    const tag = (
      await call(page, url, 'mutation {tagCreate(input:{name:"highres"}){id}}')
    ).tagCreate;
    await call(page, url, 'mutation {tagCreate(input:{name:"smile"}){id}}');
    const copyright = (
      await call(
        page,
        url,
        'mutation {copyrightCreate(input:{name:"Useless Series"}){id}}'
      )
    ).copyrightCreate;
    await call(
      page,
      url,
      'mutation {copyrightCreate(input:{name:"Kept Series"}){id}}'
    );
    const ignoredState = async () =>
      await call(
        page,
        url,
        "query($tag:ID!,$copyright:ID!){findTag(id:$tag){ignored} findCopyright(id:$copyright){ignored}}",
        { tag: tag.id, copyright: copyright.id }
      );
    const card = (name) =>
      page.locator(".tag-card").filter({ hasText: name }).first();

    // Tag: tick Ignored on its edit page.
    await page.goto(`${url}/tags/${tag.id}`, { waitUntil: "networkidle" });
    await page.getByRole("button", { name: "Edit", exact: true }).click();
    const tagIgnored = page.getByLabel(/^Ignored \(hidden/);
    await tagIgnored.check();
    await page.getByRole("button", { name: "Save", exact: true }).click();
    await page
      .getByRole("button", { name: "Edit", exact: true })
      .waitFor({ state: "visible" });
    assert.equal((await ignoredState()).findTag.ignored, true);

    // The Tags list hides it; the Ignored filter shows only ignored Tags.
    await page.goto(`${url}/tags`, { waitUntil: "networkidle" });
    await card("smile").waitFor();
    assert.equal(await card("highres").count(), 0, "ignored Tag is hidden");
    await page.goto(`${url}/tags?c=${ignoredFilter}`, {
      waitUntil: "networkidle",
    });
    await card("highres").waitFor();
    assert.equal(await card("smile").count(), 0);
    await page.reload({ waitUntil: "networkidle" });
    await card("highres").waitFor();

    // A Tag selector does not offer it.
    const found = await call(
      page,
      url,
      '{findTags(filter:{q:"highres"}){count}}'
    );
    assert.equal(found.findTags.count, 0, "selectors skip ignored Tags");

    // Untick it again from its page.
    await page.goto(`${url}/tags/${tag.id}`, { waitUntil: "networkidle" });
    await page.getByRole("button", { name: "Edit", exact: true }).click();
    await page.getByLabel(/^Ignored \(hidden/).uncheck();
    await page.getByRole("button", { name: "Save", exact: true }).click();
    await page
      .getByRole("button", { name: "Edit", exact: true })
      .waitFor({ state: "visible" });
    assert.equal((await ignoredState()).findTag.ignored, false);

    // Copyright: the edit form and the Copyrights list filter.
    await page.goto(`${url}/copyrights/${copyright.id}`, {
      waitUntil: "networkidle",
    });
    await page.getByRole("button", { name: "Edit", exact: true }).click();
    await page.locator("#copyright-ignored").check();
    await page.getByRole("button", { name: "Save", exact: true }).click();
    await page
      .getByRole("button", { name: "Edit", exact: true })
      .waitFor({ state: "visible" });
    assert.equal((await ignoredState()).findCopyright.ignored, true);
    await page.goto(`${url}/copyrights`, { waitUntil: "networkidle" });
    await page.getByText("Kept Series", { exact: true }).first().waitFor();
    assert.equal(
      await page.getByText("Useless Series", { exact: true }).count(),
      0,
      "ignored Copyright is hidden"
    );
    await page.goto(`${url}/copyrights?c=${ignoredFilter}`, {
      waitUntil: "networkidle",
    });
    await page.getByText("Useless Series", { exact: true }).first().waitFor();
    assert.equal(
      await page.getByText("Kept Series", { exact: true }).count(),
      0
    );

    assert.deepEqual(errors, []);
    await page.screenshot({
      path: join(artifacts, label + "-ignored.png"),
      fullPage: true,
    });
    console.log(
      label +
        ": Ignored Tag and Copyright edit, hidden list, Ignored filter, reload and untick passed"
    );
  } catch (error) {
    await page
      .screenshot({
        path: join(artifacts, label + "-failure.png"),
        fullPage: true,
      })
      .catch(() => {});
    throw error;
  } finally {
    await page.close();
    await instance?.stop();
    instance = undefined;
  }
}
try {
  await check({ width: 1440, height: 1000 }, "desktop");
  await check({ width: 390, height: 844 }, "mobile");
  passed = true;
} finally {
  await instance?.stop();
  await browser.close();
  if (passed) await rm(root, { recursive: true, force: true });
  else console.error("Failed fixture retained:", root);
}
