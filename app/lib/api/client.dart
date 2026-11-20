import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:http/http.dart' as http;

/// A failure the user can act on, worded for display.
class StashException implements Exception {
  StashException(this.message);
  final String message;
  @override
  String toString() => message;
}

/// Features the connected server offers beyond stock Stash.
class ServerCapabilities {
  const ServerCapabilities({
    required this.version,
    required this.copyrights,
    required this.mixedMedia,
  });

  final String version;

  /// StashBooru's native Copyright entities.
  final bool copyrights;

  /// StashBooru's combined Images + Videos query (`findMedia`), used by All.
  final bool mixedMedia;

  bool get isStashBooru => copyrights || mixedMedia;
}

/// Normalises what a user types into a server base URL.
///
/// Accepts `host:9999`, `http://host:9999/`, or a reverse-proxy prefix such as
/// `https://example.com/stash`. Returns null when nothing usable was typed.
Uri? parseServerUrl(String input) {
  var text = input.trim();
  if (text.isEmpty) return null;
  if (!text.contains('://')) text = 'http://$text';
  final uri = Uri.tryParse(text);
  if (uri == null || uri.host.isEmpty) return null;
  if (uri.scheme != 'http' && uri.scheme != 'https') return null;
  var path = uri.path;
  while (path.endsWith('/')) {
    path = path.substring(0, path.length - 1);
  }
  return uri.replace(path: path, query: null, fragment: null);
}

class StashClient {
  StashClient({
    required this.baseUrl,
    this.apiKey = '',
    http.Client? httpClient,
  }) : _http = httpClient ?? http.Client();

  final Uri baseUrl;
  final String apiKey;
  final http.Client _http;

  /// Headers for every request, including thumbnails and video streams.
  Map<String, String> get authHeaders =>
      apiKey.isEmpty ? const {} : {'ApiKey': apiKey};

  Uri get graphqlUrl => baseUrl.replace(path: '${baseUrl.path}/graphql');

  /// Resolves a server-returned path against the connected server.
  ///
  /// Stash returns absolute URLs built from the request host; a relative one
  /// is resolved against [baseUrl].
  String resolve(String url) {
    final parsed = Uri.tryParse(url);
    if (parsed == null) return url;
    if (parsed.hasScheme) return url;
    return baseUrl.resolveUri(parsed).toString();
  }

  Future<Map<String, dynamic>> query(
    String document, [
    Map<String, dynamic> variables = const {},
  ]) async {
    final http.Response response;
    try {
      response = await _http
          .post(
            graphqlUrl,
            headers: {
              ...authHeaders,
              'Content-Type': 'application/json',
              'Accept': 'application/json',
            },
            body: jsonEncode({'query': document, 'variables': variables}),
          )
          .timeout(const Duration(seconds: 30));
    } on TimeoutException {
      throw StashException('The server did not answer within 30 seconds.');
    } on SocketException catch (e) {
      throw StashException('Cannot reach the server: ${e.message}');
    } on http.ClientException catch (e) {
      throw StashException('Cannot reach the server: ${e.message}');
    } on HandshakeException {
      throw StashException(
        'The secure connection failed. Check http:// versus https://.',
      );
    }

    if (response.statusCode == 401 || response.statusCode == 403) {
      throw StashException(
        apiKey.isEmpty
            ? 'This server needs an API key (Settings → Security in the web UI).'
            : 'The server rejected this API key.',
      );
    }

    Map<String, dynamic> body;
    try {
      body =
          jsonDecode(utf8.decode(response.bodyBytes)) as Map<String, dynamic>;
    } on FormatException {
      if (response.statusCode >= 300 && response.statusCode < 400 ||
          response.body.contains('<form')) {
        throw StashException(
          'This server needs an API key (Settings → Security in the web UI).',
        );
      }
      throw StashException(
        'This address did not answer like a Stash server (HTTP ${response.statusCode}).',
      );
    }

    final errors = body['errors'];
    if (errors is List && errors.isNotEmpty) {
      final first = errors.first;
      final message = first is Map ? first['message'] : first;
      throw StashException('Server error: $message');
    }
    if (response.statusCode >= 400) {
      throw StashException('Server error (HTTP ${response.statusCode}).');
    }
    final data = body['data'];
    if (data is! Map<String, dynamic>) {
      throw StashException('The server returned no data.');
    }
    return data;
  }

  Future<ServerCapabilities> probe() async {
    final data = await query('''
      query Probe {
        version { version }
        copyright: __type(name: "Copyright") { name }
        media: __type(name: "MediaItem") { name }
      }
    ''');
    final version = (data['version'] as Map?)?['version'] as String?;
    return ServerCapabilities(
      version: version == null || version.isEmpty
          ? 'development build'
          : version,
      copyrights: data['copyright'] != null,
      mixedMedia: data['media'] != null,
    );
  }

  void close() => _http.close();
}
