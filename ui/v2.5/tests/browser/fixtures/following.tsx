import ReactDOM from "react-dom";
import { BrowserRouter, Link, Route } from "react-router-dom";
import Following from "../../../src/components/Following/Following";
import "../../../src/index.scss";
import "../../../src/booru-ui.scss";

ReactDOM.render(
  <BrowserRouter
    basename={new URL(document.baseURI).pathname.replace(/\/$/, "")}
  >
    <main className="container-fluid py-3">
      <Route exact path="/following" component={Following} />
      <Route exact path="/other">
        <p>Other fixture page</p>
        <Link to="/following">Return to Following</Link>
      </Route>
      <Link to="/other">Leave Following fixture</Link>
    </main>
  </BrowserRouter>,
  document.getElementById("root")
);
