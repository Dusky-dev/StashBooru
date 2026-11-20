import 'dart:async';

import 'package:material_ui/material_ui.dart';

import 'theme.dart';

/// The web UI's list toolbar: a centred row of search, sort and options.
class ListToolbar extends StatelessWidget {
  const ListToolbar({super.key, required this.children});
  final List<Widget> children;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.fromLTRB(16, 12, 16, 8),
    child: Wrap(
      alignment: WrapAlignment.center,
      crossAxisAlignment: WrapCrossAlignment.center,
      spacing: 8,
      runSpacing: 8,
      children: children,
    ),
  );
}

/// A debounced search box like the web UI's "Search..." field.
class SearchField extends StatefulWidget {
  const SearchField({super.key, required this.onChanged, this.hint});
  final ValueChanged<String> onChanged;
  final String? hint;

  @override
  State<SearchField> createState() => _SearchFieldState();
}

class _SearchFieldState extends State<SearchField> {
  Timer? _debounce;

  @override
  void dispose() {
    _debounce?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => SizedBox(
    width: 240,
    height: 36,
    child: TextField(
      style: const TextStyle(fontSize: 14),
      decoration: InputDecoration(hintText: widget.hint ?? 'Search...'),
      onChanged: (text) {
        _debounce?.cancel();
        _debounce = Timer(
          const Duration(milliseconds: 400),
          () => widget.onChanged(text),
        );
      },
    ),
  );
}

/// A secondary button that opens a list of choices, like the sort dropdown.
class ChoiceButton<T> extends StatelessWidget {
  const ChoiceButton({
    super.key,
    required this.value,
    required this.values,
    required this.label,
    required this.onSelected,
    this.tooltip,
  });

  final T value;
  final List<T> values;
  final String Function(T) label;
  final ValueChanged<T> onSelected;
  final String? tooltip;

  @override
  Widget build(BuildContext context) => MenuAnchor(
    alignmentOffset: const Offset(0, 4),
    menuChildren: [
      for (final v in values)
        MenuItemButton(
          onPressed: () => onSelected(v),
          style: MenuItemButton.styleFrom(
            foregroundColor: StashColors.text,
            minimumSize: const Size(160, 36),
            backgroundColor: v == value ? StashColors.primary : null,
          ),
          child: Text(label(v)),
        ),
    ],
    builder: (context, menu, _) => SecondaryButton(
      tooltip: tooltip,
      onPressed: () => menu.isOpen ? menu.close() : menu.open(),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(label(value)),
          const SizedBox(width: 6),
          const Icon(Icons.arrow_drop_down, size: 18),
        ],
      ),
    ),
  );
}

/// Underlined text tabs, like the tabs on a web detail page.
class NavTabs extends StatelessWidget {
  const NavTabs({
    super.key,
    required this.labels,
    required this.selected,
    required this.onSelected,
  });

  final List<String> labels;
  final int selected;
  final ValueChanged<int> onSelected;

  @override
  Widget build(BuildContext context) => Container(
    decoration: const BoxDecoration(
      border: Border(bottom: BorderSide(color: StashColors.divider)),
    ),
    margin: const EdgeInsets.symmetric(horizontal: 16),
    child: Row(
      mainAxisAlignment: MainAxisAlignment.center,
      children: [
        for (var i = 0; i < labels.length; i++)
          InkWell(
            onTap: () => onSelected(i),
            child: Container(
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
              decoration: BoxDecoration(
                border: Border(
                  bottom: BorderSide(
                    width: 2,
                    color: i == selected
                        ? StashColors.link
                        : Colors.transparent,
                  ),
                ),
              ),
              child: Text(
                labels[i],
                style: TextStyle(
                  fontSize: 15,
                  color: i == selected ? StashColors.link : StashColors.text,
                ),
              ),
            ),
          ),
      ],
    ),
  );
}
