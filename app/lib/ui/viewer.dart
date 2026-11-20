import 'dart:async';

import 'package:flutter/services.dart';
import 'package:material_ui/material_ui.dart';
import 'package:media_kit/media_kit.dart';
import 'package:media_kit_video/media_kit_video.dart';

import '../api/models.dart';
import 'detail_panel.dart';
import 'paging.dart';
import 'session.dart';

/// Full-screen Image/Video viewer over a list. Swipe or use the arrow keys to
/// move; nothing advances on its own.
class ViewerPage extends StatefulWidget {
  const ViewerPage({
    super.key,
    required this.controller,
    required this.initialIndex,
  });

  final PagedController<MediaRef> controller;
  final int initialIndex;

  @override
  State<ViewerPage> createState() => _ViewerPageState();
}

class _ViewerPageState extends State<ViewerPage> {
  late final PageController _pages = PageController(
    initialPage: widget.initialIndex,
  );
  late int _index = widget.initialIndex;
  bool _zoomed = false;
  bool _chrome = true;
  final _focus = FocusNode();

  PagedController<MediaRef> get _list => widget.controller;

  @override
  void initState() {
    super.initState();
    _list.addListener(_changed);
  }

  @override
  void dispose() {
    _list.removeListener(_changed);
    _pages.dispose();
    _focus.dispose();
    super.dispose();
  }

  void _changed() {
    if (mounted) setState(() {});
  }

  void _go(int delta) {
    final target = _index + delta;
    if (target < 0 || target >= _list.items.length) return;
    _pages.animateToPage(
      target,
      duration: const Duration(milliseconds: 200),
      curve: Curves.easeOut,
    );
  }

  void _showInfo() {
    final item = _list.items[_index];
    showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      showDragHandle: true,
      builder: (_) => DraggableScrollableSheet(
        expand: false,
        initialChildSize: 0.5,
        maxChildSize: 0.9,
        builder: (context, scroll) => DetailPanel(item: item, scroll: scroll),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final items = _list.items;
    if (items.isEmpty) {
      return const Scaffold(body: Center(child: Text('Nothing to show.')));
    }
    final current = items[_index.clamp(0, items.length - 1)];
    final wide = MediaQuery.sizeOf(context).width >= 700;
    final total = _list.total ?? items.length;

    return CallbackShortcuts(
      bindings: {
        const SingleActivator(LogicalKeyboardKey.arrowLeft): () => _go(-1),
        const SingleActivator(LogicalKeyboardKey.arrowRight): () => _go(1),
        const SingleActivator(LogicalKeyboardKey.pageUp): () => _go(-1),
        const SingleActivator(LogicalKeyboardKey.pageDown): () => _go(1),
        const SingleActivator(LogicalKeyboardKey.escape): () =>
            Navigator.of(context).maybePop(),
        const SingleActivator(LogicalKeyboardKey.keyI): _showInfo,
      },
      child: Focus(
        focusNode: _focus,
        autofocus: true,
        child: Scaffold(
          backgroundColor: Colors.black,
          body: Stack(
            children: [
              PageView.builder(
                controller: _pages,
                physics: _zoomed
                    ? const NeverScrollableScrollPhysics()
                    : const PageScrollPhysics(),
                itemCount: items.length,
                onPageChanged: (i) {
                  setState(() {
                    _index = i;
                    _zoomed = false;
                  });
                  if (i >= items.length - 5) _list.loadMore();
                },
                itemBuilder: (context, i) {
                  final item = items[i];
                  return item.playsAsVideo
                      ? VideoPane(
                          key: ValueKey(item.key),
                          item: item,
                          active: i == _index,
                          loop: item.kind == MediaKind.image,
                        )
                      : ImagePane(
                          key: ValueKey(item.key),
                          item: item,
                          onTap: () => setState(() => _chrome = !_chrome),
                          onZoomChanged: (z) {
                            if (z != _zoomed) setState(() => _zoomed = z);
                          },
                        );
                },
              ),
              if (_chrome || current.playsAsVideo)
                Positioned(
                  top: 0,
                  left: 0,
                  right: 0,
                  child: _TopBar(
                    title: current.title,
                    position: '${_index + 1} / $total',
                    onInfo: _showInfo,
                  ),
                ),
              if (wide && _chrome && !current.playsAsVideo) ...[
                if (_index > 0)
                  Align(
                    alignment: Alignment.centerLeft,
                    child: _NavButton(
                      icon: Icons.chevron_left,
                      tooltip: 'Previous',
                      onTap: () => _go(-1),
                    ),
                  ),
                if (_index < items.length - 1)
                  Align(
                    alignment: Alignment.centerRight,
                    child: _NavButton(
                      icon: Icons.chevron_right,
                      tooltip: 'Next',
                      onTap: () => _go(1),
                    ),
                  ),
              ],
            ],
          ),
        ),
      ),
    );
  }
}

class _TopBar extends StatelessWidget {
  const _TopBar({
    required this.title,
    required this.position,
    required this.onInfo,
  });
  final String title;
  final String position;
  final VoidCallback onInfo;

  @override
  Widget build(BuildContext context) => DecoratedBox(
    decoration: const BoxDecoration(
      gradient: LinearGradient(
        begin: Alignment.topCenter,
        end: Alignment.bottomCenter,
        colors: [Color(0xB0000000), Color(0x00000000)],
      ),
    ),
    child: SafeArea(
      bottom: false,
      child: IconTheme(
        data: const IconThemeData(color: Colors.white),
        child: Row(
          children: [
            const BackButton(color: Colors.white),
            Expanded(
              child: Text(
                title,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(color: Colors.white, fontSize: 16),
              ),
            ),
            Text(position, style: const TextStyle(color: Colors.white70)),
            IconButton(
              tooltip: 'Details (I)',
              icon: const Icon(Icons.info_outline),
              onPressed: onInfo,
            ),
          ],
        ),
      ),
    ),
  );
}

class _NavButton extends StatelessWidget {
  const _NavButton({
    required this.icon,
    required this.tooltip,
    required this.onTap,
  });
  final IconData icon;
  final String tooltip;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.all(8),
    child: IconButton.filledTonal(
      tooltip: tooltip,
      iconSize: 32,
      onPressed: onTap,
      icon: Icon(icon),
    ),
  );
}

class ImagePane extends StatefulWidget {
  const ImagePane({
    super.key,
    required this.item,
    required this.onTap,
    required this.onZoomChanged,
  });
  final MediaRef item;
  final VoidCallback onTap;
  final ValueChanged<bool> onZoomChanged;

  @override
  State<ImagePane> createState() => _ImagePaneState();
}

class _ImagePaneState extends State<ImagePane> {
  final _transform = TransformationController();
  TapDownDetails? _doubleTapAt;

  @override
  void initState() {
    super.initState();
    _transform.addListener(() {
      widget.onZoomChanged(_transform.value.getMaxScaleOnAxis() > 1.01);
    });
  }

  @override
  void dispose() {
    _transform.dispose();
    super.dispose();
  }

  void _toggleZoom() {
    if (_transform.value.getMaxScaleOnAxis() > 1.01) {
      _transform.value = Matrix4.identity();
      return;
    }
    final p = _doubleTapAt?.localPosition ?? Offset.zero;
    const s = 2.5;
    _transform.value = Matrix4.identity()
      ..translateByDouble(-p.dx * (s - 1), -p.dy * (s - 1), 0, 1)
      ..scaleByDouble(s, s, 1, 1);
  }

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: widget.onTap,
      onDoubleTapDown: (d) => _doubleTapAt = d,
      onDoubleTap: _toggleZoom,
      child: InteractiveViewer(
        transformationController: _transform,
        maxScale: 8,
        child: SizedBox.expand(
          child: Stack(
            fit: StackFit.expand,
            children: [
              // Thumbnail first, then the full image over it once loaded.
              NetImage(widget.item.thumbnail, fit: BoxFit.contain),
              NetImage(widget.item.source, fit: BoxFit.contain),
            ],
          ),
        ),
      ),
    );
  }
}

class VideoPane extends StatefulWidget {
  const VideoPane({
    super.key,
    required this.item,
    required this.active,
    this.loop = false,
  });
  final MediaRef item;
  final bool active;
  final bool loop;

  @override
  State<VideoPane> createState() => _VideoPaneState();
}

class _VideoPaneState extends State<VideoPane> {
  Player? _player;
  VideoController? _video;
  String? _error;
  Timer? _clearNotice;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (widget.active && _player == null) _start();
  }

  @override
  void didUpdateWidget(VideoPane old) {
    super.didUpdateWidget(old);
    if (widget.active && _player == null) {
      _start();
    } else if (!widget.active) {
      _player?.pause();
    }
  }

  void _start() {
    final source = widget.item.source;
    if (source == null) {
      _error = 'This item has no playable file.';
      return;
    }
    final player = Player();
    _player = player;
    _video = VideoController(player);
    // mpv reports recoverable problems too (a failed hardware decoder
    // before it falls back to software), so an error is shown over the
    // video and cleared once playback is actually running.
    player.stream.error.listen((e) {
      if (!mounted) return;
      setState(() => _error = 'Playback problem: $e');
      _clearNotice?.cancel();
      _clearNotice = Timer(const Duration(seconds: 4), () {
        if (mounted && player.state.playing) setState(() => _error = null);
      });
    });
    player.stream.playing.listen((playing) {
      if (playing && _error != null && mounted) setState(() => _error = null);
    });
    if (widget.loop) player.setPlaylistMode(PlaylistMode.single);
    player.open(
      Media(source, httpHeaders: Session.of(context).client.authHeaders),
      play: true,
    );
  }

  @override
  void dispose() {
    _clearNotice?.cancel();
    _player?.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final video = _video;
    final error = _error;
    final notice = error == null
        ? null
        : Center(
            child: Padding(
              padding: const EdgeInsets.all(24),
              child: DecoratedBox(
                decoration: BoxDecoration(
                  color: Colors.black.withValues(alpha: 0.75),
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Padding(
                  padding: const EdgeInsets.all(12),
                  child: Text(
                    error,
                    textAlign: TextAlign.center,
                    style: const TextStyle(color: Colors.white),
                  ),
                ),
              ),
            ),
          );
    if (video == null) {
      return Stack(
        fit: StackFit.expand,
        children: [
          NetImage(
            widget.item.thumbnail,
            fit: BoxFit.contain,
            icon: Icons.movie,
          ),
          ?notice,
        ],
      );
    }
    return SafeArea(
      child: Stack(
        fit: StackFit.expand,
        children: [
          Video(controller: video),
          if (notice != null) IgnorePointer(child: notice),
        ],
      ),
    );
  }
}
