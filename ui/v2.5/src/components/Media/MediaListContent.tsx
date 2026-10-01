import { useCallback, useMemo } from "react";
import * as GQL from "src/core/generated-graphql";
import { useLightbox } from "src/hooks/Lightbox/hooks";
import { ListFilterModel } from "src/models/list-filter/filter";
import { DisplayMode } from "src/models/list-filter/types";
import { MediaGrid, MediaWall } from "./MediaCards";

interface Props {
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
}: Props) {
  const images = useMemo(
    () => items.flatMap((item) => (item.image ? [item.image] : [])),
    [items]
  );
  const showLightbox = useLightbox({
    images,
    totalCount: images.length,
    page: 1,
    pages: 1,
    pageSize: images.length,
    showNavigation: false,
    slideshowEnabled: false,
  });
  const onPreviewImage = useCallback(
    (image: GQL.SlimImageDataFragment) => {
      const initialIndex = images.findIndex((item) => item.id === image.id);
      if (initialIndex >= 0) showLightbox({ initialIndex });
    },
    [images, showLightbox]
  );
  const props = {
    items,
    selectedIds,
    zoomIndex: filter.zoomIndex,
    onSelectChange,
    onPreviewImage,
  };
  return filter.displayMode === DisplayMode.Wall ? (
    <MediaWall {...props} />
  ) : (
    <MediaGrid {...props} />
  );
}
