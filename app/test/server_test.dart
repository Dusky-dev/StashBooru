// Runs every query the app sends against a live server, so field names and
// filters are checked against the real schema rather than mocks.
//
//   STASHBOORU_TEST_URL=http://127.0.0.1:9999 [STASHBOORU_TEST_KEY=...] \
//     flutter test test/server_test.dart
//
// Skipped when STASHBOORU_TEST_URL is unset.
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:stashbooru/api/client.dart';
import 'package:stashbooru/api/entities.dart';
import 'package:stashbooru/api/models.dart';

void main() {
  final url = Platform.environment['STASHBOORU_TEST_URL'];
  final skip = url == null ? 'STASHBOORU_TEST_URL is not set' : null;
  late StashClient client;
  late ServerCapabilities caps;

  setUpAll(() async {
    if (url == null) return;
    client = StashClient(
      baseUrl: parseServerUrl(url)!,
      apiKey: Platform.environment['STASHBOORU_TEST_KEY'] ?? '',
    );
    caps = await client.probe();
  });

  test('probe', () {
    // ignore: avoid_print
    print(
      'server ${caps.version} copyrights=${caps.copyrights} all=${caps.mixedMedia}',
    );
  }, skip: skip);

  test('media lists accept every sort', () async {
    final sources = <MediaSource>[
      ImageSource(client),
      VideoSource(client),
      if (caps.mixedMedia) AllMediaSource(client),
    ];
    for (final source in sources) {
      for (final sort in source.sorts) {
        final page = await source.fetch(1, 40, '', sort, 7);
        expect(
          page.items.length,
          lessThanOrEqualTo(page.total!),
          reason: '${source.label} $sort',
        );
      }
      final searched = await source.fetch(
        1,
        40,
        'zzzz-no-match',
        SortOption.newest,
        0,
      );
      expect(searched.items, isEmpty);
    }
  }, skip: skip);

  test(
    'entity lists accept every sort and their media scopes resolve',
    () async {
      for (final type in EntityType.values) {
        if (type == EntityType.copyright && !caps.copyrights) continue;
        for (final sort in EntitySort.values) {
          await fetchEntities(
            client,
            type,
            page: 1,
            perPage: 40,
            sort: sort,
            seed: 3,
          );
        }
        final page = await fetchEntities(client, type, page: 1, perPage: 40);
        for (final entity in page.items) {
          var found = 0;
          for (final source in sourcesFor(client, caps, entity.scope)) {
            found += (await source.fetch(
              1,
              100,
              '',
              SortOption.newest,
              0,
            )).total!;
          }
          final expected = (entity.imageCount ?? 0) + (entity.videoCount ?? 0);
          expect(found, expected, reason: '${type.singular} ${entity.name}');
        }
      }
    },
    skip: skip,
  );

  test('details and files load with the API key', () async {
    final items = [
      ...(await ImageSource(
        client,
      ).fetch(1, 5, '', SortOption.newest, 0)).items,
      ...(await VideoSource(
        client,
      ).fetch(1, 5, '', SortOption.newest, 0)).items,
    ];
    expect(items, isNotEmpty);
    for (final item in items) {
      final detail = await fetchDetail(client, caps, item);
      expect(detail.title, isNotEmpty);
      for (final url in [item.thumbnail, item.source]) {
        if (url == null) continue;
        final response = await http.get(
          Uri.parse(url),
          headers: {...client.authHeaders, 'Range': 'bytes=0-1023'},
        );
        expect(response.statusCode, anyOf(200, 206), reason: url);
        expect(response.bodyBytes, isNotEmpty, reason: url);
      }
    }
  }, skip: skip);
}
