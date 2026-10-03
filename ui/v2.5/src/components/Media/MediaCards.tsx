import { StackMember } from "../VisualStacks/media";
import { useCallback, useMemo, useRef, useState } from "react";
import { useHistory } from "react-router-dom";
import Gallery, { GalleryI, RenderImageProps } from "react-photo-gallery";
import * as GQL from "src/core/generated-graphql";
import { animationBadge, objectTitle } from "src/core/files";
import { useConfigurationContext } from "src/hooks/Config";
import { SceneQueue } from "src/models/sceneQueue";
import {
  useCardWidth,
  useContainerDimensions,
} from "../Shared/GridCard/GridCard";
import { ImageCard } from "../Images/ImageCard";
import { SceneCard } from "../Scenes/SceneCard";
import { ImageWallItem } from "../Images/ImageWallItem";
import { SceneWallItem } from "../Scenes/SceneWallPanel";
import {
  getFirstValidPreviewSource,
  getScenePreviewSources,
  getWallDimensions,
} from "src/utils/wallPreview";

const zoomWidths = [280, 340, 480, 640];
const zoomHeights = [180, 240, 320, 400];

interface Props {
  onStackMemberSelect?: (member: StackMember, selected: boolean) => void;
  items: GQL.MediaListItemFragment[];
  selectedIds: Set<string>;
  zoomIndex: number;
  onSelectChange: (id: string, selected: boolean, shiftKey: boolean) => void;
  onPreviewImage: (image: GQL.SlimImageDataFragment) => void;
}

export function MediaGrid({
  items,
  selectedIds,
  zoomIndex,
  onSelectChange,
  onPreviewImage,
  onStackMemberSelect,
}: Props) {
  const [ref, { width }] = useContainerDimensions();
  const cardWidth = useCardWidth(width, zoomIndex, zoomWidths);
  const queue = useMemo(
    () =>
      SceneQueue.fromSceneIDList(
        items.flatMap((i) => (i.scene ? [i.scene.id] : []))
      ),
    [items]
  );
  return (
    <div className="row justify-content-center" ref={ref}>
      {items.map((item) =>
        item.image ? (
          <ImageCard
            key={item.id}
            image={item.image}
            cardWidth={cardWidth}
            zoomIndex={zoomIndex}
            stackSelectedIds={selectedIds}
            onStackMemberSelect={onStackMemberSelect}
            selecting={selectedIds.size > 0}
            selected={selectedIds.has(item.id)}
            onSelectedChanged={(selected, shiftKey) =>
              onSelectChange(item.id, selected, shiftKey)
            }
            onPreview={
              selectedIds.size === 0
                ? (event) => {
                    event.preventDefault();
                    onPreviewImage(item.image!);
                  }
                : undefined
            }
          />
        ) : item.scene ? (
          <SceneCard
            key={item.id}
            scene={item.scene}
            width={cardWidth}
            zoomIndex={zoomIndex}
            queue={queue}
            stackSelectedIds={selectedIds}
            onStackMemberSelect={onStackMemberSelect}
            selecting={selectedIds.size > 0}
            selected={selectedIds.has(item.id)}
            onSelectedChanged={(selected, shiftKey) =>
              onSelectChange(item.id, selected, shiftKey)
            }
          />
        ) : null
      )}
    </div>
  );
}

interface MediaPhoto {
  item: GQL.MediaListItemFragment;
  mediaType: "image" | "video";
}

const MixedGallery = Gallery as unknown as GalleryI<MediaPhoto>;

export function MediaWall({
  items,
  selectedIds,
  zoomIndex,
  onSelectChange,
  onPreviewImage,
}: Props) {
  const { configuration } = useConfigurationContext();
  const history = useHistory();
  const containerRef = useRef<HTMLDivElement>(null);
  const [invalid, setInvalid] = useState<Record<string, string[]>>({});
  const playback = configuration.interface.wallPlayback;
  const queue = useMemo(
    () =>
      SceneQueue.fromSceneIDList(
        items.flatMap((item) => (item.scene ? [item.scene.id] : []))
      ),
    [items]
  );
  const rowHeight = zoomHeights[zoomIndex] ?? zoomHeights[1];
  const targetRowHeight = useCallback(() => rowHeight, [rowHeight]);
  const columns = useCallback(
    (width: number) =>
      Math.max(1, Math.round(width / (zoomWidths[zoomIndex] ?? zoomWidths[1]))),
    [zoomIndex]
  );
  const onPreviewError = useCallback((id: string, src: string) => {
    setInvalid((old) =>
      old[id]?.includes(src) ? old : { ...old, [id]: [...(old[id] ?? []), src] }
    );
  }, []);
  const photos = items.map((item) => {
    const image = item.image;
    const scene = item.scene;
    const file = image?.visual_files[0] ?? scene?.files[0];
    const source = getFirstValidPreviewSource(
      image
        ? [
            {
              src: image.paths.preview,
              mediaType: image.paths.preview?.includes("preview")
                ? "video"
                : "image",
            },
            { src: image.paths.thumbnail, mediaType: "image" },
          ]
        : getScenePreviewSources(scene?.paths ?? {}, playback),
      invalid[item.id] ?? []
    );
    return {
      ...source,
      item,
      key: item.id,
      ...getWallDimensions(
        file,
        image ? { width: 1, height: 1 } : { width: 1280, height: 720 }
      ),
      alt: objectTitle(image ?? scene ?? {}),
    };
  });
  function renderPhoto(props: RenderImageProps<MediaPhoto>) {
    const item = props.photo.item;
    const extra = {
      maxHeight:
        props.direction === "column" ? props.photo.height : rowHeight * 1.3,
      selecting: selectedIds.size > 0,
      selected: selectedIds.has(item.id),
      onSelectedChanged: (selected: boolean, shiftKey: boolean) =>
        onSelectChange(item.id, selected, shiftKey),
    };
    if (item.image)
      return (
        <ImageWallItem
          {...props}
          {...extra}
          photo={{ ...props.photo, key: item.image.id }}
          animationLabel={animationBadge(item.image)}
        />
      );
    if (!item.scene) return null;
    return (
      <SceneWallItem
        {...props}
        {...extra}
        photo={{
          ...props.photo,
          scene: item.scene,
          link: queue.makeLink(item.scene.id, {}),
          onError: () => onPreviewError(item.id, props.photo.src),
        }}
      />
    );
  }
  return (
    <div
      className="gallery scene-wall media-wall"
      ref={containerRef}
      onError={(event) => {
        const target = event.target;
        if (
          !(
            target instanceof HTMLImageElement ||
            target instanceof HTMLVideoElement
          )
        )
          return;
        const index = photos.findIndex(
          (photo) =>
            photo.item.image && photo.src === target.getAttribute("src")
        );
        if (index >= 0)
          onPreviewError(photos[index].item.id, photos[index].src);
      }}
    >
      {photos.length > 0 && (
        <MixedGallery
          photos={photos}
          renderImage={renderPhoto}
          targetRowHeight={targetRowHeight}
          columns={columns}
          direction={configuration.ui.imageWallOptions?.direction}
          margin={configuration.ui.imageWallOptions?.margin}
          onClick={(e, { index }) => {
            const item = items[index];
            if (selectedIds.size > 0) return;
            if (item.image) {
              e.preventDefault();
              onPreviewImage(item.image);
            } else if (item.scene) {
              history.push(queue.makeLink(item.scene.id, {}));
            }
          }}
        />
      )}
    </div>
  );
}
