// Validate page numbers before passing them to the native query/pagination
// controls. parseInt would accept partial numbers ("2oops"), 0 and NaN.
export function parsePositiveListInteger(
  raw: string | null | undefined
): number | undefined {
  if (!raw || !/^\d+$/.test(raw)) return undefined;
  const parsed = Number(raw);
  return Number.isSafeInteger(parsed) && parsed > 0 ? parsed : undefined;
}

// The only valid page for an empty result set is page 1. Callers must pass a
// count from a *completed* query, not a stale cached count or loading state.
export function correctedListPage(
  currentPage: number,
  totalCount: number,
  itemsPerPage: number
): number | undefined {
  if (
    !Number.isFinite(totalCount) ||
    totalCount < 0 ||
    !Number.isSafeInteger(itemsPerPage) ||
    itemsPerPage < 1
  ) {
    return undefined;
  }

  const lastPage = Math.max(1, Math.ceil(totalCount / itemsPerPage));
  const nextPage = Number.isSafeInteger(currentPage)
    ? Math.min(Math.max(1, currentPage), lastPage)
    : 1;
  return nextPage === currentPage ? undefined : nextPage;
}
