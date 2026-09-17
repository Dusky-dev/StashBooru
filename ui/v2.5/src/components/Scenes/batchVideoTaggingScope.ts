interface PaginatedFilter<T> {
  currentPage: number;
  itemsPerPage: number;
  clone: () => T;
}

interface ScenePage {
  data: {
    findScenes: {
      count: number;
      scenes: { id: string }[];
    };
  };
}

export async function collectFilteredSceneIDs<T extends PaginatedFilter<T>>(
  filter: T,
  queryPage: (filter: T) => Promise<ScenePage>,
  restrictedSceneIDs?: number[]
): Promise<string[]> {
  // An empty restricted list must never become a library-wide query.
  if (restrictedSceneIDs?.length === 0) return [];

  const restricted = restrictedSceneIDs
    ? new Set(restrictedSceneIDs.map(String))
    : undefined;
  const pageSize = 250;
  const ids = new Set<string>();
  let page = 1;
  let total = Number.POSITIVE_INFINITY;

  // Count queried pages, not accepted IDs: a restriction can reject a whole page.
  while ((page - 1) * pageSize < total) {
    const pageFilter = filter.clone();
    pageFilter.currentPage = page;
    pageFilter.itemsPerPage = pageSize;
    const result = await queryPage(pageFilter);
    const found = result.data.findScenes.scenes;
    total = result.data.findScenes.count;

    // GraphQL's scene_ids lookup bypasses filters, so intersect the filtered
    // results here rather than passing both arguments to that lookup.
    for (const scene of found) {
      if (!restricted || restricted.has(scene.id)) ids.add(scene.id);
    }

    if (found.length < pageSize || ids.size === restricted?.size) break;
    page += 1;
  }

  return [...ids];
}
