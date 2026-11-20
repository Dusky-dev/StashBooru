import 'dart:math';

import 'client.dart';

enum MediaKind { image, video }

/// One Image or Video card. [key] keeps typed identity (`image:12` versus
/// `scene:12`) so equal numeric IDs never collide.
class MediaRef {
  MediaRef({
    required this.kind,
    required this.id,
    required this.title,
    required this.thumbnail,
    required this.source,
    this.playsAsVideo = false,
    this.width,
    this.height,
    this.duration,
  });

  final MediaKind kind;
  final String id;
  final String title;
  final String? thumbnail;

  /// Full-size image or the video stream.
  final String? source;

  /// True for Videos and for Images backed by a video file (animated clips).
  final bool playsAsVideo;
  final int? width;
  final int? height;
  final double? duration;

  String get key => '${kind == MediaKind.image ? 'image' : 'scene'}:$id';

  static String _basename(String? path) {
    if (path == null || path.isEmpty) return '';
    final parts = path.split(RegExp(r'[\\/]'));
    return parts.isEmpty ? path : parts.last;
  }

  static MediaRef fromImage(Map<String, dynamic> json, StashClient client) {
    final paths = (json['paths'] as Map?) ?? const {};
    final files = (json['visual_files'] as List?) ?? const [];
    final file = files.isEmpty ? null : files.first as Map;
    final isVideoFile = file?['__typename'] == 'VideoFile';
    final title = (json['title'] as String?)?.trim();
    return MediaRef(
      kind: MediaKind.image,
      id: json['id'] as String,
      title: title?.isNotEmpty == true
          ? title!
          : _basename(file?['path'] as String?),
      thumbnail: _url(client, paths['thumbnail']),
      source: _url(client, paths['image']),
      playsAsVideo: isVideoFile,
      width: (file?['width'] as num?)?.toInt(),
      height: (file?['height'] as num?)?.toInt(),
      duration: (file?['duration'] as num?)?.toDouble(),
    );
  }

  static MediaRef fromScene(Map<String, dynamic> json, StashClient client) {
    final paths = (json['paths'] as Map?) ?? const {};
    final files = (json['files'] as List?) ?? const [];
    final file = files.isEmpty ? null : files.first as Map;
    final title = (json['title'] as String?)?.trim();
    return MediaRef(
      kind: MediaKind.video,
      id: json['id'] as String,
      title: title?.isNotEmpty == true
          ? title!
          : _basename(file?['path'] as String?),
      thumbnail: _url(client, paths['screenshot']),
      source: _url(client, paths['stream']),
      playsAsVideo: true,
      width: (file?['width'] as num?)?.toInt(),
      height: (file?['height'] as num?)?.toInt(),
      duration: (file?['duration'] as num?)?.toDouble(),
    );
  }

  static String? _url(StashClient client, Object? value) =>
      value is String && value.isNotEmpty ? client.resolve(value) : null;
}

class PageResult<T> {
  PageResult(this.items, this.total);
  final List<T> items;
  final int total;
}

enum SortOption {
  newest('Newest', 'created_at', 'DESC'),
  oldest('Oldest', 'created_at', 'ASC'),
  date('Date', 'date', 'DESC'),
  title('Title', 'title', 'ASC'),
  rating('Rating', 'rating', 'DESC'),
  random('Random', 'random', 'ASC');

  const SortOption(this.label, this.field, this.direction);
  final String label;
  final String field;
  final String direction;
}

Map<String, dynamic> findFilter({
  required int page,
  required int perPage,
  String query = '',
  required String sort,
  required String direction,
}) => {
  'page': page,
  'per_page': perPage,
  'sort': sort,
  'direction': direction,
  if (query.trim().isNotEmpty) 'q': query.trim(),
};

// ---------------------------------------------------------------------------
// Media queries. Fragments only use fields stock Stash also has; Copyright
// fields are requested only when the server reports them.

const _imageFields = '''
  id title
  paths { thumbnail image }
  visual_files {
    __typename
    ... on ImageFile { path width height }
    ... on VideoFile { path width height duration }
  }
''';

const _sceneFields = '''
  id title
  paths { screenshot stream }
  files { path width height duration }
''';

/// Restricts a media list to one Character, Artist, Tag and so on.
class MediaScope {
  const MediaScope({this.image, this.scene, this.media});

  /// `ImageFilterType` entries, or null when Images do not apply.
  final Map<String, dynamic>? image;
  final Map<String, dynamic>? scene;

  /// `MediaFilterType` entries, or null when All cannot express this scope.
  final Map<String, dynamic>? media;

  static const everything = MediaScope(image: {}, scene: {}, media: {});
}

abstract class MediaSource {
  MediaSource(this.client);
  final StashClient client;
  String get label;
  List<SortOption> get sorts;
  Future<PageResult<MediaRef>> fetch(
    int page,
    int perPage,
    String query,
    SortOption sort,
    int seed,
  );

  String sortField(SortOption sort, int seed) =>
      sort == SortOption.random ? 'random_$seed' : sort.field;
}

class ImageSource extends MediaSource {
  ImageSource(super.client, [this.filter = const {}]);
  final Map<String, dynamic> filter;
  @override
  String get label => 'Images';
  @override
  List<SortOption> get sorts => SortOption.values;

  @override
  Future<PageResult<MediaRef>> fetch(
    int page,
    int perPage,
    String query,
    SortOption sort,
    int seed,
  ) async {
    final data = await client.query(
      '''
      query Images(\$filter: FindFilterType, \$image_filter: ImageFilterType) {
        findImages(filter: \$filter, image_filter: \$image_filter) {
          count
          images { $_imageFields }
        }
      }
    ''',
      {
        'filter': findFilter(
          page: page,
          perPage: perPage,
          query: query,
          sort: sortField(sort, seed),
          direction: sort.direction,
        ),
        'image_filter': filter,
      },
    );
    final result = data['findImages'] as Map<String, dynamic>;
    return PageResult([
      for (final i in result['images'] as List)
        MediaRef.fromImage(i as Map<String, dynamic>, client),
    ], result['count'] as int);
  }
}

class VideoSource extends MediaSource {
  VideoSource(super.client, [this.filter = const {}]);
  final Map<String, dynamic> filter;
  @override
  String get label => 'Videos';
  @override
  List<SortOption> get sorts => SortOption.values;

  @override
  Future<PageResult<MediaRef>> fetch(
    int page,
    int perPage,
    String query,
    SortOption sort,
    int seed,
  ) async {
    final data = await client.query(
      '''
      query Videos(\$filter: FindFilterType, \$scene_filter: SceneFilterType) {
        findScenes(filter: \$filter, scene_filter: \$scene_filter) {
          count
          scenes { $_sceneFields }
        }
      }
    ''',
      {
        'filter': findFilter(
          page: page,
          perPage: perPage,
          query: query,
          sort: sortField(sort, seed),
          direction: sort.direction,
        ),
        'scene_filter': filter,
      },
    );
    final result = data['findScenes'] as Map<String, dynamic>;
    return PageResult([
      for (final s in result['scenes'] as List)
        MediaRef.fromScene(s as Map<String, dynamic>, client),
    ], result['count'] as int);
  }
}

/// StashBooru's combined Images + Videos list (the All page).
class AllMediaSource extends MediaSource {
  AllMediaSource(super.client, [this.filter = const {}]);
  final Map<String, dynamic> filter;
  @override
  String get label => 'All';
  @override
  List<SortOption> get sorts =>
      SortOption.values.where((s) => s != SortOption.random).toList();

  @override
  Future<PageResult<MediaRef>> fetch(
    int page,
    int perPage,
    String query,
    SortOption sort,
    int seed,
  ) async {
    final data = await client.query(
      '''
      query All(\$filter: FindFilterType, \$media_filter: MediaFilterType) {
        findMedia(filter: \$filter, media_filter: \$media_filter) {
          count
          items {
            kind
            image { $_imageFields }
            scene { $_sceneFields }
          }
        }
      }
    ''',
      {
        'filter': findFilter(
          page: page,
          perPage: perPage,
          query: query,
          sort: sort == SortOption.random
              ? SortOption.newest.field
              : sort.field,
          direction: sort.direction,
        ),
        'media_filter': filter,
      },
    );
    final result = data['findMedia'] as Map<String, dynamic>;
    final items = <MediaRef>[];
    for (final raw in result['items'] as List) {
      final item = raw as Map<String, dynamic>;
      if (item['image'] != null) {
        items.add(
          MediaRef.fromImage(item['image'] as Map<String, dynamic>, client),
        );
      } else if (item['scene'] != null) {
        items.add(
          MediaRef.fromScene(item['scene'] as Map<String, dynamic>, client),
        );
      }
    }
    return PageResult(items, result['count'] as int);
  }
}

/// The media sources that make sense for [scope] on this server.
List<MediaSource> sourcesFor(
  StashClient client,
  ServerCapabilities caps,
  MediaScope scope,
) {
  if (caps.mixedMedia &&
      scope.media != null &&
      scope.image != null &&
      scope.scene != null) {
    return [AllMediaSource(client, scope.media!)];
  }
  return [
    if (scope.image != null) ImageSource(client, scope.image!),
    if (scope.scene != null) VideoSource(client, scope.scene!),
  ];
}

int newSeed() => Random().nextInt(1 << 30);
