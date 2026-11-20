import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:stashbooru/api/client.dart';
import 'package:stashbooru/api/models.dart';
import 'package:stashbooru/ui/paging.dart';
import 'package:stashbooru/ui/session.dart';
import 'package:stashbooru/ui/viewer.dart';

MediaRef image(String id) => MediaRef(
  kind: MediaKind.image,
  id: id,
  title: 'Image $id',
  thumbnail: null,
  source: null,
);

Future<PagedController<MediaRef>> loaded(int count) async {
  final c = PagedController<MediaRef>(
    (page, perPage) async =>
        PageResult([for (var i = 1; i <= count; i++) image('$i')], count),
  );
  await c.loadMore();
  return c;
}

Widget host(PagedController<MediaRef> c, {int initial = 0}) => Session(
  client: StashClient(baseUrl: Uri.parse('http://nas:9999')),
  caps: const ServerCapabilities(
    version: 'v',
    copyrights: true,
    mixedMedia: true,
  ),
  onDisconnect: () {},
  child: MaterialApp(
    home: ViewerPage(controller: c, initialIndex: initial),
  ),
);

void main() {
  testWidgets('keys and buttons move through the list', (tester) async {
    tester.view.physicalSize = const Size(1280, 800);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    final c = await loaded(3);
    await tester.pumpWidget(host(c));
    await tester.pumpAndSettle();
    expect(find.text('1 / 3'), findsOneWidget);
    expect(find.byTooltip('Previous (Page Up)'), findsNothing);

    await tester.sendKeyEvent(LogicalKeyboardKey.arrowRight);
    await tester.pumpAndSettle();
    expect(find.text('2 / 3'), findsOneWidget);
    expect(find.text('Image 2'), findsOneWidget);

    await tester.sendKeyEvent(LogicalKeyboardKey.pageDown);
    await tester.pumpAndSettle();
    expect(find.text('3 / 3'), findsOneWidget);
    expect(find.byTooltip('Next (Page Down)'), findsNothing);

    await tester.tap(find.byTooltip('Previous (Page Up)'));
    await tester.pumpAndSettle();
    expect(find.text('2 / 3'), findsOneWidget);

    // Nothing advances on its own.
    await tester.pump(const Duration(seconds: 10));
    expect(find.text('2 / 3'), findsOneWidget);
  });

  testWidgets('swiping changes the item on touch screens', (tester) async {
    tester.view.physicalSize = const Size(400, 800);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    final c = await loaded(3);
    await tester.pumpWidget(host(c, initial: 1));
    await tester.pumpAndSettle();
    expect(find.text('2 / 3'), findsOneWidget);
    await tester.fling(find.byType(PageView), const Offset(-300, 0), 1000);
    await tester.pumpAndSettle();
    expect(find.text('3 / 3'), findsOneWidget);
  });
}
