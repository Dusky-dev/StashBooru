from pathlib import Path


def replace_once(path: str, old: str, new: str) -> None:
    p = Path(path)
    text = p.read_text()
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"{path}: expected one anchor, found {count}: {old[:100]!r}")
    p.write_text(text.replace(old, new, 1))


replace_once(
    "ui/v2.5/src/models/list-filter/criteria/criterion.ts",
    '  | "folders"\n  | undefined;',
    '  | "folders"\n  | "copyrights"\n  | undefined;',
)

replace_once(
    "ui/v2.5/src/models/list-filter/types.ts",
    '  | "folder"\n  | "parent_folder";',
    '  | "folder"\n  | "parent_folder"\n  | "copyrights";',
)

replace_once(
    "ui/v2.5/src/components/List/Filters/HierarchicalLabelValueFilter.tsx",
    '    inputType !== "performer_tags" &&\n    inputType !== "groups"',
    '    inputType !== "performer_tags" &&\n    inputType !== "groups" &&\n    inputType !== "copyrights"',
)
replace_once(
    "ui/v2.5/src/components/List/Filters/HierarchicalLabelValueFilter.tsx",
    '    if (inputType === "studios") {\n      return "include-sub-studios";\n    }\n    if (inputType === "groups") {',
    '    if (inputType === "studios") {\n      return "include-sub-studios";\n    }\n    if (inputType === "copyrights") {\n      return "include-sub-copyrights";\n    }\n    if (inputType === "groups") {',
)
replace_once(
    "ui/v2.5/src/components/List/Filters/HierarchicalLabelValueFilter.tsx",
    '    if (inputType === "studios") {\n      id = "include_sub_studios";\n    } else if (inputType === "groups") {',
    '    if (inputType === "studios") {\n      id = "include_sub_studios";\n    } else if (inputType === "copyrights") {\n      return {\n        id: "include_sub_copyrights",\n        defaultMessage: "Include descendant Copyrights",\n      };\n    } else if (inputType === "groups") {',
)

replace_once(
    "ui/v2.5/src/components/Shared/Select.tsx",
    'import { SceneIDSelect } from "../Scenes/SceneSelect";\n',
    'import { SceneIDSelect } from "../Scenes/SceneSelect";\nimport { CopyrightIDSelect } from "../Copyrights/CopyrightSelect";\n',
)
replace_once(
    "ui/v2.5/src/components/Shared/Select.tsx",
    '    | "groups"\n    | "galleries";',
    '    | "groups"\n    | "galleries"\n    | "copyrights";',
)
replace_once(
    "ui/v2.5/src/components/Shared/Select.tsx",
    '    case "galleries":\n      return <GallerySelect {...props} creatable={false} />;\n    default:',
    '    case "galleries":\n      return <GallerySelect {...props} creatable={false} />;\n    case "copyrights":\n      return <CopyrightIDSelect {...props} creatable={false} />;\n    default:',
)

replace_once(
    "ui/v2.5/src/models/list-filter/images.ts",
    'import { FolderCriterionOption } from "./criteria/folder";\n',
    'import { FolderCriterionOption } from "./criteria/folder";\nimport { CopyrightsCriterionOption } from "./criteria/copyrights";\n',
)
replace_once(
    "ui/v2.5/src/models/list-filter/images.ts",
    '  TagsCriterionOption,\n  RatingCriterionOption,',
    '  TagsCriterionOption,\n  CopyrightsCriterionOption,\n  RatingCriterionOption,',
)

replace_once(
    "ui/v2.5/src/models/list-filter/scenes.ts",
    'import { FolderCriterionOption } from "./criteria/folder";\n',
    'import { FolderCriterionOption } from "./criteria/folder";\nimport { CopyrightsCriterionOption } from "./criteria/copyrights";\n',
)
replace_once(
    "ui/v2.5/src/models/list-filter/scenes.ts",
    '  TagsCriterionOption,\n  createMandatoryNumberCriterionOption("tag_count"),',
    '  TagsCriterionOption,\n  CopyrightsCriterionOption,\n  createMandatoryNumberCriterionOption("tag_count"),',
)
