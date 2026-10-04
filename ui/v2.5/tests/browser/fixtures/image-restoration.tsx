import { useState } from "react";
import ReactDOM from "react-dom";
import { MemoryRouter } from "react-router-dom";
import { Button } from "react-bootstrap";
import { IntlProvider } from "react-intl";
import { ImageRestorationDialog } from "../../../src/components/Shared/ImageRestorationDialog";
import "../../../src/index.scss";
import "../../../src/booru-ui.scss";

function Fixture() {
  const [show, setShow] = useState(true);
  return (
    <MemoryRouter>
      <IntlProvider locale="en">
        <main className="p-3">
          <h1>Isolated restoration fixture</h1>
          <Button onClick={() => setShow(true)}>Open restoration</Button>
          {show && (
            <ImageRestorationDialog imageID="1" onHide={() => setShow(false)} />
          )}
        </main>
      </IntlProvider>
    </MemoryRouter>
  );
}
ReactDOM.render(<Fixture />, document.getElementById("root"));
