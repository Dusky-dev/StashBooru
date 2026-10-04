import { useState } from "react";
import ReactDOM from "react-dom";
import { MemoryRouter } from "react-router-dom";
import { Button } from "react-bootstrap";
import { VideoOverlapDialog } from "../../../src/components/Shared/VideoOverlapDialog";
import { VideoOverlapTask } from "../../../src/components/Settings/Tasks/VideoOverlapTask";
import "../../../src/index.scss";
import "../../../src/booru-ui.scss";

function Fixture() {
  const [show, setShow] = useState(true);
  return (
    <MemoryRouter>
      <main className="p-3">
        <h1>Isolated video overlap fixture</h1>
        <Button onClick={() => setShow(true)}>Open reference review</Button>
        <VideoOverlapTask />
        {show && (
          <VideoOverlapDialog referenceId={1} onHide={() => setShow(false)} />
        )}
      </main>
    </MemoryRouter>
  );
}
ReactDOM.render(<Fixture />, document.getElementById("root"));
