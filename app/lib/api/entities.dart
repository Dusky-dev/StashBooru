import 'client.dart';
import 'models.dart';

/// How a browseable entity type is shown and how it filters media.
enum EntityType {
  character('Characters', 'Character', 1 / 1.5),
  artist('Artists', 'Artist', 1),
  copyright('Copyrights', 'Copyright', 16 / 9),
  tag('Tags', 'Tag', 4 / 3),
  gallery('Galleries', 'Gallery', 1),
  collection('Collections', 'Collection', 1 / 1.5);

  const EntityType(this.plural, this.singular, this.imageAspect);
  final String plural;
  final String singular;

  /// Card image width / height: Characters portrait, Artists square,
  /// Copyrights landscape.
  final double imageAspect;
}

class Entity {
  Entity({
    required this.type,
    required this.id,
    required this.name,
    this.imagePath,
    this.imageCount,
    this.videoCount,
    this.favorite = false,
  });

  final EntityType type;
  final String id;
  final String name;
  final String? imagePath;
  final int? imageCount;
  final int? videoCount;
  final bool favorite;

  String get countLabel {
    final parts = <String>[
      if (imageCount != null)
        '$imageCount ${imageCount == 1 ? 'image' : 'images'}',
      if (videoCount != null)
        '$videoCount ${videoCount == 1 ? 'video' : 'videos'}',
    ];
    return parts.join(' · ');
  }

  /// The media belonging to this entity.
  MediaScope get scope {
    final single = {
      'value': [id],
      'modifier': 'INCLUDES_ALL',
    };
    Map<String, dynamic> hierarchical(int depth) => {
      'value': [id],
      'modifier': 'INCLUDES',
      'depth': depth,
    };
    switch (type) {
      case EntityType.character:
        return MediaScope(
          image: {'performers': single},
          scene: {'performers': single},
          media: {'performers': single},
        );
      case EntityType.artist:
        final c = hierarchical(0);
        return MediaScope(
          image: {'studios': c},
          scene: {'studios': c},
          media: {'studios': c},
        );
      case EntityType.copyright:
        // -1 includes every descendant Copyright, like the web UI's subtree.
        final c = hierarchical(-1);
        return MediaScope(
          image: {'copyrights': c},
          scene: {'copyrights': c},
          media: {'copyrights': c},
        );
      case EntityType.tag:
        final c = hierarchical(0);
        return MediaScope(
          image: {'tags': c},
          scene: {'tags': c},
          media: {'tags': c},
        );
      case EntityType.gallery:
        return MediaScope(
          image: {
            'galleries': {
              'value': [id],
              'modifier': 'INCLUDES',
            },
          },
        );
      case EntityType.collection:
        return MediaScope(scene: {'groups': hierarchical(0)});
    }
  }
}

enum EntitySort {
  name('Name', 'name', 'ASC'),
  newest('Newest', 'created_at', 'DESC'),
  random('Random', 'random', 'ASC');

  const EntitySort(this.label, this.field, this.direction);
  final String label;
  final String field;
  final String direction;
}

class _Spec {
  const _Spec(this.query, this.resultKey, this.listKey, this.fields);
  final String query;
  final String resultKey;
  final String listKey;
  final String fields;
}

const _specs = {
  EntityType.character: _Spec(
    'findPerformers',
    'FindPerformersResultType',
    'performers',
    'id name image_path image_count scene_count favorite',
  ),
  EntityType.artist: _Spec(
    'findStudios',
    'FindStudiosResultType',
    'studios',
    'id name image_path image_count scene_count favorite',
  ),
  // Counts include descendant Copyrights, matching the media a card opens.
  EntityType.copyright: _Spec(
    'findCopyrights',
    'FindCopyrightsResultType',
    'copyrights',
    'id name image_path favorite '
        'image_count: subtree_image_count scene_count: subtree_scene_count',
  ),
  EntityType.tag: _Spec(
    'findTags',
    'FindTagsResultType',
    'tags',
    'id name image_path image_count scene_count favorite',
  ),
  EntityType.gallery: _Spec(
    'findGalleries',
    'FindGalleriesResultType',
    'galleries',
    'id title image_count paths { cover } folder { path } files { path }',
  ),
  EntityType.collection: _Spec(
    'findGroups',
    'FindGroupsResultType',
    'groups',
    'id name front_image_path scene_count',
  ),
};

String _basename(String path) {
  final parts = path.split(RegExp(r'[\\/]'));
  return parts.isEmpty ? path : parts.last;
}

Entity entityFromJson(
  EntityType type,
  Map<String, dynamic> json,
  StashClient client,
) {
  String? url(Object? v) =>
      v is String && v.isNotEmpty ? client.resolve(v) : null;
  switch (type) {
    case EntityType.gallery:
      var name = (json['title'] as String?)?.trim() ?? '';
      if (name.isEmpty) {
        final folder = (json['folder'] as Map?)?['path'] as String?;
        final files = (json['files'] as List?) ?? const [];
        final path =
            folder ??
            (files.isEmpty ? null : (files.first as Map)['path'] as String?);
        name = path == null ? 'Gallery ${json['id']}' : _basename(path);
      }
      return Entity(
        type: type,
        id: json['id'] as String,
        name: name,
        imagePath: url((json['paths'] as Map?)?['cover']),
        imageCount: json['image_count'] as int?,
      );
    case EntityType.collection:
      return Entity(
        type: type,
        id: json['id'] as String,
        name: json['name'] as String,
        imagePath: url(json['front_image_path']),
        videoCount: json['scene_count'] as int?,
      );
    default:
      return Entity(
        type: type,
        id: json['id'] as String,
        name: json['name'] as String,
        imagePath: url(json['image_path']),
        imageCount: json['image_count'] as int?,
        videoCount: json['scene_count'] as int?,
        favorite: json['favorite'] == true,
      );
  }
}

Future<PageResult<Entity>> fetchEntities(
  StashClient client,
  EntityType type, {
  required int page,
  required int perPage,
  String query = '',
  EntitySort sort = EntitySort.name,
  int seed = 0,
}) async {
  final spec = _specs[type]!;
  // Galleries have no name; sort them by title instead.
  var field = sort == EntitySort.random ? 'random_$seed' : sort.field;
  if (type == EntityType.gallery && field == 'name') field = 'title';
  final data = await client.query(
    '''
    query Entities(\$filter: FindFilterType) {
      result: ${spec.query}(filter: \$filter) {
        count
        ${spec.listKey} { ${spec.fields} }
      }
    }
  ''',
    {
      'filter': findFilter(
        page: page,
        perPage: perPage,
        query: query,
        sort: field,
        direction: sort.direction,
      ),
    },
  );
  final result = data['result'] as Map<String, dynamic>;
  return PageResult([
    for (final e in result[spec.listKey] as List)
      entityFromJson(type, e as Map<String, dynamic>, client),
  ], result['count'] as int);
}

/// Everything shown in a media item's information panel.
class MediaDetail {
  MediaDetail({
    required this.title,
    this.details,
    this.date,
    this.path,
    this.resolution,
    this.links = const {},
  });

  final String title;
  final String? details;
  final String? date;
  final String? path;
  final String? resolution;

  /// Related entities grouped by type, in navbar order.
  final Map<EntityType, List<Entity>> links;
}

Future<MediaDetail> fetchDetail(
  StashClient client,
  ServerCapabilities caps,
  MediaRef ref,
) async {
  final copyrightField = caps.copyrights
      ? 'copyrights { id name image_path }'
      : '';
  const related = '''
    studio { id name image_path }
    performers { id name image_path }
    tags { id name image_path }
  ''';
  final Map<String, dynamic> json;
  String? path;
  if (ref.kind == MediaKind.image) {
    final data = await client.query(
      '''
      query Image(\$id: ID!) {
        findImage(id: \$id) {
          title details date
          $related
          $copyrightField
          galleries { id title }
          visual_files {
            ... on ImageFile { path }
            ... on VideoFile { path }
          }
        }
      }
    ''',
      {'id': ref.id},
    );
    json =
        (data['findImage'] as Map<String, dynamic>?) ??
        (throw StashException('This image no longer exists.'));
    final files = (json['visual_files'] as List?) ?? const [];
    if (files.isNotEmpty) path = (files.first as Map)['path'] as String?;
  } else {
    final data = await client.query(
      '''
      query Video(\$id: ID!) {
        findScene(id: \$id) {
          title details date
          $related
          $copyrightField
          groups { group { id name front_image_path } }
          files { path }
        }
      }
    ''',
      {'id': ref.id},
    );
    json =
        (data['findScene'] as Map<String, dynamic>?) ??
        (throw StashException('This video no longer exists.'));
    final files = (json['files'] as List?) ?? const [];
    if (files.isNotEmpty) path = (files.first as Map)['path'] as String?;
  }

  List<Entity> list(EntityType type, Object? raw) => [
    for (final e in (raw as List?) ?? const [])
      entityFromJson(type, e as Map<String, dynamic>, client),
  ];

  final studio = json['studio'];
  final links = <EntityType, List<Entity>>{
    EntityType.character: list(EntityType.character, json['performers']),
    EntityType.artist: studio == null
        ? []
        : [
            entityFromJson(
              EntityType.artist,
              studio as Map<String, dynamic>,
              client,
            ),
          ],
    if (caps.copyrights)
      EntityType.copyright: list(EntityType.copyright, json['copyrights']),
    EntityType.tag: list(EntityType.tag, json['tags']),
    if (ref.kind == MediaKind.image)
      EntityType.gallery: list(EntityType.gallery, json['galleries']),
    if (ref.kind == MediaKind.video)
      EntityType.collection: [
        for (final g in (json['groups'] as List?) ?? const [])
          entityFromJson(
            EntityType.collection,
            (g as Map)['group'] as Map<String, dynamic>,
            client,
          ),
      ],
  }..removeWhere((_, v) => v.isEmpty);

  final details = (json['details'] as String?)?.trim();
  return MediaDetail(
    title: ref.title,
    details: details?.isEmpty == true ? null : details,
    date: json['date'] as String?,
    path: path,
    resolution: ref.width != null && ref.height != null
        ? '${ref.width} × ${ref.height}'
        : null,
    links: links,
  );
}
