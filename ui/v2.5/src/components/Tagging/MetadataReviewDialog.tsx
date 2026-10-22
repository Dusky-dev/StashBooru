import { useEffect, useRef, useState } from "react";
import { MediaKind } from "src/core/generated-graphql";
import { ImageKnowledgeTagDialog } from "../Images/ImageDetails/ImageKnowledgeTagDialog";
import { VideoTaggingDialog } from "../Scenes/SceneDetails/VideoTaggingDialog";

export interface MetadataReviewItem {
  kind: MediaKind;
  id: string;
}

// Snapshot typed identities: a refetch/reorder must not redirect a review or write.
export function MetadataReviewDialog({
  items,
  onHide,
  onApplied,
}: {
  items: readonly MetadataReviewItem[];
  onHide: () => void;
  onApplied?: () => Promise<unknown>;
}) {
  const [selection] = useState(() => [
    ...new Map(
      items.map((item) => [`${item.kind}:${item.id}`, { ...item }])
    ).values(),
  ]);
  const [index, setIndex] = useState(0);
  const applied = useRef(false);
  const item = selection[index];
  useEffect(() => {
    if (!selection.length) onHide();
  }, [selection.length, onHide]);
  function advance() {
    applied.current = false;
    if (index + 1 < selection.length) setIndex((value) => value + 1);
    else onHide();
  }
  async function didApply() {
    await onApplied?.();
    applied.current = true;
  }
  function hide() {
    if (applied.current) advance();
    else onHide();
  }
  if (!item) return null;
  const props = {
    onHide: hide,
    onApplied: didApply,
    reviewContext: { index, total: selection.length, onSkip: advance },
  };
  return item.kind === MediaKind.Image ? (
    <ImageKnowledgeTagDialog
      key={`image:${item.id}`}
      imageId={item.id}
      {...props}
    />
  ) : (
    <VideoTaggingDialog key={`video:${item.id}`} sceneId={item.id} {...props} />
  );
}

export function metadataReviewOperation(
  items: readonly MetadataReviewItem[],
  showModal: (dialog: JSX.Element) => void,
  onHide: () => void
) {
  return {
    text: "Review metadata…",
    isDisplayed: () => items.length > 0,
    onClick: () =>
      showModal(<MetadataReviewDialog items={items} onHide={onHide} />),
  };
}
