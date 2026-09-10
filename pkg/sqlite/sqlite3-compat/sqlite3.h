#ifndef STASH_SQLITE3_COMPAT_H
#define STASH_SQLITE3_COMPAT_H

// sqlite-vec expects the system header name, while Stash links SQLite through
// mattn/go-sqlite3 and therefore must compile against its bundled declarations.
#include "sqlite3-binding.h"

#endif
