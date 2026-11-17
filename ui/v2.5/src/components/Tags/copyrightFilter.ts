export interface TagWithParents {
  parents?: Array<{
    id: string;
    name?: string | null;
  }> | null;
}

// Legacy Tag-based Copyrights sit under a parent Tag named "Copyright".
// Native Copyrights are separate records and are not detected here.
export function isCopyrightTag(tag: TagWithParents) {
  return (tag.parents ?? []).some(
    (parent) => parent.name?.trim().toLocaleLowerCase() === "copyright"
  );
}
