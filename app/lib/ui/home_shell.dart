import 'package:material_ui/material_ui.dart';

import '../api/entities.dart';
import '../api/models.dart';
import 'entity_browser.dart';
import 'media_browser.dart';
import 'session.dart';

class _Section {
  const _Section(this.label, this.icon, this.build);
  final String label;
  final IconData icon;
  final Widget Function(BuildContext) build;
}

/// Navigation in the web UI's navbar order: All, Images, Videos, Characters,
/// Artists, Copyrights, Tags, Galleries, Collections. All and Copyrights only
/// appear on StashBooru servers.
class HomeShell extends StatefulWidget {
  const HomeShell({super.key});

  @override
  State<HomeShell> createState() => _HomeShellState();
}

class _HomeShellState extends State<HomeShell> {
  int _selected = 0;

  /// Sections are built on first visit and kept alive afterwards.
  final Map<int, Widget> _built = {};

  List<_Section> _sections(Session s) => [
    if (s.caps.mixedMedia)
      _Section(
        'All',
        Icons.dashboard,
        (_) => MediaBrowser(sources: [AllMediaSource(s.client)]),
      ),
    _Section(
      'Images',
      Icons.image,
      (_) => MediaBrowser(sources: [ImageSource(s.client)]),
    ),
    _Section(
      'Videos',
      Icons.movie,
      (_) => MediaBrowser(sources: [VideoSource(s.client)]),
    ),
    for (final type in EntityType.values)
      if (type != EntityType.copyright || s.caps.copyrights)
        _Section(
          type.plural,
          entityIcon(type),
          (_) => EntityBrowser(type: type),
        ),
  ];

  void _select(int i) => setState(() => _selected = i);

  void _showServer(Session s) {
    showDialog<void>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Server'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            SelectableText(s.client.baseUrl.toString()),
            const SizedBox(height: 8),
            Text(
              '${s.caps.isStashBooru ? 'StashBooru' : 'Stash'} ${s.caps.version}',
            ),
            if (!s.caps.isStashBooru)
              const Padding(
                padding: EdgeInsets.only(top: 8),
                child: Text('All and Copyrights need a StashBooru server.'),
              ),
          ],
        ),
        actions: [
          TextButton(
            onPressed: () {
              Navigator.of(context).pop();
              s.onDisconnect();
            },
            child: const Text('Disconnect'),
          ),
          FilledButton(
            onPressed: () => Navigator.of(context).pop(),
            child: const Text('Close'),
          ),
        ],
      ),
    );
  }

  Widget _navList(
    Session s,
    List<_Section> sections, {
    required bool closeDrawer,
  }) {
    return ListView(
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 16, 16, 8),
          child: Text(
            'StashBooru',
            style: Theme.of(context).textTheme.titleLarge,
          ),
        ),
        for (var i = 0; i < sections.length; i++)
          ListTile(
            leading: Icon(sections[i].icon),
            title: Text(sections[i].label),
            selected: i == _selected,
            onTap: () {
              _select(i);
              if (closeDrawer) Navigator.of(context).pop();
            },
          ),
        const Divider(),
        ListTile(
          leading: const Icon(Icons.dns),
          title: const Text('Server'),
          onTap: () {
            if (closeDrawer) Navigator.of(context).pop();
            _showServer(s);
          },
        ),
      ],
    );
  }

  @override
  Widget build(BuildContext context) {
    final session = Session.of(context);
    final sections = _sections(session);
    if (_selected >= sections.length) _selected = 0;
    _built.putIfAbsent(_selected, () => sections[_selected].build(context));

    final body = IndexedStack(
      index: _selected,
      children: [
        for (var i = 0; i < sections.length; i++)
          KeyedSubtree(
            key: ValueKey(sections[i].label),
            child: _built[i] ?? const SizedBox(),
          ),
      ],
    );

    final wide = MediaQuery.sizeOf(context).width >= 840;
    if (wide) {
      return Scaffold(
        body: Row(
          children: [
            SizedBox(
              width: 220,
              child: Material(
                color: Theme.of(context).colorScheme.surfaceContainerLow,
                child: SafeArea(
                  child: _navList(session, sections, closeDrawer: false),
                ),
              ),
            ),
            Expanded(child: SafeArea(child: body)),
          ],
        ),
      );
    }
    return Scaffold(
      appBar: AppBar(title: Text(sections[_selected].label)),
      drawer: Drawer(
        child: SafeArea(child: _navList(session, sections, closeDrawer: true)),
      ),
      body: body,
    );
  }
}
