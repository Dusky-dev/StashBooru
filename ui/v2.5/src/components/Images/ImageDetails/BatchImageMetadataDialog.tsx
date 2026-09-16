import React, { useCallback, useRef, useState } from "react";

import { ImageKnowledgeTagDialog } from "./ImageKnowledgeTagDialog";

interface IProps {
  imageIds: string[];
  onHide: () => void;
  onApplied?: () => Promise<unknown>;
}

export const BatchImageMetadataDialog: React.FC<IProps> = ({
  imageIds,
  onHide,
  onApplied,
}) => {
  const [index, setIndex] = useState(0);
  const advanceAfterClose = useRef(false);

  const handleApplied = useCallback(async () => {
    if (onApplied) {
      await onApplied();
    }
    advanceAfterClose.current = true;
  }, [onApplied]);

  const handleHide = useCallback(() => {
    if (advanceAfterClose.current && index + 1 < imageIds.length) {
      advanceAfterClose.current = false;
      setIndex((current) => current + 1);
      return;
    }
    onHide();
  }, [imageIds.length, index, onHide]);

  const imageId = imageIds[index];
  if (!imageId) {
    onHide();
    return null;
  }

  return (
    <ImageKnowledgeTagDialog
      key={imageId}
      imageId={imageId}
      onApplied={handleApplied}
      onHide={handleHide}
    />
  );
};
