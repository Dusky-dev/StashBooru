import React, { useMemo, useState } from "react";
import { Button, ButtonGroup, Table } from "react-bootstrap";
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
      <img
        loading="lazy"
        className="tag-card-image copyright-card-image"
        alt={copyright.name}
        src={copyright.image_path ?? ""}
      />
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
  const view = View.Copyrights;
  const [treeMode, setTreeMode] = useState(false);

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
  const { selectedIds, onSelectChange } = listSelect;
  const [componentRef, { width: containerWidth }] = useContainerDimensions();
  const cardWidth = useCardWidth(containerWidth, filter.zoomIndex, zoomWidths);

  const treeResult = GQL.useFindCopyrightsQuery({
    variables: {
      filter: {
        per_page: -1,
        sort: "sort_name",
        direction: GQL.SortDirectionEnum.Asc,
      },
    },
    fetchPolicy: "network-only",
    skip: !treeMode,
  });

  const treeItems = treeResult.data?.findCopyrights.copyrights ?? [];
  const treeByID = useMemo(
    () => new Map(treeItems.map((item) => [item.id, item])),
    [treeItems]
  );
  const treeRoots = useMemo(
    () => treeItems.filter((item) => item.parents.length === 0),
    [treeItems]
  );

  function viewRandom() {
    if (copyrights.length === 0) return;
    const item = copyrights[Math.floor(Math.random() * copyrights.length)];
    history.push(`/copyrights/${item.id}`);
  }

  const operations = (
    <ListOperations
      items={copyrights.length}
      hasSelection={selectedIds.size > 0}
      operations={[
        {
          text: intl.formatMessage({ id: "actions.view_random" }),
          onClick: viewRandom,
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
    return (
      <Table responsive hover className="list-table">
        <thead>
          <tr>
            <th>Name</th>
            <th>Videos</th>
            <th>Images</th>
            <th>Characters</th>
          </tr>
        </thead>
        <tbody>
          {copyrights.map((copyright) => (
            <tr key={copyright.id}>
              <td>
                <Link to={`/copyrights/${copyright.id}`}>{copyright.name}</Link>
              </td>
              <td>{copyright.scene_count}</td>
              <td>{copyright.image_count}</td>
              <td>{copyright.performer_count}</td>
            </tr>
          ))}
        </tbody>
      </Table>
    );
  }

  function renderTreeRows(
    item: GQL.CopyrightListDataFragment,
    depth: number,
    path: Set<string>
  ): React.ReactNode[] {
    if (path.has(item.id)) return [];
    const nextPath = new Set(path);
    nextPath.add(item.id);
    const rows: React.ReactNode[] = [
      <tr key={`${item.id}-${Array.from(path).join("-")}`}>
        <td style={{ paddingLeft: `${0.75 + depth * 1.5}rem` }}>
          <Link to={`/copyrights/${item.id}`}>{item.name}</Link>
          {item.structural_role ? (
            <span className="badge badge-secondary ml-2">
              {item.structural_role}
            </span>
          ) : null}
        </td>
        <td title={`${item.scene_count} direct`}>{item.subtree_scene_count}</td>
        <td title={`${item.image_count} direct`}>{item.subtree_image_count}</td>
        <td title={`${item.performer_count} direct`}>
          {item.subtree_performer_count}
        </td>
      </tr>,
    ];

    for (const childRef of item.ordered_children) {
      const child = treeByID.get(childRef.id);
      if (child) rows.push(...renderTreeRows(child, depth + 1, nextPath));
    }
    return rows;
  }

  function renderTree() {
    return (
      <LoadedContent loading={treeResult.loading} error={treeResult.error}>
        <Table responsive hover className="list-table copyright-tree-table">
          <thead>
            <tr>
              <th>Copyright branch</th>
              <th>Videos in subtree</th>
              <th>Images in subtree</th>
              <th>Characters in subtree</th>
            </tr>
          </thead>
          <tbody>
            {treeRoots.flatMap((root) => renderTreeRows(root, 0, new Set()))}
          </tbody>
        </Table>
      </LoadedContent>
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
        zoomable={!treeMode}
      />
      <div className="d-flex align-items-center justify-content-between mb-2">
        <ButtonGroup size="sm">
          <Button
            variant={!treeMode ? "primary" : "secondary"}
            onClick={() => setTreeMode(false)}
          >
            Flat
          </Button>
          <Button
            variant={treeMode ? "primary" : "secondary"}
            onClick={() => setTreeMode(true)}
          >
            Tree
          </Button>
        </ButtonGroup>
        <span className="text-muted small">
          {treeMode
            ? "Tree counts include distinct media from all descendants."
            : "Flat counts show direct assignments."}
        </span>
      </div>

      {treeMode ? (
        renderTree()
      ) : (
        <>
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
            {filter.displayMode === DisplayMode.List
              ? renderList()
              : renderGrid()}
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
        </>
      )}
    </div>
  );
};

export default CopyrightList;
