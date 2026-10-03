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
