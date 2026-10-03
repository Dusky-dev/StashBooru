import { useCallback, useRef, useState } from "react";
import { Alert, Button } from "react-bootstrap";
import { useHistory } from "react-router-dom";
import * as GQL from "src/core/generated-graphql";
import { ILightboxImage } from "src/hooks/Lightbox/types";
import { nextStackMember, stackKey, stackPath } from "./identity";
import {
  memberImage,
  memberThumbnail,
  memberTitle,
  viewerReference,
} from "./media";
import { VisualStackDialog } from "./VisualStackDialog";

export function useViewerStack(
  image: ILightboxImage | undefined,
  enabled: boolean
) {
  const media = viewerReference(image);
  const owner = media ? stackKey(media) : "";
  const query = GQL.useFindVisualStackForMediaQuery({
    variables: {
      media: {
        kind:
          media?.kind === "VIDEO" ? GQL.MediaKind.Video : GQL.MediaKind.Image,
        id: media?.id ?? "",
      },
    },
    skip: !enabled || !media,
    fetchPolicy: "cache-and-network",
  });
  const candidate = query.data?.findVisualStackForMedia;
  const stack =
    enabled && candidate?.members.some((m) => m.id === owner)
      ? candidate
      : undefined;
  const [selection, setSelection] = useState({ owner: "", key: "" });
  const selectedKey =
    stack &&
    selection.owner === owner &&
    stack.members.some((m) => m.id === selection.key)
      ? selection.key
      : owner;
  const cursor = useRef({ owner, key: selectedKey });
  cursor.current = { owner, key: selectedKey };
  const select = useCallback(
    (key: string) => {
      cursor.current = { owner, key };
      setSelection({ owner, key });
    },
    [owner]
  );
  const step = useCallback(
    (direction: -1 | 1) => {
      const key = nextStackMember(
        stack?.members.map((m) => m.id) ?? [],
        cursor.current.key,
        direction
      );
      if (key) select(key);
    },
    [stack, select]
  );
  const member = stack?.members.find((m) => m.id === selectedKey);
  return {
    stack,
    selectedKey,
    select,
    step,
    displayImage: member ? memberImage(member) : image,
  };
}

interface Props {
  stack: GQL.VisualStackDataFragment;
  selectedKey: string;
  onSelect: (key: string) => void;
  onPreviousResult?: () => void;
  onNextResult?: () => void;
}
export function StackFilmstrip({
  stack,
  selectedKey,
  onSelect,
  onPreviousResult,
  onNextResult,
}: Props) {
  const [manage, setManage] = useState(false);
  const index = stack.members.findIndex((m) => m.id === selectedKey);
  const selected = stack.members[index];
  function step(direction: -1 | 1) {
    const next = nextStackMember(
      stack.members.map((m) => m.id),
      selectedKey,
      direction
    );
    if (next) onSelect(next);
  }
  return (
    <div
      className="visual-stack-filmstrip"
      role="group"
      aria-label="Stack variants"
      onKeyDown={(e) => {
        if (
          e.defaultPrevented ||
          e.repeat ||
          e.altKey ||
          e.ctrlKey ||
          e.metaKey ||
          e.shiftKey ||
          (e.target instanceof Element &&
            e.target.closest("input,select,textarea,[contenteditable='true']"))
        )
          return;
        if (e.key === "ArrowLeft" || e.key === "ArrowRight") {
          e.preventDefault();
          e.stopPropagation();
          step(e.key === "ArrowLeft" ? -1 : 1);
        }
      }}
    >
      <div className="visual-stack-filmstrip-heading">
        <span>
          {stack.title || "Stack"} · {index + 1}/{stack.member_count}
          {selected?.label ? ` · ${selected.label}` : ""}
        </span>
        <div className="visual-stack-filmstrip-controls">
          <Button
            size="sm"
            variant="secondary"
            disabled={stack.members.length < 2}
            onClick={() => step(-1)}
          >
            Previous variant
          </Button>
          <Button
            size="sm"
            variant="secondary"
            disabled={stack.members.length < 2}
            onClick={() => step(1)}
          >
            Next variant
          </Button>
          {selected && (
            <a
              className="btn btn-sm btn-secondary"
              href={stackPath(selected.media)}
            >
              Open this member
            </a>
          )}
          <Button size="sm" variant="secondary" onClick={() => setManage(true)}>
            Manage stack
          </Button>
          {onPreviousResult && (
            <Button size="sm" variant="link" onClick={onPreviousResult}>
              Previous result
            </Button>
          )}
          {onNextResult && (
            <Button size="sm" variant="link" onClick={onNextResult}>
              Next result
            </Button>
          )}
        </div>
      </div>
      <div className="visual-stack-thumbnails">
        {stack.members.map((m) => (
          <Button
            variant="link"
            className="visual-stack-thumbnail"
            key={m.id}
            data-member-id={m.id}
            aria-pressed={selectedKey === m.id}
            title={memberTitle(m)}
            onClick={() => onSelect(m.id)}
          >
            <img src={memberThumbnail(m)} alt={memberTitle(m)} />
            <small>
              {m.media.kind === GQL.MediaKind.Image ? "Image" : "Video"}
              {m.representative ? " ★" : ""}
            </small>
          </Button>
        ))}
      </div>
      {manage && (
        <VisualStackDialog
          stackID={stack.id}
          onClose={() => setManage(false)}
        />
      )}
    </div>
  );
}

export function DetailStackFilmstrip({
  kind,
  id,
}: {
  kind: GQL.MediaKind;
  id: string;
}) {
  const history = useHistory();
  const query = GQL.useFindVisualStackForMediaQuery({
    variables: { media: { kind, id } },
    fetchPolicy: "cache-and-network",
  });
  const key = stackKey({ kind, id });
  const stack = query.data?.findVisualStackForMedia;
  if (query.error)
    return <Alert variant="warning">{query.error.message}</Alert>;
  if (!stack?.members.some((m) => m.id === key)) return null;
  return (
    <StackFilmstrip
      stack={stack}
      selectedKey={key}
      onSelect={(next) => {
        const m = stack.members.find((m) => m.id === next);
        if (m) history.push(stackPath(m.media));
      }}
    />
  );
}
