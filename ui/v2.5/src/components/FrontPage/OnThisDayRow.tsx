import React, { useEffect, useState } from "react";
import Slider from "@ant-design/react-slick";
import { useIntl } from "react-intl";
import * as GQL from "src/core/generated-graphql";
import { getSlickSliderSettings } from "src/core/recommendations";
import { identityRequest } from "src/components/Identities/identityApi";
import { ImageCard } from "src/components/Images/ImageCard";
import { SceneCard } from "src/components/Scenes/SceneCard";
import { RecommendationRow } from "./RecommendationRow";

interface Item {
  id: string;
  date: string;
}

/** Today's month and day in the viewer's own time zone. */
function today() {
  const now = new Date();
  return {
    year: now.getFullYear(),
    month: now.getMonth() + 1,
    day: now.getDate(),
  };
}

function useOnThisDay(kind: "image" | "scene") {
  const [items, setItems] = useState<Item[]>();
  useEffect(() => {
    const controller = new AbortController();
    const { year, month, day } = today();
    const query = new URLSearchParams({
      kind,
      year: String(year),
      month: String(month),
      day: String(day),
      limit: "25",
    });
    void identityRequest<{ items: Item[] }>(
      `on-this-day?${query}`,
      controller.signal
    )
      .then((value) => {
        if (!controller.signal.aborted) setItems(value.items);
      })
      .catch(() => {
        if (!controller.signal.aborted) setItems([]);
      });
    return () => controller.abort();
  }, [kind]);
  return items;
}

/** Keep the server's newest-first order. */
function ordered<T extends { id: string }>(ids: string[], rows: T[] = []) {
  const byID = new Map(rows.map((row) => [row.id, row]));
  return ids.flatMap((id) => byID.get(id) ?? []);
}

/**
 * Main page "On this day": Images or Videos from this calendar day in
 * earlier years, by their Date or else the date they were added. Hidden on
 * days with nothing.
 */
export const OnThisDayRow: React.FC<{ kind: "image" | "scene" }> = ({
  kind,
}) => {
  const intl = useIntl();
  const items = useOnThisDay(kind);
  const ids = (items ?? []).map((item) => item.id);
  const numeric = ids.map(Number);
  const images = GQL.useFindImagesQuery({
    variables: { image_ids: numeric, filter: { per_page: ids.length } },
    skip: kind !== "image" || !ids.length,
  });
  const scenes = GQL.useFindScenesQuery({
    variables: { scene_ids: numeric, filter: { per_page: ids.length } },
    skip: kind !== "scene" || !ids.length,
  });
  if (!ids.length) return null;
  const isTouch = "ontouchstart" in window || navigator.maxTouchPoints > 0;
  const cards =
    kind === "image"
      ? ordered(ids, images.data?.findImages.images).map((image) => (
          <ImageCard key={image.id} image={image} zoomIndex={1} />
        ))
      : ordered(ids, scenes.data?.findScenes.scenes).map((scene) => (
          <SceneCard key={scene.id} scene={scene} zoomIndex={1} />
        ));
  if (!cards.length) return null;
  return (
    <RecommendationRow
      className={
        kind === "image" ? "images-recommendations" : "scene-recommendations"
      }
      header={intl.formatMessage(
        { id: "front_page.on_this_day.title" },
        { kind }
      )}
      link={<span />}
    >
      <Slider {...getSlickSliderSettings(cards.length, isTouch)}>
        {cards}
      </Slider>
    </RecommendationRow>
  );
};
