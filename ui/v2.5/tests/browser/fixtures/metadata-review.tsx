import { cloneElement, useEffect, useState } from "react";
import ReactDOM from "react-dom";
import { BrowserRouter } from "react-router-dom";
import { IntlProvider } from "react-intl";
import { Button, Form } from "react-bootstrap";
import { MediaKind } from "../../../src/core/generated-graphql";
import { ToastProvider } from "../../../src/hooks/Toast";
import { ListOperations } from "../../../src/components/List/ListOperationButtons";
import { metadataReviewOperation } from "../../../src/components/Tagging/MetadataReviewDialog";
import { ImageKnowledgeTagDialog } from "../../../src/components/Images/ImageDetails/ImageKnowledgeTagDialog";
import { VideoTaggingDialog } from "../../../src/components/Scenes/SceneDetails/VideoTaggingDialog";
import { BatchImageMetadataDialog } from "../../../src/components/Images/ImageDetails/BatchImageMetadataDialog";
import "../../../src/index.scss";

const original = [
  { kind: MediaKind.Image, id: "1" },
  { kind: MediaKind.Video, id: "1" },
  { kind: MediaKind.Image, id: "2" },
];
function Fixture() {
  const [view, setView] = useState("All");
  const [selected, setSelected] = useState(false);
  const [reverse, setReverse] = useState(false);
  const [modal, setModal] = useState<JSX.Element>();
  const [refreshes, setRefreshes] = useState(0);
  const items = (reverse ? [...original].reverse() : original).filter(
    (item) =>
      view === "All" ||
      item.kind === (view === "Images" ? MediaKind.Image : MediaKind.Video)
  );
  const selection = selected ? items : [];
  useEffect(() => {
    const change = () => setReverse(true);
    window.addEventListener("fixture-review-input-change", change);
    return () =>
      window.removeEventListener("fixture-review-input-change", change);
  }, []);
  function close() {
    setModal(undefined);
    setRefreshes((value) => value + 1);
  }
  return (
    <main className="p-3">
      <h1>Metadata review</h1>
      <Form.Label htmlFor="fixture-view">View</Form.Label>
      <Form.Control
        as="select"
        id="fixture-view"
        value={view}
        onChange={(e) => {
          setView(e.currentTarget.value);
          setReverse(false);
        }}
      >
        {["All", "Images", "Videos"].map((name) => (
          <option key={name}>{name}</option>
        ))}
      </Form.Control>
      <Form.Check
        type="switch"
        id="fixture-selected"
        label="Select items"
        checked={selected}
        onChange={(e) => setSelected(e.currentTarget.checked)}
      />
      <p>
        Selected items: {selection.length}. Refreshes: {refreshes}
      </p>
      <ListOperations
        items={items.length}
        hasSelection={selected}
        operations={[
          { text: "Other action", onClick: () => {} },
          metadataReviewOperation(selection, setModal, close),
        ]}
      />
      <Button
        onClick={() =>
          setModal(
            <ImageKnowledgeTagDialog
              imageId="1"
              onHide={close}
              onApplied={async () => {}}
            />
          )
        }
      >
        Single image
      </Button>
      <Button
        onClick={() =>
          setModal(
            <VideoTaggingDialog
              sceneId="1"
              onHide={close}
              onApplied={async () => {}}
            />
          )
        }
      >
        Single video
      </Button>
      <Button
        onClick={() =>
          setModal(
            <BatchImageMetadataDialog imageIds={["1", "2"]} onHide={close} />
          )
        }
      >
        Generate shortcut
      </Button>
      {modal && cloneElement(modal, { items: selection })}
    </main>
  );
}
ReactDOM.render(
  <BrowserRouter>
    <IntlProvider locale="en" messages={{ "actions.more": "More" }}>
      <ToastProvider>
        <Fixture />
      </ToastProvider>
    </IntlProvider>
  </BrowserRouter>,
  document.getElementById("root")
);
