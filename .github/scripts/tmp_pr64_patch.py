from pathlib import Path
import hashlib
import re

scene = Path("ui/v2.5/src/components/Scenes/SceneList.tsx")
text = scene.read_text()
marker = '''const ScenesFilterSidebarSections = PatchContainerComponent(
  "FilteredSceneList.SidebarSections"
);'''
wrapper = '''const ScenesFilterSidebarSections = PatchContainerComponent(
  "FilteredSceneList.SidebarSections"
);

// Expose the scene toolbar as a narrow patch point. Plugins that only need to
// adapt toolbar props should not have to wrap or walk the full FilteredSceneList
// render tree.
const FilteredSceneListToolbar = PatchComponent(
  "FilteredSceneList.Toolbar",
  FilteredListToolbar
);'''
if "FilteredSceneList.Toolbar" not in text:
    if marker not in text:
        raise SystemExit("SceneList toolbar insertion marker not found")
    text = text.replace(marker, wrapper, 1)
if "<FilteredSceneListToolbar" not in text:
    old = "<FilteredListToolbar\n                  filter={filter}"
    new = "<FilteredSceneListToolbar\n                  filter={filter}"
    if old not in text:
        raise SystemExit("FilteredListToolbar render call not found")
    text = text.replace(old, new, 1)
scene.write_text(text)

source = Path("ui/v2.5/builtin-source/unifiedMedia.05.part")
text = source.read_text()

toolbar_patch = '''  // Patch only the dedicated native scene-toolbar wrapper. This keeps the
  // ordinary /scenes page on Stash's native FilteredSceneList render path
  // and avoids recursively walking arbitrary React children.
  api.patch.after("FilteredSceneList.Toolbar", function () {
    const args = Array.from(arguments);
    const props = args[0] || {};
    const rendered = args[args.length - 1];
    return React.createElement(FilteredSceneListToolbarBridge, { props, rendered });
  });

  function FilteredSceneListToolbarBridge({ props, rendered }) {
    const ctx = React.useContext(MixedContext);
    if (!ctx || !React.isValidElement(rendered)) return rendered;

    const selectedIds = new Set(ctx.selectedKeys);
    const listSelect = {
      ...(props.listSelect || {}),
      selectedIds,
      selectedItems: [],
      hasSelection: selectedIds.size > 0,
      onSelectAll: ctx.selectAll,
      onSelectNone: ctx.clearSelection,
      onInvertSelection: ctx.invertSelection,
    };

    return React.cloneElement(rendered, {
      filter: makeGridWallToolbarFilter(props.filter),
      listSelect,
      operationComponent: makeSafeOperations(props.operationComponent, ctx),
      onDelete: undefined,
      onEdit: undefined,
    });
  }

'''
text, count = re.subn(
    r'''  // The native FilteredSceneList already builds Stash's real toolbar\..*?(?=  function makeGridWallToolbarFilter)''',
    toolbar_patch,
    text,
    count=1,
    flags=re.S,
)
if count != 1:
    raise SystemExit(f"old FilteredSceneList patch replacement count={count}")

text, count = re.subn(
    r'''  function isOpaqueChildPayload\(value\) \{.*?(?=  function UnifiedController)''',
    "",
    text,
    count=1,
    flags=re.S,
)
if count != 1:
    raise SystemExit(f"old tree bridge removal count={count}")
source.write_text(text)

root = Path("ui/v2.5")
source_dir = root / "builtin-source"
parts = [source_dir / f"unifiedMedia.{i:02d}.part" for i in range(7)]
extensions = [
    source_dir / "unifiedMedia.copyright.part",
    source_dir / "unifiedMedia.copyright.guard.part",
    source_dir / "unifiedMedia.visual-only.part",
    source_dir / "unifiedMedia.copyright-native-all.part",
]
close = b"})();\n"
core = b"".join(p.read_bytes() for p in parts)
if not core.endswith(close):
    raise SystemExit("Unified Media core no longer ends at expected IIFE boundary")
unified = core[:-len(close)] + b"".join(p.read_bytes() for p in extensions) + close
digest = hashlib.sha256(unified).hexdigest()

prep = root / "scripts/prepare-builtins.mjs"
prep_text = prep.read_text()
prep_text, count = re.subn(
    r'(?<=const expectedUnifiedSha256 =\n  ")[0-9a-f]{64}(?=";)',
    digest,
    prep_text,
    count=1,
)
if count != 1:
    raise SystemExit(f"checksum replacement count={count}")
prep.write_text(prep_text)
print("new Unified Media sha256:", digest)
