import React from "react";
import { Form } from "react-bootstrap";
import { Link } from "react-router-dom";
import { faFingerprint } from "@fortawesome/free-solid-svg-icons";
import type { RenderImageProps } from "react-photo-gallery";
import { useIntl } from "react-intl";
import { Icon } from "../Shared/Icon";
import { useDragMoveSelect } from "../Shared/GridCard/dragMoveSelect";
import NavUtils from "src/utils/navigation";

interface IExtraProps {
  maxHeight: number;
  selected?: boolean;
  onSelectedChanged?: (selected: boolean, shiftKey: boolean) => void;
  selecting?: boolean;
}

export const ImageWallItem: React.FC<RenderImageProps & IExtraProps> = (
  props: RenderImageProps & IExtraProps
) => {
  const intl = useIntl();
  const { dragProps } = useDragMoveSelect({
    selecting: props.selecting || false,
    selected: props.selected || false,
    onSelectedChanged: props.onSelectedChanged,
  });

  const height = Math.min(props.maxHeight, props.photo.height);
  const zoomFactor = height / props.photo.height;
  const width = props.photo.width * zoomFactor;

  type style = Record<string, string | number | undefined>;
  var divStyle: style = {
    margin: props.margin,
    display: "block",
    position: "relative",
  };

  if (props.direction === "column") {
    divStyle.position = "absolute";
    divStyle.left = props.left;
    divStyle.top = props.top;
  }

  var handleClick = function handleClick(
    event: React.MouseEvent<Element, MouseEvent>
  ) {
    if (props.selecting && props.onSelectedChanged) {
      props.onSelectedChanged(!props.selected, event.shiftKey);
      event.preventDefault();
      event.stopPropagation();
      return;
    }
    if (props.onClick) {
      props.onClick(event, { index: props.index });
    }
  };

  // Image path fields are nullable in GraphQL. The wall photo type says src is
  // always a string, but older/generated records can still reach this render
  // boundary with a null preview path. Normalize it here before any string
  // operations so one missing preview cannot crash the entire wall.
  const source = typeof props.photo.src === "string" ? props.photo.src : "";
  const video = source.includes("preview");
  const ImagePreview = video ? "video" : "img";
  const imageID = props.photo.key;
  const findSimilarLabel = intl.formatMessage({
    id: "actions.find_similar",
    defaultMessage: "Find similar",
  });

  let shiftKey = false;

  return (
    <div
      className="wall-item"
      style={divStyle}
      onClick={handleClick}
      {...dragProps}
    >
      {props.onSelectedChanged && (
        <Form.Control
          type="checkbox"
          className="wall-item-check mousetrap"
          checked={props.selected}
          onChange={() => props.onSelectedChanged!(!props.selected, shiftKey)}
          onClick={(event: React.MouseEvent<HTMLInputElement, MouseEvent>) => {
            shiftKey = event.shiftKey;
            event.stopPropagation();
          }}
        />
      )}
      {!props.selecting && imageID ? (
        <Link
          to={NavUtils.makeImagesSimilarityUrl(String(imageID))}
          className="btn btn-secondary minimal"
          title={findSimilarLabel}
          aria-label={findSimilarLabel}
          style={{
            position: "absolute",
            right: "0.35rem",
            bottom: "0.35rem",
            zIndex: 2,
          }}
          onClick={(event) => event.stopPropagation()}
        >
          <Icon icon={faFingerprint} />
        </Link>
      ) : null}
      <ImagePreview
        loop={video}
        muted={video}
        playsInline={video}
        autoPlay={video}
        key={props.photo.key}
        src={source}
        width={width}
        height={height}
        alt={props.photo.alt}
        onClick={handleClick}
      />
    </div>
  );
};
