import React from "react";
import { Card, Col, Row, Table } from "react-bootstrap";
import { Link, useHistory } from "react-router-dom";
import { useIntl } from "react-intl";

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

const zoomWidths = [250, 310, 390, 500];

function useFindCopyrightsForList(filter: ListFilterModel) {
  return GQL.useFindCopyrightsQuery({
    variables: { filter: filter.makeFindFilter() },
  });
}

const CopyrightList: React.FC = () => {
  const intl = useIntl();
  const history = useHistory();
  const view = View.Copyrights;

  const { filterState, queryResult, modalState, listSelect, showEditFilter } =
    useFilteredItemList({
      filterStateProps: {
        filterMode: GQL.FilterMode.Copyrights,
        view,
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

  function viewRandom() {
    if (copyrights.length === 0) return;
    const item = copyrights[Math.floor(Math.random() * copyrights.length)];
    history.push(`/copyrights/${item.id}`);
  }

  const operations = (
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
  );

  function renderGrid() {
    const zoomIndex = Math.max(
      0,
      Math.min(filter.zoomIndex, zoomWidths.length - 1)
    );
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
