import ReactDOM from "react-dom";
import {
  ApolloClient,
  ApolloLink,
  ApolloProvider,
  InMemoryCache,
  Observable,
} from "@apollo/client";
import { BrowserRouter } from "react-router-dom";
import { IntlProvider } from "react-intl";
import { Settings } from "../../../src/components/Settings/Settings";
import { ConfigurationProvider } from "../../../src/hooks/Config";
import { ToastProvider } from "../../../src/hooks/Toast";
import { JobQueueProvider } from "../../../src/hooks/JobQueue";
import * as GQL from "../../../src/core/generated-graphql";
import messages from "../../../src/locales/en-GB.json";
import "../../../src/index.scss";

const config = {
  general: {
    apiKey: "",
    stashes: [],
    logLevel: "Info",
    generatedPath: "/stash/generated",
    cachePath: "/stash/cache",
    metadataPath: "/stash/metadata",
    scrapersPath: "/stash/scrapers",
    pluginsPath: "/stash/plugins",
    videoExtensions: ["mp4"],
    imageExtensions: ["jpg", "png"],
    galleryExtensions: ["zip"],
    exclude: [],
    imageExclude: [],
    customPerformerImageLocation: "",
  },
  interface: { disableCustomizations: false, menuItems: [], imageLightbox: {} },
  defaults: {
    scan: {},
    generate: {},
    associationInheritance: {
      characters: true,
      copyrights: true,
      artists: true,
      tags: true,
    },
  },
  scraping: {},
  dlna: {},
  ui: { title: "StashBooru" },
  plugins: {},
} as unknown as GQL.ConfigDataFragment;

function flatten(
  value: Record<string, unknown>,
  prefix = ""
): Record<string, string> {
  return Object.fromEntries(
    Object.entries(value).flatMap(([key, item]) => {
      const id = prefix + key;
      return typeof item === "string"
        ? [[id, item]]
        : Object.entries(flatten(item as Record<string, unknown>, `${id}.`));
    })
  );
}
const changes: unknown[] = [];
const release = { version: "1.0.0", error: "" };

Object.assign(window, {
  settingsFixture: {
    changes,
    release,
    dispose() {
      ReactDOM.unmountComponentAtNode(document.getElementById("root")!);
    },
  },
});
const client = new ApolloClient({
  cache: new InMemoryCache(),
  // Partial fixture configuration is sufficient for the real Settings context;
  // no server, catalogue or writes to the user's application are involved.
  defaultOptions: {
    watchQuery: { fetchPolicy: "no-cache" },
    query: { fetchPolicy: "no-cache" },
    mutate: { fetchPolicy: "no-cache" },
  },
  link: new ApolloLink(
    (operation) =>
      new Observable((observer) => {
        if (operation.operationName === "JobsSubscribe") return () => {};
        let data: Record<string, unknown> = {};
        if (operation.operationName === "LatestVersion") {
          if (release.error) {
            observer.error(new Error(release.error));
            return;
          }
          data = {
            latestversion: {
              version: release.version,
              shorthash: "different",
              release_date: "2026-10-05",
              url: "https://github.com/Dusky-dev/StashBooru/releases/tag/stashbooru-v1.0.0",
            },
          };
        } else if (operation.operationName === "Configuration")
          data = { configuration: config };
        else if (operation.operationName === "JobQueue")
          data = { jobQueue: [] };
        else if (operation.operationName === "ConfigureDefaults") {
          changes.push(operation.variables.input);
          Object.assign(config.defaults, operation.variables.input);
          data = { configureDefaults: config.defaults };
        }
        observer.next({ data });
        observer.complete();
      })
  ),
});

ReactDOM.render(
  <ApolloProvider client={client}>
    <BrowserRouter>
      <IntlProvider locale="en-GB" messages={flatten(messages)}>
        <ConfigurationProvider configuration={config}>
          <ToastProvider>
            <JobQueueProvider>
              <main className="container-fluid pt-4">
                <Settings />
              </main>
            </JobQueueProvider>
          </ToastProvider>
        </ConfigurationProvider>
      </IntlProvider>
    </BrowserRouter>
  </ApolloProvider>,
  document.getElementById("root")
);
