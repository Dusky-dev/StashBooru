#ifndef STASH_SQLITE3_COMPAT_H
#define STASH_SQLITE3_COMPAT_H

// sqlite-vec expects BSD u_int*_t aliases on non-Windows targets, but its
// amalgamation does not include sys/types.h. FreeBSD does not expose these
// aliases through the SQLite header, so provide them before sqlite-vec.c is
// compiled.
#ifdef __FreeBSD__
#include <sys/types.h>
#endif

// sqlite-vec expects the system header name, while Stash links SQLite through
// mattn/go-sqlite3 and therefore must compile against its bundled declarations.
#include "sqlite3-binding.h"

#endif
