import 'package:material_ui/material_ui.dart';

import '../api/client.dart';
import '../api/models.dart';

typedef PageFetcher<T> = Future<PageResult<T>> Function(int page, int perPage);

/// Loads a server list one page at a time. Shared by a grid and the viewer
/// opened from it, so swiping past the last loaded item keeps loading.
class PagedController<T> extends ChangeNotifier {
  PagedController(this._fetch, {this.perPage = 40});

  PageFetcher<T> _fetch;
  final int perPage;
  final List<T> items = [];
  int? total;
  bool loading = false;
  String? error;
  int _page = 0;
  int _generation = 0;

  bool get hasMore => total == null || items.length < total!;

  /// Replaces the query (new search or sort) and starts over.
  void reset([PageFetcher<T>? fetch]) {
    if (fetch != null) _fetch = fetch;
    _generation++;
    items.clear();
    total = null;
    error = null;
    loading = false;
    _page = 0;
    notifyListeners();
    loadMore();
  }

  Future<void> loadMore() async {
    if (loading || !hasMore || error != null) return;
    loading = true;
    notifyListeners();
    final generation = _generation;
    try {
      final result = await _fetch(_page + 1, perPage);
      if (generation != _generation) return;
      _page++;
      items.addAll(result.items);
      total = result.total;
      // A short page means the server has nothing more, whatever the count.
      if (result.items.length < perPage) total = items.length;
    } on StashException catch (e) {
      if (generation == _generation) error = e.message;
    } catch (e) {
      if (generation == _generation) error = 'Something went wrong: $e';
    } finally {
      if (generation == _generation) {
        loading = false;
        notifyListeners();
      }
    }
  }

  Future<void> retry() async {
    error = null;
    await loadMore();
  }
}

/// A scrolling grid over a [PagedController] with loading, error and empty
/// states.
class PagedGrid<T> extends StatefulWidget {
  const PagedGrid({
    super.key,
    required this.controller,
    required this.itemBuilder,
    required this.maxExtent,
    required this.aspectRatio,
    required this.emptyText,
  });

  final PagedController<T> controller;
  final Widget Function(BuildContext, int index, T item) itemBuilder;
  final double maxExtent;
  final double aspectRatio;
  final String emptyText;

  @override
  State<PagedGrid<T>> createState() => _PagedGridState<T>();
}

class _PagedGridState<T> extends State<PagedGrid<T>> {
  final _scroll = ScrollController();

  @override
  void initState() {
    super.initState();
    _scroll.addListener(_maybeLoad);
    widget.controller.addListener(_changed);
    if (widget.controller.items.isEmpty && widget.controller.total == null) {
      widget.controller.loadMore();
    }
  }

  @override
  void didUpdateWidget(PagedGrid<T> old) {
    super.didUpdateWidget(old);
    if (old.controller != widget.controller) {
      old.controller.removeListener(_changed);
      widget.controller.addListener(_changed);
    }
  }

  @override
  void dispose() {
    widget.controller.removeListener(_changed);
    _scroll.dispose();
    super.dispose();
  }

  void _changed() {
    if (!mounted) return;
    setState(() {});
    // Keep loading until the viewport is filled on tall/wide screens.
    WidgetsBinding.instance.addPostFrameCallback((_) => _maybeLoad());
  }

  void _maybeLoad() {
    if (!_scroll.hasClients) return;
    final pos = _scroll.position;
    if (pos.pixels >= pos.maxScrollExtent - 800) widget.controller.loadMore();
  }

  @override
  Widget build(BuildContext context) {
    final c = widget.controller;
    final theme = Theme.of(context);
    if (c.items.isEmpty) {
      if (c.error != null) {
        return _ErrorState(message: c.error!, onRetry: c.retry);
      }
      if (c.loading || c.total == null) {
        return const Center(child: CircularProgressIndicator());
      }
      return Center(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Text(
            widget.emptyText,
            textAlign: TextAlign.center,
            style: theme.textTheme.bodyLarge,
          ),
        ),
      );
    }
    return RefreshIndicator(
      onRefresh: () async => c.reset(),
      child: CustomScrollView(
        controller: _scroll,
        physics: const AlwaysScrollableScrollPhysics(),
        slivers: [
          SliverPadding(
            padding: const EdgeInsets.all(8),
            sliver: SliverGrid(
              gridDelegate: SliverGridDelegateWithMaxCrossAxisExtent(
                maxCrossAxisExtent: widget.maxExtent,
                childAspectRatio: widget.aspectRatio,
                mainAxisSpacing: 8,
                crossAxisSpacing: 8,
              ),
              delegate: SliverChildBuilderDelegate(
                (context, i) => widget.itemBuilder(context, i, c.items[i]),
                childCount: c.items.length,
              ),
            ),
          ),
          SliverToBoxAdapter(
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: Center(
                child: c.error != null
                    ? _ErrorState(message: c.error!, onRetry: c.retry)
                    : c.loading
                    ? const CircularProgressIndicator()
                    : Text(
                        '${c.items.length} of ${c.total ?? c.items.length}',
                        style: theme.textTheme.bodySmall,
                      ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _ErrorState extends StatelessWidget {
  const _ErrorState({required this.message, required this.onRetry});
  final String message;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) => Center(
    child: Padding(
      padding: const EdgeInsets.all(24),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(
            Icons.cloud_off,
            size: 40,
            color: Theme.of(context).colorScheme.error,
          ),
          const SizedBox(height: 12),
          Text(message, textAlign: TextAlign.center),
          const SizedBox(height: 12),
          FilledButton.tonal(
            onPressed: onRetry,
            child: const Text('Try again'),
          ),
        ],
      ),
    ),
  );
}
