"""browse/edit displays (monitors) and hyprctl monitor detection."""

import json
import random
import subprocess

from .commander import Display, error_response, payload

# DISPLAY_COLUMNS lists every column of the displays table, in the order
# used by row_to_display.
DISPLAY_COLUMNS = "id, name, mirror_of, video, timer, seed, width, height, theme"


def register(c):
    """Registers "browse_displays" and "edit_display"."""
    c.register("browse_displays", browse_displays)
    c.register("edit_display", edit_display)


def browse_displays(c, req):
    """Handler for "browse_displays". Steps:
     1. Load all displays from the database.
     2. If the table is empty, detect the monitors and insert them
        (bootstrap).
     3. Make sure a worker is running for every display that has a library
        selected (theme != 0).
     4. Return the list as JSON."""
    from . import worker

    # Step 1: read the displays already saved.
    displays = list_displays(c)

    # Step 2: first run (empty table) — detect monitors and insert them.
    if not displays:
        displays = bootstrap_displays(c)

    # Step 3: start a wallpaper worker for each display that has a library.
    for d in displays:
        if d.theme != 0:
            worker.ensure_worker(c, d)

    # Step 4: answer with the list.
    return payload("browse_displays", displays)


def edit_display(c, req):
    """Handler for "edit_display". Steps:
     1. Reject an empty display name.
     2. Load the previous version of the display (to detect theme changes).
     3. Apply the patch (theme, timer, video) to the database.
     4. Only wake the worker when the library (theme) actually changed or
        the display is new — timer/video edits must not change the
        wallpaper immediately, because the slider fires an edit on every
        step while the user is dragging it.
     5. Return the updated display as JSON."""
    from . import worker

    # Step 1: the name identifies the display and is required.
    if req.display.name == "":
        return error_response("display_empty", "display object is empty")

    # Step 2: read the current values so we can compare the theme later.
    prev = get_display(c, req.display.name)

    # Step 3: save the editable fields (theme, timer, video).
    updated = update_display(c, req.display.name, req.display)

    # Step 4: only a library (theme) change wakes the worker right away.
    if prev is None or prev.theme != updated.theme:
        worker.ensure_worker(c, updated)
        worker.signal_worker(updated.name)

    # Step 5: answer with the updated display.
    return payload("edit_display", updated)


def row_to_display(row):
    """Converts one displays row to a Display. The "video" column is stored
    as 0/1, so it is converted to a bool here."""
    id_, name, mirror_of, video, timer, seed, width, height, theme = row
    return Display(
        id=id_,
        name=name,
        mirrorOf=mirror_of,
        video=video != 0,
        timer=timer,
        seed=seed,
        width=width,
        height=height,
        theme=theme,
    )


def list_displays(c):
    """Reads every display from the database, ordered by id."""
    rows = c.database.execute(f"SELECT {DISPLAY_COLUMNS} FROM displays ORDER BY id").fetchall()
    return [row_to_display(r) for r in rows]


def get_display(c, name):
    """Loads one display by its monitor name, or None when it is missing."""
    row = c.database.execute(f"SELECT {DISPLAY_COLUMNS} FROM displays WHERE name = ?", (name,)).fetchone()
    return row_to_display(row) if row else None


def update_display(c, name, patch):
    """Applies the editable fields of a display (theme, timer, video), then
    re-reads the row and returns the fresh values. Only these three fields
    are written so the update does not depend on id/geometry from the UI."""
    c.database.execute(
        "UPDATE displays SET theme = ?, timer = ?, video = ? WHERE name = ?",
        (patch.theme, patch.timer, int(patch.video), name),
    )
    updated = get_display(c, name)
    if updated is None:
        # The update matched no row: the display does not exist.
        raise LookupError(f"display not found: {name}")
    return updated


def insert_display(c, d):
    """Creates a new displays row with all fields of the given display."""
    c.database.execute(
        "INSERT INTO displays (name, mirror_of, video, timer, seed, width, height, theme) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
        (d.name, d.mirrorOf, int(d.video), d.timer, d.seed, d.width, d.height, d.theme),
    )


def bootstrap_displays(c):
    """Runs on the first launch, when the displays table is empty. Steps:
     1. Ask hyprctl for the connected monitors.
     2. For each monitor, set safe defaults (no library, random seed,
        60 second timer, video off) and insert it.
     3. Return the full list freshly read from the database."""
    for m in get_monitors():
        m.theme = 0  # no library selected yet
        m.seed = random.getrandbits(63)  # random seed for shuffling wallpapers
        m.timer = 60  # change wallpaper every 60 seconds
        m.video = False  # videos disabled by default
        m.id = 0  # let the database assign the id
        insert_display(c, m)
    return list_displays(c)


def _run(args):
    """Runs a command and returns its stdout, raising on failure."""
    return subprocess.run(args, stdin=subprocess.DEVNULL, capture_output=True, text=True, check=True).stdout


def get_monitors():
    """Asks hyprctl for the list of connected monitors. Steps:
     1. Preferred path: "hyprctl monitors -j", parsed as JSON.
     2. Fallback path: "hyprctl monitors" plain text (so a demo does not
        break if JSON parsing fails).
     3. Return the monitors as Display objects."""
    # Step 1: JSON output (preferred).
    err1 = None
    try:
        mons = json.loads(_run(["hyprctl", "monitors", "-j"]))
        displays = [
            Display(
                name=str(m.get("name") or ""),
                mirrorOf=str(m.get("mirrorOf") or ""),
                width=int(m.get("width") or 0),
                height=int(m.get("height") or 0),
            )
            for m in mons
        ]
        if displays:
            return displays
    except Exception as e:  # noqa: BLE001
        err1 = e

    # Step 2: fallback — each line that starts with "Monitor NAME ...".
    try:
        txt = _run(["hyprctl", "monitors"])
    except Exception as err2:  # noqa: BLE001
        raise RuntimeError(f"hyprctl failed (json and text): {err1} / {err2}") from err2
    displays = []
    for line in txt.split("\n"):
        line = line.strip()
        if line.startswith("Monitor "):
            parts = line.split()
            if len(parts) >= 2:
                displays.append(Display(name=parts[1]))
    if not displays:
        # Step 3: nothing could be parsed from the text.
        raise RuntimeError(f"no monitor parsed from text: {txt}")
    return displays
