from pathlib import Path

path = Path("ui/v2.5/src/components/Tags/Copyrights.tsx")
text = path.read_text()

text = text.replace(
    'import { CopyrightLink } from "src/components/Copyrights/CopyrightLink";\n',
    'import { CopyrightLink } from "src/components/Copyrights/CopyrightLink";\n'
    'import {\n'
    '  CopyrightBreadcrumb,\n'
    '  CopyrightChildrenOrderControl,\n'
    '  StructuralRoleControl,\n'
    '} from "src/components/Copyrights/CopyrightTaxonomyControls";\n',
)

text = text.replace(
    '    <div className="detail-group">\n      <DetailItem\n        id="sort_name"',
    '    <div className="detail-group">\n'
    '      <CopyrightBreadcrumb items={copyright.breadcrumb} />\n'
    '      <StructuralRoleControl\n'
    '        entityType="copyright"\n'
    '        entityID={copyright.id}\n'
    '        role={copyright.structural_role}\n'
    '      />\n'
    '      <DetailItem\n'
    '        id="subtree-media"\n'
    '        label="Taxonomy subtree"\n'
    '        value={`${copyright.subtree_image_count} images · ${copyright.subtree_scene_count} videos · ${copyright.subtree_performer_count} characters`}\n'
    '        fullWidth={fullWidth}\n'
    '      />\n'
    '      <DetailItem\n'
    '        id="sort_name"',
)

text = text.replace(
    '        value={renderRelations(copyright.children)}',
    '        value={renderRelations(copyright.ordered_children)}',
)

text = text.replace(
    '      <p className="text-muted small">\n        <FormattedMessage id="copyright_hierarchy.delete_help" />\n      </p>',
    '      <CopyrightChildrenOrderControl\n'
    '        parentID={copyright.id}\n'
    '        children={copyright.ordered_children}\n'
    '      />\n'
    '      <p className="text-muted small">\n'
    '        <FormattedMessage id="copyright_hierarchy.delete_help" />\n'
    '      </p>',
)

text = text.replace(
    '    copyright.image_count > 0\n      ? "images"\n      : copyright.scene_count > 0\n        ? "videos"',
    '    copyright.subtree_image_count > 0\n      ? "images"\n      : copyright.subtree_scene_count > 0\n        ? "videos"',
)

text = text.replace(
    '    () => copyright.scenes.map((scene) => Number(scene.id)),\n    [copyright.scenes]',
    '    () => copyright.subtree_scenes.map((scene) => Number(scene.id)),\n    [copyright.subtree_scenes]',
)

text = text.replace(
    '    () => copyright.performers.map((performer) => Number(performer.id)),\n    [copyright.performers]',
    '    () =>\n      copyright.subtree_performers.map((performer) => Number(performer.id)),\n    [copyright.subtree_performers]',
)

text = text.replace('count={copyright.image_count}', 'count={copyright.subtree_image_count}')
text = text.replace('count={copyright.scene_count}', 'count={copyright.subtree_scene_count}')
text = text.replace('count={copyright.performer_count}', 'count={copyright.subtree_performer_count}')

path.write_text(text)
