import ReactDOM from "react-dom";
import { IntlProvider } from "react-intl";
import { BrowserRouter, Link, Route } from "react-router-dom";
import BooruBrowser from "../../../src/components/BooruBrowser/BooruBrowser";
import "../../../src/index.scss";
import "../../../src/booru-ui.scss";

ReactDOM.render(
  <IntlProvider locale="en" messages={{}}>
    <BrowserRouter>
      <main className="container-fluid py-3">
        <Route path="/booru" component={BooruBrowser} />
        <Link to="/other">Leave browser fixture</Link>
      </main>
    </BrowserRouter>
  </IntlProvider>,
  document.getElementById("root")
);
