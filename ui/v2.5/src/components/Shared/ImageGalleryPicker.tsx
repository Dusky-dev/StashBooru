import React, { useMemo, useState } from "react";
import { Button, Form, Spinner } from "react-bootstrap";
import { useIntl } from "react-intl";
import * as GQL from "src/core/generated-graphql";
import { imageTitle } from "src/core/files";
import { ListFilterModel } from "src/models/list-filter/filter";
import ImageUtils from "src/utils/image";
import { useToast } from "src/hooks/Toast";
import { ModalComponent } from "./Modal";

interface IImageGalleryPickerProps {
  show: boolean;
  onHide: () => void;
  onSelect: (imageData: string) => void;
}

const PAGE_SIZE = 30;

export const ImageGalleryPicker: React.FC<IImageGalleryPickerProps> = ({
  show,
  onHide,
  onSelect,
}) => {
  const intl = useIntl();
  const Toast = useToast();
  const [searchInput, setSearchInput] = useState("");
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(1);
  const [loadingImageID, setLoadingImageID] = useState<string>();

  const filter = useMemo(() => {
    const ret = new ListFilterModel(GQL.FilterMode.Images);
    ret.itemsPerPage = PAGE_SIZE;
    ret.currentPage = page;
    ret.searchTerm = search;
    return ret;
  }, [page, search]);

  const { data, loading, error } = GQL.useFindImagesQuery({
    skip: !show,
    variables: {
      filter: filter.makeFindFilter(),
      image_filter: filter.makeFilter(),
    },
  });

  const images = data?.findImages.images ?? [];
  const count = data?.findImages.count ?? 0;
  const pageCount = Math.max(1, Math.ceil(count / PAGE_SIZE));

  function applySearch(event: React.FormEvent) {
    event.preventDefault();
    setPage(1);
    setSearch(searchInput.trim());
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
      onSelect(imageData);
      onHide();
    } catch (e) {
      Toast.error(e);
    } finally {
      setLoadingImageID(undefined);
    }
  }

  return (
    <ModalComponent
      show={show}
      onHide={onHide}
      header={intl.formatMessage({
        id: "dialogs.select_image_from_gallery",
        defaultMessage: "Select image from gallery",
      })}
      accept={{
        onClick: onHide,
        text: intl.formatMessage({ id: "actions.close" }),
      }}
      modalProps={{ size: "xl" }}
      isRunning={loadingImageID !== undefined}
    >
      <Form onSubmit={applySearch} className="mb-3">
        <div className="d-flex">
          <Form.Control
            value={searchInput}
            onChange={(event) => setSearchInput(event.currentTarget.value)}
            placeholder={intl.formatMessage({
              id: "actions.search",
              defaultMessage: "Search",
            })}
          />
          <Button type="submit" variant="secondary" className="ml-2">
            {intl.formatMessage({
              id: "actions.search",
              defaultMessage: "Search",
            })}
          </Button>
        </div>
      </Form>

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
            const thumbnail = image.paths.thumbnail ?? image.paths.image ?? "";
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
          disabled={page <= 1 || loading || loadingImageID !== undefined}
          onClick={() => setPage((value) => Math.max(1, value - 1))}
        >
          {intl.formatMessage({
            id: "actions.previous_action",
            defaultMessage: "Back",
          })}
        </Button>
        <span>
          {page} / {pageCount} ({count})
        </span>
        <Button
          type="button"
          variant="secondary"
          disabled={
            page >= pageCount || loading || loadingImageID !== undefined
          }
          onClick={() => setPage((value) => Math.min(pageCount, value + 1))}
        >
          {intl.formatMessage({
            id: "actions.next_action",
            defaultMessage: "Next",
          })}
        </Button>
      </div>
    </ModalComponent>
  );
};
