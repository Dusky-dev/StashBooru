import { useMemo, useState } from "react";
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
import { getFirstValidPreviewSource } from "src/utils/wallPreview";

interface Props {
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
}: Props) {
  const [ref, { width }] = useContainerDimensions();
  const cardWidth = useCardWidth(width, zoomIndex, [280, 340, 480, 640]);
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
            selecting={selectedIds.size > 0}
            selected={selectedIds.has(item.id)}
            onSelectedChanged={(selected, shiftKey) =>
              onSelectChange(item.id, selected, shiftKey)
            }
            onPreview={
              selectedIds.size === 0
                ? () => onPreviewImage(item.image!)
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
  const [invalid, setInvalid] = useState<Record<string, string[]>>({});
  const playback = configuration.interface.wallPlayback;
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
        : [
            {
              src:
                playback === "video"
                  ? scene?.paths.preview
                  : playback === "animation"
                    ? scene?.paths.webp
                    : scene?.paths.screenshot,
              mediaType: playback === "video" ? "video" : "image",
            },
            { src: scene?.paths.screenshot, mediaType: "image" },
          ],
      invalid[item.id] ?? []
    );
    return {
      ...source,
      item,
      key: item.id,
      width: Math.max(1, file?.width ?? 1),
      height: Math.max(1, file?.height ?? 1),
      alt: objectTitle(image ?? scene ?? {}),
    };
  });
  function renderPhoto(props: RenderImageProps<MediaPhoto>) {
    const item = props.photo.item;
    const extra = {
      maxHeight: props.photo.height,
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
          link: `/scenes/${item.scene.id}`,
          onError: () =>
            setInvalid((old) => ({
              ...old,
              [item.id]: [...(old[item.id] ?? []), props.photo.src],
            })),
        }}
      />
    );
  }
  return (
    <div className="wall">
      {photos.length > 0 && (
        <MixedGallery
          photos={photos}
          renderImage={renderPhoto}
          targetRowHeight={[180, 240, 320, 400][zoomIndex] ?? 240}
          margin={configuration.ui.imageWallOptions?.margin}
          onClick={(e, { index }) => {
            const item = items[index];
            if (item.image && selectedIds.size === 0) {
              e.preventDefault();
              onPreviewImage(item.image);
            }
          }}
        />
      )}
    </div>
  );
}
