import React, { useState } from "react";
import { Button, Spinner } from "react-bootstrap";
import { useIntl } from "react-intl";
import { useLocation } from "react-router-dom";
import * as GQL from "src/core/generated-graphql";
import { imageTitle } from "src/core/files";
import { usePerformerFilterHook } from "src/core/performers";
import { useStudioFilterHook } from "src/core/studios";
import { useTagFilterHook } from "src/core/tags";
import { ListFilterModel } from "src/models/list-filter/filter";
import ImageUtils from "src/utils/image";
import { useToast } from "src/hooks/Toast";
import { ClearableInput } from "./ClearableInput";
import { ModalComponent } from "./Modal";
import { SimpleImageEditorModal } from "./SimpleImageEditorModal";

interface IImageGalleryPickerProps {
  show: boolean;
  onHide: () => void;
  onSelect: (imageData: string) => void;
}

type GalleryEntityType = "performer" | "tag" | "studio" | "copyright";

interface IGalleryEntityContext {
  type: GalleryEntityType;
  id: string;
}

const PAGE_SIZE = 30;

function getGalleryEntityContext(
  pathname: string
): IGalleryEntityContext | undefined {
  const parts = pathname.split("/").filter(Boolean);
  if (parts.length < 2 || parts[1] === "new") return;

  const id = parts[1];
  switch (parts[0]) {
    case "performers":
    case "characters":
      return { type: "performer", id };
    case "tags":
      return { type: "tag", id };
    case "studios":
    case "artists":
      return { type: "studio", id };
    case "copyrights":
      return { type: "copyright", id };
    default:
      return;
  }
}

function createInitialFilter() {
  const ret = new ListFilterModel(GQL.FilterMode.Images);
  ret.itemsPerPage = PAGE_SIZE;
  return ret;
}

export const ImageGalleryPicker: React.FC<IImageGalleryPickerProps> = ({
  show,
  onHide,
  onSelect,
}) => {
  const intl = useIntl();
  const Toast = useToast();
  const location = useLocation();
  const entityContext = getGalleryEntityContext(location.pathname);
  const [filter, setFilter] = useState(createInitialFilter);
  const [searchInput, setSearchInput] = useState("");
  const [loadingImageID, setLoadingImageID] = useState<string>();
  const [editorSource, setEditorSource] = useState<string>();
  const [editorTitle, setEditorTitle] = useState<string>();

  // Reuse the same filters as the entity Images tabs. The placeholder objects
  // only need id/name because those are the only fields consumed by the hooks.
  const performerFilterHook = usePerformerFilterHook({
    id: entityContext?.id ?? "",
    name: entityContext?.id ?? "",
  } as GQL.PerformerDataFragment);
  const tagFilterHook = useTagFilterHook({
    id: entityContext?.id ?? "",
    name: entityContext?.id ?? "",
  } as GQL.TagDataFragment);
  const studioFilterHook = useStudioFilterHook({
    id: entityContext?.id ?? "",
    name: entityContext?.id ?? "",
  } as GQL.StudioDataFragment);

  let effectiveFilter = filter.clone();
  switch (entityContext?.type) {
    case "performer":
      effectiveFilter = performerFilterHook(effectiveFilter);
      break;
    case "tag":
      effectiveFilter = tagFilterHook(effectiveFilter);
      break;
    case "studio":
      effectiveFilter = studioFilterHook(effectiveFilter);
      break;
  }

  const copyrightID =
    entityContext?.type === "copyright" ? entityContext.id : "";
  const {
    data: copyrightData,
    loading: copyrightLoading,
    error: copyrightError,
  } = GQL.useFindCopyrightQuery({
    skip: !show || !!editorSource || !copyrightID,
    variables: { id: copyrightID },
  });
  const copyrightImageIDs =
    entityContext?.type === "copyright"
      ? (copyrightData?.findCopyright?.images ?? []).map((image) =>
          Number(image.id)
        )
      : undefined;

  const {
    data,
    loading: imagesLoading,
    error: imagesError,
  } = GQL.useFindImagesQuery({
    skip:
      !show ||
      !!editorSource ||
      copyrightLoading ||
      (copyrightImageIDs !== undefined && copyrightImageIDs.length === 0),
    variables: {
      filter: effectiveFilter.makeFindFilter(),
      image_filter: effectiveFilter.makeFilter(),
      image_ids: copyrightImageIDs,
    },
  });

  const images = data?.findImages.images ?? [];
  const count = data?.findImages.count ?? 0;
  const pageCount = Math.max(1, Math.ceil(count / PAGE_SIZE));
  const loading = copyrightLoading || imagesLoading;
  const error = copyrightError ?? imagesError;

  function applySearch(value = searchInput) {
    setFilter((current) => {
      const next = current.clone();
      next.searchTerm = value.trim();
      next.currentPage = 1;
      return next;
    });
  }

  function updateSearchInput(value: string) {
    setSearchInput(value);
    if (!value) applySearch("");
  }

  function changePage(page: number) {
    setFilter((current) => {
      const next = current.clone();
      next.currentPage = page;
      return next;
    });
  }

  function closePicker() {
    setEditorSource(undefined);
    setEditorTitle(undefined);
    onHide();
  }

  function backToGallery() {
    setEditorSource(undefined);
    setEditorTitle(undefined);
  }

  function applyEditedImage(imageData: string) {
    onSelect(imageData);
    closePicker();
  }

  async function selectImage(image: GQL.SlimImageDataFragment) {
    const source = image.paths.image ?? image.paths.thumbnail ?? "";
    if (!source) {
      Toast.error(
        intl.formatMessage({
          id: "toast.image_source_unavailable",
          defaultMessage: "The selected image is not available.",
        })
      );
      return;
    }

    setLoadingImageID(image.id);
    try {
      const imageData = await ImageUtils.imageToDataURL(source);
      setEditorTitle(imageTitle(image) || `#${image.id}`);
      setEditorSource(imageData);
    } catch (e) {
      Toast.error(e);
    } finally {
      setLoadingImageID(undefined);
    }
  }

  return (
    <>
      <ModalComponent
        show={show && !editorSource}
        onHide={closePicker}
        header={intl.formatMessage({
          id: "dialogs.select_image_from_gallery",
          defaultMessage: "Select image from gallery",
        })}
        accept={{
          onClick: closePicker,
          text: intl.formatMessage({ id: "actions.close" }),
        }}
        modalProps={{ size: "xl" }}
        isRunning={loadingImageID !== undefined}
      >
        <div className="d-flex mb-3">
          <ClearableInput
            className="search-term-input flex-grow-1"
            value={searchInput}
            setValue={updateSearchInput}
            onEnter={() => applySearch()}
            placeholder={`${intl.formatMessage({ id: "actions.search" })}…`}
          />
          <Button
            type="button"
            variant="secondary"
            className="ml-2"
            onClick={() => applySearch()}
          >
            {intl.formatMessage({
              id: "actions.search",
              defaultMessage: "Search",
            })}
          </Button>
        </div>

        {error ? (
          <div className="text-danger">{error.message}</div>
        ) : loading ? (
          <div className="text-center py-5">
            <Spinner animation="border" role="status" />
          </div>
        ) : images.length === 0 ? (
          <div className="text-muted text-center py-5">
            {intl.formatMessage({
              id: "no_images_found",
              defaultMessage: "No images found.",
            })}
          </div>
        ) : (
          <div
            style={{
              display: "grid",
              gridTemplateColumns: "repeat(auto-fill, minmax(130px, 1fr))",
              gap: "0.75rem",
              maxHeight: "65vh",
              overflowY: "auto",
            }}
          >
            {images.map((image) => {
              const thumbnail =
                image.paths.thumbnail ?? image.paths.image ?? "";
              const title = imageTitle(image) || `#${image.id}`;
              const selecting = loadingImageID === image.id;

              return (
                <Button
                  key={image.id}
                  type="button"
                  variant="secondary"
                  className="p-1 text-left"
                  disabled={loadingImageID !== undefined}
                  onClick={() => void selectImage(image)}
                  title={title}
                  style={{ minWidth: 0 }}
                >
                  <div
                    style={{
                      position: "relative",
                      width: "100%",
                      paddingTop: "100%",
                      overflow: "hidden",
                    }}
                  >
                    <img
                      src={thumbnail}
                      alt={title}
                      loading="lazy"
                      style={{
                        position: "absolute",
                        inset: 0,
                        width: "100%",
                        height: "100%",
                        objectFit: "cover",
                      }}
                    />
                    {selecting && (
                      <span
                        style={{
                          position: "absolute",
                          inset: 0,
                          display: "flex",
                          alignItems: "center",
                          justifyContent: "center",
                          background: "rgba(0, 0, 0, 0.55)",
                        }}
                      >
                        <Spinner animation="border" role="status" size="sm" />
                      </span>
                    )}
                  </div>
                  <div className="text-truncate px-1 pt-1">{title}</div>
                </Button>
              );
            })}
          </div>
        )}

        <div className="d-flex justify-content-between align-items-center mt-3">
          <Button
            type="button"
            variant="secondary"
            disabled={
              filter.currentPage <= 1 || loading || loadingImageID !== undefined
            }
            onClick={() => changePage(Math.max(1, filter.currentPage - 1))}
          >
            {intl.formatMessage({
              id: "actions.previous_action",
              defaultMessage: "Back",
            })}
          </Button>
          <span>
            {filter.currentPage} / {pageCount} ({count})
          </span>
          <Button
            type="button"
            variant="secondary"
            disabled={
              filter.currentPage >= pageCount ||
              loading ||
              loadingImageID !== undefined
            }
            onClick={() =>
              changePage(Math.min(pageCount, filter.currentPage + 1))
            }
          >
            {intl.formatMessage({
              id: "actions.next_action",
              defaultMessage: "Next",
            })}
          </Button>
        </div>
      </ModalComponent>

      {editorSource ? (
        <SimpleImageEditorModal
          show={show}
          source={editorSource}
          title={editorTitle}
          onBack={backToGallery}
          onApply={applyEditedImage}
        />
      ) : null}
    </>
  );
};
