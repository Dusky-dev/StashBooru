import { metadataReviewOperation } from "../Tagging/MetadataReviewDialog";
import { VisualStackDialog } from "../VisualStacks/VisualStackDialog";
import { StackProposalsDialog } from "../VisualStacks/StackProposalsDialog";
import { StackMember } from "../VisualStacks/media";
import { stackKey } from "../VisualStacks/identity";
import { CollapseStacksOption } from "src/models/list-filter/media";
import { BooleanCriterion } from "src/models/list-filter/criteria/criterion";
import { useCallback, useEffect, useMemo } from "react";
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
import { useListScrollRestoration } from "../List/useListScrollRestoration";
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
import { SidebarCopyrightsFilter } from "../List/Filters/CopyrightsFilter";
import { SidebarRatingFilter } from "../List/Filters/RatingFilter";
import { SidebarDurationFilter } from "../List/Filters/SidebarDurationFilter";
import { SidebarBooleanFilter } from "../List/Filters/BooleanFilter";
import { SidebarOptionFilter } from "../List/Filters/OptionFilter";
import { MediaKindCriterionOption } from "src/models/list-filter/criteria/media-types";
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
import { useInterfaceLocalForage } from "src/hooks/LocalForage";
import { MEDIA_UPLOAD_IMPORTED_EVENT } from "../Shared/mediaUploadSettings";

const emptyItems: GQL.MediaListItemFragment[] = [];
const getItems = (result: GQL.FindMediaQueryResult) =>
  result.data?.findMedia.items ?? emptyItems;
const getCount = (result: GQL.FindMediaQueryResult) =>
  result.data?.findMedia.count ?? 0;
function useResult(filter: ListFilterModel) {
  const [{ data: localInterface }] = useInterfaceLocalForage();
  const explicit = filter.criteriaFor("collapse_stacks") as BooleanCriterion[];
  const collapse = explicit.length
    ? explicit.some((criterion) => criterion.value === "true")
    : localInterface?.hideStackVariants !== false;
  return GQL.useFindMediaQuery({
    variables: {
      filter: filter.makeFindFilter(),
      media_filter: { ...filter.makeFilter(), collapse_stacks: collapse },
    },
  });
}

const historySelection = new Map<string, GQL.MediaListItemFragment[]>();

interface IFilteredMediaList {
  filterHook?: (filter: ListFilterModel) => ListFilterModel;
  view?: View;
}

const FilteredMediaListComponent = ({
  filterHook,
  view = View.Media,
}: IFilteredMediaList) => {
  const intl = useIntl();
  const history = useHistory();
  const location = useLocation();
  const searchFocus = useFocus();
  const historyKey = location.key ?? location.pathname + location.search;
  const restore = useMemo(
    () =>
      history.action === "POP" ? historySelection.get(historyKey) : undefined,
    [historyKey, history.action]
  );
  const { filterState, queryResult, listSelect, modalState, showEditFilter } =
    useFilteredItemList({
      filterStateProps: {
        filterMode: GQL.FilterMode.Media,
        view,
        defaultSort: "date",
        useURL: true,
      },
      queryResultProps: { useResult, getCount, getItems, filterHook },
      initialSelected: restore,
    });
  const {
    showSidebar,
    setShowSidebar,
    sectionOpen,
    setSectionOpen,
    loading: sidebarStateLoading,
  } = useSidebarState(view);
  const { filter, setFilter } = filterState;
  const { effectiveFilter, result, items, totalCount, cachedResult } =
    queryResult;
  const listReady = !result.loading && !result.error && !!result.data;
  const listRef = useListScrollRestoration({
    view,
    filter,
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
  const { modal, showModal, closeModal } = modalState;
  const { setPage, removeCriterion } = useFilterOperations({
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
    onDeselectIDs,
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
    if (restore) restoreSelection(restore);
  }, [restore, restoreSelection]);
  useEffect(() => {
    historySelection.set(historyKey, selectedItems);
    if (historySelection.size > 8) {
      historySelection.delete(historySelection.keys().next().value!);
    }
  }, [historyKey, selectedItems]);

  const refreshed = useCallback(() => {
    closeModal();
    void result.refetch();
  }, [closeModal, result]);
  const onEdit = useCallback(
    () =>
      showModal(
        <EditMediaDialog
          selected={selectedItems}
          onClose={(confirmed) => {
            if (confirmed.length > 0) onDeselectIDs(confirmed);
            refreshed();
          }}
        />
      ),
    [showModal, selectedItems, onDeselectIDs, refreshed]
  );
  const explicitCollapse = filter.criteriaFor(
    "collapse_stacks"
  ) as BooleanCriterion[];
  const collapsed = result.variables?.media_filter?.collapse_stacks === true;
  const hiddenSelected = selectedItems.filter(
    (m) => !items.some((item) => item.id === m.id)
  ).length;
  const selection = selectedItems.map((m) => ({
    kind: m.kind,
    id: m.image?.id ?? m.scene!.id,
  }));
  const selectStackMember = (member: StackMember, checked: boolean) => {
    const next = selectedItems.filter((m) => m.id !== member.id);
    if (checked)
      next.push({
        id: member.id,
        kind: member.media.kind,
        image: member.image,
        scene: member.scene,
        stack_match_count: 1,
      });
    restoreSelection(next);
  };
  const onStacked = (media: GQL.MediaReferenceInput[]) =>
    onDeselectIDs(media.map(stackKey));
  const operations = (
    <ListOperations
      items={items.length}
      hasSelection={hasSelection}
      onEdit={onEdit}
      operationsMenuClassName="media-list-operations-dropdown"
      operations={[
        {
          text: "Create stack from selection…",
          isDisplayed: () =>
            selectedItems.length >= 2 && selectedItems.length <= 200,
          onClick: () =>
            showModal(
              <VisualStackDialog
                selection={selection}
                onStacked={onStacked}
                onClose={closeModal}
              />
            ),
        },
        {
          text: "Review stack proposals…",
          isDisplayed: () =>
            selectedItems.length >= 2 && selectedItems.length <= 100,
          onClick: () =>
            showModal(
              <StackProposalsDialog
                selection={selection}
                onStacked={onStacked}
                onClose={closeModal}
              />
            ),
        },
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
        metadataReviewOperation(selection, showModal, () => refreshed()),
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
      ref={listRef}
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
              view={view}
              focus={searchFocus}
            />
            <SidebarOptionFilter
              title="Media type"
              option={MediaKindCriterionOption}
              filter={filter}
              setFilter={setFilter}
              sectionID="media-types"
            />
            <SidebarStudiosFilter filter={filter} setFilter={setFilter} />
            <SidebarPerformersFilter filter={filter} setFilter={setFilter} />
            <SidebarTagsFilter filter={filter} setFilter={setFilter} />
            <SidebarCopyrightsFilter filter={filter} setFilter={setFilter} />
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
              view={view}
              zoomable
              maxPageSize={500}
              onEdit={onEdit}
              operationComponent={operations}
              filterCount={filter.count() - explicitCollapse.length}
              additionalViewOption={{
                label: "Show individual variants",
                checked: !collapsed,
                onChange: (shown) => {
                  // Keep the explicit per-list override in the URL/filter state.
                  // An explicit false still overrides the browser-wide default.
                  setFilter((current) => {
                    const criterion = new BooleanCriterion(
                      CollapseStacksOption
                    );
                    criterion.value = shown ? "false" : "true";
                    return current.replaceCriteria("collapse_stacks", [
                      criterion,
                    ]);
                  });
                },
              }}
            />
            {hiddenSelected > 0 && (
              <Alert variant="info">
                {hiddenSelected} selected members are hidden by grouping or
                filters. Actions still target only the selected members.
              </Alert>
            )}
            <FilterTags
              view={view}
              criteria={filter.criteria.filter(
                (criterion) =>
                  criterion.criterionOption.type !== "collapse_stacks"
              )}
              onEditCriterion={(criterion) =>
                showEditFilter(criterion.criterionOption.type)
              }
              onRemoveCriterion={removeCriterion}
              onRemoveAll={() =>
                setFilter((current) =>
                  current
                    .clearCriteria()
                    .setCriteria(current.criteriaFor("collapse_stacks"))
                )
              }
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
                groupedStacks={collapsed}
                filter={effectiveFilter}
                selectedIds={selectedIds}
                onSelectChange={onSelectChange}
                onStackMemberSelect={selectStackMember}
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
};

export const FilteredMediaList = PatchComponent(
  "FilteredMediaList",
  FilteredMediaListComponent
);
