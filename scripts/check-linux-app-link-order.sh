#!/usr/bin/env bash
# The Linux app must load libgcc_s before libflutter_linux_gtk.so. Flutter's
# engine exports its own _Unwind_* functions; if they win symbol lookup, an
# error inside a LuaJIT-built libmpv (Arch and others) crashes the app.
set -euo pipefail
binary="${1:-app/build/linux/x64/release/bundle/stashbooru}"
needed=$(readelf -d "$binary" | sed -n 's/.*(NEEDED).*\[\(.*\)\]/\1/p')
gcc_line=$(grep -n -x 'libgcc_s.so.1' <<<"$needed" | cut -d: -f1 || true)
flutter_line=$(grep -n -x 'libflutter_linux_gtk.so' <<<"$needed" | cut -d: -f1 || true)
if [[ -z "$gcc_line" || -z "$flutter_line" || "$gcc_line" -ge "$flutter_line" ]]; then
  echo "libgcc_s.so.1 must be NEEDED before libflutter_linux_gtk.so in $binary:" >&2
  echo "$needed" >&2
  exit 1
fi
echo "Link order OK: libgcc_s.so.1 (#$gcc_line) before libflutter_linux_gtk.so (#$flutter_line)"
