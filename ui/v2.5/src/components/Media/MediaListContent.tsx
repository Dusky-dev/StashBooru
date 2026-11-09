import { mixedMediaLightboxImage, StackMember } from "../VisualStacks/media";
import { useCallback, useMemo } from "react";
import * as GQL from "src/core/generated-graphql";
import { useLightbox } from "src/hooks/Lightbox/hooks";
import { ListFilterModel } from "src/models/list-filter/filter";
import { DisplayMode } from "src/models/list-filter/types";
import { MediaGrid, MediaWall } from "./MediaCards";

interface Props {
  onStackMemberSelect?: (member: StackMember, selected: boolean) => void;
  items: GQL.MediaListItemFragment[];
  filter: ListFilterModel;
  selectedIds: Set<string>;
  onSelectChange: (id: string, selected: boolean, shiftKey: boolean) => void;
}

// As on Images, preview state belongs to the loaded list body. Loading/error
// renders must not feed fresh empty arrays back into the Lightbox provider.
export function MediaListContent({
  items,
  filter,
  selectedIds,
  onSelectChange,
  onStackMemberSelect,
}: Props) {
  // One ordered preview sequence for both Images and Videos. Building an
  // Image-only list here and a separate scene-only queue in cards caused the
  // next/previous controls to skip every item of the other kind.
  const previewItems = useMemo(
    () => items.map(mixedMediaLightboxImage),
    [items]
  );
  const showLightbox = useLightbox({
    images: previewItems,
    totalCount: previewItems.length,
    page: 1,
    pages: 1,
    pageSize: previewItems.length,
    showNavigation: false,
    slideshowEnabled: false,
  });
  const onPreviewMedia = useCallback(
    (item: GQL.MediaListItemFragment) => {
      // Typed result keys distinguish image:123 from scene:123.
      const initialIndex = items.findIndex((value) => value.id === item.id);
      if (initialIndex >= 0)
        showLightbox({
          initialIndex,
          images: previewItems,
          onClose: undefined,
          slideshowEnabled: false,
        });
    },
    [items, previewItems, showLightbox]
  );
  const props = {
    items,
    selectedIds,
    zoomIndex: filter.zoomIndex,
    onSelectChange,
    onPreviewMedia,
    onStackMemberSelect,
  };
  return filter.displayMode === DisplayMode.Wall ? (
    <MediaWall {...props} />
  ) : (
    <MediaGrid {...props} />
  );
}
