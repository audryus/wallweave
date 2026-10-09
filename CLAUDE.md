# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

Wall Weave is an Omarchy (Quickshell/QML) bar-widget plugin that rotates wallpapers per monitor. QML UI in `ui/`, a Python backend (standard library only, no build step, no dependencies to install) in `backend/`. README.md has the full architecture write-up; this is the condensed version.

## Commands

```bash
make test                     # python3 -m unittest discover -s tests -t .
python3 -m unittest tests.test_display.TestDisplayCommands.test_edit_display   # single test
make run                      # standalone preview window (qs -p ui/_preview.qml), F5 reloads
make run-pt / make run-zh     # preview forcing a language (WALLWEAVE_LANG)
make validate                 # omarchy plugin validate . (runs `unlinks` first)
make rescan / make restart    # rescan plugins / restart omarchy shell
```

- `make run` creates dev-only symlinks (`ui/Commons`, `ui/Ui`, `ui/qs/shell` → `/usr/share/omarchy/shell`). They are gitignored and **must not be committed** — the plugin validator rejects symlinks (hence `validate` depends on `unlinks`).
- Env vars: `WALLWEAVE_LANG=<tag>` forces UI language; `WALLWEAVE_DEV=1` keeps the popup open.
- Run tests from the repo root (`-t .` makes `backend` and `tests` importable as packages).

## Architecture

**Two processes, JSON Lines over stdin/stdout.** `ui/Backend.qml` spawns `python3 -u -m backend` with the plugin root as working directory, writes `{"cmd": ...}` lines, and turns each response line into a typed signal. Responses are always `{"type", "code"?, "message"?}`; on success `message` is a JSON *string* of the payload (QML re-parses it). On error `type` is `"error"`, and `code` is a stable i18n key (e.g. `folder_exists`) — the backend never sends user-facing English; technical errors have only `message` and stay untranslated.

**Adding a command** touches both sides: a handler registered via `register(commander)` in a `backend/*.py` module (`Commander.__init__` calls `status/library/display.register`), plus a `case` in `Backend.handleLine` and a signal if the UI consumes the result. Handler exceptions become technical errors; they never kill the process.

**One backend per shell.** `manifest.json` declares two entry points: `barWidget` (`ui/shell.qml`, instantiated once per monitor) and `service` (`ui/Service.qml`, mounted once per shell, which is just a `Backend`). `Main.qml` gets the shared backend via `bar.shell.serviceFor("audryus.wallweave")` and only starts its own when there is no shell (the preview).

**Workers and master election.** `worker.py` runs one thread per display with a library selected (`theme != 0`): wait for the timer or a wake signal, then show the next file from a seeded shuffle. Because a second backend can still exist (preview + shell), `worker_lock.py` elects a master with `flock` on `wallweave.workers.lock`; only the master runs workers and listens on abstract socket `@wallweave.wake`. Others poll every 5s to take over, and forward wakes over the socket. UI edits (library change/delete) must wake workers.

**Wallpaper application.** `mpv.py` keeps **one long-lived mpvpaper per monitor** and swaps files with `loadfile` over its IPC socket (`$XDG_RUNTIME_DIR/wallweave/mpv-<monitor>.sock`). Don't restart mpvpaper per rotation: recreating the layer surface makes Hyprland re-send pointer events, so fullscreen videos show cursor/controls. Without mpvpaper, images fall back to `omarchy theme bg set` (global) and videos are skipped.

**Database.** SQLite `wallweave.db` in the cwd (= plugin root), WAL mode, one connection per thread (`db.Database.conn()`), schema in `backend/schema.sql` re-applied on every start (`CREATE TABLE IF NOT EXISTS` — there's no migration system, so schema changes must be additive/idempotent). The `.db*` and `.lock` files in the repo root are local runtime state.

**Go heritage.** The backend was ported from Go; `commander.py` deliberately mimics Go's `json.Unmarshal` semantics (missing fields → zero values, strict types, `omitempty` output). Keep that behavior when changing request/response types.

## i18n

All QML strings go through `I18n.tr("key")` / `I18n.trCount(...)` (`ui/i18n/I18n.qml`). English is embedded in `I18n.qml` and mirrored in `ui/i18n/en.json`; keep both in sync and add new keys to `pt.json` and `zh.json`. New languages: add `<tag>.json` and append the tag to `available` in `I18n.qml`. The product name "Wall Weave" is never translated.

## Tests

`tests/helpers.CommanderTestCase` gives a `Commander` on a temp DB with one seeded display `TEST-1`, so handlers are tested via `self.c.exec(Request(...))` without hyprctl. Tests needing real system tools (hyprctl, etc.) `skipTest` when unavailable.

## Repo conventions

- Default branch is `trunk` (no `main`); `omarchy plugin add` clones the default branch.
- Python modules and QML files carry explanatory docstrings/comments, often with numbered "Step N" walkthroughs — match that style.
