import { useCallback, useEffect, useRef, useState } from "react";
import { useApolloClient } from "@apollo/client";
import { Alert, Button } from "react-bootstrap";
import { useHistory } from "react-router-dom";
import * as GQL from "src/core/generated-graphql";
import { ILightboxImage } from "src/hooks/Lightbox/types";
import { useLightbox } from "src/hooks/Lightbox/hooks";
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
  const client = useApolloClient();
  const kind =
    media?.kind === "VIDEO" ? GQL.MediaKind.Video : GQL.MediaKind.Image;
  const id = media?.id;
  const [result, setResult] = useState<{
    owner: string;
    stack?: GQL.VisualStackDataFragment | null;
  }>();
  useEffect(() => {
    if (!enabled || !id) return;
    let disposed = false;
    // Own the cache observer for exactly this viewer lifetime. Apollo's React
    // useQuery defers unsubscribe; an immediate caller switch or late mutation
    // could otherwise notify the dismissed viewer during that delay.
    const subscription = client
      .watchQuery<GQL.FindVisualStackForMediaQuery>({
        query: GQL.FindVisualStackForMediaDocument,
        variables: { media: { kind, id } },
        fetchPolicy: "cache-and-network",
      })
      .subscribe({
        next: ({ data }) => {
          if (!disposed)
            setResult({ owner, stack: data?.findVisualStackForMedia });
        },
        error: () => {
          if (!disposed) setResult({ owner });
        },
      });
    return () => {
      disposed = true;
      subscription.unsubscribe();
    };
  }, [client, enabled, kind, id, owner]);
  const candidate = result?.owner === owner ? result.stack : undefined;
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
  onCompareRepresentative?: () => void;
  comparingRepresentative?: boolean;
}
export function StackFilmstrip({
  stack,
  selectedKey,
  onSelect,
  onPreviousResult,
  onNextResult,
  onCompareRepresentative,
  comparingRepresentative = false,
}: Props) {
  const client = useApolloClient();
  const [update] = GQL.useVisualStackUpdateMutation();
  const [manage, setManage] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const operation = useRef({ mounted: true, busy: false });
  useEffect(() => {
    const current = operation.current;
    return () => {
      current.mounted = false;
    };
  }, []);
  const index = stack.members.findIndex((m) => m.id === selectedKey);
  const selected = stack.members[index];
  const representative = stack.members.find((m) => m.representative);
  const canCompare = Boolean(
    selected?.image &&
      representative?.image &&
      selected.id !== representative.id
  );
  async function makeRepresentative() {
    const current = operation.current;
    if (!selected || selected.representative || current.busy) return;
    current.busy = true;
    setBusy(true);
    setError("");
    try {
      await update({
        variables: {
          input: {
            id: stack.id,
            version: stack.version,
            title: stack.title,
            members: stack.members.map((member) => ({
              media: { kind: member.media.kind, id: member.media.id },
              label: member.label,
            })),
            representative: {
              kind: selected.media.kind,
              id: selected.media.id,
            },
          },
        },
      });
      for (const member of stack.members) {
        const id = client.cache.identify({
          __typename: member.image ? "Image" : "Scene",
          id: member.media.id,
        });
        if (id) client.cache.evict({ id, fieldName: "visual_stack" });
      }
      await client.refetchQueries({
        include: [
          "FindMedia",
          "FindImages",
          "FindScenes",
          "FindImage",
          "FindScene",
          "FindVisualStack",
          "FindVisualStackForMedia",
        ],
      });
    } catch (e) {
      if (current.mounted) setError(String(e));
    } finally {
      current.busy = false;
      if (current.mounted) setBusy(false);
    }
  }
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
          <Button
            size="sm"
            variant="secondary"
            disabled={busy || !selected || selected.representative}
            onClick={() => void makeRepresentative()}
          >
            Make this representative
          </Button>
          <Button
            size="sm"
            variant="secondary"
            disabled={
              !comparingRepresentative &&
              (!canCompare || !onCompareRepresentative)
            }
            onClick={onCompareRepresentative}
            title={
              canCompare || comparingRepresentative
                ? undefined
                : "Select a different Image member to compare with the representative Image."
            }
          >
            {comparingRepresentative
              ? "Stop comparing"
              : "Compare with representative"}
          </Button>
          <Button
            size="sm"
            variant="secondary"
            disabled={busy}
            onClick={() => setManage(true)}
          >
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
      {error && (
        <Alert variant="danger" className="mt-2 mb-0">
          {error}{" "}
          <Button
            variant="link"
            size="sm"
            onClick={() =>
              void client
                .refetchQueries({ include: ["FindVisualStackForMedia"] })
                .catch((e) => {
                  if (operation.current.mounted) setError(String(e));
                })
            }
          >
            Reload stack
          </Button>
        </Alert>
      )}
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
  const showLightbox = useLightbox();
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
      key={stack.id}
      stack={stack}
      selectedKey={key}
      onSelect={(next) => {
        const m = stack.members.find((m) => m.id === next);
        if (m) history.push(stackPath(m.media));
      }}
      onCompareRepresentative={() => {
        const selected = stack.members.find((member) => member.id === key);
        const representative = stack.members.find(
          (member) => member.representative
        );
        if (
          !selected?.image ||
          !representative?.image ||
          selected.id === representative.id
        )
          return;
        showLightbox({
          images: [memberImage(selected)],
          referenceImage: memberImage(representative),
          showNavigation: false,
          slideshowEnabled: false,
        });
      }}
    />
  );
}
