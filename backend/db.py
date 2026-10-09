"""Opens the SQLite database used by WallWeave and applies the schema
(tables) on startup."""

import os
import sqlite3
import threading
import time

# schema holds the contents of schema.sql. It is executed (as a migration)
# every time the DB opens.
with open(os.path.join(os.path.dirname(__file__), "schema.sql"), encoding="utf-8") as _f:
    SCHEMA = _f.read()


class Database:
    """Thin wrapper around SQLite that hands every thread its own
    connection: the stdin loop, each wallpaper worker and each wake handler
    run in different threads, and a sqlite3 connection must not be shared
    between them."""

    def __init__(self, path):
        self.path = path
        self._local = threading.local()
        self._conns = []
        self._conns_lock = threading.Lock()

    def conn(self):
        """Returns this thread's connection, opening it on first use.
        busy_timeout and WAL are set on EVERY connection, so concurrent
        access — including a second wallweave process on another monitor —
        waits instead of failing with "database is locked". busy_timeout
        comes first so switching to WAL already waits on locks."""
        c = getattr(self._local, "conn", None)
        if c is None:
            # isolation_level=None = autocommit, like database/sql in Go.
            c = sqlite3.connect(self.path, timeout=5, isolation_level=None, check_same_thread=False)
            c.execute("PRAGMA busy_timeout=5000")
            c.execute("PRAGMA journal_mode=WAL")
            self._local.conn = c
            with self._conns_lock:
                self._conns.append(c)
        return c

    def execute(self, sql, params=()):
        """Runs one statement on this thread's connection."""
        return self.conn().execute(sql, params)

    def close(self):
        """Closes every connection opened by any thread."""
        with self._conns_lock:
            for c in self._conns:
                try:
                    c.close()
                except sqlite3.Error:
                    pass
            self._conns.clear()
        self._local = threading.local()


def new_database():
    """Opens the default database file, which is "wallweave.db" in the
    current working directory."""
    return open_database("wallweave.db")


def open_database(path):
    """Opens (or creates) a SQLite database at the given file path.
    Steps:
     1. Create the parent folder if the path contains one.
     2. Open the SQLite file (pragmas are applied per connection, see
        Database.conn).
     3. Run the schema (create tables if they do not exist), retrying
        briefly while the file is locked: creating a fresh WAL database or
        recovering one after an unclean shutdown returns SQLITE_BUSY right
        away, without honoring busy_timeout.
     4. Return the Database."""
    # Step 1: make sure the parent directory exists.
    parent = os.path.dirname(path)
    if parent and parent != ".":
        os.makedirs(parent, mode=0o755, exist_ok=True)

    # Step 2 and 3: open the connection and create the tables.
    database = Database(path)
    deadline = time.monotonic() + 5
    while True:
        try:
            database.conn().executescript(SCHEMA)
            break
        except sqlite3.OperationalError as e:
            if "database is locked" not in str(e) or time.monotonic() > deadline:
                database.close()
                raise
            # A failed connection may be left half-initialized: start over.
            database.close()
            time.sleep(0.05)

    # Step 4: hand the database back.
    return database
