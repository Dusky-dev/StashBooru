import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../api/entities.dart';
import '../api/models.dart';
import 'media_browser.dart';
import 'paging.dart';
import 'session.dart';

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
  Timer? _debounce;
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
    _debounce?.cancel();
    _controller?.dispose();
    super.dispose();
  }

  void _reload() => _controller!.reset(_fetcher());

  @override
  Widget build(BuildContext context) {
    final type = widget.type;
    // Leave room under the image for the name and counts.
    const textHeight = 52.0;
    const width = 170.0;
    final aspect = width / (width / type.imageAspect + textHeight);
    return Column(
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(8, 8, 8, 0),
          child: Wrap(
            spacing: 8,
            runSpacing: 8,
            crossAxisAlignment: WrapCrossAlignment.center,
            children: [
              SizedBox(
                width: 280,
                child: TextField(
                  decoration: InputDecoration(
                    prefixIcon: const Icon(Icons.search),
                    hintText: 'Search ${type.plural.toLowerCase()}',
                    isDense: true,
                    border: const OutlineInputBorder(),
                  ),
                  onChanged: (text) {
                    _debounce?.cancel();
                    _debounce = Timer(const Duration(milliseconds: 400), () {
                      _query = text;
                      _reload();
                    });
                  },
                ),
              ),
              DropdownButton<EntitySort>(
                value: _sort,
                underline: const SizedBox(),
                items: [
                  for (final s in EntitySort.values)
                    DropdownMenuItem(value: s, child: Text('Sort: ${s.label}')),
                ],
                onChanged: (s) {
                  if (s == null) return;
                  setState(() {
                    _sort = s;
                    _seed = newSeed();
                  });
                  _reload();
                },
              ),
            ],
          ),
        ),
        Expanded(
          child: PagedGrid<Entity>(
            controller: _controller!,
            maxExtent: width + 30,
            aspectRatio: aspect,
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

class EntityCard extends StatelessWidget {
  const EntityCard({super.key, required this.entity});
  final Entity entity;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    // Artist logos are often transparent and not square: show them whole.
    final fit = entity.type == EntityType.artist
        ? BoxFit.contain
        : BoxFit.cover;
    return Card(
      clipBehavior: Clip.antiAlias,
      margin: EdgeInsets.zero,
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
                cacheWidth: 200,
                icon: _icon(entity.type),
              ),
            ),
            Padding(
              padding: const EdgeInsets.fromLTRB(8, 6, 8, 6),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Expanded(
                        child: Text(
                          entity.name,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: theme.textTheme.titleSmall,
                        ),
                      ),
                      if (entity.favorite)
                        Icon(
                          Icons.favorite,
                          size: 14,
                          color: theme.colorScheme.primary,
                        ),
                    ],
                  ),
                  Text(
                    entity.countLabel,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: theme.colorScheme.onSurfaceVariant,
                    ),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

IconData _icon(EntityType type) => switch (type) {
  EntityType.character => Icons.person,
  EntityType.artist => Icons.brush,
  EntityType.copyright => Icons.copyright,
  EntityType.tag => Icons.sell,
  EntityType.gallery => Icons.photo_library,
  EntityType.collection => Icons.video_library,
};

IconData entityIcon(EntityType type) => _icon(type);

/// The Images and Videos belonging to one entity.
class EntityMediaPage extends StatelessWidget {
  const EntityMediaPage({super.key, required this.entity});
  final Entity entity;

  @override
  Widget build(BuildContext context) {
    final session = Session.of(context);
    final sources = sourcesFor(session.client, session.caps, entity.scope);
    return Scaffold(
      appBar: AppBar(
        title: Text(entity.name),
        bottom: PreferredSize(
          preferredSize: const Size.fromHeight(20),
          child: Padding(
            padding: const EdgeInsets.only(bottom: 6),
            child: Text(
              entity.type.singular,
              style: Theme.of(context).textTheme.labelMedium,
            ),
          ),
        ),
      ),
      body: MediaBrowser(
        sources: sources,
        emptyText: 'Nothing is linked to this ${entity.type.singular} yet.',
      ),
    );
  }
}
