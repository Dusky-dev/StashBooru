import 'client.dart';

/// A booru site the server is configured to search (`booru-browser.json`).
class BooruSource {
  const BooruSource({
    required this.id,
    required this.name,
    this.engine = '',
    this.queryHelp = '',
  });

  factory BooruSource.fromJson(Map<String, dynamic> json) => BooruSource(
    id: json['id'] as String? ?? '',
    name: json['name'] as String? ?? json['id'] as String? ?? '',
    engine: json['engine'] as String? ?? '',
    queryHelp: json['queryHelp'] as String? ?? '',
  );

  final String id;
  final String name;
  final String engine;
  final String queryHelp;
}

class BooruTag {
  const BooruTag(this.name, this.category, {String? label})
    : label = label ?? name;

  /// The provider's tag, used for searching.
  final String name;

  /// What the provider shows for it.
  final String label;
  final String category;
}

/// Where downloads go and whether the server converts them.
class BooruDownloadSettings {
  const BooruDownloadSettings({
    this.defaultDirectory = '',
    this.roots = const [],
    this.convert = false,
  });

  factory BooruDownloadSettings.fromJson(Map<String, dynamic> json) =>
      BooruDownloadSettings(
        defaultDirectory: json['defaultDirectory'] as String? ?? '',
        roots: [
          for (final r in json['roots'] as List? ?? const [])
            if (r is String && r.isNotEmpty) r,
        ],
        convert: json['convert'] == true,
      );

  final String defaultDirectory;

  /// The server's library folders, the allowed destinations.
  final List<String> roots;
  final bool convert;
}

/// One search result. Media is fetched through the server, which holds the
/// remote URLs and credentials behind a short-lived [mediaKey].
class BooruPost {
  const BooruPost({
    required this.id,
    required this.mediaKey,
    this.postUrl = '',
    this.hasPreview = false,
    this.hasSample = false,
    this.downloadAvailable = false,
    this.width = 0,
    this.height = 0,
    this.bytes = 0,
    this.rating = '',
    this.format = '',
    this.title = '',
    this.author = '',
    this.score,
    this.tags = const [],
    this.attachments = 0,
  });

  factory BooruPost.fromJson(Map<String, dynamic> json) {
    int asInt(Object? v) => v is num ? v.toInt() : 0;
    final tags = <BooruTag>[];
    for (final t in json['tags'] as List? ?? const []) {
      if (t is Map) {
        final name = t['name'] as String?;
        if (name != null && name.isNotEmpty) {
          tags.add(
            BooruTag(
              name,
              t['category'] as String? ?? '',
              label: t['label'] as String?,
            ),
          );
        }
      }
    }
    return BooruPost(
      id: '${json['id'] ?? ''}',
      mediaKey: json['mediaKey'] as String? ?? '',
      postUrl: json['postURL'] as String? ?? '',
      hasPreview: (json['previewURL'] as String? ?? '').isNotEmpty,
      hasSample: (json['sampleURL'] as String? ?? '').isNotEmpty,
      downloadAvailable: json['downloadAvailable'] == true,
      width: asInt(json['width']),
      height: asInt(json['height']),
      bytes: asInt(json['bytes']),
      rating: json['rating'] as String? ?? '',
      format: json['format'] as String? ?? '',
      title: json['title'] as String? ?? '',
      author: json['author'] as String? ?? '',
      score: json['score'] is num ? (json['score'] as num).toInt() : null,
      tags: tags,
      attachments: (json['attachments'] as List?)?.length ?? 0,
    );
  }

  final String id;
  final String mediaKey;
  final String postUrl;
  final bool hasPreview;
  final bool hasSample;
  final bool downloadAvailable;
  final int width;
  final int height;
  final int bytes;
  final String rating;
  final String format;
  final String title;
  final String author;
  final int? score;
  final List<BooruTag> tags;
  final int attachments;

  bool get _validKey => RegExp(r'^[a-f0-9]{48}$').hasMatch(mediaKey);

  bool get isVideo => const {'mp4', 'webm'}.contains(format.toLowerCase());
}

class BooruPage {
  const BooruPage({
    required this.posts,
    required this.hasMore,
    this.notice = '',
    this.blacklisted = 0,
  });

  final List<BooruPost> posts;
  final bool hasMore;
  final String notice;
  final int blacklisted;
}

/// State of a server-side download into the library.
class BooruDownload {
  const BooruDownload({
    required this.jobId,
    required this.token,
    required this.status,
    this.error = '',
    this.filename = '',
  });

  factory BooruDownload.fromJson(Map<String, dynamic> json) => BooruDownload(
    jobId: (json['jobID'] as num?)?.toInt() ?? 0,
    token: json['token'] as String? ?? '',
    status: json['status'] as String? ?? '',
    error: json['error'] as String? ?? '',
    filename: json['filename'] as String? ?? '',
  );

  final int jobId;
  final String token;
  final String status;
  final String error;
  final String filename;

  bool get finished =>
      const {'complete', 'failed', 'cancelled'}.contains(status);
  bool get succeeded => status == 'complete';
}

/// The server's booru browser routes, the same ones the web UI uses.
class BooruApi {
  const BooruApi(this.client);
  final StashClient client;

  static const pageSize = 40;

  Future<List<BooruSource>> sources() async {
    final list = await client.rest('booru-browser/sources');
    return [
      for (final s in list as List? ?? const [])
        if (s is Map<String, dynamic>) BooruSource.fromJson(s),
    ].where((s) => s.id.isNotEmpty).toList();
  }

  Future<BooruPage> search(
    BooruSource source,
    String query, {
    int page = 1,
  }) async {
    final json = await client.rest(
      'booru-browser/search',
      query: {
        'source': source.id,
        'q': query,
        'page': '$page',
        'limit': '$pageSize',
      },
    );
    final map = json as Map<String, dynamic>;
    return BooruPage(
      posts: [
        for (final p in map['posts'] as List? ?? const [])
          if (p is Map<String, dynamic>) BooruPost.fromJson(p),
      ],
      hasMore: map['hasMore'] == true,
      notice: map['searchNotice'] as String? ?? '',
      blacklisted: (map['blacklistedCount'] as num?)?.toInt() ?? 0,
    );
  }

  /// The server-proxied preview (small) or sample (large) image.
  String? mediaUrl(BooruPost post, {bool sample = false}) {
    if (!post._validKey) return null;
    if (sample ? !post.hasSample : !post.hasPreview) return null;
    return client.restUrl('booru-browser/media', {
      'key': post.mediaKey,
      'kind': sample ? 'sample' : 'preview',
    }).toString();
  }

  Future<BooruDownloadSettings> downloadSettings() async {
    final json = await client.rest('booru-browser/download-settings');
    return BooruDownloadSettings.fromJson(json as Map<String, dynamic>);
  }

  /// Asks the server to download the original into [destination], a folder
  /// inside one of its libraries.
  Future<BooruDownload> download(
    BooruPost post, {
    required String destination,
    bool convert = false,
  }) async {
    final json = await client.rest(
      'booru-browser/download',
      body: {
        'key': post.mediaKey,
        'convert': convert,
        'destination': destination,
      },
    );
    return BooruDownload.fromJson(json as Map<String, dynamic>);
  }

  Future<BooruDownload> downloadStatus(BooruDownload d) async {
    final json = await client.rest(
      'booru-browser/download',
      query: {'jobID': '${d.jobId}', 'token': d.token},
    );
    return BooruDownload.fromJson(json as Map<String, dynamic>);
  }
}
