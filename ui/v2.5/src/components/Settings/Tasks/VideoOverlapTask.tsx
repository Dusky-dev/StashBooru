import { useState } from "react";
import { Button } from "react-bootstrap";
import { VideoOverlapDialog } from "../../Shared/VideoOverlapDialog";

export function VideoOverlapTask() {
  const [show, setShow] = useState(false);
  return (
    <div id="video-overlap-task">
      <h2>Video overlap / containment</h2>
      <p>
        Incrementally index timestamped frames, then review trims, re-encodes
        and compilation segments. Index checkpoints can resume after
        cancellation or restart. Videos stay in their native catalogue.
      </p>
      <Button variant="secondary" onClick={() => setShow(true)}>
        Open video overlap review
      </Button>
      {show && <VideoOverlapDialog onHide={() => setShow(false)} />}
    </div>
  );
}
