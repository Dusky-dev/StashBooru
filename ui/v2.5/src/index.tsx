import { ApolloProvider } from "@apollo/client";
import ReactDOM from "react-dom";
import { BrowserRouter } from "react-router-dom";
import { App } from "./App";
import { getClient } from "./core/StashService";
import { baseURL, getPlatformURL } from "./core/createClient";
import "./index.scss";
import * as serviceWorker from "./serviceWorker";

function loadBuiltinStylesheet(name: string) {
  const link = document.createElement("link");
  link.rel = "stylesheet";
  link.type = "text/css";
  link.href = getPlatformURL(`builtin/${name}`).toString();
  link.dataset.builtinUi = name;
  document.head.appendChild(link);
}

function loadBuiltinScript(name: string) {
  return new Promise<void>((resolve, reject) => {
    const script = document.createElement("script");
    script.src = getPlatformURL(`builtin/${name}`).toString();
    script.dataset.builtinUi = name;
    script.onload = () => resolve();
    script.onerror = () =>
      reject(new Error(`Could not load built-in UI script ${name}`));
    document.head.appendChild(script);
  });
}

async function loadBuiltinEnhancements() {
  loadBuiltinStylesheet("unifiedMedia.css");

  for (const script of [
    "unifiedMedia.js",
    "universal-file-info.js",
    "bundle-extra.js",
  ]) {
    try {
      await loadBuiltinScript(script);
    } catch (error) {
      console.error("[built-in-ui]", error);
    }
  }
}

async function bootstrap() {
  // App imports pluginApi as a side effect, so window.PluginApi already exists
  // here. Register the former plugins before React mounts any patchable UI.
  await loadBuiltinEnhancements();

  ReactDOM.render(
    <>
      <link
        rel="stylesheet"
        type="text/css"
        href={getPlatformURL("css").toString()}
      />
      <BrowserRouter basename={baseURL}>
        <ApolloProvider client={getClient()}>
          <App />
        </ApolloProvider>
      </BrowserRouter>
    </>,
    document.getElementById("root")
  );

  const script = document.createElement("script");
  script.src = getPlatformURL("javascript").toString();
  document.body.appendChild(script);
}

void bootstrap();

// If you want your app to work offline and load faster, you can change
// unregister() to register() below. Note this comes with some pitfalls.
// Learn more about service workers: http://bit.ly/CRA-PWA
serviceWorker.unregister();
