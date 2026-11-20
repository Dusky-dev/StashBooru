import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:material_ui/material_ui.dart';
import 'package:stashbooru/api/client.dart';
import 'package:stashbooru/state/app_settings.dart';
import 'package:stashbooru/ui/booru_browser.dart';
import 'package:stashbooru/ui/session.dart';
import 'package:stashbooru/ui/settings_page.dart';

const _key = 'a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6';

Map<String, dynamic> _post(String id) => {
  'id': id,
  'sourceID': 'dan',
  'postURL': 'https://booru.example/posts/$id',
  'previewURL': 'https://cdn.example/$id.jpg',
  'mediaKey': _key,
  'downloadAvailable': true,
  'width': 800,
  'height': 600,
  'format': 'png',
  'rating': 'g',
  'tags': [
    {'name': 'blue_sky', 'label': 'blue sky', 'category': 'general'},
    {'name': 'someone', 'label': 'someone', 'category': 'artist'},
  ],
};

class _FakeBooru {
  final searches = <Map<String, String>>[];
  final downloads = <Object?>[];
  int statusCalls = 0;

  http.Client get client => MockClient((r) async {
    expect(r.headers['ApiKey'], 'k');
    switch (r.url.path) {
      case '/booru-browser/sources':
        return http.Response(
          jsonEncode([
            {'id': 'dan', 'name': 'Danbooru', 'engine': 'danbooru'},
          ]),
          200,
        );
      case '/booru-browser/search':
        searches.add(r.url.queryParameters);
        return http.Response(
          jsonEncode({
            'sourceID': 'dan',
            'posts': [_post('1'), _post('2')],
            'page': 1,
            'limit': 40,
            'hasMore': false,
          }),
          200,
        );
      case '/booru-browser/download-settings':
        return http.Response(
          jsonEncode({
            'convert': false,
            'defaultDirectory': '',
            'roots': ['/library'],
          }),
          200,
        );
      case '/booru-browser/download':
        if (r.method == 'POST') {
          expect(r.headers['Content-Type'], startsWith('application/json'));
          downloads.add(jsonDecode(r.body));
          return http.Response(
            jsonEncode({'jobID': 7, 'token': 't', 'status': 'queued'}),
            200,
          );
        }
        statusCalls++;
        expect(r.url.queryParameters, {'jobID': '7', 'token': 't'});
        return http.Response(
          jsonEncode({
            'jobID': 7,
            'token': 't',
            'status': 'complete',
            'filename': 'post-1.png',
          }),
          200,
        );
    }
    return http.Response(jsonEncode({'error': 'not found'}), 404);
  });
}

Widget _app(StashClient client, Widget child, {AppSettings? settings}) =>
    MaterialApp(
      home: AppSettingsScope(
        settings: settings ?? AppSettings.defaults(),
        child: Session(
          client: client,
          caps: const ServerCapabilities(
            version: 'v1',
            copyrights: true,
            mixedMedia: true,
          ),
          onDisconnect: () {},
          child: child,
        ),
      ),
    );

void main() {
  testWidgets('searches a site, downloads a post and searches its tag', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(1280, 800);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    final fake = _FakeBooru();
    final client = StashClient(
      baseUrl: Uri.parse('http://nas:9999'),
      apiKey: 'k',
      httpClient: fake.client,
    );
    await tester.pumpWidget(_app(client, const Scaffold(body: BooruBrowser())));
    await tester.pumpAndSettle();

    expect(find.text('Danbooru'), findsOneWidget);
    expect(fake.searches.single['source'], 'dan');
    expect(find.text('800×600 · PNG'), findsNWidgets(2));

    await tester.enterText(find.byKey(const Key('booru-search')), 'cat');
    await tester.testTextInput.receiveAction(TextInputAction.search);
    await tester.pumpAndSettle();
    expect(fake.searches.last['q'], 'cat');

    await tester.tap(find.text('800×600 · PNG').first);
    await tester.pumpAndSettle();
    expect(find.text('Post 1'), findsOneWidget);
    expect(find.text('Artist'), findsOneWidget);

    await tester.tap(find.byKey(const Key('booru-download')));
    await tester.pump();
    await tester.pump();
    expect(fake.downloads.single, {
      'key': _key,
      'convert': false,
      'destination': '/library',
    });
    expect(find.text('Waiting to download...'), findsOneWidget);
    await tester.pump(const Duration(seconds: 2));
    await tester.pumpAndSettle();
    expect(fake.statusCalls, 1);
    expect(find.text('Saved to the library as post-1.png.'), findsOneWidget);
    expect(find.text('Downloaded'), findsOneWidget);

    // A tag goes back to the grid and searches for it.
    await tester.tap(find.text('blue sky'));
    await tester.pumpAndSettle();
    expect(fake.searches.last['q'], 'blue_sky');
    expect(find.byKey(const Key('booru-search')), findsOneWidget);
  });

  testWidgets('explains a server without booru sites', (tester) async {
    final client = StashClient(
      baseUrl: Uri.parse('http://nas:9999'),
      httpClient: MockClient((_) async => http.Response('[]', 200)),
    );
    await tester.pumpWidget(_app(client, const Scaffold(body: BooruBrowser())));
    await tester.pumpAndSettle();
    expect(find.textContaining('No booru sites are set up'), findsOneWidget);
  });

  testWidgets('settings change device preferences and disconnect', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(1280, 1200);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    final settings = AppSettings.defaults();
    var disconnected = false;
    final client = StashClient(baseUrl: Uri.parse('http://nas:9999'));
    await tester.pumpWidget(
      MaterialApp(
        home: AppSettingsScope(
          settings: settings,
          child: Session(
            client: client,
            caps: const ServerCapabilities(
              version: 'v1',
              copyrights: true,
              mixedMedia: true,
            ),
            onDisconnect: () => disconnected = true,
            child: const SettingsPage(),
          ),
        ),
      ),
    );
    expect(find.text('http://nas:9999'), findsOneWidget);
    expect(find.text('Version $kAppVersion'), findsOneWidget);

    await tester.tap(find.text('Large'));
    await tester.tap(find.byKey(const Key('autoplay')));
    await tester.tap(find.byKey(const Key('full-size-images')));
    await tester.pump();
    expect(settings.cardSize, CardSize.large);
    expect(settings.autoplay, isFalse);
    expect(settings.fullSizeImages, isFalse);

    await tester.tap(find.byKey(const Key('disconnect')));
    await tester.pump();
    expect(disconnected, isTrue);
  });

  test('app version matches pubspec and the server version file', () {
    final pubspec = File('pubspec.yaml').readAsStringSync();
    final version = RegExp(
      r'^version: ([0-9.]+)\+',
      multiLine: true,
    ).firstMatch(pubspec)!.group(1);
    expect(version, kAppVersion);
    expect(
      File('../internal/build/stashbooru-version.txt')
          .readAsStringSync()
          .trim(),
      kAppVersion,
    );
  });
}
