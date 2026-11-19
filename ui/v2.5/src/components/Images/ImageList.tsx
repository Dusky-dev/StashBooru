import { metadataReviewOperation } from "../Tagging/MetadataReviewDialog";
import { OrganizedOnly } from "src/hooks/Config";
import { VisualStackDialog } from "../VisualStacks/VisualStackDialog";
import { StackProposalsDialog } from "../VisualStacks/StackProposalsDialog";
import React, {
  useCallback,
  useState,
  useMemo,
  MouseEvent,
  useEffect,
} from "react";
import { FormattedMessage, FormattedNumber, useIntl } from "react-intl";
import { useHistory } from "react-router-dom";
import Mousetrap from "mousetrap";
import * as GQL from "src/core/generated-graphql";
import { queryFindImages } from "src/core/StashService";
import { useFilteredItemList } from "../List/ItemList";
import { useListScrollRestoration } from "../List/useListScrollRestoration";
import { useToast } from "src/hooks/Toast";
import { useLightbox } from "src/hooks/Lightbox/hooks";
import { ListFilterModel } from "src/models/list-filter/filter";
import { DisplayMode } from "src/models/list-filter/types";
import { ImageWallItem } from "./ImageWallItem";
import { VisualStackBadge } from "../VisualStacks/VisualStackBadge";
import { EditImagesDialog } from "./EditImagesDialog";
import { DeleteImagesDialog } from "./DeleteImagesDialog";
import "flexbin/flexbin.css";
import Gallery, { RenderImageProps } from "react-photo-gallery";
import { ExportDialog } from "../Shared/ExportDialog";
import { objectTitle, animationBadge } from "src/core/files";
import { useConfigurationContext } from "src/hooks/Config";
import { ImageCardGrid } from "./ImageCardGrid";
import { View } from "../List/views";
import { useInterfaceLocalForage } from "src/hooks/LocalForage";
import {
  FilteredListToolbar,
  IItemListOperation,
} from "../List/FilteredListToolbar";
import { FileSize } from "../Shared/FileSize";
import { PatchComponent, PatchContainerComponent } from "src/patch";
import { GenerateDialog } from "../Dialogs/GenerateDialog";
import { MediaConversionDialog } from "../Shared/MediaConversionDialog";
import { MediaUpscalingDialog } from "../Shared/MediaUpscalingDialog";
import {
  Sidebar,
  SidebarPane,
  SidebarPaneContent,
  SidebarStateContext,
  useSidebarState,
} from "../Shared/Sidebar";
import { useCloseEditDelete, useFilterOperations } from "../List/util";
import {
  FilteredSidebarHeader,
  useFilteredSidebarKeybinds,
} from "../List/Filters/FilterSidebar";
import {
  IListFilterOperation,
  ListOperations,
} from "../List/ListOperationButtons";
import { FilterTags } from "../List/FilterTags";
import { Pagination, PaginationIndex } from "../List/Pagination";
import { LoadedContent } from "../List/PagedList";
import useFocus from "src/utils/focus";
import cx from "classnames";
import { SidebarStudiosFilter } from "../List/Filters/StudiosFilter";
import { SidebarPerformersFilter } from "../List/Filters/PerformersFilter";
import { SidebarTagsFilter } from "../List/Filters/TagsFilter";
import { SidebarRatingFilter } from "../List/Filters/RatingFilter";
import { SidebarBooleanFilter } from "../List/Filters/BooleanFilter";
import { Alert, Button } from "react-bootstrap";
import { OrganizedCriterionOption } from "src/models/list-filter/criteria/organized";
import { SidebarAgeFilter } from "../List/Filters/SidebarAgeFilter";
import { PerformerAgeCriterionOption } from "src/models/list-filter/images";
import { SidebarFolderFilter } from "../List/Filters/FolderFilter";
import { MEDIA_UPLOAD_IMPORTED_EVENT } from "../Shared/mediaUploadSettings";

interface IImageWallProps {
  images: GQL.SlimImageDataFragment[];
  onChangePage: (page: number) => void;
  currentPage: number;
  pageCount: number;
  handleImageOpen: (index: number) => void;
  zoomIndex: number;
  selectedIds?: Set<string>;
  onSelectChange?: (id: string, selected: boolean, shiftKey: boolean) => void;
  selecting?: boolean;
}

const zoomWidths = [280, 340, 480, 640];
const breakpointZoomHeights = [
  { minWidth: 576, heights: [100, 120, 240, 360] },
  { minWidth: 768, heights: [120, 160, 240, 480] },
  { minWidth: 1200, heights: [120, 160, 240, 300] },
  { minWidth: 1400, heights: [160, 240, 300, 480] },
];

const ImageWall: React.FC<IImageWallProps> = ({
  images,
  zoomIndex,
  handleImageOpen,
  selectedIds,
  onSelectChange,
  selecting,
}) => {
  const { configuration } = useConfigurationContext();
  const uiConfig = configuration?.ui;

  const containerRef = React.useRef<HTMLDivElement>(null);
  const safeZoomIndex = Math.max(
    0,
    Math.min(
      Number.isFinite(zoomIndex) ? Math.round(zoomIndex) : 1,
      zoomWidths.length - 1
    )
  );

  const photos: {
    src: string;
    srcSet?: string | string[] | undefined;
    sizes?: string | string[] | undefined;
    width: number;
    height: number;
    alt?: string | undefined;
    key?: string | undefined;
  }[] = [];

  images.forEach((image, index) => {
    const width = image.visual_files?.[0]?.width;
    const height = image.visual_files?.[0]?.height;
    const imageData = {
      src:
        image.paths.preview !== ""
          ? image.paths.preview!
          : image.paths.thumbnail!,
      width:
        typeof width === "number" && Number.isFinite(width) && width > 0
          ? width
          : 1,
      height:
        typeof height === "number" && Number.isFinite(height) && height > 0
          ? height
          : 1,
      tabIndex: index,
      key: image.id,
      loading: "lazy",
      className: "gallery-image",
      alt: objectTitle(image),
    };
    photos.push(imageData);
  });

  const showLightboxOnClick = useCallback(
    (_event: React.MouseEvent, { index }: { index: number }) => {
      handleImageOpen(index);
    },
    [handleImageOpen]
  );

  function columns(containerWidth: number) {
    if (!Number.isFinite(containerWidth) || containerWidth <= 0) return 1;
    const preferredSize = zoomWidths[safeZoomIndex] ?? zoomWidths[1];
    const columnCount = containerWidth / preferredSize;
    return Math.max(1, Math.round(columnCount));
  }

  const targetRowHeight = useCallback(
    (containerWidth: number) => {
      let zoomHeight = 280;
      breakpointZoomHeights.forEach((e) => {
        if (containerWidth >= e.minWidth) {
          const nextHeight = e.heights[safeZoomIndex];
          if (Number.isFinite(nextHeight) && nextHeight > 0) {
            zoomHeight = nextHeight;
          }
        }
      });
      return zoomHeight;
    },
    [safeZoomIndex]
  );

  const maxHeightFactor = 1.3;

  const renderImage = useCallback(
    (props: RenderImageProps) => {
      const maxHeight =
        props.direction === "column"
          ? props.photo.height
          : targetRowHeight(containerRef.current?.offsetWidth ?? 0) *
            maxHeightFactor;
      const imageId = props.photo.key;
      if (!imageId) {
        return null;
      }
      return (
        <ImageWallItem
          {...props}
          animationLabel={animationBadge(
            images.find((image) => image.id === imageId)
          )}
          stackBadge={
            <VisualStackBadge
              stack={images.find((image) => image.id === imageId)?.visual_stack}
            />
          }
          maxHeight={maxHeight}
          selected={selectedIds?.has(imageId)}
          onSelectedChanged={
            onSelectChange
              ? (selected, shiftKey) =>
                  onSelectChange(imageId, selected, shiftKey)
              : undefined
          }
          selecting={selecting}
        />
      );
    },
    [targetRowHeight, selectedIds, onSelectChange, selecting, images]
  );

  return (
    <div className="gallery" ref={containerRef}>
      {photos.length ? (
        <Gallery
          photos={photos}
          renderImage={renderImage}
          onClick={showLightboxOnClick}
          margin={uiConfig?.imageWallOptions?.margin}
          direction={uiConfig?.imageWallOptions?.direction}
          columns={columns}
          targetRowHeight={targetRowHeight}
        />
      ) : null}
    </div>
  );
};

interface IImageListImages {
  images: GQL.SlimImageDataFragment[];
  filter: ListFilterModel;
  selectedIds: Set<string>;
  onChangePage: (page: number) => void;
  pageCount: number;
  totalCount: number;
  onSelectChange: (id: string, selected: boolean, shiftKey: boolean) => void;
  slideshowRunning: boolean;
  setSlideshowRunning: (running: boolean) => void;
  chapters?: GQL.GalleryChapterDataFragment[];
}

const ImageList: React.FC<IImageListImages> = PatchComponent(
  "ImageList",
  ({
    images,
    filter,
    selectedIds,
    onChangePage,
    pageCount,
    totalCount,
    onSelectChange,
    slideshowRunning,
    setSlideshowRunning,
    chapters = [],
  }) => {
    const handleLightBoxPage = useCallback(
      (props: { direction?: number; page?: number }) => {
        const { direction, page: newPage } = props;

        if (direction !== undefined) {
          if (direction < 0) {
            if (filter.currentPage === 1) {
              onChangePage(pageCount);
            } else {
              onChangePage(filter.currentPage + direction);
            }
          } else if (direction > 0) {
            if (filter.currentPage === pageCount) {
              onChangePage(1);
            } else {
              onChangePage(filter.currentPage + direction);
            }
          }
        } else if (newPage !== undefined) {
          onChangePage(newPage);
        }
      },
      [onChangePage, filter.currentPage, pageCount]
    );

    const handleClose = useCallback(() => {
      setSlideshowRunning(false);
    }, [setSlideshowRunning]);

    const lightboxState = useMemo(() => {
      return {
        images,
        showNavigation: false,
        pageCallback: pageCount > 1 ? handleLightBoxPage : undefined,
        page: filter.currentPage,
        pages: pageCount,
        pageSize: filter.itemsPerPage,
        totalCount,
        slideshowEnabled: slideshowRunning,
        onClose: handleClose,
      };
    }, [
      images,
      pageCount,
      totalCount,
      filter.currentPage,
      filter.itemsPerPage,
      slideshowRunning,
      handleClose,
      handleLightBoxPage,
    ]);

    const showLightbox = useLightbox(
      lightboxState,
      filter.sortBy === "path" &&
        filter.sortDirection === GQL.SortDirectionEnum.Asc
        ? chapters
        : []
    );

    const handleImageOpen = useCallback(
      (index: number) => {
        setSlideshowRunning(true);
        showLightbox({ initialIndex: index, slideshowEnabled: true });
      },
      [showLightbox, setSlideshowRunning]
    );

    function onPreview(index: number, ev: MouseEvent) {
      handleImageOpen(index);
      ev.preventDefault();
    }

    if (filter.displayMode === DisplayMode.Grid) {
      return (
        <ImageCardGrid
          images={images}
          selectedIds={selectedIds}
          zoomIndex={filter.zoomIndex}
          onSelectChange={onSelectChange}
          onPreview={onPreview}
        />
      );
    }
    if (filter.displayMode === DisplayMode.Wall) {
      return (
        <ImageWall
          images={images}
          onChangePage={onChangePage}
          currentPage={filter.currentPage}
          pageCount={pageCount}
          handleImageOpen={handleImageOpen}
          zoomIndex={filter.zoomIndex}
          selectedIds={selectedIds}
          onSelectChange={onSelectChange}
          selecting={!!selectedIds && selectedIds.size > 0}
        />
      );
    }

    return null;
  }
);

function renderMetadataByline(
  metadataInfo: GQL.FindImagesMetadataQueryResult | undefined
) {
  const megapixels = metadataInfo?.data?.findImages?.megapixels;
  const size = metadataInfo?.data?.findImages?.filesize;

  if (metadataInfo?.loading) {
    return <span className="images-stats">&nbsp;(...)</span>;
  }

  if (!megapixels && !size) {
    return;
  }

  const separator = megapixels && size ? " - " : "";

  return (
    <span className="images-stats">
      &nbsp;(
      {megapixels ? (
        <span className="images-megapixels">
          <FormattedNumber value={megapixels} /> Megapixels
        </span>
      ) : undefined}
      {separator}
      {size ? (
        <span className="images-size">
          <FileSize size={size} />
        </span>
      ) : undefined}
      )
    </span>
  );
}

const ImageFilterSidebarSections = PatchContainerComponent(
  "FilteredImageList.SidebarSections"
);

const SidebarContent: React.FC<{
  filter: ListFilterModel;
  setFilter: (filter: ListFilterModel) => void;
  filterHook?: (filter: ListFilterModel) => ListFilterModel;
  view?: View;
  sidebarOpen: boolean;
  onClose?: () => void;
  showEditFilter: (editingCriterion?: string) => void;
  count?: number;
  focus?: ReturnType<typeof useFocus>;
}> = ({
  filter,
  setFilter,
  filterHook,
  view,
  showEditFilter,
  sidebarOpen,
  onClose,
  count,
  focus,
}) => {
  const showResultsId =
    count !== undefined ? "actions.show_count_results" : "actions.show_results";

  const hideStudios = view === View.StudioScenes;

  return (
    <>
      <FilteredSidebarHeader
        sidebarOpen={sidebarOpen}
        showEditFilter={showEditFilter}
        filter={filter}
        setFilter={setFilter}
        view={view}
        focus={focus}
      />

      <ImageFilterSidebarSections>
        {!hideStudios && (
          <SidebarStudiosFilter
            filter={filter}
            setFilter={setFilter}
            filterHook={filterHook}
          />
        )}
        <SidebarPerformersFilter
          filter={filter}
          setFilter={setFilter}
          filterHook={filterHook}
        />
        <SidebarTagsFilter
          filter={filter}
          setFilter={setFilter}
          filterHook={filterHook}
        />
        <SidebarRatingFilter filter={filter} setFilter={setFilter} />
        <SidebarFolderFilter
          text={<FormattedMessage id="folder" />}
          filter={filter}
          setFilter={setFilter}
          sectionID="folder"
        />
        <OrganizedOnly>
          <SidebarBooleanFilter
            title={<FormattedMessage id="organized" />}
            data-type={OrganizedCriterionOption.type}
            option={OrganizedCriterionOption}
            filter={filter}
            setFilter={setFilter}
            sectionID="organized"
          />
        </OrganizedOnly>
        <SidebarAgeFilter
          title={<FormattedMessage id="performer_age" />}
          option={PerformerAgeCriterionOption}
          filter={filter}
          setFilter={setFilter}
          sectionID="performer_age"
        />
      </ImageFilterSidebarSections>

      <div className="sidebar-footer">
        <Button className="sidebar-close-button" onClick={onClose}>
          <FormattedMessage id={showResultsId} values={{ count }} />
        </Button>
      </div>
    </>
  );
};

function useViewRandom(
  filter: ListFilterModel,
  count: number,
  collapseStacks: boolean
) {
  const history = useHistory();
  const toast = useToast();

  return useCallback(async () => {
    if (count === 0) return;

    const location = history.location;
    const index = Math.floor(Math.random() * count);
    const filterCopy = filter.changePage(index + 1);
    filterCopy.itemsPerPage = 1;
    try {
      const singleResult = await queryFindImages(filterCopy, {
        ...filterCopy.makeFilter(),
        collapse_stacks: collapseStacks,
      });
      const image = singleResult.data.findImages.images[0];
      if (image && history.location === location) {
        history.push(`/images/${image.id}`);
      }
    } catch (error) {
      if (history.location === location) toast.error(error);
    }
  }, [history, toast, filter, count, collapseStacks]);
}

function useRandomKeybind(viewRandom: () => Promise<void>, enabled: boolean) {
  useEffect(() => {
    if (!enabled) return;

    Mousetrap.bind("p r", () => {
      void viewRandom();
    });

    return () => {
      Mousetrap.unbind("p r");
    };
  }, [viewRandom, enabled]);
}

interface IImageList {
  filterHook?: (filter: ListFilterModel) => ListFilterModel;
  view?: View;
  alterQuery?: boolean;
  extraOperations?: IItemListOperation<GQL.FindImagesQueryResult>[];
  chapters?: GQL.GalleryChapterDataFragment[];
  imageIDs?: number[];
}

export const FilteredImageList = PatchComponent(
  "FilteredImageList",
  (props: IImageList) => {
    const intl = useIntl();
    const [{ data: localInterface }] = useInterfaceLocalForage();
    const defaultShowVariants = localInterface?.hideStackVariants === false;

    const [slideshowRunning, setSlideshowRunning] = useState<boolean>(false);

    const searchFocus = useFocus();

    const withSidebar = props.view !== View.GalleryImages;

    const {
      filterHook,
      view,
      alterQuery,
      extraOperations: providedOperations = [],
      chapters,
      imageIDs,
    } = props;
    const canToggleVariants = withSidebar && imageIDs === undefined;

    function stacksCollapsed(filter: ListFilterModel) {
      // Galleries always keep their own members and reading order.
      if (!withSidebar) return false;
      if (canToggleVariants && filter.showStackVariants !== undefined) {
        return !filter.showStackVariants;
      }
      return !defaultShowVariants;
    }

    function useResult(filter: ListFilterModel) {
      return GQL.useFindImagesQuery({
        skip: imageIDs !== undefined && imageIDs.length === 0,
        variables: {
          filter: filter.makeFindFilter(),
          image_filter: {
            ...filter.makeFilter(),
            collapse_stacks: stacksCollapsed(filter),
          },
          image_ids: imageIDs,
        },
      });
    }

    function useMetadataInfo(filter: ListFilterModel) {
      return GQL.useFindImagesMetadataQuery({
        skip: imageIDs !== undefined && imageIDs.length === 0,
        variables: {
          filter: filter.makeFindFilter(),
          image_filter: {
            ...filter.makeFilter(),
            collapse_stacks: stacksCollapsed(filter),
          },
          image_ids: imageIDs,
        },
      });
    }

    const {
      showSidebar,
      setShowSidebar,
      sectionOpen,
      setSectionOpen,
      loading: sidebarStateLoading,
    } = useSidebarState(view);

    const {
      filterState,
      queryResult,
      metadataInfo,
      modalState,
      listSelect,
      showEditFilter,
    } = useFilteredItemList({
      filterStateProps: {
        filterMode: GQL.FilterMode.Images,
        view,
        useURL: alterQuery,
      },
      queryResultProps: {
        useResult,
        useMetadataInfo,
        getCount: (r) => r.data?.findImages.count ?? 0,
        getItems: (r) => r.data?.findImages.images ?? [],
        filterHook,
      },
    });

    const { filter, setFilter } = filterState;

    const { effectiveFilter, result, cachedResult, items, totalCount } =
      queryResult;
    const listReady = !result.loading && !result.error && !!result.data;
    const listRef = useListScrollRestoration({
      view: view ?? View.Images,
      filter,
      active: alterQuery !== false,
      ready: listReady && !sidebarStateLoading,
    });

    useEffect(() => {
      const refetchAfterUpload = () => {
        void result.refetch();
      };
      window.addEventListener(MEDIA_UPLOAD_IMPORTED_EVENT, refetchAfterUpload);
      return () =>
        window.removeEventListener(
          MEDIA_UPLOAD_IMPORTED_EVENT,
          refetchAfterUpload
        );
    }, [result.refetch]);

    const metadataByline = useMemo(() => {
      if (cachedResult.loading) return null;

      return renderMetadataByline(metadataInfo) ?? null;
    }, [cachedResult.loading, metadataInfo]);

    const {
      selectedIds,
      selectedItems,
      onSelectChange,
      onSelectAll,
      onSelectNone,
      onDeselectIDs,
      onInvertSelection,
      hasSelection,
    } = listSelect;

    const { modal, showModal, closeModal } = modalState;

    const { setPage, removeCriterion, clearAllCriteria } = useFilterOperations({
      filter,
      setFilter,
    });

    const count = listReady ? (result.data?.findImages.count ?? 0) : 0;
    const randomEnabled =
      alterQuery !== false && imageIDs === undefined && count > 0;
    const viewRandom = useViewRandom(
      effectiveFilter,
      count,
      stacksCollapsed(effectiveFilter)
    );
    useRandomKeybind(viewRandom, randomEnabled);
    useFilteredSidebarKeybinds({
      showSidebar,
      setShowSidebar,
    });

    const onCloseEditDelete = useCloseEditDelete({
      closeModal,
      onSelectNone,
      result,
    });

    function onExport(all: boolean) {
      const exportAllRestricted = all && imageIDs !== undefined;
      showModal(
        <ExportDialog
          exportInput={{
            images: {
              ids: exportAllRestricted
                ? imageIDs.map(String)
                : Array.from(selectedIds.values()),
              all: all && !exportAllRestricted,
            },
          }}
          onClose={() => closeModal()}
        />
      );
    }

    const onEdit = useCallback(() => {
      showModal(
        <EditImagesDialog
          selected={selectedItems}
          onClose={onCloseEditDelete}
        />
      );
    }, [showModal, selectedItems, onCloseEditDelete]);

    const onDelete = useCallback(() => {
      showModal(
        <DeleteImagesDialog
          selected={selectedItems}
          onClose={onCloseEditDelete}
        />
      );
    }, [showModal, selectedItems, onCloseEditDelete]);

    useEffect(() => {
      Mousetrap.bind("e", () => {
        if (hasSelection) {
          onEdit?.();
        }
      });

      Mousetrap.bind("d d", () => {
        if (hasSelection) {
          onDelete?.();
        }
      });

      return () => {
        Mousetrap.unbind("e");
        Mousetrap.unbind("d d");
      };
    }, [hasSelection, onEdit, onDelete]);

    const convertedExtraOperations: IListFilterOperation[] =
      providedOperations.map((o) => ({
        ...o,
        isDisplayed: o.isDisplayed
          ? () => o.isDisplayed!(result, filter, selectedIds)
          : undefined,
        onClick: () => {
          o.onClick(result, filter, selectedIds, onSelectNone);
        },
      }));

    const onStacked = (media: GQL.MediaReferenceInput[]) =>
      onDeselectIDs(
        media
          .filter((member) => member.kind === GQL.MediaKind.Image)
          .map((member) => member.id)
      );

    const otherOperations: IListFilterOperation[] = [
      {
        text: "Create stack from selection…",
        isDisplayed: () => selectedIds.size >= 2 && selectedIds.size <= 200,
        onClick: () =>
          showModal(
            <VisualStackDialog
              selection={Array.from(selectedIds).map((id) => ({
                kind: GQL.MediaKind.Image,
                id,
              }))}
              onStacked={onStacked}
              onClose={closeModal}
            />
          ),
      },
      {
        text: "Review stack proposals…",
        isDisplayed: () => selectedIds.size >= 2 && selectedIds.size <= 100,
        onClick: () =>
          showModal(
            <StackProposalsDialog
              selection={Array.from(selectedIds).map((id) => ({
                kind: GQL.MediaKind.Image,
                id,
              }))}
              onStacked={onStacked}
              onClose={closeModal}
            />
          ),
      },

      ...convertedExtraOperations,
      {
        text: intl.formatMessage({ id: "actions.select_all" }),
        onClick: () => onSelectAll(),
        isDisplayed: () => totalCount > 0,
      },
      {
        text: intl.formatMessage({ id: "actions.select_none" }),
        onClick: () => onSelectNone(),
        isDisplayed: () => hasSelection,
      },
      {
        text: intl.formatMessage({ id: "actions.invert_selection" }),
        onClick: () => onInvertSelection(),
        isDisplayed: () => totalCount > 0,
      },
      {
        text: intl.formatMessage({ id: "actions.view_random" }),
        onClick: viewRandom,
        isDisplayed: () => randomEnabled,
      },
      metadataReviewOperation(
        Array.from(selectedIds).map((id) => ({
          kind: GQL.MediaKind.Image,
          id,
        })),
        showModal,
        () => {
          closeModal();
          void result.refetch();
        }
      ),
      {
        text: `${intl.formatMessage({ id: "actions.generate" })}…`,
        onClick: () => {
          showModal(
            <GenerateDialog
              type="image"
              selectedIds={Array.from(selectedIds.values())}
              onClose={() => closeModal()}
            />
          );
        },
        isDisplayed: () => hasSelection,
      },
      {
        text: intl.formatMessage({ id: "actions.export" }),
        onClick: () => onExport(false),
        isDisplayed: () => hasSelection,
      },
      {
        text: "Upscale selected images…",
        onClick: () =>
          showModal(
            <MediaUpscalingDialog
              selectedIds={Array.from(selectedIds)}
              onHide={() => {
                closeModal();
                void result.refetch();
              }}
            />
          ),
        isDisplayed: () => hasSelection,
      },
      {
        text: "Convert selected media…",
        onClick: () =>
          showModal(
            <MediaConversionDialog
              kind="image"
              selectedIds={Array.from(selectedIds)}
              onHide={() => {
                closeModal();
                void result.refetch();
              }}
            />
          ),
        isDisplayed: () => hasSelection,
      },
      {
        text: intl.formatMessage({ id: "actions.export_all" }),
        onClick: () => onExport(true),
      },
    ];

    if (sidebarStateLoading) return null;

    const operations = (
      <div className="d-flex align-items-center">
        <ListOperations
          items={items.length}
          hasSelection={hasSelection}
          operations={otherOperations}
          onEdit={onEdit}
          onDelete={onDelete}
          operationsMenuClassName="image-list-operations-dropdown"
        />
      </div>
    );

    const pageCount = Math.ceil(totalCount / filter.itemsPerPage);
    const visibleIds = new Set(items.map((image) => image.id));
    const hiddenSelected = listReady
      ? selectedItems.filter((image) => !visibleIds.has(image.id)).length
      : 0;

    const content = (
      <>
        <FilteredListToolbar
          filter={filter}
          listSelect={listSelect}
          setFilter={setFilter}
          showEditFilter={showEditFilter}
          onDelete={onDelete}
          onEdit={onEdit}
          operationComponent={operations}
          view={view}
          zoomable
          additionalViewOption={
            canToggleVariants
              ? {
                  label: "Show individual variants",
                  checked: !stacksCollapsed(effectiveFilter),
                  onChange: (show) =>
                    setFilter((current) =>
                      current.setShowStackVariants(
                        show === defaultShowVariants ? undefined : show
                      )
                    ),
                }
              : undefined
          }
        />

        {hiddenSelected > 0 && (
          <Alert variant="info" className="image-hidden-selection">
            <FormattedMessage
              id="actions.hidden_image_selection"
              values={{ count: hiddenSelected }}
            />
            <Button
              className="ml-2"
              variant="link"
              size="sm"
              onClick={onSelectNone}
            >
              <FormattedMessage id="actions.select_none" />
            </Button>
          </Alert>
        )}

        <FilterTags
          view={view}
          criteria={filter.criteria}
          onEditCriterion={(c) => showEditFilter(c.criterionOption.type)}
          onRemoveCriterion={removeCriterion}
          onRemoveAll={clearAllCriteria}
        />

        <div className="pagination-index-container">
          <Pagination
            currentPage={filter.currentPage}
            itemsPerPage={filter.itemsPerPage}
            totalItems={totalCount}
            onChangePage={setPage}
          />
          <PaginationIndex
            loading={cachedResult.loading}
            itemsPerPage={filter.itemsPerPage}
            currentPage={filter.currentPage}
            totalItems={totalCount}
            metadataByline={metadataByline}
          />
        </div>

        <LoadedContent loading={result.loading} error={result.error}>
          <ImageList
            filter={filter}
            images={items}
            onChangePage={setPage}
            onSelectChange={onSelectChange}
            pageCount={pageCount}
            totalCount={totalCount}
            selectedIds={selectedIds}
            slideshowRunning={slideshowRunning}
            setSlideshowRunning={setSlideshowRunning}
            chapters={chapters}
          />
        </LoadedContent>

        {totalCount > filter.itemsPerPage && (
          <div className="pagination-footer-container">
            <div className="pagination-footer">
              <Pagination
                itemsPerPage={filter.itemsPerPage}
                currentPage={filter.currentPage}
                totalItems={totalCount}
                onChangePage={setPage}
                pagePopupPlacement="top"
                metadataByline={metadataByline}
              />
            </div>
          </div>
        )}
      </>
    );

    return (
      <>
        {modal}
        {!withSidebar ? (
          <div className="item-list-container image-list" ref={listRef}>
            {content}
          </div>
        ) : (
          <div
            ref={listRef}
            className={cx("item-list-container image-list", {
              "hide-sidebar": !showSidebar,
            })}
          >
            <SidebarStateContext.Provider
              value={{ sectionOpen, setSectionOpen }}
            >
              <SidebarPane hideSidebar={!showSidebar}>
                <Sidebar
                  hide={!showSidebar}
                  onHide={() => setShowSidebar(false)}
                >
                  <SidebarContent
                    filter={filter}
                    setFilter={setFilter}
                    filterHook={filterHook}
                    showEditFilter={showEditFilter}
                    view={view}
                    sidebarOpen={showSidebar}
                    onClose={() => setShowSidebar(false)}
                    count={cachedResult.loading ? undefined : totalCount}
                    focus={searchFocus}
                  />
                </Sidebar>
                <SidebarPaneContent
                  onSidebarToggle={() => setShowSidebar(!showSidebar)}
                >
                  {content}
                </SidebarPaneContent>
              </SidebarPane>
            </SidebarStateContext.Provider>
          </div>
        )}
      </>
    );
  }
);
