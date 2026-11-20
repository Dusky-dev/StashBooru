import 'package:material_ui/material_ui.dart';

import '../api/client.dart';

typedef ConnectCallback = Future<String?> Function(Uri url, String apiKey);

/// Asks for the server address and API key. [onConnect] returns an error to
/// show, or null on success.
class ConnectPage extends StatefulWidget {
  const ConnectPage({
    super.key,
    required this.onConnect,
    this.initialUrl = '',
    this.initialKey = '',
    this.initialError,
  });

  final ConnectCallback onConnect;
  final String initialUrl;
  final String initialKey;
  final String? initialError;

  @override
  State<ConnectPage> createState() => _ConnectPageState();
}

class _ConnectPageState extends State<ConnectPage> {
  late final _url = TextEditingController(text: widget.initialUrl);
  late final _key = TextEditingController(text: widget.initialKey);
  late String? _error = widget.initialError;
  bool _busy = false;
  bool _showKey = false;

  @override
  void dispose() {
    _url.dispose();
    _key.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    final url = parseServerUrl(_url.text);
    if (url == null) {
      setState(
        () =>
            _error = 'Enter the server address, for example 192.168.1.20:9999.',
      );
      return;
    }
    setState(() {
      _busy = true;
      _error = null;
    });
    final error = await widget.onConnect(url, _key.text.trim());
    if (mounted) {
      setState(() {
        _busy = false;
        _error = error;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Scaffold(
      body: SafeArea(
        child: Center(
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(24),
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 420),
              child: Card(
                child: Padding(
                  padding: const EdgeInsets.all(24),
                  child: AutofillGroup(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        Icon(
                          Icons.photo_library,
                          size: 56,
                          color: theme.colorScheme.primary,
                        ),
                        const SizedBox(height: 12),
                        Text(
                          'StashBooru',
                          textAlign: TextAlign.center,
                          style: theme.textTheme.headlineMedium,
                        ),
                        const SizedBox(height: 4),
                        Text(
                          'Connect to your StashBooru or Stash server.',
                          textAlign: TextAlign.center,
                          style: theme.textTheme.bodyMedium,
                        ),
                        const SizedBox(height: 24),
                        TextField(
                          key: const Key('server-url'),
                          controller: _url,
                          enabled: !_busy,
                          keyboardType: TextInputType.url,
                          autofillHints: const [AutofillHints.url],
                          decoration: const InputDecoration(
                            labelText: 'Server address',
                            hintText: '192.168.1.20:9999',
                          ),
                          onSubmitted: (_) => _submit(),
                        ),
                        const SizedBox(height: 12),
                        TextField(
                          key: const Key('api-key'),
                          controller: _key,
                          enabled: !_busy,
                          obscureText: !_showKey,
                          autocorrect: false,
                          enableSuggestions: false,
                          decoration: InputDecoration(
                            labelText: 'API key',
                            helperText: 'Only needed if the server has a password. Find it in the web UI under Settings → Security.',
                            helperMaxLines: 3,
                            suffixIcon: IconButton(
                              tooltip: _showKey ? 'Hide key' : 'Show key',
                              icon: Icon(
                                _showKey
                                    ? Icons.visibility_off
                                    : Icons.visibility,
                              ),
                              onPressed: () =>
                                  setState(() => _showKey = !_showKey),
                            ),
                          ),
                          onSubmitted: (_) => _submit(),
                        ),
                        if (_error != null) ...[
                          const SizedBox(height: 12),
                          Text(
                            _error!,
                            key: const Key('connect-error'),
                            style: TextStyle(color: theme.colorScheme.error),
                          ),
                        ],
                        const SizedBox(height: 20),
                        FilledButton(
                          onPressed: _busy ? null : _submit,
                          child: Padding(
                            padding: const EdgeInsets.all(12),
                            child: _busy
                                ? const SizedBox.square(
                                    dimension: 20,
                                    child: CircularProgressIndicator(
                                      strokeWidth: 2,
                                    ),
                                  )
                                : const Text('Connect'),
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
