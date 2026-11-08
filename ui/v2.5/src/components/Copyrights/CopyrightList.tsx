import React from "react";
import { ButtonGroup } from "react-bootstrap";
import { useApolloClient } from "@apollo/client";
import { Link, useHistory } from "react-router-dom";
import { FormattedMessage, useIntl } from "react-intl";

import * as GQL from "src/core/generated-graphql";
import { ListFilterModel } from "src/models/list-filter/filter";
import { DisplayMode } from "src/models/list-filter/types";
import { useFilteredItemList } from "src/components/List/ItemList";
import { FilteredListToolbar } from "src/components/List/FilteredListToolbar";
import { ListOperations } from "src/components/List/ListOperationButtons";
import { Pagination, PaginationIndex } from "src/components/List/Pagination";
import { LoadedContent } from "src/components/List/PagedList";
import { View } from "src/components/List/views";
import { TruncatedText } from "src/components/Shared/TruncatedText";
import {
  GridCard,
  useCardWidth,
  useContainerDimensions,
} from "src/components/Shared/GridCard/GridCard";
import { PopoverCountButton } from "src/components/Shared/PopoverCountButton";
import { ListTable, IColumn } from "src/components/List/ListTable";
import { useTableColumns } from "src/hooks/useTableColumns";
import { useToast } from "src/hooks/Toast";

const zoomWidths = [280, 340, 480, 640];
const COPYRIGHT_LIST_ZOOM_PREFERENCE_KEY = "copyrights-list";

function useFindCopyrightsForList(filter: ListFilterModel) {
  return GQL.useFindCopyrightsQuery({
    variables: { filter: filter.makeFindFilter() },
    fetchPolicy: "network-only",
  });
}

interface CopyrightCardProps {
  copyright: GQL.CopyrightListDataFragment;
  cardWidth?: number;
  zoomIndex: number;
  selecting: boolean;
  selected: boolean;
  onSelectedChanged: (selected: boolean, shiftKey: boolean) => void;
}

const CopyrightCard: React.FC<CopyrightCardProps> = ({
  copyright,
  cardWidth,
  zoomIndex,
  selecting,
  selected,
  onSelectedChanged,
}) => (
  <GridCard
    className={`tag-card copyright-card zoom-${zoomIndex}`}
    url={`/copyrights/${copyright.id}`}
    width={cardWidth}
    title={copyright.name}
    linkClassName="tag-card-header copyright-card-header"
    image={
      copyright.image_path ? (
        <img
          loading="lazy"
          className="tag-card-image copyright-card-image"
          alt={copyright.name}
          src={copyright.image_path}
        />
      ) : (
        <span
          className="tag-card-image copyright-card-image"
          aria-hidden="true"
        />
      )
    }
    details={
      copyright.description ? (
        <TruncatedText
          className="tag-description"
          text={copyright.description}
          lineCount={3}
        />
      ) : undefined
    }
    popovers={
      <>
        <hr />
        <ButtonGroup className="card-popovers">
          <PopoverCountButton
            className="scene-count"
            type="scene"
            count={copyright.scene_count}
            url={`/copyrights/${copyright.id}/videos`}
            showZero={false}
          />
          <PopoverCountButton
            className="image-count"
            type="image"
            count={copyright.image_count}
            url={`/copyrights/${copyright.id}/images`}
            showZero={false}
          />
          <PopoverCountButton
            className="performer-count"
            type="performer"
            count={copyright.performer_count}
            url={`/copyrights/${copyright.id}/characters`}
            showZero={false}
          />
        </ButtonGroup>
      </>
    }
    selected={selected}
    selecting={selecting}
    onSelectedChanged={onSelectedChanged}
  />
);

const CopyrightList: React.FC = () => {
  const intl = useIntl();
  const history = useHistory();
  const client = useApolloClient();
  const Toast = useToast();
  const view = View.Copyrights;

  const { filterState, queryResult, modalState, listSelect, showEditFilter } =
    useFilteredItemList({
      filterStateProps: {
        filterMode: GQL.FilterMode.Copyrights,
        view,
        zoomPreferenceKey: COPYRIGHT_LIST_ZOOM_PREFERENCE_KEY,
      },
      queryResultProps: {
        useResult: useFindCopyrightsForList,
        getCount: (result) => result.data?.findCopyrights.count ?? 0,
        getItems: (result) => result.data?.findCopyrights.copyrights ?? [],
      },
    });

  const { filter, setFilter } = filterState;
  const { result, cachedResult, items: copyrights, totalCount } = queryResult;
  const { modal } = modalState;
  const {
    selectedIds,
    selectedItems,
    onSelectChange,
    onSelectAll,
    onSelectNone,
    onInvertSelection,
  } = listSelect;
  const columns: IColumn[] = [
    { value: "name", label: "Name", mandatory: true },
    { value: "scene_count", label: "Videos" },
    { value: "image_count", label: "Images" },
    { value: "performer_count", label: "Characters" },
  ];
  const { selectedColumns, saveColumns } = useTableColumns("copyrights", [
    "name",
    "scene_count",
    "image_count",
    "performer_count",
  ]);
  const [componentRef, { width: containerWidth }] = useContainerDimensions();
  const cardWidth = useCardWidth(containerWidth, filter.zoomIndex, zoomWidths);

  async function viewRandom() {
    if (totalCount === 0) return;
    const randomFilter = filter.clone();
    randomFilter.currentPage = Math.floor(Math.random() * totalCount) + 1;
    randomFilter.itemsPerPage = 1;
    try {
      const { data } = await client.query<GQL.FindCopyrightsQuery>({
        query: GQL.FindCopyrightsDocument,
        variables: { filter: randomFilter.makeFindFilter() },
        fetchPolicy: "network-only",
      });
      const selected = data.findCopyrights.copyrights[0];
      if (selected) history.push(`/copyrights/${selected.id}`);
    } catch (error) {
      Toast.error(error);
    }
  }

  const operations = (
    <ListOperations
      items={copyrights.length}
      hasSelection={selectedIds.size > 0}
      operations={[
        {
          text: "New Copyright",
          onClick: () => history.push("/copyrights/new"),
          isDisplayed: () => selectedIds.size === 0,
        },
        {
          text: intl.formatMessage({ id: "actions.select_all" }),
          onClick: onSelectAll,
          isDisplayed: () => copyrights.length > 0,
        },
        {
          text: intl.formatMessage({ id: "actions.select_none" }),
          onClick: onSelectNone,
          isDisplayed: () => selectedIds.size > 0,
        },
        {
          text: intl.formatMessage({ id: "actions.invert_selection" }),
          onClick: onInvertSelection,
          isDisplayed: () => copyrights.length > 0,
        },
        {
          text: "Open selected Copyright",
          onClick: () => history.push(`/copyrights/${selectedItems[0].id}`),
          isDisplayed: () => selectedItems.length === 1,
        },
        {
          text: intl.formatMessage({ id: "actions.view_random" }),
          onClick: () => void viewRandom(),
          isDisplayed: () => totalCount > 0 && selectedIds.size === 0,
        },
      ]}
    />
  );

  function renderGrid() {
    return (
      <div className="row justify-content-center" ref={componentRef}>
        {copyrights.map((copyright) => (
          <CopyrightCard
            key={copyright.id}
            copyright={copyright}
            cardWidth={cardWidth}
            zoomIndex={filter.zoomIndex}
            selecting={selectedIds.size > 0}
            selected={selectedIds.has(copyright.id)}
            onSelectedChanged={(selected, shiftKey) =>
              onSelectChange(copyright.id, selected, shiftKey)
            }
          />
        ))}
      </div>
    );
  }

  function renderList() {
    const renderCell = (
      column: IColumn,
      copyright: GQL.CopyrightListDataFragment
    ) => {
      switch (column.value) {
        case "name":
          return (
            <div
              className="copyright-list-name"
              style={{
                paddingInlineStart: `${
                  Math.min(Math.max(copyright.breadcrumb.length - 1, 0), 12) *
                  1.25
                }rem`,
              }}
              title={copyright.breadcrumb.map((item) => item.name).join(" / ")}
            >
              <Link to={`/copyrights/${copyright.id}`}>{copyright.name}</Link>
              {copyright.ordered_children.length > 0 && (
                <span className="text-muted small ml-2">
                  {copyright.ordered_children.length} Sub-Copyrights
                </span>
              )}
              {copyright.breadcrumb.length > 1 && (
                <div className="small text-muted copyright-list-path">
                  {copyright.breadcrumb.slice(0, -1).map((ancestor, index) => (
                    <React.Fragment key={ancestor.id}>
                      {index > 0 ? " / " : null}
                      <Link to={`/copyrights/${ancestor.id}`}>
                        {ancestor.name}
                      </Link>
                    </React.Fragment>
                  ))}
                </div>
              )}
            </div>
          );
        case "scene_count":
          return (
            <Link to={`/copyrights/${copyright.id}/videos`}>
              {copyright.scene_count}
            </Link>
          );
        case "image_count":
          return (
            <Link to={`/copyrights/${copyright.id}/images`}>
              {copyright.image_count}
            </Link>
          );
        case "performer_count":
          return (
            <Link to={`/copyrights/${copyright.id}/characters`}>
              {copyright.performer_count}
            </Link>
          );
      }
    };
    return (
      <ListTable
        className="copyright-table"
        items={copyrights}
        allColumns={columns}
        columns={selectedColumns}
        setColumns={saveColumns}
        selectedIds={selectedIds}
        onSelectChange={onSelectChange}
        renderCell={renderCell}
      />
    );
  }

  return (
    <div className="item-list-container copyright-list">
      {modal}

      <FilteredListToolbar
        filter={filter}
        listSelect={listSelect}
        setFilter={setFilter}
        showEditFilter={showEditFilter}
        operationComponent={operations}
        view={view}
        zoomable
      />

      <p className="text-muted small">
        <FormattedMessage id="copyright_hierarchy.direct_count_help" />
      </p>

      <div className="pagination-index-container">
        <Pagination
          currentPage={filter.currentPage}
          itemsPerPage={filter.itemsPerPage}
          totalItems={totalCount}
          onChangePage={(page) => setFilter(filter.changePage(page))}
        />
        <PaginationIndex
          loading={cachedResult.loading}
          currentPage={filter.currentPage}
          itemsPerPage={filter.itemsPerPage}
          totalItems={totalCount}
        />
      </div>

      <LoadedContent loading={result.loading} error={result.error}>
        {filter.displayMode === DisplayMode.List ? renderList() : renderGrid()}
      </LoadedContent>

      {totalCount > filter.itemsPerPage && (
        <div className="pagination-footer-container">
          <div className="pagination-footer">
            <Pagination
              currentPage={filter.currentPage}
              itemsPerPage={filter.itemsPerPage}
              totalItems={totalCount}
              onChangePage={(page) => setFilter(filter.changePage(page))}
              pagePopupPlacement="top"
            />
          </div>
        </div>
      )}
    </div>
  );
};

export default CopyrightList;
