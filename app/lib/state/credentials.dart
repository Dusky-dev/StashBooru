import 'dart:io';

import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:path_provider/path_provider.dart';
import 'package:shared_preferences/shared_preferences.dart';

class SavedServer {
  const SavedServer(this.url, this.apiKey);
  final String url;
  final String apiKey;
}

/// Remembers the last server. The API key lives in the Android Keystore or
/// Windows Credential Manager; on Linux in an owner-only file (see
/// [_FileVault]).
abstract class CredentialStore {
  Future<SavedServer?> load();
  Future<void> save(SavedServer server);
  Future<void> forgetKey();

  /// False when the keychain was unavailable and the key was kept only for
  /// this session.
  bool get keyPersisted;
}

abstract class _Vault {
  Future<String?> read();
  Future<void> write(String value);
  Future<void> delete();
}

class _SecureVault implements _Vault {
  static const _key = 'api_key';
  final _secure = const FlutterSecureStorage();
  @override
  Future<String?> read() => _secure.read(key: _key);
  @override
  Future<void> write(String value) => _secure.write(key: _key, value: value);
  @override
  Future<void> delete() => _secure.delete(key: _key);
}

/// Linux: libsecret aborts the whole process when no keyring service is
/// running, which Dart cannot catch. Keep the key in a file only the user
/// can read instead, like command-line tools do without a keyring.
class _FileVault implements _Vault {
  Future<File> _file() async {
    final dir = await getApplicationSupportDirectory();
    await dir.create(recursive: true);
    return File('${dir.path}/api-key');
  }

  @override
  Future<String?> read() async {
    final f = await _file();
    return await f.exists() ? (await f.readAsString()).trim() : null;
  }

  @override
  Future<void> write(String value) async {
    final f = await _file();
    // Create it private before the key is written.
    await f.writeAsString('');
    await Process.run('chmod', ['600', f.path]);
    await f.writeAsString(value, flush: true);
  }

  @override
  Future<void> delete() async {
    final f = await _file();
    if (await f.exists()) await f.delete();
  }
}

class DeviceCredentialStore implements CredentialStore {
  static const _urlKey = 'server_url';
  final _Vault _secure = Platform.isLinux ? _FileVault() : _SecureVault();
  bool _keyPersisted = true;

  @override
  bool get keyPersisted => _keyPersisted;

  @override
  Future<SavedServer?> load() async {
    final prefs = await SharedPreferences.getInstance();
    final url = prefs.getString(_urlKey);
    if (url == null) return null;
    String key = '';
    try {
      key = await _secure.read() ?? '';
    } catch (_) {
      _keyPersisted = false;
    }
    return SavedServer(url, key);
  }

  @override
  Future<void> save(SavedServer server) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_urlKey, server.url);
    try {
      if (server.apiKey.isEmpty) {
        await _secure.delete();
      } else {
        await _secure.write(server.apiKey);
      }
      _keyPersisted = true;
    } catch (_) {
      _keyPersisted = false;
    }
  }

  @override
  Future<void> forgetKey() async {
    try {
      await _secure.delete();
    } catch (_) {}
  }
}

class MemoryCredentialStore implements CredentialStore {
  MemoryCredentialStore([this._saved]);
  SavedServer? _saved;
  @override
  bool get keyPersisted => true;
  @override
  Future<SavedServer?> load() async => _saved;
  @override
  Future<void> save(SavedServer server) async => _saved = server;
  @override
  Future<void> forgetKey() async {
    if (_saved != null) _saved = SavedServer(_saved!.url, '');
  }
}
