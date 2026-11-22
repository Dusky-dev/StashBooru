import 'package:material_ui/material_ui.dart';
import 'package:shared_preferences/shared_preferences.dart';

/// The app's version; kept equal to `pubspec.yaml` and the server's
/// `internal/build/stashbooru-version.txt` (a test checks both).
const kAppVersion = '1.9.0';

enum CardSize {
  small('Small', 0.75),
  medium('Medium', 1),
  large('Large', 1.4);

  const CardSize(this.label, this.factor);
  final String label;

  /// Multiplies each grid's minimum card width.
  final double factor;
}

/// Preferences that belong to this device, not to the server.
class AppSettings extends ChangeNotifier {
  AppSettings._(this._prefs);

  /// Defaults only, nothing saved (tests, or before loading finishes).
  AppSettings.defaults() : _prefs = null;

  static Future<AppSettings> load() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      return AppSettings._(prefs).._read();
    } catch (_) {
      return AppSettings.defaults();
    }
  }

  final SharedPreferences? _prefs;

  CardSize cardSize = CardSize.medium;
  bool autoplay = true;
  bool startMuted = false;
  bool loopVideos = false;
  bool fullSizeImages = true;

  void _read() {
    final p = _prefs!;
    cardSize = CardSize.values.firstWhere(
      (c) => c.name == p.getString('cardSize'),
      orElse: () => CardSize.medium,
    );
    autoplay = p.getBool('autoplay') ?? true;
    startMuted = p.getBool('startMuted') ?? false;
    loopVideos = p.getBool('loopVideos') ?? false;
    fullSizeImages = p.getBool('fullSizeImages') ?? true;
  }

  void setCardSize(CardSize v) {
    cardSize = v;
    _prefs?.setString('cardSize', v.name);
    notifyListeners();
  }

  void setAutoplay(bool v) => _setBool('autoplay', v, () => autoplay = v);
  void setStartMuted(bool v) => _setBool('startMuted', v, () => startMuted = v);
  void setLoopVideos(bool v) => _setBool('loopVideos', v, () => loopVideos = v);
  void setFullSizeImages(bool v) =>
      _setBool('fullSizeImages', v, () => fullSizeImages = v);

  void _setBool(String key, bool value, VoidCallback apply) {
    apply();
    _prefs?.setBool(key, value);
    notifyListeners();
  }
}

class AppSettingsScope extends InheritedNotifier<AppSettings> {
  const AppSettingsScope({
    super.key,
    required AppSettings settings,
    required super.child,
  }) : super(notifier: settings);

  static final _fallback = AppSettings.defaults();

  static AppSettings of(BuildContext context) =>
      context
          .dependOnInheritedWidgetOfExactType<AppSettingsScope>()
          ?.notifier ??
      _fallback;
}
