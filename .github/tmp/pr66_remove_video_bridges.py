from pathlib import Path
import hashlib
import re

scene_list = Path("ui/v2.5/src/components/Scenes/SceneList.tsx")
text = scene_list.read_text()

# Remove the PR #65 patch wrapper; Unified Media will use explicit optional props instead.
text, count = re.subn(
    r'\n// Expose the scene toolbar as a narrow patch point\..*?const FilteredSceneListToolbar = PatchComponent\(\n  "FilteredSceneList\.Toolbar",\n  FilteredListToolbar\n\);\n',
    "\n",
    text,
    count=1,
    flags=re.S,
)
if count != 1:
    raise SystemExit(f"toolbar wrapper removal count={count}")

# Add extension prop types and fields.
marker = 'interface IFilteredScenes {\n'
if marker not in text:
    raise SystemExit("IFilteredScenes marker missing")
text = text.replace(
    marker,
    'type FilteredSceneListBodyProps = React.ComponentProps<typeof SceneList>;\n'
    'type FilteredSceneListToolbarProps = React.ComponentProps<typeof FilteredListToolbar>;\n\n'
    + marker,
    1,
)
text = text.replace(
    '  sceneIDs?: number[];\n}\n\nexport const FilteredSceneList',
    '  sceneIDs?: number[];\n'
    '  toolbarPropsAdapter?: (\n'
    '    props: FilteredSceneListToolbarProps\n'
    '  ) => FilteredSceneListToolbarProps;\n'
    '  renderList?: (props: FilteredSceneListBodyProps) => React.ReactNode;\n'
    '}\n\nexport const FilteredSceneList',
    1,
)

# Build the normal native toolbar/list props once, then optionally adapt/render them.
render_marker = '    // render\n    if (sidebarStateLoading) return null;\n\n'
if render_marker not in text:
    raise SystemExit("render marker missing")
injected = '''    // render\n    if (sidebarStateLoading) return null;\n\n    const toolbarProps: FilteredSceneListToolbarProps = {\n      filter,\n      listSelect,\n      setFilter,\n      showEditFilter,\n      onDelete,\n      onEdit,\n      operationComponent: operations,\n      view,\n      zoomable: true,\n    };\n    const adaptedToolbarProps = props.toolbarPropsAdapter\n      ? props.toolbarPropsAdapter(toolbarProps)\n      : toolbarProps;\n    const listProps: FilteredSceneListBodyProps = {\n      filter: effectiveFilter,\n      scenes: items,\n      selectedIds,\n      onSelectChange,\n      fromGroupId,\n      sceneIDs,\n    };\n\n'''
# operations is declared immediately after the render marker in current source, so move the injected block after operations instead.
# First restore original marker and insert only after the operations block.
ops_end = '''    const operations = (\n      <ListOperations\n        items={items.length}\n        hasSelection={hasSelection}\n        operations={otherOperations}\n        onEdit={onEdit}\n        onDelete={onDelete}\n        onPlay={onPlay}\n        operationsMenuClassName="scene-list-operations-dropdown"\n      />\n    );\n\n'''
if ops_end not in text:
    raise SystemExit("operations block missing")
text = text.replace(ops_end, ops_end + injected.split(render_marker, 1)[1], 1)

old_toolbar = '''                <FilteredSceneListToolbar\n                  filter={filter}\n                  listSelect={listSelect}\n                  setFilter={setFilter}\n                  showEditFilter={showEditFilter}\n                  onDelete={onDelete}\n                  onEdit={onEdit}\n                  operationComponent={operations}\n                  view={view}\n                  zoomable\n                />'''
new_toolbar = '''                <FilteredListToolbar {...adaptedToolbarProps} />'''
if old_toolbar not in text:
    raise SystemExit("toolbar JSX missing")
text = text.replace(old_toolbar, new_toolbar, 1)

old_list = '''                  <SceneList\n                    filter={effectiveFilter}\n                    scenes={items}\n                    selectedIds={selectedIds}\n                    onSelectChange={onSelectChange}\n                    fromGroupId={fromGroupId}\n                    sceneIDs={sceneIDs}\n                  />'''
new_list = '''                  {props.renderList ? (\n                    props.renderList(listProps)\n                  ) : (\n                    <SceneList {...listProps} />\n                  )}'''
if old_list not in text:
    raise SystemExit("SceneList JSX missing")
text = text.replace(old_list, new_list, 1)
scene_list.write_text(text)

# Remove the global SceneCard.Image bridge entirely. It is unsafe on native /scenes.
source0 = Path("ui/v2.5/builtin-source/unifiedMedia.00.part")
text = source0.read_text()
text, count = re.subn(
    r'\n\n  // Scene cards do not expose the ImageCard-style preview callback\..*?(?=\n  // Stash treats animated GIF images)',
    "\n",
    text,
    count=1,
    flags=re.S,
)
if count != 1:
    raise SystemExit(f"SceneCard.Image bridge removal count={count}")
source0.write_text(text)

# Remove SceneList + toolbar global bridges and switch UnifiedController to explicit props.
source5 = Path("ui/v2.5/builtin-source/unifiedMedia.05.part")
text = source5.read_text()
text, count = re.subn(
    r'\n  // SceneList is the list body produced by our real native FilteredSceneList.*?(?=\n  function makeGridWallToolbarFilter)',
    "\n",
    text,
    count=1,
    flags=re.S,
)
if count != 1:
    raise SystemExit(f"SceneList/toolbar bridge removal count={count}")

old_props = '''          React.createElement(FilteredSceneList, {\n            filterHook,\n            defaultSort: "date",\n            alterQuery: false,\n            view,\n          })'''
new_props = '''          React.createElement(FilteredSceneList, {\n            filterHook,\n            defaultSort: "date",\n            alterQuery: false,\n            view,\n            toolbarPropsAdapter: (toolbarProps) => {\n              const selectedIds = new Set(mixedContextValue.selectedKeys);\n              return {\n                ...toolbarProps,\n                filter: makeGridWallToolbarFilter(toolbarProps.filter),\n                listSelect: {\n                  ...(toolbarProps.listSelect || {}),\n                  selectedIds,\n                  selectedItems: [],\n                  hasSelection: selectedIds.size > 0,\n                  onSelectAll: mixedContextValue.selectAll,\n                  onSelectNone: mixedContextValue.clearSelection,\n                  onInvertSelection: mixedContextValue.invertSelection,\n                },\n                operationComponent: makeSafeOperations(\n                  toolbarProps.operationComponent,\n                  mixedContextValue\n                ),\n                onDelete: undefined,\n                onEdit: undefined,\n              };\n            },\n            renderList: (listProps) =>\n              React.createElement(MixedGrid, {\n                ctx: mixedContextValue,\n                filter: listProps.filter,\n              }),\n          })'''
if old_props not in text:
    raise SystemExit("FilteredSceneList controller props block missing")
text = text.replace(old_props, new_props, 1)
source5.write_text(text)

# Recompute checksum for assembled Unified Media built-in.
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
