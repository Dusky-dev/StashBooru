import { QueryResult } from "@apollo/client";
import { ListFilterModel } from "src/models/list-filter/filter";
import { useShowEditFilter } from "src/components/List/EditFilterDialog";
import { IHasID } from "src/utils/data";
import { useModal } from "src/hooks/modal";
import {
  IFilterStateHook,
  IQueryResultHook,
  useEnsureValidPage,
  useFilterOperations,
  useFilterState,
  useListKeyboardShortcuts,
  useListSelect,
  useQueryResult,
  useScrollToTopOnPageChange,
} from "./util";
import { useConfigurationContext } from "src/hooks/Config";

interface IFilteredItemList<
  T extends QueryResult,
  E extends IHasID = IHasID,
  M = unknown,
> {
  filterStateProps: IFilterStateHook;
  queryResultProps: IQueryResultHook<T, E, M>;
  initialSelected?: E[];
}

// Provides the common state and behaviour for filtered item list components
export function useFilteredItemList<
  T extends QueryResult,
  E extends IHasID = IHasID,
  M = unknown,
>(props: IFilteredItemList<T, E, M>) {
  const { configuration: config } = useConfigurationContext();

  // States
  const filterState = useFilterState({
    config,
    ...props.filterStateProps,
  });

  const { filter, setFilter } = filterState;

  const queryResult = useQueryResult({
    filter,
    ...props.queryResultProps,
  });
  const { result, items, totalCount, pages, metadataInfo } = queryResult;

  const listSelect = useListSelect(items, props.initialSelected);
  const { onSelectAll, onSelectNone, onInvertSelection } = listSelect;

  const modalState = useModal();
  const { showModal, closeModal } = modalState;

  // Utility hooks
  const { setPage } = useFilterOperations({ filter, setFilter });

  // scroll to the top of the page when the page changes
  useScrollToTopOnPageChange(filter.currentPage, result.loading);

  // Validate against this query's completed count, never the count cache
  // retained while a different filter or entity is still loading.
  const completedCount =
    !result.loading && !result.error && result.data
      ? props.queryResultProps.getCount(result)
      : undefined;
  useEnsureValidPage(filter, completedCount, setFilter);

  const showEditFilter = useShowEditFilter({
    showModal,
    closeModal,
    filter,
    setFilter,
  });

  useListKeyboardShortcuts({
    currentPage: filter.currentPage,
    onChangePage: setPage,
    onSelectAll,
    onSelectNone,
    onInvertSelection,
    pages,
    showEditFilter,
  });

  return {
    filterState,
    queryResult,
    metadataInfo,
    listSelect,
    modalState,
    showEditFilter,
  };
}

export const showWhenSelected = <T extends QueryResult>(
  _result: T,
  _filter: ListFilterModel,
  selectedIds: Set<string>
) => {
  return selectedIds.size > 0;
};

export const showWhenSingleSelection = <T extends QueryResult>(
  _result: T,
  _filter: ListFilterModel,
  selectedIds: Set<string>
) => {
  return selectedIds.size === 1;
};

export const showWhenNoneSelected = <T extends QueryResult>(
  _result: T,
  _filter: ListFilterModel,
  selectedIds: Set<string>
) => {
  return selectedIds.size === 0;
};
