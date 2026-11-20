import 'package:material_ui/material_ui.dart';
import 'package:media_kit/media_kit.dart';

import 'api/client.dart';
import 'state/credentials.dart';
import 'ui/connect_page.dart';
import 'ui/home_shell.dart';
import 'ui/session.dart';

void main() {
  WidgetsFlutterBinding.ensureInitialized();
  MediaKit.ensureInitialized();
  runApp(StashBooruApp(store: DeviceCredentialStore()));
}

typedef ClientFactory = StashClient Function(Uri url, String apiKey);

class StashBooruApp extends StatefulWidget {
  const StashBooruApp({super.key, required this.store, this.clientFactory});

  final CredentialStore store;
  final ClientFactory? clientFactory;

  @override
  State<StashBooruApp> createState() => _StashBooruAppState();
}

class _StashBooruAppState extends State<StashBooruApp> {
  bool _loading = true;
  StashClient? _client;
  ServerCapabilities? _caps;
  SavedServer? _saved;
  String? _startupError;

  @override
  void initState() {
    super.initState();
    _restore();
  }

  StashClient _makeClient(Uri url, String key) =>
      widget.clientFactory?.call(url, key) ??
      StashClient(baseUrl: url, apiKey: key);

  Future<void> _restore() async {
    final saved = await widget.store.load();
    _saved = saved;
    final url = saved == null ? null : parseServerUrl(saved.url);
    if (url != null) {
      final error = await _connect(url, saved!.apiKey, save: false);
      _startupError = error;
    }
    if (mounted) setState(() => _loading = false);
  }

  Future<String?> _connect(Uri url, String key, {bool save = true}) async {
    final client = _makeClient(url, key);
    try {
      final caps = await client.probe();
      if (save) {
        final server = SavedServer(url.toString(), key);
        await widget.store.save(server);
        _saved = server;
      }
      if (!mounted) return null;
      setState(() {
        _client?.close();
        _client = client;
        _caps = caps;
      });
      if (!widget.store.keyPersisted && key.isNotEmpty && mounted) {
        _messenger.currentState?.showSnackBar(
          const SnackBar(
            content: Text(
              'No system keychain found: the API key will be asked for again next time.',
            ),
          ),
        );
      }
      return null;
    } on StashException catch (e) {
      client.close();
      return e.message;
    } catch (e) {
      client.close();
      return 'Could not connect: $e';
    }
  }

  Future<void> _disconnect() async {
    _navigator.currentState?.popUntil((route) => route.isFirst);
    await widget.store.forgetKey();
    setState(() {
      _client?.close();
      _client = null;
      _caps = null;
      _startupError = null;
      if (_saved != null) _saved = SavedServer(_saved!.url, '');
    });
  }

  final _messenger = GlobalKey<ScaffoldMessengerState>();
  final _navigator = GlobalKey<NavigatorState>();

  @override
  Widget build(BuildContext context) {
    Widget home;
    if (_loading) {
      home = const Scaffold(body: Center(child: CircularProgressIndicator()));
    } else if (_client == null) {
      home = ConnectPage(
        onConnect: _connect,
        initialUrl: _saved?.url ?? '',
        initialKey: _saved?.apiKey ?? '',
        initialError: _startupError,
      );
    } else {
      home = const HomeShell();
    }
    return MaterialApp(
      title: 'StashBooru',
      debugShowCheckedModeBanner: false,
      scaffoldMessengerKey: _messenger,
      navigatorKey: _navigator,
      themeMode: ThemeMode.dark,
      theme: ThemeData(
        colorScheme: ColorScheme.fromSeed(seedColor: const Color(0xFF5B8DEF)),
      ),
      darkTheme: ThemeData(
        colorScheme: ColorScheme.fromSeed(
          seedColor: const Color(0xFF5B8DEF),
          brightness: Brightness.dark,
        ),
      ),
      // Session sits above the navigator so pushed pages (viewer, entity
      // media) can reach the connection too.
      builder: (context, child) => _client == null
          ? child!
          : Session(
              client: _client!,
              caps: _caps!,
              onDisconnect: _disconnect,
              child: child!,
            ),
      home: home,
    );
  }
}
