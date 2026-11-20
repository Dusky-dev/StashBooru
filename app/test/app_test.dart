import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:material_ui/material_ui.dart';
import 'package:stashbooru/api/client.dart';
import 'package:stashbooru/main.dart';
import 'package:stashbooru/state/credentials.dart';

/// A fake server answering just enough GraphQL for the shell to render.
http.Client fakeServer({required bool booru, String? requiredKey}) =>
    MockClient((r) async {
      if (requiredKey != null && r.headers['ApiKey'] != requiredKey) {
        return http.Response('', 401);
      }
      final q = (jsonDecode(r.body) as Map)['query'] as String;
      Map<String, dynamic> data;
      if (q.contains('Probe')) {
        data = {
          'version': {'version': 'v0.30.0'},
          'copyright': booru ? {'name': 'Copyright'} : null,
          'media': booru ? {'name': 'MediaItem'} : null,
        };
      } else if (q.contains('findMedia')) {
        data = {
          'findMedia': {'count': 0, 'items': []},
        };
      } else if (q.contains('findImages')) {
        data = {
          'findImages': {'count': 0, 'images': []},
        };
      } else {
        data = {};
      }
      return http.Response(jsonEncode({'data': data}), 200);
    });

Future<void> connect(WidgetTester tester, {String key = ''}) async {
  await tester.enterText(find.byKey(const Key('server-url')), 'nas:9999');
  await tester.enterText(find.byKey(const Key('api-key')), key);
  await tester.tap(find.text('Connect'));
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('StashBooru server shows All and Copyrights in navbar order', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(1280, 800);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    final store = MemoryCredentialStore();
    await tester.pumpWidget(
      StashBooruApp(
        store: store,
        clientFactory: (url, key) => StashClient(
          baseUrl: url,
          apiKey: key,
          httpClient: fakeServer(booru: true),
        ),
      ),
    );
    await tester.pumpAndSettle();
    await connect(tester);

    final labels = tester
        .widgetList<ListTile>(find.byType(ListTile))
        .map((t) => ((t.title as Text).data!))
        .toList();
    expect(labels, [
      'All',
      'Images',
      'Videos',
      'Characters',
      'Artists',
      'Copyrights',
      'Tags',
      'Galleries',
      'Collections',
      'Server',
    ]);
    expect(find.text('Nothing here yet.'), findsOneWidget);
    expect((await store.load())!.url, 'http://nas:9999');
  });

  testWidgets('stock Stash hides All and Copyrights', (tester) async {
    tester.view.physicalSize = const Size(1280, 800);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(
      StashBooruApp(
        store: MemoryCredentialStore(),
        clientFactory: (url, key) => StashClient(
          baseUrl: url,
          apiKey: key,
          httpClient: fakeServer(booru: false),
        ),
      ),
    );
    await tester.pumpAndSettle();
    await connect(tester);
    expect(find.text('All'), findsNothing);
    expect(find.text('Copyrights'), findsNothing);
    expect(find.text('Images'), findsOneWidget);
  });

  testWidgets('a wrong key keeps the connect form with an explanation', (
    tester,
  ) async {
    await tester.pumpWidget(
      StashBooruApp(
        store: MemoryCredentialStore(),
        clientFactory: (url, key) => StashClient(
          baseUrl: url,
          apiKey: key,
          httpClient: fakeServer(booru: true, requiredKey: 'right'),
        ),
      ),
    );
    await tester.pumpAndSettle();
    await connect(tester, key: 'wrong');
    expect(find.byKey(const Key('connect-error')), findsOneWidget);
    expect(find.textContaining('rejected'), findsOneWidget);
  });

  testWidgets('a saved server reconnects on start', (tester) async {
    await tester.pumpWidget(
      StashBooruApp(
        store: MemoryCredentialStore(
          const SavedServer('http://nas:9999', 'right'),
        ),
        clientFactory: (url, key) => StashClient(
          baseUrl: url,
          apiKey: key,
          httpClient: fakeServer(booru: true, requiredKey: 'right'),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('server-url')), findsNothing);
    expect(find.text('All'), findsWidgets);
  });

  testWidgets('phones get a drawer with the same sections', (tester) async {
    tester.view.physicalSize = const Size(400, 800);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(
      StashBooruApp(
        store: MemoryCredentialStore(),
        clientFactory: (url, key) => StashClient(
          baseUrl: url,
          apiKey: key,
          httpClient: fakeServer(booru: true),
        ),
      ),
    );
    await tester.pumpAndSettle();
    await connect(tester);
    expect(find.widgetWithText(AppBar, 'All'), findsOneWidget);
    await tester.tap(find.byTooltip('Open navigation menu'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Images'));
    await tester.pumpAndSettle();
    expect(find.widgetWithText(AppBar, 'Images'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });
}
