import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { once } from "node:events";
import { createWriteStream } from "node:fs";
import { createServer } from "node:net";
import { join, resolve } from "node:path";

// Callers create fresh configuration/database folders. Never inherit Stash path
// or credential overrides from the environment into an acceptance instance.
export async function startIsolatedServer({
  binary,
  directory,
  artifacts,
  label,
  ui,
}) {
  const listener = createServer();
  listener.listen(0, "127.0.0.1");
  await once(listener, "listening");
  const port = listener.address().port;
  await new Promise((done) => listener.close(done));
  const url = "http://127.0.0.1:" + port;
  const env = Object.fromEntries(
    Object.entries(process.env).filter(([key]) => !key.startsWith("STASH_"))
  );
  env.STASH_LOCAL_HTTPS_ENABLED = "false";
  if (ui) env.STASH_UI = resolve(ui);
  const output = createWriteStream(join(artifacts, label + "-server.log"));
  const server = spawn(
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
  async function stop() {
    if (server.exitCode === null && server.signalCode === null) {
      const exited = once(server, "exit");
      server.kill("SIGTERM");
      await exited;
    }
    output.end();
  }
  try {
    for (let attempt = 0; attempt < 120; attempt++) {
      if (spawnError) throw spawnError;
      assert.equal(server.exitCode, null, "Isolated server exited");
      try {
        const response = await fetch(url + "/graphql", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ query: "{ systemStatus { status } }" }),
          signal: AbortSignal.timeout(1000),
        });
        if ((await response.json()).data?.systemStatus) return { url, stop };
      } catch {
        /* Wait for this new process to bind and initialize. */
      }
      await new Promise((done) => setTimeout(done, 250));
    }
    throw new Error("Isolated server did not become ready");
  } catch (error) {
    await stop();
    throw error;
  }
}
