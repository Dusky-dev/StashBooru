import 'package:flutter/services.dart';
import 'package:material_ui/material_ui.dart';

import '../api/entities.dart';
import '../api/models.dart';
import 'media_browser.dart';
import 'paging.dart';
import 'session.dart';
import 'theme.dart';
import 'toolbar.dart';

/// A searchable grid of Characters, Artists, Copyrights, Tags, Galleries or
/// Collections.
class EntityBrowser extends StatefulWidget {
  const EntityBrowser({super.key, required this.type});
  final EntityType type;

  @override
  State<EntityBrowser> createState() => _EntityBrowserState();
}

class _EntityBrowserState extends State<EntityBrowser> {
  String _query = '';
  EntitySort _sort = EntitySort.name;
  int _seed = newSeed();
  PagedController<Entity>? _controller;

  PageFetcher<Entity> _fetcher() {
    final client = Session.of(context).client;
    final type = widget.type;
    final query = _query;
    final sort = _sort;
    final seed = _seed;
    return (page, perPage) => fetchEntities(
      client,
      type,
      page: page,
      perPage: perPage,
      query: query,
      sort: sort,
      seed: seed,
    );
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    _controller ??= PagedController<Entity>(_fetcher());
  }

  @override
  void dispose() {
    _controller?.dispose();
    super.dispose();
  }

  void _reload() => _controller!.reset(_fetcher());

  @override
  Widget build(BuildContext context) {
    final type = widget.type;
    return Column(
      children: [
        ListToolbar(
          children: [
            SearchField(
              onChanged: (text) {
                _query = text;
                _reload();
              },
            ),
            ChoiceButton<EntitySort>(
              tooltip: 'Sort',
              value: _sort,
              values: EntitySort.values,
              label: (s) => s.label,
              onSelected: (s) {
                setState(() {
                  _sort = s;
                  _seed = newSeed();
                });
                _reload();
              },
            ),
          ],
        ),
        Expanded(
          child: PagedGrid<Entity>(
            controller: _controller!,
            minTileWidth: EntityCard.minWidth(type),
            tileHeight: (w) => EntityCard.heightFor(type, w),
            emptyText: _query.isEmpty
                ? 'No ${type.plural.toLowerCase()} yet.'
                : 'No ${type.plural.toLowerCase()} match “$_query”.',
            itemBuilder: (context, _, e) => EntityCard(entity: e),
          ),
        ),
      ],
    );
  }
}

/// A web UI entity card: image, name, then a divider and the media counts.
class EntityCard extends StatelessWidget {
  const EntityCard({super.key, required this.entity});
  final Entity entity;

  static const _captionHeight = 46.0;
  static const _footerHeight = 38.0;

  static double minWidth(EntityType type) => switch (type) {
    EntityType.character || EntityType.collection => 200,
    _ => 260,
  };

  static double heightFor(EntityType type, double width) =>
      width / type.imageAspect + _captionHeight + 1 + _footerHeight;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    // Artist logos are often transparent and not square: show them whole.
    final fit = switch (entity.type) {
      EntityType.artist || EntityType.tag => BoxFit.contain,
      _ => BoxFit.cover,
    };
    return Card(
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: () => Navigator.of(context).push(
          MaterialPageRoute(builder: (_) => EntityMediaPage(entity: entity)),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Expanded(
              child: NetImage(
                entity.imagePath,
                fit: fit,
                cacheWidth: 300,
                icon: _icon(entity.type),
              ),
            ),
            SizedBox(
              height: _captionHeight,
              child: Padding(
                padding: const EdgeInsets.symmetric(horizontal: 14),
                child: Row(
                  children: [
                    Expanded(
                      child: Text(
                        entity.name,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: theme.textTheme.titleMedium,
                      ),
                    ),
                    if (entity.favorite)
                      const Icon(
                        Icons.favorite,
                        size: 16,
                        color: Color(0xFFFF7373),
                      ),
                  ],
                ),
              ),
            ),
            const Divider(indent: 10, endIndent: 10),
            SizedBox(height: _footerHeight, child: EntityCounts(entity)),
          ],
        ),
      ),
    );
  }
}

/// The icon-and-number counts under a web UI card.
class EntityCounts extends StatelessWidget {
  const EntityCounts(this.entity, {super.key});
  final Entity entity;

  @override
  Widget build(BuildContext context) {
    final items = [
      if (entity.videoCount != null)
        (Icons.play_circle, entity.videoCount!, 'Videos'),
      if (entity.imageCount != null)
        (Icons.image, entity.imageCount!, 'Images'),
    ];
    return Row(
      mainAxisAlignment: MainAxisAlignment.center,
      children: [
        for (final (icon, count, label) in items)
          Tooltip(
            message: '$count $label',
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: 10),
              child: Row(
                children: [
                  Icon(icon, size: 15, color: StashColors.text),
                  const SizedBox(width: 6),
                  Text('$count', style: const TextStyle(fontSize: 14)),
                ],
              ),
            ),
          ),
      ],
    );
  }
}

IconData _icon(EntityType type) => switch (type) {
  EntityType.character => Icons.person,
  EntityType.artist => Icons.videocam,
  EntityType.copyright => Icons.copyright,
  EntityType.tag => Icons.sell,
  EntityType.gallery => Icons.photo_library,
  EntityType.collection => Icons.movie,
};

IconData entityIcon(EntityType type) => _icon(type);

/// The Images and Videos belonging to one entity, under a header like the
/// web detail page.
class EntityMediaPage extends StatelessWidget {
  const EntityMediaPage({super.key, required this.entity});
  final Entity entity;

  @override
  Widget build(BuildContext context) {
    final session = Session.of(context);
    final sources = sourcesFor(session.client, session.caps, entity.scope);
    final theme = Theme.of(context);
    final imageHeight = MediaQuery.sizeOf(context).width >= 600 ? 120.0 : 84.0;
    // Escape goes back on desktop, as it closes the viewer.
    return CallbackShortcuts(
      bindings: {
        const SingleActivator(LogicalKeyboardKey.escape): () =>
            Navigator.of(context).maybePop(),
      },
      child: Focus(
        autofocus: true,
        child: Scaffold(
          appBar: AppBar(title: Text(entity.type.plural)),
          body: MediaBrowser(
            sources: sources,
            emptyText: 'Nothing is linked to this ${entity.type.singular} yet.',
            header: Padding(
              padding: const EdgeInsets.fromLTRB(16, 16, 16, 8),
              child: Row(
                children: [
                  if (entity.imagePath != null)
                    Padding(
                      padding: const EdgeInsets.only(right: 16),
                      child: ClipRRect(
                        borderRadius: BorderRadius.circular(kCardRadius),
                        child: SizedBox(
                          height: imageHeight,
                          width: imageHeight * entity.type.imageAspect,
                          child: NetImage(
                            entity.imagePath,
                            fit: BoxFit.cover,
                            cacheWidth: 240,
                            icon: _icon(entity.type),
                          ),
                        ),
                      ),
                    ),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Row(
                          children: [
                            Flexible(
                              child: Text(
                                entity.name,
                                maxLines: 2,
                                overflow: TextOverflow.ellipsis,
                                style: theme.textTheme.headlineSmall,
                              ),
                            ),
                            if (entity.favorite)
                              const Padding(
                                padding: EdgeInsets.only(left: 8),
                                child: Icon(
                                  Icons.favorite,
                                  size: 18,
                                  color: Color(0xFFFF7373),
                                ),
                              ),
                          ],
                        ),
                        const SizedBox(height: 4),
                        Text(
                          entity.countLabel.isEmpty
                              ? entity.type.singular
                              : '${entity.type.singular} · ${entity.countLabel}',
                          style: theme.textTheme.bodyMedium?.copyWith(
                            color: theme.colorScheme.onSurfaceVariant,
                          ),
                        ),
                      ],
                    ),
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
