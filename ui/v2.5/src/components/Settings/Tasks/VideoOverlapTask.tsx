import { useState } from "react";
import { Button } from "react-bootstrap";
import { VideoOverlapDialog } from "../../Shared/VideoOverlapDialog";
import { TaskHelpHeading } from "./TaskHelpHeading";

export function VideoOverlapTask() {
  const [show, setShow] = useState(false);
  return (
    <div id="video-overlap-task">
      <TaskHelpHeading title="Video overlap / containment" />
      <Button variant="secondary" onClick={() => setShow(true)}>
        Open video overlap review
      </Button>
      {show && <VideoOverlapDialog onHide={() => setShow(false)} />}
    </div>
  );
}
