import 'dart:async';

import 'package:flutter/services.dart';
import 'package:material_ui/material_ui.dart';

import '../api/booru.dart';
import '../api/client.dart';
import '../api/models.dart';
import 'paging.dart';
import 'session.dart';
import 'stash_image.dart';
import 'theme.dart';
import 'toolbar.dart';

/// Searches the booru sites configured on the server and downloads posts
/// into the library, through the same server routes as the web UI's Booru
/// browser. Sources and their credentials are managed in the web UI.
class BooruBrowser extends StatefulWidget {
  const BooruBrowser({super.key});

  @override
  State<BooruBrowser> createState() => _BooruBrowserState();
}

class _BooruBrowserState extends State<BooruBrowser> {
  late BooruApi _api;
  List<BooruSource>? _sources;
  String? _error;
  BooruSource? _source;
  String _query = '';
  PagedController<BooruPost>? _controller;
  final _search = TextEditingController();

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    final api = BooruApi(Session.of(context).client);
    if (_sources == null && _error == null) {
      _api = api;
      _loadSources();
    }
  }

  @override
  void dispose() {
    _controller?.dispose();
    _search.dispose();
    super.dispose();
  }

  Future<void> _loadSources() async {
    setState(() => _error = null);
    try {
      final sources = await _api.sources();
      if (!mounted) return;
      setState(() {
        _sources = sources;
        if (sources.isNotEmpty) _select(sources.first);
      });
    } on StashException catch (e) {
      if (mounted) setState(() => _error = e.message);
    }
  }

  PageFetcher<BooruPost> _fetcher() {
    final source = _source!;
    final query = _query;
    return (page, _) async {
      final result = await _api.search(source, query, page: page);
      return PageResult.open(result.posts, hasMore: result.hasMore);
    };
  }

  void _select(BooruSource source) {
    _source = source;
    if (_controller == null) {
      _controller = PagedController(_fetcher(), perPage: BooruApi.pageSize);
    } else {
      _controller!.reset(_fetcher());
    }
  }

  void _searchFor(String query) {
    _query = query.trim();
    if (_search.text != query) _search.text = query;
    _controller?.reset(_fetcher());
    setState(() {});
  }

  void _open(int index) async {
    final tag = await Navigator.of(context).push<String>(
      MaterialPageRoute(
        builder: (_) => BooruPostPage(
          api: _api,
          controller: _controller!,
          initialIndex: index,
          session: Session.of(context),
        ),
      ),
    );
    if (tag != null && mounted) _searchFor(tag);
  }

  @override
  Widget build(BuildContext context) {
    final sources = _sources;
    if (_error != null) {
      return _Message(
        text: _error!,
        action: SecondaryButton(
          onPressed: _loadSources,
          child: const Text('Try again'),
        ),
      );
    }
    if (sources == null) {
      return const Center(child: CircularProgressIndicator());
    }
    if (sources.isEmpty) {
      return const _Message(
        text:
            'No booru sites are set up on this server. Add them in the web '
            'UI under Booru → Settings.',
      );
    }
    final source = _source!;
    return Column(
      children: [
        ListToolbar(
          children: [
            ChoiceButton<BooruSource>(
              key: const Key('booru-source'),
              value: source,
              values: sources,
              label: (s) => s.name,
              tooltip: 'Site',
              onSelected: (s) => setState(() => _select(s)),
            ),
            SizedBox(
              width: 280,
              height: 36,
              child: TextField(
                key: const Key('booru-search'),
                controller: _search,
                style: const TextStyle(fontSize: 14),
                textInputAction: TextInputAction.search,
                decoration: InputDecoration(
                  hintText: 'Tags...',
                  suffixIcon: _query.isEmpty
                      ? null
                      : IconButton(
                          tooltip: 'Clear',
                          icon: const Icon(Icons.close, size: 16),
                          onPressed: () => _searchFor(''),
                        ),
                ),
                onSubmitted: _searchFor,
              ),
            ),
            SecondaryButton(
              tooltip: 'Search',
              onPressed: () => _searchFor(_search.text),
              child: const Icon(Icons.search, size: 18),
            ),
          ],
        ),
        if (source.queryHelp.isNotEmpty)
          Padding(
            padding: const EdgeInsets.only(bottom: 4),
            child: Text(
              source.queryHelp,
              style: const TextStyle(color: StashColors.muted, fontSize: 12),
              textAlign: TextAlign.center,
            ),
          ),
        Expanded(
          child: PagedGrid<BooruPost>(
            key: ValueKey(source.id),
            controller: _controller!,
            minTileWidth: 200,
            tileHeight: (w) => w + 28,
            emptyText: _query.isEmpty
                ? 'Nothing found.'
                : 'Nothing matches “$_query” on ${source.name}.',
            itemBuilder: (context, i, post) => _PostTile(
              post: post,
              imageUrl: _api.mediaUrl(post),
              onTap: () => _open(i),
            ),
          ),
        ),
      ],
    );
  }
}

class _Message extends StatelessWidget {
  const _Message({required this.text, this.action});
  final String text;
  final Widget? action;

  @override
  Widget build(BuildContext context) => Center(
    child: Padding(
      padding: const EdgeInsets.all(24),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(text, textAlign: TextAlign.center),
          if (action != null) ...[const SizedBox(height: 12), action!],
        ],
      ),
    ),
  );
}

String _size(int bytes) {
  if (bytes <= 0) return '';
  if (bytes < 1 << 20) return '${(bytes / 1024).round()} KB';
  return '${(bytes / (1 << 20)).toStringAsFixed(1)} MB';
}

String _caption(BooruPost p) => [
  if (p.width > 0 && p.height > 0) '${p.width}×${p.height}',
  if (p.format.isNotEmpty) p.format.toUpperCase(),
  _size(p.bytes),
].where((s) => s.isNotEmpty).join(' · ');

class _PostTile extends StatelessWidget {
  const _PostTile({
    required this.post,
    required this.imageUrl,
    required this.onTap,
  });

  final BooruPost post;
  final String? imageUrl;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final headers = Session.of(context).client.authHeaders;
    final dpr = MediaQuery.devicePixelRatioOf(context);
    return Card(
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
                  ColoredBox(
                    color: Colors.black,
                    child: imageUrl == null
                        ? const Icon(Icons.image_not_supported)
                        : Image(
                            image: StashNetworkImage(
                              imageUrl!,
                              headers: headers,
                              cacheWidth: (300 * dpr).round(),
                            ),
                            fit: BoxFit.contain,
                            gaplessPlayback: true,
                            errorBuilder: (_, _, _) =>
                                const Icon(Icons.broken_image),
                          ),
                  ),
                  if (post.isVideo || post.attachments > 1)
                    Positioned(
                      right: 6,
                      bottom: 6,
                      child: _Tag(
                        post.isVideo
                            ? post.format.toUpperCase()
                            : '${post.attachments} files',
                      ),
                    ),
                ],
              ),
            ),
            SizedBox(
              height: 28,
              child: Center(
                child: Text(
                  _caption(post),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(
                    fontSize: 12,
                    color: StashColors.muted,
                  ),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _Tag extends StatelessWidget {
  const _Tag(this.text);
  final String text;

  @override
  Widget build(BuildContext context) => DecoratedBox(
    decoration: BoxDecoration(
      color: Colors.black.withValues(alpha: 0.7),
      borderRadius: BorderRadius.circular(3),
    ),
    child: Padding(
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
      child: Text(
        text,
        style: const TextStyle(color: Colors.white, fontSize: 12),
      ),
    ),
  );
}

/// A full-screen post with its tags and a Download button. Swipe or use the
/// arrow keys for the next post; tapping a tag searches for it.
class BooruPostPage extends StatefulWidget {
  const BooruPostPage({
    super.key,
    required this.api,
    required this.controller,
    required this.initialIndex,
    required this.session,
  });

  final BooruApi api;
  final PagedController<BooruPost> controller;
  final int initialIndex;

  /// Passed in because the route sits above the browser's own context.
  final Session session;

  @override
  State<BooruPostPage> createState() => _BooruPostPageState();
}

class _BooruPostPageState extends State<BooruPostPage> {
  late final PageController _pages = PageController(
    initialPage: widget.initialIndex,
  );
  late int _index = widget.initialIndex;

  /// Download state per post id, kept while the page is open.
  final Map<String, BooruDownload> _downloads = {};
  final Map<String, String> _downloadErrors = {};
  final Set<String> _starting = {};
  Timer? _poll;

  @override
  void initState() {
    super.initState();
    widget.controller.addListener(_changed);
  }

  @override
  void dispose() {
    widget.controller.removeListener(_changed);
    _poll?.cancel();
    _pages.dispose();
    super.dispose();
  }

  void _changed() {
    if (mounted) setState(() {});
  }

  void _go(int delta) {
    final next = _index + delta;
    if (next < 0 || next >= widget.controller.items.length) return;
    _pages.animateToPage(
      next,
      duration: const Duration(milliseconds: 200),
      curve: Curves.easeOut,
    );
  }

  /// Download folder chosen for this visit, when the server has no default.
  static String? _chosenFolder;

  Future<String?> _destination(BooruDownloadSettings prefs) async {
    if (prefs.defaultDirectory.isNotEmpty) return prefs.defaultDirectory;
    if (prefs.roots.length == 1) return prefs.roots.single;
    if (_chosenFolder != null && prefs.roots.contains(_chosenFolder)) {
      return _chosenFolder;
    }
    if (prefs.roots.isEmpty || !mounted) return null;
    final chosen = await showDialog<String>(
      context: context,
      builder: (context) => SimpleDialog(
        title: const Text('Download to'),
        children: [
          for (final root in prefs.roots)
            SimpleDialogOption(
              onPressed: () => Navigator.of(context).pop(root),
              child: Text(root),
            ),
        ],
      ),
    );
    if (chosen != null) _chosenFolder = chosen;
    return chosen;
  }

  Future<void> _download(BooruPost post) async {
    setState(() {
      _starting.add(post.id);
      _downloadErrors.remove(post.id);
    });
    try {
      final prefs = await widget.api.downloadSettings();
      final destination = await _destination(prefs);
      if (destination == null) {
        if (prefs.roots.isEmpty) {
          _downloadErrors[post.id] =
              'The server has no library folder to download into.';
        }
        return;
      }
      final d = await widget.api.download(
        post,
        destination: destination,
        convert: prefs.convert,
      );
      _downloads[post.id] = d;
      _schedulePoll();
    } on StashException catch (e) {
      _downloadErrors[post.id] = e.message;
    } finally {
      _starting.remove(post.id);
      if (mounted) setState(() {});
    }
  }

  void _schedulePoll() {
    _poll?.cancel();
    if (_downloads.values.every((d) => d.finished)) return;
    _poll = Timer(const Duration(milliseconds: 1500), () async {
      for (final entry in _downloads.entries.toList()) {
        if (entry.value.finished) continue;
        try {
          _downloads[entry.key] = await widget.api.downloadStatus(entry.value);
        } on StashException catch (e) {
          _downloads.remove(entry.key);
          _downloadErrors[entry.key] = e.message;
        }
      }
      if (!mounted) return;
      setState(() {});
      _schedulePoll();
    });
  }

  @override
  Widget build(BuildContext context) {
    final items = widget.controller.items;
    final post = _index < items.length ? items[_index] : null;
    final wide = MediaQuery.sizeOf(context).width >= 900;
    final viewer = PageView.builder(
      controller: _pages,
      itemCount: items.length,
      onPageChanged: (i) {
        setState(() => _index = i);
        if (i >= items.length - 5) widget.controller.loadMore();
      },
      itemBuilder: (context, i) => _PostImage(
        key: ValueKey(items[i].mediaKey),
        api: widget.api,
        post: items[i],
        headers: widget.session.client.authHeaders,
      ),
    );
    final info = post == null ? const SizedBox() : _info(post);
    return CallbackShortcuts(
      bindings: {
        const SingleActivator(LogicalKeyboardKey.escape): () =>
            Navigator.of(context).maybePop(),
        const SingleActivator(LogicalKeyboardKey.arrowRight): () => _go(1),
        const SingleActivator(LogicalKeyboardKey.arrowLeft): () => _go(-1),
      },
      child: Focus(
        autofocus: true,
        child: Scaffold(
          appBar: AppBar(
            title: Text(
              post == null
                  ? 'Booru'
                  : post.title.isNotEmpty
                  ? post.title
                  : 'Post ${post.id}',
            ),
          ),
          body: wide
              ? Row(
                  children: [
                    Expanded(
                      child: ColoredBox(color: Colors.black, child: viewer),
                    ),
                    SizedBox(
                      width: 340,
                      child: ColoredBox(color: StashColors.card, child: info),
                    ),
                  ],
                )
              : Column(
                  children: [
                    Expanded(
                      flex: 3,
                      child: ColoredBox(color: Colors.black, child: viewer),
                    ),
                    Expanded(
                      flex: 2,
                      child: ColoredBox(color: StashColors.card, child: info),
                    ),
                  ],
                ),
        ),
      ),
    );
  }

  Widget _info(BooruPost post) {
    final download = _downloads[post.id];
    final error = _downloadErrors[post.id];
    final busy =
        _starting.contains(post.id) || (download != null && !download.finished);
    String? status;
    if (error != null) {
      status = error;
    } else if (download != null) {
      status = switch (download.status) {
        'complete' =>
          download.filename.isEmpty
              ? 'Saved to the library.'
              : 'Saved to the library as ${download.filename}.',
        'failed' =>
          download.error.isEmpty ? 'Download failed.' : download.error,
        'cancelled' => 'Download cancelled.',
        'queued' => 'Waiting to download...',
        _ => 'Downloading...',
      };
    }
    final groups = <String, List<BooruTag>>{};
    for (final t in post.tags) {
      groups
          .putIfAbsent(t.category.isEmpty ? 'general' : t.category, () => [])
          .add(t);
    }
    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        Wrap(
          spacing: 8,
          runSpacing: 8,
          crossAxisAlignment: WrapCrossAlignment.center,
          children: [
            FilledButton.icon(
              key: const Key('booru-download'),
              onPressed:
                  !post.downloadAvailable || busy || download?.succeeded == true
                  ? null
                  : () => _download(post),
              icon: busy
                  ? const SizedBox.square(
                      dimension: 16,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : Icon(
                      download?.succeeded == true
                          ? Icons.check
                          : Icons.download,
                      size: 18,
                    ),
              label: Text(
                download?.succeeded == true
                    ? 'Downloaded'
                    : 'Download to library',
              ),
            ),
            if (post.postUrl.isNotEmpty)
              SecondaryButton(
                tooltip: post.postUrl,
                onPressed: () {
                  Clipboard.setData(ClipboardData(text: post.postUrl));
                  ScaffoldMessenger.of(context).showSnackBar(
                    const SnackBar(content: Text('Post link copied')),
                  );
                },
                child: const Text('Copy link'),
              ),
          ],
        ),
        if (!post.downloadAvailable)
          const Padding(
            padding: EdgeInsets.only(top: 8),
            child: Text(
              'This post has no downloadable file.',
              style: TextStyle(color: StashColors.muted),
            ),
          ),
        if (status != null)
          Padding(
            padding: const EdgeInsets.only(top: 8),
            child: Text(
              status,
              key: const Key('booru-download-status'),
              style: TextStyle(
                color: error != null || download?.status == 'failed'
                    ? StashColors.danger
                    : StashColors.muted,
              ),
            ),
          ),
        const SizedBox(height: 12),
        Text(
          [
            _caption(post),
            if (post.rating.isNotEmpty) 'Rating ${post.rating}',
            if (post.score != null) 'Score ${post.score}',
            if (post.author.isNotEmpty) 'By ${post.author}',
          ].where((s) => s.isNotEmpty).join(' · '),
          style: const TextStyle(color: StashColors.muted),
        ),
        for (final entry in groups.entries) ...[
          const SizedBox(height: 12),
          Text(
            entry.key[0].toUpperCase() + entry.key.substring(1),
            style: const TextStyle(fontWeight: FontWeight.w500),
          ),
          const SizedBox(height: 6),
          Wrap(
            spacing: 6,
            runSpacing: 6,
            children: [
              for (final t in entry.value)
                ActionChip(
                  label: Text(t.label),
                  tooltip: 'Search for ${t.name}',
                  onPressed: () => Navigator.of(context).pop(t.name),
                ),
            ],
          ),
        ],
      ],
    );
  }
}

class _PostImage extends StatelessWidget {
  const _PostImage({
    super.key,
    required this.api,
    required this.post,
    required this.headers,
  });

  final BooruApi api;
  final BooruPost post;
  final Map<String, String> headers;

  @override
  Widget build(BuildContext context) {
    final preview = api.mediaUrl(post);
    final sample = api.mediaUrl(post, sample: true);
    Widget image(String url) => Image(
      image: StashNetworkImage(url, headers: headers),
      fit: BoxFit.contain,
      gaplessPlayback: true,
      frameBuilder: (context, child, frame, sync) =>
          frame == null && !sync ? const SizedBox() : child,
      errorBuilder: (_, _, _) => const SizedBox(),
    );
    return InteractiveViewer(
      maxScale: 8,
      child: SizedBox.expand(
        child: Stack(
          fit: StackFit.expand,
          children: [
            if (preview != null) image(preview),
            if (sample != null) image(sample),
            if (preview == null && sample == null)
              const Center(
                child: Text(
                  'No preview available.',
                  style: TextStyle(color: Colors.white70),
                ),
              ),
            if (post.isVideo)
              const Positioned(
                left: 0,
                right: 0,
                bottom: 12,
                child: Center(
                  child: _Tag('Video: download it to play in the library'),
                ),
              ),
          ],
        ),
      ),
    );
  }
}
