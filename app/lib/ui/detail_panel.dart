import 'package:material_ui/material_ui.dart';

import '../api/entities.dart';
import '../api/models.dart';
import 'entity_browser.dart';
import 'session.dart';

/// Title, related Characters/Artist/Copyrights/Tags and file facts for one
/// Image or Video. Tapping a chip opens that entity's media.
class DetailPanel extends StatefulWidget {
  const DetailPanel({super.key, required this.item, this.scroll});
  final MediaRef item;
  final ScrollController? scroll;

  @override
  State<DetailPanel> createState() => _DetailPanelState();
}

class _DetailPanelState extends State<DetailPanel> {
  Future<MediaDetail>? _detail;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    final session = Session.of(context);
    _detail ??= fetchDetail(session.client, session.caps, widget.item);
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return FutureBuilder<MediaDetail>(
      future: _detail,
      builder: (context, snap) {
        if (snap.hasError) {
          return Padding(
            padding: const EdgeInsets.all(24),
            child: Text('${snap.error}', textAlign: TextAlign.center),
          );
        }
        final d = snap.data;
        if (d == null) {
          return const Padding(
            padding: EdgeInsets.all(32),
            child: Center(child: CircularProgressIndicator()),
          );
        }
        return ListView(
          controller: widget.scroll,
          padding: const EdgeInsets.fromLTRB(20, 0, 20, 24),
          children: [
            Text(d.title, style: theme.textTheme.titleLarge),
            const SizedBox(height: 4),
            Text(
              [
                widget.item.kind == MediaKind.image ? 'Image' : 'Video',
                if (d.date != null) d.date!,
                if (d.resolution != null) d.resolution!,
              ].join(' · '),
              style: theme.textTheme.bodyMedium?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
            ),
            if (d.details != null) ...[
              const SizedBox(height: 12),
              Text(d.details!),
            ],
            for (final entry in d.links.entries) ...[
              const SizedBox(height: 16),
              Text(
                entry.value.length == 1 ? entry.key.singular : entry.key.plural,
                style: theme.textTheme.titleSmall,
              ),
              const SizedBox(height: 6),
              Wrap(
                spacing: 6,
                runSpacing: 6,
                children: [
                  for (final e in entry.value)
                    ActionChip(
                      avatar: e.imagePath == null || e.type == EntityType.tag
                          ? null
                          : CircleAvatar(
                              child: ClipOval(
                                child: SizedBox.expand(
                                  child: NetImage(e.imagePath, cacheWidth: 48),
                                ),
                              ),
                            ),
                      label: Text(e.name),
                      onPressed: () => Navigator.of(context).push(
                        MaterialPageRoute(
                          builder: (_) => EntityMediaPage(entity: e),
                        ),
                      ),
                    ),
                ],
              ),
            ],
            if (d.path != null) ...[
              const SizedBox(height: 16),
              Text('File', style: theme.textTheme.titleSmall),
              const SizedBox(height: 4),
              SelectableText(d.path!, style: theme.textTheme.bodySmall),
            ],
          ],
        );
      },
    );
  }
}
