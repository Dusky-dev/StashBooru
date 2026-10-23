import { useMemo, useState } from "react";
import ReactDOM from "react-dom";
import {
  ApolloClient,
  ApolloLink,
  ApolloProvider,
  InMemoryCache,
  Observable,
} from "@apollo/client";
import { BrowserRouter, Link, Route, Switch } from "react-router-dom";
import { IntlProvider } from "react-intl";
import { ConfigurationProvider } from "../../../src/hooks/Config";
import { Welcome } from "../../../src/components/Setup/Welcome";
import { ReleaseNotesDialog } from "../../../src/components/Dialogs/ReleaseNotesDialog";
import Changelog from "../../../src/components/Changelog/Changelog";
import { unseenReleaseNotes } from "../../../src/docs/en/ReleaseNotes";
import { useConfigureUI } from "../../../src/core/StashService";
import * as GQL from "../../../src/core/generated-graphql";
import messages from "../../../src/locales/en-GB.json";
import flattenMessages from "../../../src/utils/flattenMessages";
import "../../../src/index.scss";

const calls: { name: string; input?: Record<string, unknown> }[] = [];
Object.assign(window, { firstUseFixture: { calls } });

function Notes({ ui }: { ui: Record<string, unknown> }) {
  const [saveUI] = useConfigureUI();
  const notes = unseenReleaseNotes(ui.lastNoteSeen as number | undefined);
  return notes.length ? (
    <ReleaseNotesDialog
      notes={notes}
      onClose={() => {
        void saveUI({
          variables: { input: { ...ui, lastNoteSeen: notes[0].date } },
        });
      }}
    />
  ) : (
    <h1>All notes acknowledged</h1>
  );
}

function Fixture() {
  const [ui, setUI] = useState<Record<string, unknown>>(() =>
    JSON.parse(
      localStorage.getItem("first-use-fixture-ui") ??
        '{"title":"Retained title","customCSS":"retained preference"}'
    )
  );
  const client = useMemo(
    () =>
      new ApolloClient({
        cache: new InMemoryCache(),
        defaultOptions: {
          watchQuery: { fetchPolicy: "no-cache" },
          query: { fetchPolicy: "no-cache" },
        },
        link: new ApolloLink(
          (operation) =>
            new Observable((observer) => {
              calls.push({
                name: operation.operationName,
                input: operation.variables.input,
              });
              if (operation.operationName === "SystemStatus") {
                observer.next({
                  data: {
                    systemStatus: {
                      databaseSchema: 1,
                      appSchema: 1,
                      status: "OK",
                      databasePath: "/fixture/stash.sqlite",
                      configPath: "/fixture/config.yml",
                      os: "linux",
                      workingDir: "/fixture",
                      homeDir: "/fixture",
                      ffmpegPath: "/fixture/ffmpeg",
                      ffprobePath: "/fixture/ffprobe",
                    },
                  },
                });
              } else if (operation.operationName === "ConfigureUI") {
                const next = operation.variables.input;
                localStorage.setItem(
                  "first-use-fixture-ui",
                  JSON.stringify(next)
                );
                setUI(next);
                observer.next({ data: { configureUI: next } });
              } else
                observer.error(
                  new Error(
                    `Unexpected first-use operation: ${operation.operationName}`
                  )
                );
              observer.complete();
            })
        ),
      }),
    []
  );
  const configuration = { ui } as unknown as GQL.ConfigDataFragment;
  return (
    <ApolloProvider client={client}>
      <ConfigurationProvider configuration={configuration}>
        <BrowserRouter
          basename={new URL(document.baseURI).pathname.replace(/\/$/, "")}
        >
          <IntlProvider locale="en-GB" messages={flattenMessages(messages)}>
            <main className="container-fluid p-3">
              <nav className="mb-3 d-flex flex-wrap">
                <Link className="mr-3" to="/notes">
                  Notes fixture
                </Link>
                <Link className="mr-3" to="/welcome">
                  Welcome fixture
                </Link>
                <Link to="/settings?tab=changelog">Changelog fixture</Link>
              </nav>
              <Switch>
                <Route path="/welcome">
                  <Welcome />
                </Route>
                <Route
                  path="/settings"
                  render={({ location }) =>
                    new URLSearchParams(location.search).get("tab") ===
                    "changelog" ? (
                      <Changelog />
                    ) : (
                      <h1>Library destination fixture</h1>
                    )
                  }
                />
                <Route>
                  <Notes ui={ui} />
                </Route>
              </Switch>
            </main>
          </IntlProvider>
        </BrowserRouter>
      </ConfigurationProvider>
    </ApolloProvider>
  );
}
ReactDOM.render(<Fixture />, document.getElementById("root"));
