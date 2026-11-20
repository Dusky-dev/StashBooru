import 'package:material_ui/material_ui.dart';

import '../api/models.dart';
import 'paging.dart';
import 'session.dart';
import 'theme.dart';
import 'toolbar.dart';
import 'viewer.dart';

String formatDuration(double seconds) {
  final d = Duration(milliseconds: (seconds * 1000).round());
  final h = d.inHours;
  final m = d.inMinutes.remainder(60).toString().padLeft(h > 0 ? 2 : 1, '0');
  final s = d.inSeconds.remainder(60).toString().padLeft(2, '0');
  return h > 0 ? '$h:$m:$s' : '$m:$s';
}

/// A searchable, sortable grid of Images and/or Videos.
class MediaBrowser extends StatefulWidget {
  const MediaBrowser({
    super.key,
    required this.sources,
    this.emptyText,
    this.header,
  });

  /// One source, or Images and Videos shown as tabs.
  final List<MediaSource> sources;
  final String? emptyText;

  /// Shown above the tabs, such as an entity's name and image.
  final Widget? header;

  @override
  State<MediaBrowser> createState() => _MediaBrowserState();
}

class _MediaBrowserState extends State<MediaBrowser> {
  int _sourceIndex = 0;
  String _query = '';
  SortOption _sort = SortOption.newest;
  int _seed = newSeed();
  late final PagedController<MediaRef> _controller = PagedController<MediaRef>(
    _fetcher(),
  );

  MediaSource get _source => widget.sources[_sourceIndex];

  PageFetcher<MediaRef> _fetcher() {
    final source = _source;
    final query = _query;
    final sort = _sort;
    final seed = _seed;
    return (page, perPage) => source.fetch(page, perPage, query, sort, seed);
  }

  void _reload() => _controller.reset(_fetcher());

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (widget.sources.isEmpty) {
      return const Center(child: Text('Nothing to show here.'));
    }
    final sorts = _source.sorts;
    if (!sorts.contains(_sort)) _sort = sorts.first;
    return Column(
      children: [
        ?widget.header,
        if (widget.sources.length > 1)
          NavTabs(
            labels: [for (final s in widget.sources) s.label],
            selected: _sourceIndex,
            onSelected: (i) {
              if (i == _sourceIndex) return;
              setState(() => _sourceIndex = i);
              _reload();
            },
          ),
        ListToolbar(
          children: [
            SearchField(
              onChanged: (text) {
                _query = text;
                _reload();
              },
            ),
            ChoiceButton<SortOption>(
              tooltip: 'Sort',
              value: _sort,
              values: sorts,
              label: (s) => s.label,
              onSelected: (s) {
                setState(() {
                  _sort = s;
                  _seed = newSeed();
                });
                _reload();
              },
            ),
            if (_sort == SortOption.random)
              SecondaryButton(
                tooltip: 'Shuffle again',
                onPressed: () {
                  _seed = newSeed();
                  _reload();
                },
                child: const Icon(Icons.shuffle, size: 18),
              ),
          ],
        ),
        Expanded(
          child: PagedGrid<MediaRef>(
            controller: _controller,
            minTileWidth: 280,
            tileHeight: MediaTile.heightFor,
            emptyText: _query.isEmpty
                ? (widget.emptyText ?? 'Nothing here yet.')
                : 'Nothing matches “$_query”.',
            itemBuilder: (context, index, item) => MediaTile(
              item: item,
              onTap: () => Navigator.of(context).push(
                MaterialPageRoute(
                  builder: (_) =>
                      ViewerPage(controller: _controller, initialIndex: index),
                ),
              ),
            ),
          ),
        ),
      ],
    );
  }
}

/// A web UI media card: the thumbnail shown whole, then the title.
class MediaTile extends StatelessWidget {
  const MediaTile({super.key, required this.item, required this.onTap});
  final MediaRef item;
  final VoidCallback onTap;

  static const _captionHeight = 48.0;

  /// The web cards are about 300 wide with a 240 high preview.
  static double heightFor(double width) => width * 0.79 + _captionHeight;

  @override
  Widget build(BuildContext context) {
    final duration = item.duration;
    return Semantics(
      label: item.title,
      button: true,
      child: Card(
        clipBehavior: Clip.antiAlias,
        child: InkWell(
          onTap: onTap,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Expanded(
                child: Stack(
                  fit: StackFit.expand,
                  children: [
                    NetImage(
                      item.thumbnail,
                      fit: BoxFit.contain,
                      cacheWidth: 320,
                      icon: item.playsAsVideo ? Icons.movie : Icons.image,
                    ),
                    if (item.playsAsVideo)
                      Positioned(
                        right: 6,
                        bottom: 6,
                        child: _Badge(
                          icon: Icons.play_arrow,
                          text: duration != null && duration > 0
                              ? formatDuration(duration)
                              : null,
                        ),
                      ),
                  ],
                ),
              ),
              SizedBox(
                height: _captionHeight,
                child: Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 14),
                  child: Align(
                    alignment: Alignment.centerLeft,
                    child: Text(
                      item.title,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: Theme.of(context).textTheme.titleMedium,
                    ),
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _Badge extends StatelessWidget {
  const _Badge({required this.icon, this.text});
  final IconData icon;
  final String? text;

  @override
  Widget build(BuildContext context) => DecoratedBox(
    decoration: BoxDecoration(
      color: Colors.black.withValues(alpha: 0.7),
      borderRadius: BorderRadius.circular(3),
    ),
    child: Padding(
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 14, color: Colors.white),
          if (text != null)
            Text(
              text!,
              style: const TextStyle(color: Colors.white, fontSize: 12),
            ),
        ],
      ),
    ),
  );
}
