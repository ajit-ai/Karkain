# Vendored SQLite amalgamation

Karkain v1.2.22 vendors the official SQLite amalgamation so a real database
driver works on Windows, Linux and macOS CI without runner package installs.

- Version: **3.46.1** (`2024-08-13`)
- Upstream archive: `https://www.sqlite.org/2024/sqlite-amalgamation-3460100.zip`
- Archive SHA-256: `77823CB110929C2BCB0F5D48E4833B5C59A8A6E40CDEA3936B99E199DBBE5784`
- `sqlite3.c` SHA-256: `6C35BC5F7F85EAC9C49928BACBB02BB694B547AABF69197E058CCA245AD80E83`
- `sqlite3.h` SHA-256: `89B62C671C5964E137409CE034941B7B05A3AF2C9875ABA41F47F9483D0C2515`

Only `sqlite3.c` and `sqlite3.h` are vendored; `shell.c` and `sqlite3ext.h`
from the upstream archive are not needed. SQLite is in the public domain; see
the license text at the top of `sqlite3.c`.

Do not edit these files. Upgrade by replacing both files from a verified
upstream archive and updating the hashes above in the same change.
