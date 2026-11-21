import 'package:material_ui/material_ui.dart';

import '../api/client.dart';
import 'stash_image.dart';

/// The connected server, available to every page below the home shell.
class Session extends InheritedWidget {
  const Session({
    super.key,
    required this.client,
    required this.caps,
    required this.onDisconnect,
    required super.child,
  });

  final StashClient client;
  final ServerCapabilities caps;
  final VoidCallback onDisconnect;

  static Session of(BuildContext context) =>
      context.dependOnInheritedWidgetOfExactType<Session>()!;

  @override
  bool updateShouldNotify(Session old) =>
      client != old.client || caps != old.caps;
}

/// A network image that sends the API key and shows a quiet placeholder.
class NetImage extends StatelessWidget {
  const NetImage(
    this.url, {
    super.key,
    this.fit = BoxFit.cover,
    this.cacheWidth,
    this.icon = Icons.image,
  });

  final String? url;
  final BoxFit fit;
  final int? cacheWidth;
  final IconData icon;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final placeholder = ColoredBox(
      color: scheme.surfaceContainer,
      child: Center(child: Icon(icon, color: scheme.onSurfaceVariant)),
    );
    if (url == null) return placeholder;
    final dpr = MediaQuery.devicePixelRatioOf(context);
    return Image(
      image: StashNetworkImage(
        url!,
        headers: Session.of(context).client.authHeaders,
        cacheWidth: cacheWidth == null ? null : (cacheWidth! * dpr).round(),
      ),
      fit: fit,
      gaplessPlayback: true,
      errorBuilder: (_, _, _) => placeholder,
      frameBuilder: (context, child, frame, sync) =>
          frame == null && !sync ? placeholder : child,
    );
  }
}
