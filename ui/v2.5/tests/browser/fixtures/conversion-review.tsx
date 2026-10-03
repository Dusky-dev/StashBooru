import { useState } from "react";
import ReactDOM from "react-dom";
import { MemoryRouter } from "react-router-dom";
import { Button } from "react-bootstrap";
import { ToastProvider } from "../../../src/hooks/Toast";
import { IntlProvider } from "react-intl";
import { MediaConversionDialog } from "../../../src/components/Shared/MediaConversionDialog";
import { MediaConversionSettings } from "../../../src/components/Settings/MediaConversionSettings";
import "../../../src/index.scss";
import "../../../src/booru-ui.scss";

function Fixture() {
  const [show, setShow] = useState(true);
  return (
    <MemoryRouter>
      <IntlProvider locale="en">
        <ToastProvider>
          <main className="p-3">
            <h1>Isolated conversion review fixture</h1>
            <Button onClick={() => setShow(true)}>Open converter</Button>
            <MediaConversionSettings />
            {show && (
              <MediaConversionDialog
                kind="image"
                selectedIds={["1", "2"]}
                targets={[
                  { kind: "image", id: 1 },
                  { kind: "scene", id: 1 },
                ]}
                onHide={() => setShow(false)}
              />
            )}
          </main>
        </ToastProvider>
      </IntlProvider>
    </MemoryRouter>
  );
}

ReactDOM.render(<Fixture />, document.getElementById("root"));
