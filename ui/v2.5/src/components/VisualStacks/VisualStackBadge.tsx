import { useMemo, useRef, useState } from "react";
import { Alert, Button, Form, Overlay, Popover } from "react-bootstrap";
import * as GQL from "src/core/generated-graphql";
import { useLightbox } from "src/hooks/Lightbox/hooks";
import { VisualStackDialog } from "./VisualStackDialog";
import {
  StackMember,
  memberImage,
  memberThumbnail,
  memberTitle,
} from "./media";
import { stackPath } from "./identity";

interface Props {
  stack?: GQL.VisualStackSummaryFragment | null;
  selectedIds?: Set<string>;
  onSelectMember?: (member: StackMember, selected: boolean) => void;
}
function ExpandedMembers({
  stack,
  selectedIds,
  onSelectMember,
}: Props & { stack: GQL.VisualStackDataFragment }) {
  const images = useMemo(() => stack.members.map(memberImage), [stack.members]);
  const preview = useLightbox({
    images,
    showNavigation: false,
    slideshowEnabled: false,
    page: 1,
    pages: 1,
    pageSize: images.length,
    totalCount: images.length,
  });
  return (
    <>
      {stack.members.map((m, i) => (
        <div className="visual-stack-expanded-member" key={m.id}>
          {onSelectMember && (
            <Form.Check
              type="checkbox"
              aria-label={`Select ${m.id}`}
              checked={selectedIds?.has(m.id) ?? false}
              onChange={(e) => onSelectMember(m, e.target.checked)}
            />
          )}
          <img src={memberThumbnail(m)} alt="" />
          <div>
            <a href={stackPath(m.media)}>{memberTitle(m)}</a>
            <small>
              {m.label ||
                (m.media.kind === GQL.MediaKind.Image ? "Image" : "Video")}
              {m.representative ? " · Representative" : ""}
            </small>
            <Button
              size="sm"
              variant="link"
              aria-label={`Preview ${m.id}`}
              onClick={() => preview({ images, initialIndex: i })}
            >
              Preview
            </Button>
          </div>
        </div>
      ))}
    </>
  );
}
export function VisualStackBadge({
  stack,
  selectedIds,
  onSelectMember,
}: Props) {
  const [expanded, setExpanded] = useState(false);
  const [manage, setManage] = useState(false);
  const button = useRef<HTMLButtonElement>(null);
  const query = GQL.useFindVisualStackQuery({
    variables: { id: stack?.id ?? "" },
    skip: !expanded || !stack,
  });
  if (!stack) return null;
  return (
    <div className="visual-stack-card" onClick={(e) => e.stopPropagation()}>
      <Button
        ref={button}
        size="sm"
        variant="secondary"
        aria-expanded={expanded}
        onClick={(e) => {
          e.preventDefault();
          setExpanded(!expanded);
        }}
      >
        Stack · {stack.member_count}
      </Button>
      <Overlay
        show={expanded}
        target={button}
        placement="bottom-start"
        container={document.body}
        popperConfig={{
          modifiers: [
            {
              name: "preventOverflow",
              options: { boundary: "viewport", padding: 16 },
            },
          ],
        }}
      >
        <Popover
          id={`visual-stack-expanded-${stack.id}`}
          className="visual-stack-expanded"
          onClick={(e) => e.stopPropagation()}
        >
          <small>
            All members are shown here, including members outside the current
            filters. File actions target selected members only.
          </small>
          {query.error && <Alert variant="danger">{query.error.message}</Alert>}
          {query.data?.findVisualStack && (
            <ExpandedMembers
              stack={query.data.findVisualStack}
              selectedIds={selectedIds}
              onSelectMember={onSelectMember}
            />
          )}
          <Button size="sm" onClick={() => setManage(true)}>
            Manage stack
          </Button>
        </Popover>
      </Overlay>
      {manage && (
        <VisualStackDialog
          stackID={stack.id}
          onClose={() => setManage(false)}
        />
      )}
    </div>
  );
}
