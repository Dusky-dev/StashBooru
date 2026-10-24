import assert from "node:assert/strict";
import { mkdtemp, mkdir, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { startIsolatedServer } from "./helpers/isolated-server.mjs";

// Only our local JS/UI fixtures are installed, in private disposable libraries.
assert.ok(process.env.STASH_BROWSER_BINARY, "Set STASH_BROWSER_BINARY");
const binary = resolve(process.env.STASH_BROWSER_BINARY);
const { chromium } = await import(
  process.env.PLAYWRIGHT_MODULE ?? "playwright"
);
const root = await mkdtemp(join(tmpdir(), "stashbooru-plugins-"));
const artifacts =
  process.env.STASH_BROWSER_SCREENSHOTS ?? join(root, "artifacts");
await mkdir(artifacts, { recursive: true });
const browser = await chromium.launch({
  executablePath: process.env.CHROMIUM_EXECUTABLE,
  args: ["--no-sandbox", "--disable-dev-shm-usage"],
});
let instance;
let successful = false;
async function stop() {
  await instance?.stop();
  instance = undefined;
}
async function start(directory, label) {
  instance = await startIsolatedServer({
    binary,
    directory,
    artifacts,
    label,
    ui: process.env.STASH_BROWSER_UI,
  });
  return instance.url;
}
async function call(page, url, query, variables = {}) {
  const response = await page.request.post(url + "/graphql", {
    data: { query, variables },
  });
  const result = await response.json();
  assert.equal(result.errors, undefined, JSON.stringify(result.errors));
  return result.data;
}
const definitions = [
  ["Fixture reported failure", "report"],
  ["Fixture script exception", "throw"],
  ["Fixture successful task", "success"],
  ["Fixture cancellable task", "wait"],
];
async function fixture(directory) {
  for (const name of ["", "media", "generated", "cache", "metadata", "plugins"])
    await mkdir(join(directory, name), { recursive: true });
  await writeFile(
    join(directory, "config.yml"),
    JSON.stringify({
      stash: [{ path: join(directory, "media") }],
      database: join(directory, "library.sqlite"),
      generated: join(directory, "generated"),
      cache: join(directory, "cache"),
      metadata: join(directory, "metadata"),
      blobs_storage: "DATABASE",
      plugins_path: join(directory, "plugins"),
    })
  );
  const plugins = join(directory, "plugins");
  await writeFile(
    join(plugins, "compat-base.yml"),
    JSON.stringify({
      name: "Compatibility base",
      version: "1.0",
      ui: { javascript: ["base.js"] },
    })
  );
  await writeFile(join(plugins, "base.js"), 'window.__compatOrder = ["base"];');
  await writeFile(
    join(plugins, "compat.yml"),
    JSON.stringify({
      name: "Compatibility fixture",
      version: "1.0",
      interface: "js",
      exec: ["task.js"],
      ui: { requires: ["compat-base"], javascript: ["ui.js"], css: ["ui.css"] },
      settings: { marker: { type: "STRING", displayName: "Fixture marker" } },
      tasks: definitions.map(([name, mode]) => ({
        name,
        defaultArgs: { mode },
      })),
    })
  );
  await writeFile(
    join(plugins, "ui.css"),
    ".compat-fixture { --compat-ready: 1; }"
  );
  await writeFile(
    join(plugins, "ui.js"),
    `(function () {
    if (!window.__compatOrder || window.__compatOrder.join() !== "base") throw new Error("Dependency order is wrong");
    window.__compatOrder.push("fixture");
    var api = window.PluginApi, R = api.React, Link = api.libraries.ReactRouterDOM.Link;
    api.register.route("/compat-fixture", function () {
      var state = R.useState(0);
      return R.createElement("main", {className:"compat-fixture container my-3"},
        R.createElement("h1", null, "Compatibility fixture route"),
        R.createElement(api.libraries.Bootstrap.Button, {variant:"secondary",onClick:function () { state[1](state[0]+1); }}, "Fixture count " + state[0]),
        R.createElement(Link, {to:"/scenes/1",className:"d-block mt-3"}, "Open native Video"),
        R.createElement(Link, {to:"/settings?tab=plugins",className:"d-block mt-3"}, "Open plugin settings"),
        R.createElement(Link, {to:"/settings?tab=tasks#tasks-plugins",className:"d-block mt-3"}, "Open plugin tasks"));
    });
  })();`
  );
  await writeFile(
    join(plugins, "task.js"),
    `function main() {
    if (input.Args.mode === "report") return {Error:"Fixture reported failure"};
    if (input.Args.mode === "throw") throw new Error("Fixture script exception");
    if (input.Args.mode === "wait") { while (true) { log.Progress(0.25); util.Sleep(25); } }
    var settings = gql.Do("{configuration {plugins}}").configuration.plugins.compat;
    var name = "Plugin receipt: " + settings.marker;
    var tags = gql.Do("{allTags {id name}}").allTags;
    for (var i=0;i<tags.length;i++) if (tags[i].name === name) return {Output:tags[i].id};
    var tag = gql.Do("mutation($input:TagCreateInput!) {tagCreate(input:$input) {id}}", {input:{name:name}}).tagCreate;
    return {Output:tag.id};
  } main();`
  );
}
async function check(viewport, label) {
  const directory = join(root, label);
  await fixture(directory);
  let url = await start(directory, label);
  let page = await browser.newPage({ viewport });
  const errors = [];
  const track = () =>
    page.on("pageerror", (error) => errors.push(String(error)));
  track();
  await call(
    page,
    url,
    "mutation {configureUI(input:{lastNoteSeen:99999999})}"
  );
  const tag = (
    await call(
      page,
      url,
      'mutation {tagCreate(input:{name:"Native fixture tag"}) {id}}'
    )
  ).tagCreate;
  const scene = (
    await call(
      page,
      url,
      "mutation($input:SceneCreateInput!) {sceneCreate(input:$input) {id}}",
      {
        input: {
          title: "Plugin fixture Video",
          details: "Native metadata stays",
          tag_ids: [tag.id],
          custom_fields: { note: "Retain value" },
        },
      }
    )
  ).sceneCreate;
  assert.equal(scene.id, "1");
  const snapshot = async () =>
    await call(
      page,
      url,
      "query($id:ID!) {findScene(id:$id) {id title details custom_fields tags {id} files {id} groups {group {id} scene_index}}}",
      { id: scene.id }
    );
  const before = await snapshot();
  const config = async () =>
    (await call(page, url, "{configuration {plugins}}")).configuration.plugins;
  async function route() {
    await page.goto(url + "/compat-fixture", { waitUntil: "networkidle" });
    await page
      .getByRole("heading", {
        name: "Compatibility fixture route",
        exact: true,
      })
      .waitFor();
    assert.deepEqual(await page.evaluate(() => window.__compatOrder), [
      "base",
      "fixture",
    ]);
    assert.equal(await page.locator(".compat-fixture").count(), 1);
    await page.waitForFunction(
      () =>
        getComputedStyle(document.querySelector(".compat-fixture"))
          .getPropertyValue("--compat-ready")
          .trim() === "1"
    );
  }
  await route();
  await page
    .getByRole("button", { name: "Fixture count 0", exact: true })
    .click();
  await page
    .getByRole("button", { name: "Fixture count 1", exact: true })
    .waitFor();
  await page
    .getByRole("link", { name: "Open native Video", exact: true })
    .click();
  await page
    .getByText("Plugin fixture Video", { exact: true })
    .first()
    .waitFor();
  await page.goBack();
  await page
    .getByRole("heading", { name: "Compatibility fixture route", exact: true })
    .waitFor();
  assert.deepEqual(await page.evaluate(() => window.__compatOrder), [
    "base",
    "fixture",
  ]);
  await page
    .getByRole("link", { name: "Open plugin settings", exact: true })
    .click();
  const group = () =>
    page
      .locator(".setting-group")
      .filter({
        has: page.getByText("Compatibility fixture (1.0)", { exact: true }),
      })
      .first();
  await group().locator(".setting-group-collapse-button").click();
  async function actionResponse(operation, action) {
    const [response] = await Promise.all([
      page.waitForResponse(
        (r) => r.request().postDataJSON()?.operationName === operation
      ),
      action(),
    ]);
    const payload = await response.json();
    assert.equal(payload.errors, undefined, JSON.stringify(payload.errors));
    return payload.data;
  }
  await page
    .locator("#plugin-compat-marker")
    .getByRole("button", { name: "Edit", exact: true })
    .click();
  const settingDialog = page.locator("#setting-dialog");
  await settingDialog.locator("input.text-input").fill("saved marker");
  await actionResponse("ConfigurePlugin", () =>
    settingDialog.getByRole("button", { name: "Confirm", exact: true }).click()
  );
  await settingDialog.waitFor({ state: "detached" });
  assert.equal((await config()).compat.marker, "saved marker");
  const reloaded = await actionResponse("ReloadPlugins", () =>
    page.getByRole("button", { name: "Reload plugins", exact: true }).click()
  );
  assert.equal(reloaded.reloadPlugins, true);
  assert.deepEqual(await page.evaluate(() => window.__compatOrder), [
    "base",
    "fixture",
  ]);
  await page.reload({ waitUntil: "networkidle" });
  await group().locator(".setting-group-collapse-button").click();
  assert.equal(
    await page.locator("#plugin-compat-marker .value").textContent(),
    "saved marker"
  );
  await route();
  await page
    .getByRole("link", { name: "Open plugin tasks", exact: true })
    .click();
  async function tasks() {
    const disclosure = page.locator("#tasks-plugins");
    await disclosure.waitFor();
    if (!(await disclosure.evaluate((e) => e.open)))
      await disclosure.locator(":scope > summary").click();
  }
  await tasks();
  const job = async (id) =>
    (
      await call(
        page,
        url,
        "query($id:ID!){findJob(input:{id:$id}) {id status error progress}}",
        { id }
      )
    ).findJob;
  async function terminal(id, wanted) {
    let result;
    for (let attempt = 0; attempt < 120; attempt++) {
      result = await job(id);
      if (result && ["FINISHED", "FAILED", "CANCELLED"].includes(result.status))
        break;
      await new Promise((done) => setTimeout(done, 100));
    }
    assert.equal(result?.status, wanted, JSON.stringify(result));
    return result;
  }
  async function run(name, wanted) {
    const result = await actionResponse("RunPluginTask", () =>
      page.getByRole("button", { name, exact: true }).click()
    );
    const id = result.runPluginTask;
    if (wanted) await terminal(id, wanted);
    return id;
  }
  const reported = await run("Fixture reported failure", "FAILED");
  assert.match((await job(reported)).error, /Fixture reported failure/);
  const failedRow = (name) =>
    page.locator("li.job").filter({ hasText: "Running plugin task: " + name });
  await failedRow("Fixture reported failure")
    .locator(".job-status.failed")
    .waitFor();
  assert.match(
    await failedRow("Fixture reported failure")
      .locator(".job-error")
      .textContent(),
    /Fixture reported failure/
  );
  const thrown = await run("Fixture script exception", "FAILED");
  assert.match((await job(thrown)).error, /Fixture script exception/);
  await failedRow("Fixture script exception")
    .locator(".job-status.failed")
    .waitFor();
  await run("Fixture successful task", "FINISHED");
  const tags = (await call(page, url, "{allTags {name}}")).allTags;
  assert.equal(
    tags.filter((t) => t.name === "Plugin receipt: saved marker").length,
    1
  );
  const cancellable = await run("Fixture cancellable task");
  let running;
  for (let attempt = 0; attempt < 120; attempt++) {
    running = await job(cancellable);
    if (running?.status === "RUNNING" && running.progress >= 0.25) break;
    await new Promise((done) => setTimeout(done, 100));
  }
  assert.equal(running?.status, "RUNNING");
  assert.ok(
    running.progress >= 0.25,
    "Cancel an executing VM, not a queued task"
  );
  await failedRow("Fixture cancellable task")
    .getByRole("button", {
      name: "Stop Running plugin task: Fixture cancellable task",
      exact: true,
    })
    .click();
  await terminal(cancellable, "CANCELLED");
  await run("Fixture successful task", "FINISHED");
  // Failed notices must remain after the live queue removes its completed jobs.
  for (let attempt = 0; attempt < 80; attempt++) {
    if (!(await call(page, url, "{jobQueue {id}}")).jobQueue?.length) break;
    await new Promise((done) => setTimeout(done, 100));
  }
  assert.deepEqual(
    (await call(page, url, "{jobQueue {id}}")).jobQueue ?? [],
    []
  );
  await page.reload({ waitUntil: "networkidle" });
  await failedRow("Fixture reported failure")
    .locator(".job-status.failed")
    .waitFor();
  await failedRow("Fixture script exception")
    .locator(".job-status.failed")
    .waitFor();
  assert.match(
    await failedRow("Fixture script exception")
      .locator(".job-error")
      .textContent(),
    /Fixture script exception/
  );
  await page
    .locator(".job-status.finished, .job-status.cancelled")
    .first()
    .waitFor({ state: "detached" });
  await page
    .locator("#job-queue")
    .screenshot({ path: join(artifacts, label + "-failures.png") });
  await failedRow("Fixture reported failure")
    .getByRole("button", { name: "Dismiss", exact: true })
    .click();
  await failedRow("Fixture reported failure").waitFor({ state: "detached" });
  await page.reload({ waitUntil: "networkidle" });
  assert.equal(await failedRow("Fixture reported failure").count(), 0);
  await failedRow("Fixture script exception")
    .locator(".job-status.failed")
    .waitFor();
  // Troubleshooting disables UI customizations, not server-side task availability.
  await call(
    page,
    url,
    "mutation {configureInterface(input:{disableCustomizations:true}) {disableCustomizations}}"
  );
  await page.reload({ waitUntil: "networkidle" });
  assert.equal(await page.evaluate(() => window.__compatOrder), undefined);
  await tasks();
  await page
    .getByRole("button", { name: "Fixture successful task", exact: true })
    .waitFor();
  await call(
    page,
    url,
    "mutation {configureInterface(input:{disableCustomizations:false}) {disableCustomizations}}"
  );
  await route();
  await page
    .getByRole("link", { name: "Open plugin settings", exact: true })
    .click();
  await group().getByRole("button", { name: "Disable", exact: true }).click();
  await group().getByRole("button", { name: "Enable", exact: true }).waitFor();
  await group().getByRole("button", { name: "Reload UI", exact: true }).click();
  await page.waitForLoadState("networkidle");
  assert.deepEqual(await page.evaluate(() => window.__compatOrder), ["base"]);
  const listed = (await call(page, url, "{plugins {id enabled}}")).plugins;
  assert.equal(listed.find((p) => p.id === "compat").enabled, false);
  await page.goto(url + "/settings?tab=tasks#tasks-plugins", {
    waitUntil: "networkidle",
  });
  await tasks();
  assert.equal(
    await page
      .getByRole("button", { name: "Fixture successful task", exact: true })
      .count(),
    0
  );
  await page.goto(url + "/compat-fixture", { waitUntil: "networkidle" });
  assert.equal(
    await page
      .getByRole("heading", {
        name: "Compatibility fixture route",
        exact: true,
      })
      .count(),
    0
  );
  await page.goto(url + "/settings?tab=plugins", { waitUntil: "networkidle" });
  await group().getByRole("button", { name: "Enable", exact: true }).click();
  await group().getByRole("button", { name: "Disable", exact: true }).waitFor();
  await group().getByRole("button", { name: "Reload UI", exact: true }).click();
  await page.waitForLoadState("networkidle");
  await route();
  assert.equal((await config()).compat.marker, "saved marker");
  assert.deepEqual(await snapshot(), before);
  assert.equal(
    await page.evaluate(
      () => document.documentElement.scrollWidth > innerWidth
    ),
    false
  );
  await page.screenshot({ path: join(artifacts, label + "-route.png") });
  await page.close();
  await stop();
  url = await start(directory, label + "-restart");
  page = await browser.newPage({ viewport });
  track();
  await route();
  assert.equal((await config()).compat.marker, "saved marker");
  assert.deepEqual(await snapshot(), before);
  assert.equal(
    (await call(page, url, "{plugins {id enabled}}")).plugins.find(
      (p) => p.id === "compat"
    ).enabled,
    true
  );
  assert.deepEqual(errors, []);
  await page.close();
  await stop();
  console.log(
    label +
      ": plugin reported/thrown errors, success/cancel/retry, retained failure details, dependency order/CSS/route, settings, customization bypass, disable/re-enable, native metadata and restart passed."
  );
}
try {
  await check({ width: 1440, height: 1000 }, "desktop");
  await check({ width: 390, height: 844 }, "mobile");
  successful = true;
} catch (error) {
  console.error(error);
  for (const page of browser.contexts().flatMap((c) => c.pages()))
    await page
      .screenshot({
        path: join(artifacts, "plugin-failure.png"),
        fullPage: true,
      })
      .catch(() => {});
  throw error;
} finally {
  await browser.close();
  await stop();
  if (successful && process.env.STASH_BROWSER_SCREENSHOTS)
    await rm(root, { recursive: true, force: true });
  else console.log("Isolated plugin fixture: " + root);
}
