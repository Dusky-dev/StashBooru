import { useCallback, useEffect, useMemo, useRef } from "react";
import { Alert, Button } from "react-bootstrap";
import { FormattedMessage, useIntl } from "react-intl";
import { useHistory, useLocation } from "react-router-dom";
import cx from "classnames";
import * as GQL from "src/core/generated-graphql";
import { ListFilterModel } from "src/models/list-filter/filter";
import { OrganizedCriterionOption } from "src/models/list-filter/criteria/organized";
import { useZoomKeybinds } from "../List/ZoomSlider";
import useFocus from "src/utils/focus";
import { useFilteredItemList } from "../List/ItemList";
import { FilteredListToolbar } from "../List/FilteredListToolbar";
import { FilterTags } from "../List/FilterTags";
import { ListOperations } from "../List/ListOperationButtons";
import { Pagination, PaginationIndex } from "../List/Pagination";
import { LoadedContent } from "../List/PagedList";
import { View } from "../List/views";
import { useFilterOperations } from "../List/util";
import {
  FilteredSidebarHeader,
  useFilteredSidebarKeybinds,
} from "../List/Filters/FilterSidebar";
import { SidebarStudiosFilter } from "../List/Filters/StudiosFilter";
import { SidebarPerformersFilter } from "../List/Filters/PerformersFilter";
import { SidebarTagsFilter } from "../List/Filters/TagsFilter";
import { SidebarRatingFilter } from "../List/Filters/RatingFilter";
import { SidebarDurationFilter } from "../List/Filters/SidebarDurationFilter";
import { SidebarBooleanFilter } from "../List/Filters/BooleanFilter";
import {
  Sidebar,
  SidebarPane,
  SidebarPaneContent,
  SidebarStateContext,
  useSidebarState,
} from "../Shared/Sidebar";
import { MediaConversionDialog } from "../Shared/MediaConversionDialog";
import { MediaUpscalingDialog } from "../Shared/MediaUpscalingDialog";
import { PatchComponent } from "src/patch";
import { MediaListContent } from "./MediaListContent";
import { EditMediaDialog } from "./EditMediaDialog";
import { mediaConversionTargets, splitMediaSelection } from "./mediaSelection";

const emptyItems: GQL.MediaListItemFragment[] = [];
const getItems = (result: GQL.FindMediaQueryResult) =>
  result.data?.findMedia.items ?? emptyItems;
const getCount = (result: GQL.FindMediaQueryResult) =>
  result.data?.findMedia.count ?? 0;
function useResult(filter: ListFilterModel) {
  return GQL.useFindMediaQuery({
    variables: {
      filter: filter.makeFindFilter(),
      media_filter: filter.makeFilter(),
    },
  });
}

const historyState = new Map<
  string,
  { selected: GQL.MediaListItemFragment[]; scroll: number }
>();

export const FilteredMediaList = PatchComponent("FilteredMediaList", () => {
  const intl = useIntl();
  const history = useHistory();
  const location = useLocation();
  const searchFocus = useFocus();
  const historyKey = location.key ?? location.pathname + location.search;
  const restore = useMemo(
    () => (history.action === "POP" ? historyState.get(historyKey) : undefined),
    [historyKey, history.action]
  );
  const pendingScroll = useRef(restore?.scroll);
  const { filterState, queryResult, listSelect, modalState, showEditFilter } =
    useFilteredItemList({
      filterStateProps: {
        filterMode: GQL.FilterMode.Media,
        view: View.Media,
        defaultSort: "date",
        useURL: true,
      },
      queryResultProps: { useResult, getCount, getItems },
      initialSelected: restore?.selected,
    });
  const {
    showSidebar,
    setShowSidebar,
    sectionOpen,
    setSectionOpen,
    loading: sidebarStateLoading,
  } = useSidebarState(View.Media);
  const { filter, setFilter } = filterState;
  const { effectiveFilter, result, items, totalCount, cachedResult } =
    queryResult;
  const { modal, showModal, closeModal } = modalState;
  const { setPage, removeCriterion, clearAllCriteria } = useFilterOperations({
    filter,
    setFilter,
  });
  const {
    selectedItems,
    selectedIds,
    hasSelection,
    onSelectChange,
    onSelectAll,
    onSelectNone,
    onInvertSelection,
    restoreSelection,
  } = listSelect;
  const { imageIDs, sceneIDs } = splitMediaSelection(selectedItems);

  useFilteredSidebarKeybinds({ showSidebar, setShowSidebar });
  useZoomKeybinds({
    zoomIndex: filter.zoomIndex,
    onChangeZoom: (zoom) => setFilter(filter.setZoom(zoom)),
  });
  useEffect(() => {
    if (restore) {
      pendingScroll.current = restore.scroll;
      restoreSelection(restore.selected);
    } else if (history.action !== "REPLACE") pendingScroll.current = undefined;
  }, [restore, restoreSelection, history.action]);
  useEffect(() => {
    const scroll = pendingScroll.current;
    if (result.loading || sidebarStateLoading || scroll === undefined) return;
    const frame = requestAnimationFrame(() => {
      window.scrollTo(0, scroll);
      pendingScroll.current = undefined;
    });
    return () => cancelAnimationFrame(frame);
  }, [result.loading, sidebarStateLoading]);
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

  const refreshed = useCallback(
    (applied = false) => {
      closeModal();
      if (applied) onSelectNone();
      void result.refetch();
    },
    [closeModal, onSelectNone, result]
  );
  const onEdit = useCallback(
    () =>
      showModal(
        <EditMediaDialog selected={selectedItems} onClose={refreshed} />
      ),
    [showModal, selectedItems, refreshed]
  );
  const operations = (
    <ListOperations
      items={items.length}
      hasSelection={hasSelection}
      onEdit={onEdit}
      operationsMenuClassName="media-list-operations-dropdown"
      operations={[
        {
          text: intl.formatMessage({ id: "actions.select_all" }),
          onClick: onSelectAll,
          isDisplayed: () => items.length > 0,
        },
        {
          text: intl.formatMessage({ id: "actions.select_none" }),
          onClick: onSelectNone,
          isDisplayed: () => hasSelection,
        },
        {
          text: intl.formatMessage({ id: "actions.invert_selection" }),
          onClick: onInvertSelection,
          isDisplayed: () => items.length > 0,
        },
        {
          text: "Convert selected media…",
          onClick: () =>
            showModal(
              <MediaConversionDialog
                kind={sceneIDs.length ? "scene" : "image"}
                selectedIds={[]}
                targets={mediaConversionTargets(selectedItems)}
                onHide={() => refreshed()}
              />
            ),
          isDisplayed: () => hasSelection,
        },
        {
          text: `Upscale ${imageIDs.length} Images…${sceneIDs.length ? ` (${sceneIDs.length} Videos excluded)` : ""}`,
          onClick: () =>
            showModal(
              <MediaUpscalingDialog
                selectedIds={imageIDs}
                excludedVideoCount={sceneIDs.length}
                onHide={() => refreshed()}
              />
            ),
          isDisplayed: () => imageIDs.length > 0,
        },
      ]}
    />
  );
  if (sidebarStateLoading) return null;
  return (
    <div
      className={cx("item-list-container media-list", {
        "hide-sidebar": !showSidebar,
      })}
    >
      {modal}
      <SidebarStateContext.Provider value={{ sectionOpen, setSectionOpen }}>
        <SidebarPane hideSidebar={!showSidebar}>
          <Sidebar hide={!showSidebar} onHide={() => setShowSidebar(false)}>
            <FilteredSidebarHeader
              sidebarOpen={showSidebar}
              showEditFilter={showEditFilter}
              filter={filter}
              setFilter={setFilter}
              view={View.Media}
              focus={searchFocus}
            />
            <SidebarStudiosFilter filter={filter} setFilter={setFilter} />
            <SidebarPerformersFilter filter={filter} setFilter={setFilter} />
            <SidebarTagsFilter filter={filter} setFilter={setFilter} />
            <SidebarRatingFilter filter={filter} setFilter={setFilter} />
            <SidebarDurationFilter filter={filter} setFilter={setFilter} />
            <SidebarBooleanFilter
              title={<FormattedMessage id="organized" />}
              option={OrganizedCriterionOption}
              filter={filter}
              setFilter={setFilter}
              sectionID="organized"
            />
            <div className="sidebar-footer">
              <Button
                className="sidebar-close-button"
                onClick={() => setShowSidebar(false)}
              >
                <FormattedMessage
                  id="actions.show_count_results"
                  values={{ count: totalCount }}
                />
              </Button>
            </div>
          </Sidebar>
          <SidebarPaneContent
            onSidebarToggle={() => setShowSidebar(!showSidebar)}
          >
            <FilteredListToolbar
              filter={filter}
              setFilter={setFilter}
              listSelect={listSelect}
              showEditFilter={showEditFilter}
              view={View.Media}
              zoomable
              maxPageSize={500}
              onEdit={onEdit}
              operationComponent={operations}
            />
            <FilterTags
              view={View.Media}
              criteria={filter.criteria}
              onEditCriterion={(criterion) =>
                showEditFilter(criterion.criterionOption.type)
              }
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
                onChangePage={setPage}
              />
              <PaginationIndex
                loading={cachedResult.loading}
                currentPage={filter.currentPage}
                itemsPerPage={filter.itemsPerPage}
                totalItems={totalCount}
              />
            </div>
            <LoadedContent loading={result.loading} error={result.error}>
              <MediaListContent
                items={items}
                filter={effectiveFilter}
                selectedIds={selectedIds}
                onSelectChange={onSelectChange}
              />
            </LoadedContent>
            {totalCount > filter.itemsPerPage && (
              <div className="pagination-footer-container">
                <div className="pagination-footer">
                  <Pagination
                    currentPage={filter.currentPage}
                    itemsPerPage={filter.itemsPerPage}
                    totalItems={totalCount}
                    onChangePage={setPage}
                    pagePopupPlacement="top"
                  />
                </div>
              </div>
            )}
          </SidebarPaneContent>
        </SidebarPane>
      </SidebarStateContext.Provider>
    </div>
  );
});
