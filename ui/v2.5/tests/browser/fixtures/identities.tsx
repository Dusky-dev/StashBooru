import ReactDOM from "react-dom";
import { Dropdown } from "react-bootstrap";
import { BrowserRouter, Link, Route, useLocation } from "react-router-dom";
import Identities from "../../../src/components/Identities/Identities";
import {
  IdentityLink,
  IdentitySources,
} from "../../../src/components/Identities/identity";
import "../../../src/index.scss";
import "../../../src/booru-ui.scss";

function ProviderTag() {
  const params = new URLSearchParams(useLocation().search);
  const kind = params.get("kind") ?? "artist";
  const target =
    kind === "character"
      ? "character_a"
      : kind === "copyright"
        ? "series_a"
        : "waligner_(artist)";
  return (
    <Dropdown>
      <Dropdown.Toggle>Provider tag</Dropdown.Toggle>
      <Dropdown.Menu>
        <IdentityLink
          source={{
            id: "gelbooru",
            instance: "gel",
            name: "Gelbooru",
            engine: "gelbooru",
            url: "https://gelbooru.com",
            queryHelp: "",
            authentication: "",
            credentialsConfigured: false,
            categorizedTags: true,
          }}
          kind={kind}
          target={target}
          identityID={params.get("identity") ?? "2"}
        />
      </Dropdown.Menu>
    </Dropdown>
  );
}

ReactDOM.render(
  <BrowserRouter
    basename={new URL(document.baseURI).pathname.replace(/\/$/, "")}
  >
    <main className="container-fluid py-3">
      <Route exact path="/identities" component={Identities} />
      <Route exact path="/studios/7">
        <h2>Waligner</h2>
        <IdentitySources kind="artist" entityID="7" entityName="Waligner" />
        <Link to="/identities?kind=artist">All identities</Link>
      </Route>
      <Route exact path="/booru" component={ProviderTag} />
      <Link to="/booru">Browse fixture</Link>
    </main>
  </BrowserRouter>,
  document.getElementById("root")
);
