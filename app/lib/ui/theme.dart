import 'package:flutter/foundation.dart';
import 'package:material_ui/material_ui.dart';

/// The web UI's colours (Blueprint dark on Bootstrap, `ui/v2.5/src/styles/
/// _theme.scss`), so the app reads as the same product.
abstract final class StashColors {
  static const body = Color(0xFF202B33);
  static const card = Color(0xFF30404D);
  static const secondary = Color(0xFF394B59);
  static const secondaryHover = Color(0xFF30404D);
  static const navbar = Color(0xFF394B59);
  static const primary = Color(0xFF137CBD);
  static const link = Color(0xFF48AFF0);
  static const text = Color(0xFFF5F8FA);
  static const muted = Color(0xFFBFCCD6);
  static const danger = Color(0xFFDB3737);
  static const textField = Color(0x4D10161A);
  static const border = Color(0x6610161A);
  static const divider = Color(0x26FFFFFF);
}

/// Bootstrap's default radius for buttons and inputs; cards use 3px.
const double kRadius = 4;
const double kCardRadius = 3;

bool get isDesktop =>
    kIsWeb ||
    switch (defaultTargetPlatform) {
      TargetPlatform.linux ||
      TargetPlatform.windows ||
      TargetPlatform.macOS => true,
      _ => false,
    };

ThemeData buildStashTheme() {
  const scheme = ColorScheme(
    brightness: Brightness.dark,
    primary: StashColors.primary,
    onPrimary: Colors.white,
    primaryContainer: StashColors.primary,
    onPrimaryContainer: Colors.white,
    secondary: StashColors.secondary,
    onSecondary: StashColors.text,
    secondaryContainer: StashColors.secondary,
    onSecondaryContainer: StashColors.text,
    tertiary: StashColors.link,
    onTertiary: Colors.white,
    error: StashColors.danger,
    onError: Colors.white,
    surface: StashColors.body,
    onSurface: StashColors.text,
    onSurfaceVariant: StashColors.muted,
    surfaceContainerLowest: StashColors.body,
    surfaceContainerLow: StashColors.body,
    surfaceContainer: StashColors.card,
    surfaceContainerHigh: StashColors.card,
    surfaceContainerHighest: StashColors.secondary,
    surfaceTint: Colors.transparent,
    outline: Color(0xFF5C7080),
    outlineVariant: StashColors.divider,
    inverseSurface: StashColors.text,
    onInverseSurface: StashColors.body,
    shadow: Colors.black,
    scrim: Colors.black,
  );
  final shape = RoundedRectangleBorder(
    borderRadius: BorderRadius.circular(kRadius),
  );
  const buttonPadding = EdgeInsets.symmetric(horizontal: 12, vertical: 6);
  final base = ThemeData(
    useMaterial3: true,
    colorScheme: scheme,
    visualDensity: isDesktop ? VisualDensity.compact : VisualDensity.standard,
  );
  final text = base.textTheme.apply(
    bodyColor: StashColors.text,
    displayColor: StashColors.text,
  );
  return base.copyWith(
    scaffoldBackgroundColor: StashColors.body,
    canvasColor: StashColors.body,
    splashFactory: NoSplash.splashFactory,
    highlightColor: Colors.transparent,
    hoverColor: Colors.white.withValues(alpha: 0.06),
    focusColor: StashColors.link.withValues(alpha: 0.25),
    dividerColor: StashColors.divider,
    textTheme: text.copyWith(
      bodyLarge: text.bodyLarge?.copyWith(fontSize: 16),
      bodyMedium: text.bodyMedium?.copyWith(fontSize: 14),
      titleMedium: text.titleMedium?.copyWith(
        fontSize: 17,
        fontWeight: FontWeight.w500,
      ),
    ),
    appBarTheme: const AppBarTheme(
      backgroundColor: StashColors.navbar,
      foregroundColor: StashColors.text,
      surfaceTintColor: Colors.transparent,
      elevation: 0,
      scrolledUnderElevation: 0,
      centerTitle: false,
      toolbarHeight: 49,
      titleTextStyle: TextStyle(fontSize: 18, color: StashColors.text),
    ),
    cardTheme: CardThemeData(
      color: StashColors.card,
      surfaceTintColor: Colors.transparent,
      elevation: 0,
      margin: EdgeInsets.zero,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(kCardRadius),
        side: const BorderSide(color: StashColors.border),
      ),
    ),
    dividerTheme: const DividerThemeData(
      color: StashColors.divider,
      thickness: 1,
      space: 1,
    ),
    filledButtonTheme: FilledButtonThemeData(
      style: FilledButton.styleFrom(
        backgroundColor: StashColors.primary,
        foregroundColor: Colors.white,
        shape: shape,
        padding: buttonPadding,
        elevation: 0,
      ),
    ),
    elevatedButtonTheme: ElevatedButtonThemeData(
      style: ElevatedButton.styleFrom(
        backgroundColor: StashColors.secondary,
        foregroundColor: StashColors.text,
        shape: shape,
        padding: buttonPadding,
        elevation: 0,
      ),
    ),
    outlinedButtonTheme: OutlinedButtonThemeData(
      style: OutlinedButton.styleFrom(
        foregroundColor: StashColors.text,
        side: const BorderSide(color: StashColors.secondary),
        shape: shape,
        padding: buttonPadding,
      ),
    ),
    textButtonTheme: TextButtonThemeData(
      style: TextButton.styleFrom(
        foregroundColor: StashColors.link,
        shape: shape,
        padding: buttonPadding,
      ),
    ),
    iconButtonTheme: IconButtonThemeData(
      style: IconButton.styleFrom(
        foregroundColor: StashColors.text,
        shape: shape,
      ),
    ),
    inputDecorationTheme: InputDecorationTheme(
      filled: true,
      fillColor: StashColors.textField,
      isDense: true,
      contentPadding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
      hintStyle: const TextStyle(color: Color(0xFF8A9BA8)),
      labelStyle: const TextStyle(color: StashColors.muted),
      helperStyle: const TextStyle(color: StashColors.muted),
      border: OutlineInputBorder(
        borderRadius: BorderRadius.circular(kRadius),
        borderSide: BorderSide.none,
      ),
      enabledBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(kRadius),
        borderSide: BorderSide.none,
      ),
      focusedBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(kRadius),
        borderSide: const BorderSide(color: StashColors.primary),
      ),
    ),
    dialogTheme: DialogThemeData(
      backgroundColor: StashColors.card,
      surfaceTintColor: Colors.transparent,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(6)),
    ),
    popupMenuTheme: PopupMenuThemeData(
      color: StashColors.secondary,
      surfaceTintColor: Colors.transparent,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(kRadius),
      ),
      textStyle: const TextStyle(color: StashColors.text, fontSize: 14),
    ),
    menuTheme: MenuThemeData(
      style: MenuStyle(
        backgroundColor: const WidgetStatePropertyAll(StashColors.secondary),
        surfaceTintColor: const WidgetStatePropertyAll(Colors.transparent),
        shape: WidgetStatePropertyAll(shape),
      ),
    ),
    bottomSheetTheme: const BottomSheetThemeData(
      backgroundColor: StashColors.card,
      dragHandleColor: Color(0xFF5C7080),
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(6)),
      ),
      surfaceTintColor: Colors.transparent,
    ),
    chipTheme: ChipThemeData(
      backgroundColor: StashColors.secondary,
      labelStyle: const TextStyle(color: StashColors.text, fontSize: 13),
      side: BorderSide.none,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(kRadius),
      ),
      padding: const EdgeInsets.symmetric(horizontal: 4),
    ),
    snackBarTheme: const SnackBarThemeData(
      backgroundColor: StashColors.secondary,
      contentTextStyle: TextStyle(color: StashColors.text),
    ),
    tooltipTheme: TooltipThemeData(
      decoration: BoxDecoration(
        color: StashColors.secondary,
        borderRadius: BorderRadius.circular(kRadius),
      ),
      textStyle: const TextStyle(color: StashColors.text, fontSize: 13),
    ),
    progressIndicatorTheme: const ProgressIndicatorThemeData(
      color: StashColors.link,
    ),
    scrollbarTheme: ScrollbarThemeData(
      thumbVisibility: WidgetStatePropertyAll(isDesktop),
      thumbColor: const WidgetStatePropertyAll(Color(0xFF5C7080)),
      radius: const Radius.circular(4),
      thickness: const WidgetStatePropertyAll(8),
    ),
    listTileTheme: const ListTileThemeData(
      iconColor: StashColors.text,
      textColor: StashColors.text,
      selectedColor: StashColors.text,
      selectedTileColor: Color(0x1AFFFFFF),
    ),
  );
}

/// A Bootstrap `btn-secondary`: the toolbar and navbar button of the web UI.
class SecondaryButton extends StatelessWidget {
  const SecondaryButton({
    super.key,
    required this.onPressed,
    required this.child,
    this.active = false,
    this.tooltip,
  });

  final VoidCallback? onPressed;
  final Widget child;
  final bool active;
  final String? tooltip;

  @override
  Widget build(BuildContext context) {
    final button = TextButton(
      onPressed: onPressed,
      style: TextButton.styleFrom(
        backgroundColor: active
            ? StashColors.secondaryHover
            : StashColors.secondary,
        foregroundColor: StashColors.text,
        minimumSize: const Size(36, 36),
        padding: const EdgeInsets.symmetric(horizontal: 12),
        textStyle: const TextStyle(fontSize: 14),
      ),
      child: child,
    );
    return tooltip == null ? button : Tooltip(message: tooltip!, child: button);
  }
}
