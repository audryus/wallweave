# Wall Weave

**Wall Weave** is an [Omarchy](https://omarchy.org/) bar-widget plugin that lets you rotate wallpapers per monitor.

You add folders full of images (and videos), pick one folder per display, set a timer, and Wall Weave keeps changing the wallpaper in the background. On multi-monitor setups each screen can run its own library, its own interval, and its own video setting.

---

> [!IMPORTANT]
> **Honest disclaimer:** A human with a soul and a caffeine addiction designed the structure, drove the logic, and made the calls. An AI (with a lot more free time) played a real role too: it helped design, wrote and refactored large parts of the code, translated comments, wrote the unit tests, and validated the whole thing — always with the human in the loop reviewing, correcting, and taking responsibility. So: **human-led, AI-assisted, human-approved.** If it starts speaking in binary, pull the plug.

---

## Requirements

| Requirement | Why | How to install |
|---|---|---|
| **Omarchy** | Host system (bar, shell, plugins) | Already on your machine if you are reading this on Omarchy |
| **Go** | Builds the backend binary; also a runtime fallback (`go run .`) when `bin/wallweave` is missing | `mise install go` |
| **hyprpaper** *(optional)* | Per-monitor image wallpapers | Install if you want different images per screen; otherwise Omarchy's global background is used |
| **mpvpaper** *(optional)* | Video wallpapers | Install to enable the video toggle |
| **ffmpeg** | Generates thumbnail previews for libraries | Usually already present; needed only for previews |
| **zenity** | "Add folder" file picker dialog | Needed only when adding a library from the UI |

> **Note:** The UI prefers the prebuilt `bin/wallweave` (from `make build`). If that binary is missing, `Backend.qml` falls back to `go run .`, which needs `go` on your shell's `PATH` (with mise, that means Go stays activated in your config).

## Installation

### 1. Install Go (once)

```bash
mise install go
```

Make sure the installed version satisfies `go.mod` (this project targets Go **1.27.1**):

```bash
go version
```

### 2. Install the plugin

```bash
omarchy plugin add https://github.com/audryus/wallweave.git --enable
```

(`omarchy plugin install` is an alias for the same command. Drop `--enable` if you prefer to enable it later.)

What this does:

1. Clones this repository into `~/.config/omarchy/plugins/audryus.wallweave/`
2. Validates `manifest.json`
3. Rescans plugins
4. With `--enable`, asks which bar section to place it in (`left` / `center` / `right`; the manifest default is **right**)

### Which branch does it install?

**`trunk` — and you do not need to rename anything.**

- This repository's default branch is **`trunk`** (`origin/HEAD` points to `origin/trunk`). There is no `main` branch.
- `omarchy plugin add` runs a plain `git clone <url>`, and `git clone` always checks out the remote's **default** branch.
- Therefore the command above installs `trunk` as-is. No rename, no extra flags.

### Enable / disable / update / remove

```bash
omarchy plugin enable  audryus.wallweave   # add it to the bar (asks for section)
omarchy plugin disable audryus.wallweave   # hide it from the bar
omarchy plugin update  audryus.wallweave   # git pull the latest trunk
omarchy plugin remove  audryus.wallweave   # uninstall
```

After enabling, the widget appears in the chosen bar section. Click it to open the popup.

## Usage

1. Click the **Wall Weave** icon in the bar (looks like ``, Nerd Font `nf-md-wallpaper`).
2. Open the **Libraries** tab and click **＋ Add folder**. Pick a folder with images or videos. Wall Weave counts the files and generates thumbnails (via ffmpeg) into a `thumbs/` subfolder.
3. Open the **Displays** tab. For each monitor you can:
   - **Library** — choose which folder this monitor rotates through (`Omarchy global` means "do not manage this screen").
   - **Change wallpaper** — set the rotation interval (60–300 seconds).
   - **Play videos** — allow video wallpapers on this screen (requires mpvpaper).
4. Watch the status badge in the header:
   - **Running** (green) — mpvpaper and hyprpaper are both installed.
   - **Degraded** (yellow) — something is missing; hover for details.
   - **Loading...** (red) — the backend has not answered yet.

## Architecture

Wall Weave is split into two processes that talk over **stdin/stdout, one JSON object per line**:

```
┌──────────────────────────────────────────────────────────────────┐
│  omarchy-shell (Quickshell / QML)                               │
│                                                                  │
│  ui/shell.qml          bar button + PopupCard host               │
│    └─ ui/Main.qml      full popup layout                         │
│         ├─ Backend.qml  spawns & talks to the Go process         │
│         ├─ Status.qml   health badge (header)                    │
│         ├─ Menu.qml     left nav (Libraries / Displays)          │
│         ├─ Library.qml  add/remove wallpaper folders             │
│         └─ Display.qml  per-monitor settings                     │
│              │                                                   │
│              │  write: {"cmd":"edit_display", ...}\n             │
│              │  read:  {"type":"edit_display", ...}\n            │
└──────────────┼───────────────────────────────────────────────────┘
               │  stdin / stdout (JSON Lines)
┌──────────────┼───────────────────────────────────────────────────┐
│  bin/wallweave (or go run .) ▼                                │
│                                                                  │
│  main.go               line loop: read Request → write Response  │
│    └─ Commander         registry of commands                     │
│         ├─ status.go    probe mpvpaper / hyprpaper               │
│         ├─ library.go   scan folders, ffmpeg thumbnails          │
│         ├─ display.go   monitor list & settings                  │
│         ├─ display_worker.go   one rotation loop per display     │
│         └─ worker_lock.go      multi-process master election     │
│              │                                                   │
│              ▼                                                   │
│         SQLite (wallweave.db)  displays / libraries / status     │
│              │                                                   │
│              ▼                                                   │
│         hyprpaper · mpvpaper · omarchy theme bg                  │
└──────────────────────────────────────────────────────────────────┘
```

### How a command flows

1. The user clicks something in QML (for example, picks a library for a monitor).
2. `Backend.qml` writes one JSON line to the Go process: `{"cmd":"edit_display","display":{...}}`.
3. `main.go` reads the line, unmarshals it into a `Request`, and `Commander.Exec` looks up the handler by name.
4. The handler updates SQLite, does its work, and returns a `Response`.
5. `Backend.qml` parses the response line and emits a typed signal (`displaysReceived`, `statusReceived`, …).
6. The matching QML component reacts and updates the UI.

### Commands (JSON API)

| `cmd` | Purpose | Extra fields |
|---|---|---|
| `get_status` | Probe tools and return/save health | — |
| `browse_libraries` | List all libraries | — |
| `add_library` | Scan a folder, make thumbnails, insert row | `path` |
| `del_library` | Delete a library (wakes workers) | `id` **or** `path` |
| `browse_displays` | List monitors (bootstraps from `hyprctl` on first run) | — |
| `edit_display` | Update theme/timer/video for one monitor | `display` |

Every answer has the shape `{"type": "<same or error>", "code": "<error key, errors only>", "message": "<payload or text>"}`.
Known errors carry a stable `code` (e.g. `folder_exists`) that the UI translates; technical failures only have `message`.

### Wallpaper workers

- One **worker goroutine per display** that has a library selected (`theme != 0`).
- Each worker loop: read settings → wait for timer **or** wake signal → pick the next file from a shuffled list (wraps forever) → apply it.
- **Images:** hyprpaper first (per-monitor); if missing or it fails, fall back to `omarchy theme bg set` (global, same image on every screen).
- **Videos:** mpvpaper, started detached with loop + no audio.
- **Changing library or deleting one** wakes the workers immediately so they do not wait for the timer.

### Multi-monitor = multiple processes

With two monitors, the Omarchy bar loads the widget twice → **two wallweave processes**. Only one of them may run the wallpaper workers:

1. Each process tries to take an exclusive `flock` on `wallweave.workers.lock`.
2. The winner becomes the **master**: it starts the workers and listens on an abstract Unix socket (`@wallweave.wake`).
3. The loser polls every 5 seconds. If the master dies, the next poll takes over.
4. When a non-master process needs to wake a worker (UI edit), it sends a wake message over the socket instead.

### Database

SQLite file `wallweave.db` (created next to the plugin root; ignored by git):

| Table | Holds |
|---|---|
| `displays` | One row per monitor: name, library id (`theme`), timer, video flag, shuffle seed, resolution |
| `libraries` | One row per wallpaper folder: path, thumbnail list (JSON), image/video counts |
| `status` | Single row with the last health probe (mpvpaper/hyprpaper installed, label, color) |

The schema lives in `db/schema.sql` and is applied automatically on every start (`CREATE TABLE IF NOT EXISTS`).

## Project layout

```
wallweave/
├── manifest.json          Omarchy plugin manifest (id: audryus.wallweave, bar-widget)
├── main.go                Process entry: DB open, workers start, stdin/stdout loop
├── go.mod / go.sum        Go module (github.com/audryus/wallweave)
├── Makefile               build / preview / validate / rescan helpers
├── db/
│   ├── db.go              Open SQLite, WAL mode, run migration
│   └── schema.sql         Table definitions
├── command/
│   ├── command.go         Commander, Request/Response types, registry
│   ├── status.go          get_status: probe tools, save/load health
│   ├── library.go         add/del/browse libraries, ffmpeg thumbnails
│   ├── display.go         browse/edit displays, hyprctl monitor detection
│   ├── display_worker.go  Rotation loop, hyprpaper/mpvpaper/omarchy apply
│   ├── worker_lock.go     flock master election + Unix-socket wake
│   └── *_test.go          Unit tests
└── ui/
    ├── shell.qml          Plugin entry (BarWidget + PopupCard)  ← from manifest
    ├── Main.qml           Full popup content (used by shell + preview)
    ├── Backend.qml        Spawns `bin/wallweave` (fallback: `go run .`), JSON-lines bridge
    ├── Status.qml         Header health badge
    ├── Menu.qml           Left navigation
    ├── Library.qml        Libraries panel
    ├── Display.qml        Displays panel
    ├── _preview.qml       Standalone dev preview window
    ├── i18n/              Translation singleton + catalogs (en / pt / zh)
    └── Commons, Ui, qs    Symlinks into /usr/share/omarchy/shell (QML imports)
```

## Languages

The popup UI follows your **OS language** (via `Qt.locale().uiLanguages`), with **English** as the source and fallback language. For development/testing you can force a language with `WALLWEAVE_LANG=pt` (or `zh`) without changing the system locale.

| Language | Tag | Catalog |
|---|---|---|
| English | `en` | Embedded in `ui/i18n/I18n.qml` (+ `ui/i18n/en.json` mirror) |
| Português | `pt` | `ui/i18n/pt.json` |
| 中文 | `zh` | `ui/i18n/zh.json` |

How it works:

- All human-facing QML strings go through `I18n.tr("some.key")` / `I18n.trCount(...)`.
- The Go backend never sends English sentences for the badge or known errors — it sends stable codes (`running`, `degraded`, `omarchy_fallback`, `folder_exists`, …) that the UI translates.
- Technical Go errors (`err.Error()`, worker logs) stay untranslated and only appear in the console.
- The product name **Wall Weave** is intentionally not translated.

### Adding a language

1. Copy `ui/i18n/en.json` to `ui/i18n/<tag>.json` (e.g. `fr.json`).
2. Translate the values (keep the keys and `%1` placeholders).
3. Add the tag to `available` in `ui/i18n/I18n.qml` (`["en", "pt", "zh", "fr"]`).
4. Restart the shell (or reload the plugin). The OS locale picks the catalog automatically.
5. To try it without changing the OS locale: `WALLWEAVE_LANG=<tag> make run` (or `make run-pt` / `make run-zh`).

## Development

```bash
make build      # go build -o bin/wallweave .
make run        # build + open the isolated preview window (qs -p ui/_preview.qml)
make run-pt     # same, forcing Portuguese (WALLWEAVE_LANG=pt)
make run-zh     # same, forcing Chinese (WALLWEAVE_LANG=zh)
make debug      # same, but without optimizations
make validate   # omarchy plugin validate .
make rescan     # omarchy-shell shell rescanPlugins
make restart    # omarchy restart shell
```

- **Preview without the bar:** `make run` opens `ui/_preview.qml` in its own window. Press **F5** to reload.
- **Live reload inside Omarchy:** saving any file under the installed plugin folder (`~/.config/omarchy/plugins/audryus.wallweave/`) reloads the QML automatically. The Go side restarts whenever the popup opens (it is started by `Backend.qml` via `bin/wallweave`, falling back to `go run .`).
- **Keep the popup open while developing:** run with `WALLWEAVE_DEV=1` in the environment.
- **Force a language:** set `WALLWEAVE_LANG=pt` (or `zh`) in the environment; otherwise the OS locale decides.

### Tests

```bash
go test ./...
```

## Optional tools cheat-sheet

```bash
# Per-monitor static wallpapers (recommended for multi-monitor)
omarchy pkg add hyprpaper

# Video wallpapers
omarchy pkg add mpvpaper

# Thumbnails + folder picker (if missing)
omarchy pkg add ffmpeg zenity
```

If hyprpaper or mpvpaper is missing, Wall Weave still works: images fall back to Omarchy's global background, and the video toggle is disabled with a warning in the UI.

## License

[MIT](LICENSE) © Audryus
