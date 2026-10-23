import ReactDOM from "react-dom";
import {
  ApolloClient,
  ApolloLink,
  ApolloProvider,
  InMemoryCache,
  Observable,
} from "@apollo/client";
import { BrowserRouter, Route, Switch } from "react-router-dom";
import { useState } from "react";
import { PerformerDetailsPanel } from "../../../src/components/Performers/PerformerDetails/PerformerDetailsPanel";
import { StudioDetailsPanel } from "../../../src/components/Studios/StudioDetails/StudioDetailsPanel";
import { CopyrightDetailsPanel } from "../../../src/components/Tags/Copyrights";
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
        else if (operation.operationName === "ConfigureUI") {
          changes.push({ ui: operation.variables.partial });
          Object.assign(config.ui, operation.variables.partial);
          data = { configureUI: config.ui };
        } else if (operation.operationName === "MetadataAutoTag") {
          changes.push({ autoTagRun: operation.variables.input });
          data = { metadataAutoTag: "17" };
        } else if (operation.operationName === "ConfigureDefaults") {
          changes.push(operation.variables.input);
          Object.assign(config.defaults, operation.variables.input);
          data = { configureDefaults: config.defaults };
        }
        observer.next({ data });
        observer.complete();
      })
  ),
});

function ProfileFixture({
  kind,
}: {
  kind: "character" | "artist" | "copyright";
}) {
  const [draft, setDraft] = useState("Retained fixture note");
  const name = "Name & alias/タグ #7?";
  const common = { id: "7", name, tags: [], stash_ids: [], custom_fields: {} };
  return (
    <>
      <h1>{name}</h1>
      <label htmlFor="profile-fixture-note">Fixture draft</label>
      <input
        id="profile-fixture-note"
        className="text-input form-control"
        value={draft}
        onChange={(event) => setDraft(event.currentTarget.value)}
      />
      {kind === "character" && (
        <PerformerDetailsPanel
          performer={
            {
              ...common,
              copyrights: [],
            } as unknown as GQL.PerformerDataFragment
          }
          fullWidth
        />
      )}
      {kind === "artist" && (
        <StudioDetailsPanel
          studio={{ ...common, urls: [] } as unknown as GQL.StudioDataFragment}
          fullWidth
        />
      )}
      {kind === "copyright" && (
        <CopyrightDetailsPanel
          copyright={
            {
              ...common,
              parents: [],
              ordered_children: [],
              breadcrumb: [],
            } as unknown as GQL.CopyrightDataFragment
          }
          fullWidth
        />
      )}
    </>
  );
}

ReactDOM.render(
  <ApolloProvider client={client}>
    <BrowserRouter
      basename={new URL(document.baseURI).pathname.replace(/\/$/, "")}
    >
      <IntlProvider locale="en-GB" messages={flatten(messages)}>
        <ConfigurationProvider configuration={config}>
          <ToastProvider>
            <JobQueueProvider>
              <main className="container-fluid pt-4">
                <Switch>
                  <Route path="/performers/7">
                    <ProfileFixture kind="character" />
                  </Route>
                  <Route path="/studios/7">
                    <ProfileFixture kind="artist" />
                  </Route>
                  <Route path="/copyrights/7">
                    <ProfileFixture kind="copyright" />
                  </Route>
                  <Route>
                    <Settings />
                  </Route>
                </Switch>
              </main>
            </JobQueueProvider>
          </ToastProvider>
        </ConfigurationProvider>
      </IntlProvider>
    </BrowserRouter>
  </ApolloProvider>,
  document.getElementById("root")
);
