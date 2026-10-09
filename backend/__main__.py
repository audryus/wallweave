"""Entry point of the WallWeave backend (run with: python3 -u -m backend).

It opens the database, starts the wallpaper workers, then reads JSON
commands from stdin (one per line) and writes JSON answers to stdout."""

import json
import sys

from .commander import Commander, Request, Response
from .db import new_database


def write_response(resp):
    """Writes a Response as one single JSON line to stdout. The trailing
    newline marks the end of the message, and the flush sends it right away
    (stdout is a pipe, which Python would otherwise buffer)."""
    sys.stdout.write(resp.to_json() + "\n")
    sys.stdout.flush()


def main():
    """In order, it:
     1. Opens the SQLite database (creates the file if missing).
     2. Creates a Commander that can run commands against that database.
     3. Starts the background workers that rotate wallpapers.
     4. Loops until stdin closes: reads one line of JSON, runs the command,
        and writes one line of JSON response. The worker threads are
        daemons, so the process exits with the loop."""
    # Step 1: open the database file "wallweave.db".
    try:
        database = new_database()
    except Exception as e:  # noqa: BLE001
        print(f"db: {e}", file=sys.stderr, flush=True)
        sys.exit(1)

    try:
        # Step 2 and 3: the command runner and the wallpaper workers (right
        # at boot, no need to wait for the UI).
        commander = Commander(database)
        commander.start_workers()

        # Step 4: read stdin line by line until the input is closed.
        for line in sys.stdin:
            if not line.strip():
                # Go's json.Unmarshal fails on an empty line too.
                write_response(Response(type="error", message="invalid json"))
                continue
            try:
                req = Request.from_json(json.loads(line))
            except ValueError:
                # Not valid JSON (or wrong field types): answer and go on.
                write_response(Response(type="error", message="invalid json"))
                continue
            write_response(commander.exec(req))
    finally:
        database.close()


if __name__ == "__main__":
    main()
