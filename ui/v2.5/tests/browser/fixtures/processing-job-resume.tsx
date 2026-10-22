import { useState } from "react";
import ReactDOM from "react-dom";
import { MemoryRouter } from "react-router-dom";
import { IntlProvider } from "react-intl";
import { Button } from "react-bootstrap";
import { ToastProvider } from "../../../src/hooks/Toast";
import { MediaConversionDialog } from "../../../src/components/Shared/MediaConversionDialog";
import { MediaUpscalingDialog } from "../../../src/components/Shared/MediaUpscalingDialog";
import "../../../src/index.scss";
import "../../../src/booru-ui.scss";
function Fixture() {
  const [show, setShow] = useState(true);
  const upscale =
    new URLSearchParams(location.search).get("mode") === "upscale";
  return (
    <main className="p-3">
      <h1>Processing resume fixture</h1>
      <Button onClick={() => setShow(true)}>Reopen processing</Button>
      {show &&
        (upscale ? (
          <MediaUpscalingDialog
            selectedIds={["1"]}
            onHide={() => setShow(false)}
          />
        ) : (
          <MediaConversionDialog
            kind="image"
            selectedIds={["1"]}
            onHide={() => setShow(false)}
          />
        ))}
    </main>
  );
}
ReactDOM.render(
  <MemoryRouter>
    <IntlProvider locale="en">
      <ToastProvider>
        <Fixture />
      </ToastProvider>
    </IntlProvider>
  </MemoryRouter>,
  document.getElementById("root")
);
