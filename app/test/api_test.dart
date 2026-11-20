import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:stashbooru/api/client.dart';
import 'package:stashbooru/api/entities.dart';
import 'package:stashbooru/api/models.dart';

StashClient clientWith(
  Object? Function(Map<String, dynamic> body) answer, {
  String key = '',
  int status = 200,
}) {
  return StashClient(
    baseUrl: Uri.parse('http://nas:9999'),
    apiKey: key,
    httpClient: MockClient((request) async {
      final body = jsonDecode(request.body) as Map<String, dynamic>;
      return http.Response(
        jsonEncode(answer(body)),
        status,
        headers: {'content-type': 'application/json'},
      );
    }),
  );
}

void main() {
  group('parseServerUrl', () {
    test('adds http and drops trailing slashes', () {
      expect(
        parseServerUrl('192.168.1.5:9999/').toString(),
        'http://192.168.1.5:9999',
      );
      expect(
        parseServerUrl(' https://example.com/stash/ ').toString(),
        'https://example.com/stash',
      );
    });
    test('rejects junk', () {
      expect(parseServerUrl(''), isNull);
      expect(parseServerUrl('ftp://x'), isNull);
    });
    test('keeps a reverse-proxy prefix for /graphql', () {
      final c = StashClient(
        baseUrl: parseServerUrl('https://example.com/stash')!,
      );
      expect(c.graphqlUrl.toString(), 'https://example.com/stash/graphql');
    });
  });

  group('StashClient', () {
    test('sends the API key header', () async {
      late Map<String, String> headers;
      final c = StashClient(
        baseUrl: Uri.parse('http://nas:9999'),
        apiKey: 'secret',
        httpClient: MockClient((r) async {
          headers = r.headers;
          return http.Response(
            '{"data":{"version":{"version":"v1"},"copyright":null,"media":null}}',
            200,
          );
        }),
      );
      final caps = await c.probe();
      expect(headers['ApiKey'], 'secret');
      expect(caps.isStashBooru, isFalse);
      expect(caps.version, 'v1');
    });

    test('detects StashBooru features', () async {
      final c = clientWith(
        (_) => {
          'data': {
            'version': {'version': 'v0.30'},
            'copyright': {'name': 'Copyright'},
            'media': {'name': 'MediaItem'},
          },
        },
      );
      final caps = await c.probe();
      expect(caps.copyrights, isTrue);
      expect(caps.mixedMedia, isTrue);
    });

    test('explains a missing or rejected key', () async {
      expect(
        clientWith((_) => {}, status: 401).probe(),
        throwsA(
          isA<StashException>().having(
            (e) => e.message,
            'message',
            contains('needs an API key'),
          ),
        ),
      );
      expect(
        clientWith((_) => {}, key: 'x', status: 401).probe(),
        throwsA(
          isA<StashException>().having(
            (e) => e.message,
            'message',
            contains('rejected'),
          ),
        ),
      );
    });

    test('surfaces GraphQL errors', () async {
      final c = clientWith(
        (_) => {
          'errors': [
            {'message': 'boom'},
          ],
        },
      );
      expect(
        c.probe(),
        throwsA(
          isA<StashException>().having(
            (e) => e.message,
            'message',
            'Server error: boom',
          ),
        ),
      );
    });
  });

  group('media', () {
    test('keeps typed identity and detects video-backed images', () {
      final c = StashClient(baseUrl: Uri.parse('http://nas:9999'));
      final image = MediaRef.fromImage({
        'id': '7',
        'title': '',
        'paths': {
          'thumbnail': '/image/7/thumbnail',
          'image': 'http://nas:9999/image/7/image',
        },
        'visual_files': [
          {
            '__typename': 'VideoFile',
            'path': '/lib/clip.webm',
            'width': 640,
            'height': 480,
            'duration': 3.5,
          },
        ],
      }, c);
      final scene = MediaRef.fromScene({
        'id': '7',
        'title': 'Seven',
        'paths': {
          'screenshot': null,
          'stream': 'http://nas:9999/scene/7/stream',
        },
        'files': [],
      }, c);
      expect(image.key, 'image:7');
      expect(scene.key, 'scene:7');
      expect(image.playsAsVideo, isTrue);
      expect(image.title, 'clip.webm');
      expect(image.thumbnail, 'http://nas:9999/image/7/thumbnail');
      expect(scene.thumbnail, isNull);
    });

    test('All mixes images and videos in server order', () async {
      final c = clientWith((body) {
        expect(body['query'], contains('findMedia'));
        expect((body['variables'] as Map)['filter']['sort'], 'created_at');
        return {
          'data': {
            'findMedia': {
              'count': 2,
              'items': [
                {
                  'kind': 'VIDEO',
                  'image': null,
                  'scene': {'id': '1', 'title': 'v', 'paths': {}, 'files': []},
                },
                {
                  'kind': 'IMAGE',
                  'scene': null,
                  'image': {
                    'id': '1',
                    'title': 'i',
                    'paths': {},
                    'visual_files': [],
                  },
                },
              ],
            },
          },
        };
      });
      final page = await AllMediaSource(c)
          .fetch(1, 40, '', SortOption.newest, 1);
      expect(page.items.map((m) => m.key), ['scene:1', 'image:1']);
      expect(AllMediaSource(c).sorts, isNot(contains(SortOption.random)));
    });

    test('random sort sends a seed', () async {
      final c = clientWith((body) {
        expect((body['variables'] as Map)['filter']['sort'], 'random_42');
        return {
          'data': {
            'findImages': {'count': 0, 'images': []},
          },
        };
      });
      await ImageSource(c).fetch(1, 40, '', SortOption.random, 42);
    });
  });

  group('entities', () {
    final stock = const ServerCapabilities(
      version: 'v',
      copyrights: false,
      mixedMedia: false,
    );
    final booru = const ServerCapabilities(
      version: 'v',
      copyrights: true,
      mixedMedia: true,
    );
    final c = StashClient(baseUrl: Uri.parse('http://nas:9999'));

    test('Copyright includes descendants, Tag does not', () {
      final cr = Entity(type: EntityType.copyright, id: '3', name: 'x').scope;
      expect(cr.media!['copyrights']['depth'], -1);
      final tag = Entity(type: EntityType.tag, id: '3', name: 'x').scope;
      expect(tag.image!['tags']['depth'], 0);
    });

    test('StashBooru shows one mixed list; stock Stash gets Images/Videos', () {
      final character = Entity(type: EntityType.character, id: '1', name: 'x');
      expect(
        sourcesFor(c, booru, character.scope).single,
        isA<AllMediaSource>(),
      );
      expect(sourcesFor(c, stock, character.scope).map((s) => s.label), [
        'Images',
        'Videos',
      ]);
    });

    test('Galleries hold images only; Collections videos only', () {
      final gallery = Entity(type: EntityType.gallery, id: '1', name: 'g');
      final collection = Entity(
        type: EntityType.collection,
        id: '1',
        name: 'c',
      );
      expect(sourcesFor(c, booru, gallery.scope).map((s) => s.label), [
        'Images',
      ]);
      expect(sourcesFor(c, booru, collection.scope).map((s) => s.label), [
        'Videos',
      ]);
    });

    test('untitled galleries are named after their folder', () {
      final g = entityFromJson(EntityType.gallery, {
        'id': '4',
        'title': null,
        'image_count': 3,
        'paths': {'cover': '/gallery/4/cover'},
        'folder': {'path': '/library/Summer trip'},
        'files': [],
      }, c);
      expect(g.name, 'Summer trip');
      expect(g.countLabel, '3 images');
    });
  });
}
