import 'dart:async';
import 'dart:ui' as ui;

import 'package:flutter/foundation.dart';
import 'package:http/http.dart' as http;
import 'package:koni_jxl_flutter/koni_jxl_flutter.dart';
import 'package:material_ui/material_ui.dart';

/// A network image that sends the API key and decodes JPEG XL in Dart.
///
/// Flutter's engine has no JPEG XL decoder, and the Android and Windows
/// builds of mpv lack one too, so JPEG XL bytes go through a pure-Dart
/// decoder on a background isolate (stills and animations). Everything else
/// uses the engine's codecs as `Image.network` would.
@immutable
class StashNetworkImage extends ImageProvider<StashNetworkImage> {
  const StashNetworkImage(
    this.url, {
    this.headers,
    this.cacheWidth,
    this.client,
  });

  final String url;
  final Map<String, String>? headers;

  /// Decode no wider than this many physical pixels (never upscaled).
  final int? cacheWidth;

  /// Only for tests; the shared default client is used otherwise.
  final http.Client? client;

  @override
  Future<StashNetworkImage> obtainKey(ImageConfiguration configuration) =>
      SynchronousFuture(this);

  @override
  ImageStreamCompleter loadImage(
    StashNetworkImage key,
    ImageDecoderCallback decode,
  ) => MultiFrameImageStreamCompleter(
    codec: _load(decode),
    scale: 1,
    debugLabel: url,
    informationCollector: () => [DiagnosticsProperty('URL', url)],
  );

  Future<ui.Codec> _load(ImageDecoderCallback decode) async {
    final uri = Uri.parse(url);
    final response =
        await (client?.get(uri, headers: headers) ??
            http.get(uri, headers: headers));
    if (response.statusCode != 200) {
      throw NetworkImageLoadException(
        statusCode: response.statusCode,
        uri: uri,
      );
    }
    final bytes = response.bodyBytes;
    if (bytes.isEmpty) throw StateError('Empty image: $url');
    if (looksLikeJxl(bytes)) {
      return decodeJxlToUiCodec(bytes, cacheWidth: cacheWidth);
    }
    final buffer = await ui.ImmutableBuffer.fromUint8List(bytes);
    return decode(
      buffer,
      getTargetSize: (width, height) =>
          cacheWidth != null && cacheWidth! < width
          ? ui.TargetImageSize(width: cacheWidth)
          : ui.TargetImageSize(width: width, height: height),
    );
  }

  @override
  bool operator ==(Object other) =>
      other is StashNetworkImage &&
      other.url == url &&
      other.cacheWidth == cacheWidth;

  @override
  int get hashCode => Object.hash(url, cacheWidth);

  @override
  String toString() => 'StashNetworkImage($url, cacheWidth: $cacheWidth)';
}
