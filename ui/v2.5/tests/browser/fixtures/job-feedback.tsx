import { useState } from "react";
import ReactDOM from "react-dom";
import { BrowserRouter, useLocation } from "react-router-dom";
import { Button, Form } from "react-bootstrap";
import {
  ProcessingJobFeedback,
  type ProcessingJobItem,
} from "../../../src/components/Shared/JobFeedback";
import "../../../src/index.scss";
import "../../../src/booru-ui.scss";

declare global {
  interface Window {
    releaseJobCancel?: () => void;
  }
}
const initial: ProcessingJobItem[] = [
  {
    target: { kind: "image", id: 1 },
    error: "Image decode failed\nDecoder detail",
  },
  { target: { kind: "scene", id: 1 }, error: "Video decode failed" },
  {
    target: { kind: "image", id: 0 },
    trialID: "trial-a",
    error: "Missing trial",
  },
];
function Fixture() {
  const [status, setStatus] = useState("complete");
  const [items, setItems] = useState(initial);
  const [show, setShow] = useState(true);
  const [cancelling, setCancelling] = useState(false);
  const [cancellations, setCancellations] = useState(0);
  const location = useLocation();
  async function cancel() {
    setCancelling(true);
    setCancellations((value) => value + 1);
    await new Promise<void>((resolve) => {
      window.releaseJobCancel = resolve;
    });
    setStatus("cancelled");
    setCancelling(false);
  }
  return (
    <main className="p-3">
      <h1>Processing job feedback</h1>
      <Form.Label htmlFor="fixture-status">Job state</Form.Label>
      <Form.Control
        as="select"
        id="fixture-status"
        value={status}
        onChange={(e) => setStatus(e.currentTarget.value)}
      >
        {[
          "ready",
          "queued",
          "running",
          "stopping",
          "finished",
          "complete",
          "cancelled",
          "failed",
          "future-state",
        ].map((value) => (
          <option key={value}>{value}</option>
        ))}
      </Form.Control>
      <Button onClick={() => setItems([])}>Clear errors</Button>
      <Button
        onClick={() =>
          setItems(
            Array.from({ length: 53 }, (_, index) => ({
              target: { kind: "image", id: index + 1 },
              error: `Failure ${index + 1}\nFull detail ${index + 1}`,
            }))
          )
        }
      >
        Many errors
      </Button>
      <Button onClick={() => setShow(true)}>Reopen feedback</Button>
      <p>Selected items: 2 · Cancellation requests: {cancellations}</p>
      <p aria-label="Fixture location">
        {location.pathname}
        {location.search}
      </p>
      {show && (
        <ProcessingJobFeedback
          job={{ jobID: 9, status, total: 53, items }}
          cancelling={cancelling}
          onCancel={() => void cancel()}
          onNavigate={() => setShow(false)}
        />
      )}
    </main>
  );
}
ReactDOM.render(
  <BrowserRouter>
    <Fixture />
  </BrowserRouter>,
  document.getElementById("root")
);
