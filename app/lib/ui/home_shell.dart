import 'package:material_ui/material_ui.dart';

import '../api/entities.dart';
import '../api/models.dart';
import 'entity_browser.dart';
import 'media_browser.dart';
import 'session.dart';
import 'theme.dart';

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
        Icons.grid_view,
        (_) => MediaBrowser(sources: [AllMediaSource(s.client)]),
      ),
    _Section(
      'Images',
      Icons.image,
      (_) => MediaBrowser(sources: [ImageSource(s.client)]),
    ),
    _Section(
      'Videos',
      Icons.play_circle,
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

    // The web UI keeps the navbar on top on wide screens and moves it to the
    // bottom, collapsed behind a menu button, on phones.
    final wide = MediaQuery.sizeOf(context).width >= 768;
    if (wide) {
      return Scaffold(
        body: Column(
          children: [
            _Navbar(
              children: [
                const _Brand(),
                const SizedBox(width: 8),
                Expanded(
                  child: SingleChildScrollView(
                    scrollDirection: Axis.horizontal,
                    child: Row(
                      children: [
                        for (var i = 0; i < sections.length; i++)
                          _NavItem(
                            section: sections[i],
                            active: i == _selected,
                            onTap: () => _select(i),
                          ),
                      ],
                    ),
                  ),
                ),
                IconButton(
                  tooltip: 'Server',
                  icon: const Icon(Icons.settings, size: 20),
                  onPressed: () => _showServer(session),
                ),
              ],
            ),
            Expanded(child: body),
          ],
        ),
      );
    }
    return Scaffold(
      body: SafeArea(bottom: false, child: body),
      bottomNavigationBar: _Navbar(
        bottom: true,
        children: [
          const _Brand(),
          Expanded(
            child: Text(
              sections[_selected].label,
              key: const Key('current-section'),
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(color: StashColors.muted, fontSize: 15),
            ),
          ),
          OutlinedButton(
            key: const Key('nav-menu'),
            onPressed: () => _showMenu(session, sections),
            style: OutlinedButton.styleFrom(
              minimumSize: const Size(52, 38),
              side: const BorderSide(color: Color(0x33FFFFFF)),
            ),
            child: const Icon(Icons.menu, semanticLabel: 'Menu'),
          ),
        ],
      ),
    );
  }

  void _showMenu(Session session, List<_Section> sections) {
    showModalBottomSheet<void>(
      context: context,
      useSafeArea: true,
      isScrollControlled: true,
      shape: const RoundedRectangleBorder(),
      builder: (context) => ListView(
        shrinkWrap: true,
        padding: const EdgeInsets.symmetric(vertical: 8),
        children: [
          for (var i = 0; i < sections.length; i++)
            ListTile(
              leading: Icon(sections[i].icon, size: 20),
              title: Text(sections[i].label),
              selected: i == _selected,
              onTap: () {
                Navigator.of(context).pop();
                _select(i);
              },
            ),
          const Divider(),
          ListTile(
            leading: const Icon(Icons.settings, size: 20),
            title: const Text('Server'),
            onTap: () {
              Navigator.of(context).pop();
              _showServer(session);
            },
          ),
        ],
      ),
    );
  }
}

class _Navbar extends StatelessWidget {
  const _Navbar({required this.children, this.bottom = false});
  final List<Widget> children;
  final bool bottom;

  @override
  Widget build(BuildContext context) => Material(
    color: StashColors.navbar,
    child: SafeArea(
      top: !bottom,
      bottom: bottom,
      child: SizedBox(
        height: bottom ? 54 : 49,
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 12),
          child: Row(children: children),
        ),
      ),
    ),
  );
}

class _Brand extends StatelessWidget {
  const _Brand();

  @override
  Widget build(BuildContext context) => const Padding(
    padding: EdgeInsets.only(left: 8, right: 16),
    child: Text(
      'StashBooru',
      style: TextStyle(fontSize: 16, color: StashColors.text),
    ),
  );
}

class _NavItem extends StatelessWidget {
  const _NavItem({
    required this.section,
    required this.active,
    required this.onTap,
  });

  final _Section section;
  final bool active;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.symmetric(horizontal: 2),
    child: TextButton.icon(
      key: Key('nav-${section.label}'),
      onPressed: onTap,
      icon: Icon(section.icon, size: 16),
      label: Text(section.label),
      style: TextButton.styleFrom(
        foregroundColor: StashColors.text,
        backgroundColor: active ? const Color(0x26FFFFFF) : null,
        minimumSize: const Size(0, 34),
        padding: const EdgeInsets.symmetric(horizontal: 10),
        textStyle: const TextStyle(fontSize: 15),
      ),
    ),
  );
}
