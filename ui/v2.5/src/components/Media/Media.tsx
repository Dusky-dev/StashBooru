import { useEffect, useMemo, useRef } from "react";
import { Alert, Button, Nav } from "react-bootstrap";
import { Link, useHistory, useLocation } from "react-router-dom";
import { Helmet } from "react-helmet";
import * as GQL from "src/core/generated-graphql";
import { ListFilterModel } from "src/models/list-filter/filter";
import { DisplayMode } from "src/models/list-filter/types";
import { useFilteredItemList } from "../List/ItemList";
import { FilteredListToolbar } from "../List/FilteredListToolbar";
import { FilterTags } from "../List/FilterTags";
import { Pagination, PaginationIndex } from "../List/Pagination";
import { LoadedContent } from "../List/PagedList";
import { View } from "../List/views";
import { useFilterOperations } from "../List/util";
import { useLightbox } from "src/hooks/Lightbox/hooks";
import { MediaConversionDialog } from "../Shared/MediaConversionDialog";
import { MediaUpscalingDialog } from "../Shared/MediaUpscalingDialog";
import { MediaGrid, MediaWall } from "./MediaCards";
import { EditMediaDialog } from "./EditMediaDialog";
import { mediaConversionTargets, splitMediaSelection } from "./mediaSelection";

// Back-navigation snapshots are bounded and kept only for this browser session.
const historyState = new Map<
  string,
  { selected: GQL.MediaListItemFragment[]; scroll: number }
>();

export default function Media() {
  const history = useHistory();
  const location = useLocation();
  const historyKey = location.key ?? location.pathname + location.search;
  const restore = useMemo(
    () => (history.action === "POP" ? historyState.get(historyKey) : undefined),
    [historyKey, history.action]
  );
  const pendingScroll = useRef(restore?.scroll);
  const mediaType =
    location.pathname === "/media/images"
      ? GQL.MediaKind.Image
      : location.pathname === "/media/videos"
        ? GQL.MediaKind.Video
        : undefined;
  function useResult(filter: ListFilterModel) {
    return GQL.useFindMediaQuery({
      fetchPolicy: "cache-and-network",
      notifyOnNetworkStatusChange: true,
      variables: {
        filter: filter.makeFindFilter(),
        media_filter: {
          ...filter.makeFilter(),
          media_types: mediaType ? [mediaType] : undefined,
        },
      },
    });
  }
  const { filterState, queryResult, listSelect, modalState, showEditFilter } =
    useFilteredItemList({
      filterStateProps: {
        filterMode: GQL.FilterMode.Media,
        view: View.Media,
        defaultSort: "date",
        useURL: true,
      },
      queryResultProps: {
        useResult,
        getCount: (r) => r.data?.findMedia.count ?? 0,
        getItems: (r) => r.data?.findMedia.items ?? [],
      },
      initialSelected: restore?.selected,
    });
  const { filter, setFilter } = filterState;
  const { result, items, totalCount, cachedResult } = queryResult;
  const { modal, showModal, closeModal } = modalState;
  const { removeCriterion, clearAllCriteria } = useFilterOperations({
    filter,
    setFilter,
  });
  const {
    selectedItems,
    selectedIds,
    onSelectChange,
    onSelectNone,
    restoreSelection,
  } = listSelect;
  const { imageIDs, sceneIDs } = splitMediaSelection(selectedItems);
  const images = useMemo(
    () => items.flatMap((i) => (i.image ? [i.image] : [])),
    [items]
  );
  const lightboxState = useMemo(
    () => ({
      images,
      totalCount: images.length,
      page: 1,
      pages: 1,
      pageSize: images.length,
      showNavigation: false,
    }),
    [images]
  );
  const showLightbox = useLightbox(lightboxState);
  useEffect(() => {
    if (restore) {
      pendingScroll.current = restore.scroll;
      restoreSelection(restore.selected);
    } else if (history.action !== "REPLACE") {
      pendingScroll.current = undefined;
    }
  }, [restore, restoreSelection, history.action]);
  useEffect(() => {
    const scroll = pendingScroll.current;
    if (
      result.loading ||
      scroll === undefined ||
      (!restore && history.action !== "REPLACE")
    )
      return;
    const frame = requestAnimationFrame(() => {
      window.scrollTo(0, scroll);
      pendingScroll.current = undefined;
    });
    return () => cancelAnimationFrame(frame);
  }, [result.loading, restore, history.action]);
  useEffect(() => {
    const save = () => {
      historyState.set(historyKey, {
        selected: selectedItems,
        scroll: pendingScroll.current ?? window.scrollY,
      });
      if (historyState.size > 8)
        historyState.delete(historyState.keys().next().value!);
    };
    save();
    window.addEventListener("scroll", save, { passive: true });
    return () => {
      save();
      window.removeEventListener("scroll", save);
    };
  }, [historyKey, selectedItems]);

  function refreshed(applied = false) {
    closeModal();
    if (applied) {
      onSelectNone();
      void result.refetch();
    } else void result.refetch();
  }
  function edit() {
    showModal(<EditMediaDialog selected={selectedItems} onClose={refreshed} />);
  }
  const operationComponent = (
    <div className="d-flex flex-wrap align-items-center">
      {selectedIds.size > 0 && (
        <>
          <Button className="ml-2" onClick={edit}>
            Edit selected media…
          </Button>
          <Button
            className="ml-2"
            onClick={() =>
              showModal(
                <MediaConversionDialog
                  kind={sceneIDs.length ? "scene" : "image"}
                  selectedIds={[]}
                  targets={mediaConversionTargets(selectedItems)}
                  onHide={() => refreshed()}
                />
              )
            }
          >
            Convert selected media…
          </Button>
          <Button
            className="ml-2"
            disabled={!imageIDs.length}
            title={
              sceneIDs.length
                ? `${sceneIDs.length} selected Videos are excluded from image upscaling`
                : undefined
            }
            onClick={() =>
              showModal(
                <MediaUpscalingDialog
                  selectedIds={imageIDs}
                  excludedVideoCount={sceneIDs.length}
                  onHide={() => refreshed()}
                />
              )
            }
          >
            Upscale {imageIDs.length} Images…
          </Button>
          <span className="ml-2">
            {imageIDs.length} Images · {sceneIDs.length} Videos selected
          </span>
        </>
      )}
    </div>
  );
  const cards = {
    items,
    selectedIds,
    zoomIndex: filter.zoomIndex,
    onSelectChange,
    onPreviewImage: (image: GQL.SlimImageDataFragment) =>
      showLightbox({
        initialIndex: images.findIndex((i) => i.id === image.id),
      }),
  };
  return (
    <>
      <Helmet>
        <title>Media · StashBooru</title>
      </Helmet>
      {modal}
      <Nav variant="tabs" className="mb-3" aria-label="Media type">
        {[
          ["All", "/media"],
          ["Images", "/media/images"],
          ["Videos", "/media/videos"],
        ].map(([label, path]) => (
          <Nav.Item key={path}>
            <Nav.Link
              as={Link}
              to={`${path}${location.search.replace(/([?&])p=\d+/, "$1p=1")}`}
              active={location.pathname === path}
            >
              {label}
            </Nav.Link>
          </Nav.Item>
        ))}
      </Nav>
      <div className="item-list-container media-list">
        <FilteredListToolbar
          filter={filter}
          setFilter={setFilter}
          listSelect={listSelect}
          showEditFilter={showEditFilter}
          view={View.Media}
          zoomable
          maxPageSize={500}
          operationComponent={operationComponent}
        />
        <FilterTags
          view={View.Media}
          criteria={filter.criteria}
          onEditCriterion={(c) => showEditFilter(c.criterionOption.type)}
          onRemoveCriterion={removeCriterion}
          onRemoveAll={clearAllCriteria}
        />
        {filter.criteriaFor("duration").length > 0 && (
          <Alert variant="info">
            Duration filters apply to Videos. Images are excluded while this
            filter is active.
          </Alert>
        )}
        <div className="pagination-index-container">
          <Pagination
            currentPage={filter.currentPage}
            itemsPerPage={filter.itemsPerPage}
            totalItems={totalCount}
            onChangePage={(p) => setFilter(filter.changePage(p))}
          />
          <PaginationIndex
            loading={cachedResult.loading}
            currentPage={filter.currentPage}
            itemsPerPage={filter.itemsPerPage}
            totalItems={totalCount}
          />
        </div>
        <LoadedContent loading={result.loading} error={result.error}>
          {filter.displayMode === DisplayMode.Wall ? (
            <MediaWall {...cards} />
          ) : (
            <MediaGrid {...cards} />
          )}
        </LoadedContent>
        {totalCount > filter.itemsPerPage && (
          <Pagination
            currentPage={filter.currentPage}
            itemsPerPage={filter.itemsPerPage}
            totalItems={totalCount}
            onChangePage={(p) => setFilter(filter.changePage(p))}
          />
        )}
      </div>
    </>
  );
}
