import 'dart:async';
import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:koni_jxl_flutter/koni_jxl_flutter.dart';
import 'package:material_ui/material_ui.dart';
import 'package:stashbooru/ui/stash_image.dart';

void main() {
  testWidgets('JPEG XL responses decode to an image', (tester) async {
    const w = 24, h = 16;
    final rgb = Uint8List(w * h * 3);
    for (var i = 0; i < rgb.length; i++) {
      rgb[i] = (i * 7) & 0xFF;
    }
    final jxl = JxlEncoder.encodeLossless(rgb, width: w, height: h);
    String? sentKey;
    final client = MockClient((request) async {
      sentKey = request.headers['ApiKey'];
      return http.Response.bytes(jxl, 200);
    });
    final provider = StashNetworkImage(
      'http://server/image/1/image',
      headers: const {'ApiKey': 'k'},
      client: client,
    );

    ImageInfo? info;
    Object? error;
    await tester.runAsync(() async {
      final stream = provider.resolve(ImageConfiguration.empty);
      final done = Completer<void>();
      stream.addListener(
        ImageStreamListener(
          (i, _) {
            info = i;
            done.complete();
          },
          onError: (e, _) {
            error = e;
            done.complete();
          },
        ),
      );
      await done.future.timeout(const Duration(seconds: 20));
    });

    expect(error, isNull);
    expect(sentKey, 'k');
    expect(info!.image.width, w);
    expect(info!.image.height, h);
  });
}
