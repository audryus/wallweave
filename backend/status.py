"""get_status: probe the wallpaper tools and save/load the health row."""

import shutil
import sys
from dataclasses import dataclass

from .commander import payload


@dataclass
class Status:
    """Health information of the wallpaper tools: whether mpvpaper is
    installed, plus a color and stable message codes the UI translates, and
    whether the backend is connected. label, video and image are i18n keys
    (e.g. "running", "mpvpaper_missing", "omarchy_fallback") — never English
    sentences."""

    mpvpaper: bool = False
    video: str = ""
    image: str = ""
    color: str = ""
    label: str = ""
    connected: bool = False


def register(c):
    """Registers "get_status" so the frontend can ask for the current
    health of the system."""
    c.register("get_status", get_status)


def get_status(c, req):
    """Handler for "get_status". It:
     1. Probes the system (checks if mpvpaper exists).
     2. Saves the result in the database. A failed save is only logged: the
        probe itself is still valid, and answering with an error would leave
        the UI badge stuck on red.
     3. Returns the status as JSON."""
    status = probe_status()
    try:
        save_status(c, status)
    except Exception as e:  # noqa: BLE001
        print(f"[status] save: {e}", file=sys.stderr, flush=True)
    return payload("get_status", status)


def probe_status():
    """Checks the machine for the required tools:
      - If mpvpaper is missing, the status becomes "degraded" (yellow):
        videos cannot play and images fall back to Omarchy's global
        background (same image on every monitor).
      - Otherwise the status is "running" (green).
    It never fails; it only returns what it found."""
    # Assume the best case first: everything installed and running.
    status = Status(color="#2ecc71", label="running", connected=True)
    if shutil.which("mpvpaper"):
        status.mpvpaper = True
    else:
        # mpvpaper is missing: mark as degraded and set the reason codes.
        status.color = "#f1c40f"
        status.video = "mpvpaper_missing"
        status.image = "omarchy_fallback"
        status.label = "degraded"
    return status


def save_status(c, s):
    """Writes (or overwrites) the single status row (id = 1) with an
    upsert."""
    c.database.execute(
        """INSERT INTO status (id, mpvpaper, video, image, color, label, connected, updated_at)
		 VALUES (1, ?, ?, ?, ?, ?, ?, datetime('now'))
		 ON CONFLICT(id) DO UPDATE SET
			mpvpaper = excluded.mpvpaper,
			video = excluded.video,
			image = excluded.image,
			color = excluded.color,
			label = excluded.label,
			connected = excluded.connected,
			updated_at = excluded.updated_at""",
        (int(s.mpvpaper), s.video, s.image, s.color, s.label, int(s.connected)),
    )


def load_status(c):
    """Reads the saved status. Returns an empty Status when no row exists
    yet. Booleans are stored as 0/1 integers and converted back here."""
    row = c.database.execute(
        "SELECT mpvpaper, video, image, color, label, connected FROM status WHERE id = 1"
    ).fetchone()
    if row is None:
        return Status()
    mpvpaper, video, image, color, label, connected = row
    return Status(
        mpvpaper=mpvpaper != 0,
        video=video,
        image=image,
        color=color,
        label=label,
        connected=connected != 0,
    )
