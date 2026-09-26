type FileFilterInput = Record<string, unknown>;

export function imageFormatFileFilterInput(format: unknown): FileFilterInput {
  return {
    video_file_filter: { format },
    OR: { image_file_filter: { format } },
  };
}

export function sceneFormatFileFilterInput(format: unknown): FileFilterInput {
  return {
    video_file_filter: { format },
  };
}

const filterOperators = ["AND", "OR", "NOT"] as const;

function hasOperator(filter: FileFilterInput): boolean {
  return filterOperators.some((operator) => filter[operator] !== undefined);
}

export function addFileFilterInput(
  input: Record<string, unknown>,
  filter: FileFilterInput
) {
  const existing = input.files_filter as FileFilterInput | undefined;

  if (!existing) {
    input.files_filter = filter;
    return;
  }

  const existingIsComposite = hasOperator(existing);
  const filterIsComposite = hasOperator(filter);

  if (existingIsComposite && filterIsComposite) {
    throw new Error("Cannot combine multiple composite file filters");
  }

  if (existingIsComposite) {
    input.files_filter = { ...filter, AND: existing };
  } else if (filterIsComposite) {
    input.files_filter = { ...existing, AND: filter };
  } else {
    input.files_filter = { ...existing, ...filter };
  }
}
