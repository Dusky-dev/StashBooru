import ReactDOM from "react-dom";
import { BrowserRouter, Link, Route, useLocation } from "react-router-dom";
import { IdentitySources } from "../../../src/components/Identities/identity";
import Following from "../../../src/components/Following/Following";
import "../../../src/index.scss";
import "../../../src/booru-ui.scss";

function ProfileFixture() {
  const value = new URLSearchParams(useLocation().search).get("kind");
  const kind =
    value === "character" || value === "copyright" ? value : "artist";
  return (
    <>
      <h2>Native profile fixture</h2>
      <label htmlFor="profile-draft">Retained fixture input</label>
      <input id="profile-draft" className="form-control" />
      <IdentitySources kind={kind} entityID="7" entityName="Local profile" />
    </>
  );
}

ReactDOM.render(
  <BrowserRouter
    basename={new URL(document.baseURI).pathname.replace(/\/$/, "")}
  >
    <main className="container-fluid py-3">
      <Route exact path="/following" component={Following} />
      <Route exact path="/profile" component={ProfileFixture} />
      <Route exact path="/other">
        <p>Other fixture page</p>
        <Link to="/following">Return to Following</Link>
      </Route>
      <Link to="/other">Leave Following fixture</Link>
    </main>
  </BrowserRouter>,
  document.getElementById("root")
);
