import { lazy, Suspense } from "react";
import * as GQL from "src/core/generated-graphql";
import { useFindScene } from "src/core/StashService";
import { ILightboxImage } from "src/hooks/Lightbox/types";
import { stackPath, parseMemberURL, StackRef } from "./identity";

const ScenePlayer = lazy(() => import("../ScenePlayer/ScenePlayer"));
const noop = () => {};
export type StackMember = GQL.VisualStackDataFragment["members"][number];
export const memberThumbnail = (m: StackMember) =>
  m.image?.paths.thumbnail ?? m.scene?.paths.screenshot ?? "";
export const memberTitle = (m: StackMember) =>
  m.image?.title ||
  m.scene?.title ||
  `${m.media.kind === "IMAGE" ? "Image" : "Video"} ${m.media.id}`;

function StackVideo({
  id,
  onNext,
  onPrevious,
}: {
  id: string;
  onNext: () => void;
  onPrevious: () => void;
}) {
  const { data, error } = useFindScene(id);
  const scene = data?.findScene;
  if (error) return <p role="alert">{error.message}</p>;
  if (!scene || scene.id !== id) return null;
  return (
    <div className="unified-media-native-scene-player" data-scene-id={id}>
      <Suspense fallback={null}>
        <ScenePlayer
          scene={scene}
          playerId={`StashBooruStack-${id}`}
          hideScrubberOverride
          autoplay
          permitLoop={false}
          initialTimestamp={0}
          sendSetTimestamp={noop}
          onComplete={noop}
          onNext={onNext}
          onPrevious={onPrevious}
        />
      </Suspense>
    </div>
  );
}

// Preserve the native mixed findMedia order in All's shared Lightbox. Images
// retain their own native identity; Videos use the existing lazy native player
// but deliberately have no Image id (Image-only edit/delete controls stay off).
export function mixedMediaLightboxImage(
  item: GQL.MediaListItemFragment
): ILightboxImage {
  if (item.image) {
    return {
      ...item.image,
      mediaReference: { kind: "IMAGE", id: item.image.id },
      href: `/images/${item.image.id}`,
    };
  }
  if (!item.scene) {
    throw new Error(`Mixed media result ${item.id} has no Image or Video`);
  }
  const scene = item.scene;
  return {
    title: scene.title || `Video ${scene.id}`,
    paths: {
      image: scene.paths.screenshot,
      thumbnail: scene.paths.screenshot,
    },
    visual_files: scene.files,
    href: `/scenes/${scene.id}`,
    mediaReference: { kind: "VIDEO", id: scene.id },
    renderMedia: ({ onNext, onPrevious }) => (
      <StackVideo id={scene.id} onNext={onNext} onPrevious={onPrevious} />
    ),
  };
}

export function memberImage(member: StackMember): ILightboxImage {
  if (member.image)
    return {
      ...member.image,
      href: stackPath(member.media),
      mediaReference: member.media,
    };
  const scene = member.scene;
  return {
    title: memberTitle(member),
    paths: {
      image: scene?.paths.screenshot,
      thumbnail: scene?.paths.screenshot,
    },
    visual_files: scene?.files,
    href: stackPath(member.media),
    mediaReference: member.media,
    renderMedia: ({ onNext, onPrevious }) => (
      <StackVideo
        id={member.media.id}
        onNext={onNext}
        onPrevious={onPrevious}
      />
    ),
  };
}

export function viewerReference(image?: ILightboxImage): StackRef | undefined {
  if (!image) return undefined;
  if (image.mediaReference) return image.mediaReference;
  if (image.href) return parseMemberURL(image.href);
  if (image.id && /^[1-9]\d*$/.test(image.id))
    return { kind: "IMAGE", id: image.id };
  return undefined;
}
