import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../api/models.dart';
import 'paging.dart';
import 'session.dart';
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
  const MediaBrowser({super.key, required this.sources, this.emptyText});

  /// One source, or Images and Videos shown as a switch.
  final List<MediaSource> sources;
  final String? emptyText;

  @override
  State<MediaBrowser> createState() => _MediaBrowserState();
}

class _MediaBrowserState extends State<MediaBrowser> {
  int _sourceIndex = 0;
  String _query = '';
  SortOption _sort = SortOption.newest;
  int _seed = newSeed();
  Timer? _debounce;
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
    _debounce?.cancel();
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
                  decoration: const InputDecoration(
                    prefixIcon: Icon(Icons.search),
                    hintText: 'Search',
                    isDense: true,
                    border: OutlineInputBorder(),
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
              DropdownButton<SortOption>(
                value: _sort,
                underline: const SizedBox(),
                items: [
                  for (final s in sorts)
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
              if (_sort == SortOption.random)
                IconButton(
                  tooltip: 'Shuffle again',
                  icon: const Icon(Icons.shuffle),
                  onPressed: () {
                    _seed = newSeed();
                    _reload();
                  },
                ),
              if (widget.sources.length > 1)
                SegmentedButton<int>(
                  segments: [
                    for (var i = 0; i < widget.sources.length; i++)
                      ButtonSegment(
                        value: i,
                        label: Text(widget.sources[i].label),
                      ),
                  ],
                  selected: {_sourceIndex},
                  onSelectionChanged: (s) {
                    setState(() => _sourceIndex = s.first);
                    _reload();
                  },
                ),
            ],
          ),
        ),
        Expanded(
          child: PagedGrid<MediaRef>(
            controller: _controller,
            maxExtent: 220,
            aspectRatio: 1,
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

class MediaTile extends StatelessWidget {
  const MediaTile({super.key, required this.item, required this.onTap});
  final MediaRef item;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final duration = item.duration;
    return Semantics(
      label: item.title,
      button: true,
      child: Material(
        clipBehavior: Clip.antiAlias,
        borderRadius: BorderRadius.circular(8),
        child: InkWell(
          onTap: onTap,
          child: Stack(
            fit: StackFit.expand,
            children: [
              NetImage(
                item.thumbnail,
                cacheWidth: 220,
                icon: item.playsAsVideo ? Icons.movie : Icons.image,
              ),
              if (item.playsAsVideo)
                Positioned(
                  right: 6,
                  bottom: 6,
                  child: DecoratedBox(
                    decoration: BoxDecoration(
                      color: Colors.black.withValues(alpha: 0.7),
                      borderRadius: BorderRadius.circular(4),
                    ),
                    child: Padding(
                      padding: const EdgeInsets.symmetric(
                        horizontal: 6,
                        vertical: 2,
                      ),
                      child: Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          const Icon(
                            Icons.play_arrow,
                            size: 14,
                            color: Colors.white,
                          ),
                          if (duration != null && duration > 0)
                            Text(
                              formatDuration(duration),
                              style: const TextStyle(
                                color: Colors.white,
                                fontSize: 12,
                              ),
                            ),
                        ],
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
