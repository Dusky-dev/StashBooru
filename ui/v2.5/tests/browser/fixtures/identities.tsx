import ReactDOM from "react-dom";
import { Dropdown } from "react-bootstrap";
import { BrowserRouter, Link, Route } from "react-router-dom";
import Identities from "../../../src/components/Identities/Identities";
import {
  IdentityLink,
  IdentitySources,
} from "../../../src/components/Identities/identity";
import "../../../src/index.scss";
import "../../../src/booru-ui.scss";

ReactDOM.render(
  <BrowserRouter
    basename={new URL(document.baseURI).pathname.replace(/\/$/, "")}
  >
    <main className="container-fluid py-3">
      <Route exact path="/identities" component={Identities} />
      <Route exact path="/studios/7">
        <h2>Waligner</h2>
        <IdentitySources kind="artist" entityID="7" />
        <Link to="/identities?kind=artist">All identities</Link>
      </Route>
      <Route exact path="/booru">
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
                enabled: true,
              }}
              kind="artist"
              target="waligner_(artist)"
              identityID="2"
            />
          </Dropdown.Menu>
        </Dropdown>
      </Route>
      <Link to="/booru">Browse fixture</Link>
    </main>
  </BrowserRouter>,
  document.getElementById("root")
);
