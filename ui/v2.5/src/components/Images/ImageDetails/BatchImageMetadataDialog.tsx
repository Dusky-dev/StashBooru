import React from "react";
import { MediaKind } from "src/core/generated-graphql";
import { MetadataReviewDialog } from "src/components/Tagging/MetadataReviewDialog";

// Keep the Generate shortcut on the same reviewed selection workflow.
export const BatchImageMetadataDialog: React.FC<{
  imageIds: string[];
  onHide: () => void;
  onApplied?: () => Promise<unknown>;
}> = ({ imageIds, onHide, onApplied }) => (
  <MetadataReviewDialog
    items={imageIds.map((id) => ({ kind: MediaKind.Image, id }))}
    onHide={onHide}
    onApplied={onApplied}
  />
);
