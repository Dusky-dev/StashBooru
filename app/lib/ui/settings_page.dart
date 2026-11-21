import 'package:flutter/services.dart';
import 'package:material_ui/material_ui.dart';

import '../state/app_settings.dart';
import 'session.dart';
import 'theme.dart';

/// This device's preferences and the connected server. Server-wide settings
/// (library, tasks, booru sources) stay in the web UI.
class SettingsPage extends StatelessWidget {
  const SettingsPage({super.key});

  @override
  Widget build(BuildContext context) {
    final settings = AppSettingsScope.of(context);
    final session = Session.of(context);
    return CallbackShortcuts(
      bindings: {
        const SingleActivator(LogicalKeyboardKey.escape): () =>
            Navigator.of(context).maybePop(),
      },
      child: Focus(
        autofocus: true,
        child: Scaffold(
          appBar: AppBar(title: const Text('Settings')),
          body: Align(
            alignment: Alignment.topCenter,
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 720),
              child: ListView(
                padding: const EdgeInsets.all(16),
                children: [
                  _Group(
                    title: 'Server',
                    children: [
                      ListTile(
                        title: SelectableText(
                          session.client.baseUrl.toString(),
                        ),
                        subtitle: Text(
                          '${session.caps.isStashBooru ? 'StashBooru' : 'Stash'} '
                          '${session.caps.version}'
                          '${session.caps.isStashBooru ? '' : ' · All, Copyrights and Booru need a StashBooru server'}',
                        ),
                        trailing: SecondaryButton(
                          key: const Key('disconnect'),
                          onPressed: () {
                            Navigator.of(context).maybePop();
                            session.onDisconnect();
                          },
                          child: const Text('Disconnect'),
                        ),
                      ),
                    ],
                  ),
                  _Group(
                    title: 'Browsing',
                    children: [
                      ListTile(
                        title: const Text('Card size'),
                        subtitle: const Text('How large items are in lists'),
                        trailing: SegmentedButton<CardSize>(
                          showSelectedIcon: false,
                          segments: [
                            for (final c in CardSize.values)
                              ButtonSegment(value: c, label: Text(c.label)),
                          ],
                          selected: {settings.cardSize},
                          onSelectionChanged: (v) =>
                              settings.setCardSize(v.first),
                        ),
                      ),
                      SwitchListTile(
                        key: const Key('full-size-images'),
                        title: const Text('Load full-size images'),
                        subtitle: const Text(
                          'Turn off to show only thumbnails and save data',
                        ),
                        value: settings.fullSizeImages,
                        onChanged: settings.setFullSizeImages,
                      ),
                    ],
                  ),
                  _Group(
                    title: 'Playback',
                    children: [
                      SwitchListTile(
                        key: const Key('autoplay'),
                        title: const Text('Play videos automatically'),
                        value: settings.autoplay,
                        onChanged: settings.setAutoplay,
                      ),
                      SwitchListTile(
                        title: const Text('Start muted'),
                        value: settings.startMuted,
                        onChanged: settings.setStartMuted,
                      ),
                      SwitchListTile(
                        title: const Text('Loop videos'),
                        value: settings.loopVideos,
                        onChanged: settings.setLoopVideos,
                      ),
                    ],
                  ),
                  _Group(
                    title: 'Storage',
                    children: [
                      ListTile(
                        title: const Text('Image cache'),
                        subtitle: const Text(
                          'Thumbnails kept in memory while the app runs',
                        ),
                        trailing: SecondaryButton(
                          onPressed: () {
                            PaintingBinding.instance.imageCache
                              ..clear()
                              ..clearLiveImages();
                            ScaffoldMessenger.of(context).showSnackBar(
                              const SnackBar(
                                content: Text('Image cache cleared'),
                              ),
                            );
                          },
                          child: const Text('Clear'),
                        ),
                      ),
                    ],
                  ),
                  const _Group(
                    title: 'About',
                    children: [
                      ListTile(
                        title: Text('StashBooru app'),
                        subtitle: Text('Version $kAppVersion'),
                      ),
                    ],
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _Group extends StatelessWidget {
  const _Group({required this.title, required this.children});
  final String title;
  final List<Widget> children;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.only(bottom: 16),
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(4, 0, 4, 8),
          child: Text(
            title,
            style: const TextStyle(fontSize: 18, fontWeight: FontWeight.w500),
          ),
        ),
        Card(child: Column(children: children)),
      ],
    ),
  );
}
