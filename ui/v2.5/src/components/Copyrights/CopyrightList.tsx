import React, { useMemo, useState } from "react";
import {
  ButtonToolbar,
  Card,
  Col,
  Row,
  Spinner,
  Table,
} from "react-bootstrap";
import { Link, useHistory, useLocation } from "react-router-dom";
import { useIntl } from "react-intl";

import * as GQL from "src/core/generated-graphql";
import { ListFilterModel } from "src/models/list-filter/filter";
import { DisplayMode } from "src/models/list-filter/types";
import { PageSizeSelector, SearchTermInput } from "src/components/List/ListFilter";
import { SortBySelect } from "src/components/List/SortBySelect";
import { ListViewButtonGroup } from "src/components/List/ListViewOptions";
import { ListOperations } from "src/components/List/ListOperationButtons";
import { Pagination, PaginationIndex } from "src/components/List/Pagination";
import { TruncatedText } from "src/components/Shared/TruncatedText";

const copyrightDisplayModes = [DisplayMode.Grid, DisplayMode.List];
const copyrightSorts = new Set(["name", "created_at", "updated_at"]);
const zoomWidths = [250, 310, 390, 500];

function makeInitialFilter(search: string) {
  const filter = new ListFilterModel(GQL.FilterMode.Tags, undefined, {
    defaultSortBy: "name",
    defaultSortDir: GQL.SortDirectionEnum.Asc,
  });
  filter.configureFromQueryString(search);

  if (!filter.sortBy || !copyrightSorts.has(filter.sortBy)) {
    filter.sortBy = "name";
    filter.sortDirection = GQL.SortDirectionEnum.Asc;
  }
  if (!copyrightDisplayModes.includes(filter.displayMode)) {
    filter.displayMode = DisplayMode.Grid;
  }

  return filter;
}

const CopyrightList: React.FC = () => {
  const intl = useIntl();
  const history = useHistory();
  const location = useLocation();
  const [filter, setFilterState] = useState(() =>
    makeInitialFilter(location.search)
  );

  const sortOptions = useMemo(
    () =>
      filter.options.sortByOptions.filter((option) =>
        copyrightSorts.has(option.value)
      ),
    [filter.options.sortByOptions]
  );

  const { data, loading, error } = GQL.useFindCopyrightsQuery({
    variables: { filter: filter.makeFindFilter() },
  });

  const copyrights = data?.findCopyrights.copyrights ?? [];
  const totalCount = data?.findCopyrights.count ?? 0;

  function setFilter(next: ListFilterModel) {
    setFilterState(next);
    const query = next.makeQueryParameters();
    history.replace({
      pathname: location.pathname,
      search: query ? `?${query}` : "",
    });
  }

  function setPage(page: number) {
    const next = filter.clone();
    next.currentPage = page;
    setFilter(next);
  }

  function setDisplayMode(displayMode: DisplayMode) {
    const next = filter.clone();
    next.displayMode = displayMode;
    setFilter(next);
  }

  function setZoom(zoomIndex: number) {
    const next = filter.clone();
    next.zoomIndex = zoomIndex;
    setFilter(next);
  }

  function viewRandom() {
    if (copyrights.length === 0) return;
    const item = copyrights[Math.floor(Math.random() * copyrights.length)];
    history.push(`/copyrights/${item.id}`);
  }

  function renderGrid() {
    const zoomIndex = Math.max(0, Math.min(filter.zoomIndex, zoomWidths.length - 1));
    const cardWidth = zoomWidths[zoomIndex];

    return (
      <Row className="justify-content-center">
        {copyrights.map((copyright) => (
          <Col
            key={copyright.id}
            className="mb-3"
            style={{ flex: `0 0 ${cardWidth}px`, maxWidth: `${cardWidth}px` }}
          >
            <Card
              as={Link}
              to={`/copyrights/${copyright.id}`}
              className={`tag-card zoom-${zoomIndex} h-100 text-reset text-decoration-none`}
            >
              <img
                className="tag-card-image"
                src={copyright.image_path}
                alt={copyright.name}
                loading="lazy"
              />
              <Card.Body>
                <Card.Title>{copyright.name}</Card.Title>
                {copyright.description ? (
                  <TruncatedText
                    className="tag-description"
                    text={copyright.description}
                    lineCount={3}
                  />
                ) : null}
                <small className="text-muted d-block mt-2">
                  {copyright.scene_count} Videos · {copyright.image_count} Images ·{" "}
                  {copyright.performer_count} Characters
                </small>
              </Card.Body>
            </Card>
          </Col>
        ))}
      </Row>
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

  return (
    <div className="item-list-container copyright-list">
      <ButtonToolbar className="filtered-list-toolbar">
        <SearchTermInput filter={filter} onFilterUpdate={setFilter} />
        <SortBySelect
          sortBy={filter.sortBy}
          sortDirection={filter.sortDirection}
          options={sortOptions}
          onChangeSortBy={(sortBy) => setFilter(filter.setSortBy(sortBy ?? undefined))}
          onChangeSortDirection={() => setFilter(filter.toggleSortDirection())}
          onReshuffleRandomSort={() => {}}
        />
        <PageSizeSelector
          pageSize={filter.itemsPerPage}
          setPageSize={(pageSize) => setFilter(filter.setPageSize(pageSize))}
        />
        <ListOperations
          items={copyrights.length}
          operations={[
            {
              text: intl.formatMessage({ id: "actions.view_random" }),
              onClick: viewRandom,
              isDisplayed: () => totalCount > 0,
            },
          ]}
        />
        <ListViewButtonGroup
          displayMode={filter.displayMode}
          displayModeOptions={copyrightDisplayModes}
          onSetDisplayMode={setDisplayMode}
          zoomIndex={filter.zoomIndex}
          onSetZoom={setZoom}
          preferenceKey="copyrights"
        />
      </ButtonToolbar>

      <div className="pagination-index-container">
        <Pagination
          currentPage={filter.currentPage}
          itemsPerPage={filter.itemsPerPage}
          totalItems={totalCount}
          onChangePage={setPage}
        />
        <PaginationIndex
          loading={loading}
          currentPage={filter.currentPage}
          itemsPerPage={filter.itemsPerPage}
          totalItems={totalCount}
        />
      </div>

      {error ? <div className="alert alert-danger">{error.message}</div> : null}
      {loading ? (
        <Spinner animation="border" />
      ) : filter.displayMode === DisplayMode.List ? (
        renderList()
      ) : (
        renderGrid()
      )}
    </div>
  );
};

export default CopyrightList;
