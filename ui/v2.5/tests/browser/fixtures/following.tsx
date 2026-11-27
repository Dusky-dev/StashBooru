import ReactDOM from "react-dom";
import { IntlProvider } from "react-intl";
import { BrowserRouter, Link, Route, useLocation } from "react-router-dom";
import { IdentitySources } from "../../../src/components/Identities/identity";
import Following from "../../../src/components/Following/Following";
import { FollowedPosts } from "../../../src/components/Following/FollowedPosts";
import "../../../src/index.scss";
import "../../../src/booru-ui.scss";
import { FollowingEnabled } from "./followingEnabled";
import { messages } from "./messages";

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
  <IntlProvider locale="en-GB" messages={messages}>
    <FollowingEnabled>
      <BrowserRouter
        basename={new URL(document.baseURI).pathname.replace(/\/$/, "")}
      >
        <main className="container-fluid py-3">
          <Route exact path="/following" component={Following} />
          <Route exact path="/profile" component={ProfileFixture} />
          <Route exact path="/artist">
            <h2>Artist fixture</h2>
            <FollowedPosts kind="artist" entityID="7" />
            <h2>Character fixture</h2>
            <FollowedPosts kind="character" entityID="7" />
          </Route>
          <Route exact path="/other">
            <p>Other fixture page</p>
            <Link to="/following?view=posts">Return to Following</Link>
          </Route>
          <Link to="/other">Leave Following fixture</Link>
        </main>
      </BrowserRouter>
    </FollowingEnabled>
  </IntlProvider>,
  document.getElementById("root")
);
