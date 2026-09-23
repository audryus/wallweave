// I18n.qml
// Translation singleton for the Wall Weave popup. Language is chosen from
// the OS locale (Qt.locale().uiLanguages) with English as the source and
// fallback language. Extra languages ship as flat JSON catalogs next to
// this file (pt.json, zh.json, ...). English lives as a plain JS object
// so it is always available without waiting for a file load.
//
// Usage from any QML file in ui/:
//   import "./i18n"
//   I18n.tr("menu.libraries")
//   I18n.tr("status.connected", [I18n.tr("common.yes")])
//   I18n.trCount("library.images", n)
pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io

QtObject {
    id: root

    // Language tags shipped with the plugin (must match catalog files).
    readonly property var available: ["en", "pt", "zh"]

    // Detected OS language (primary subtag match: pt-BR -> pt, zh-Hans-CN -> zh).
    readonly property string lang: root.detectLanguage()

    // Active non-English catalog loaded from <lang>.json (empty for English).
    property var catalog: ({})

    // English source of truth — always present, no async load race.
    readonly property var enCatalog: ({
        "menu.libraries": "Libraries",
        "menu.displays": "Displays",
        "status.loading": "Loading...",
        "status.running": "Running",
        "status.degraded": "Degraded",
        "status.connected": "Connected: %1",
        "common.yes": "Yes",
        "common.no": "No",
        "status.mpvpaper": "mpvpaper: %1",
        "status.installed": "installed",
        "status.not_installed": "not installed",
        "status.mpvpaper_missing": "→ mpvpaper not installed",
        "status.hyprpaper": "hyprpaper: %1",
        "status.omarchy_fallback": "→ Will use Omarchy (global, not per-monitor)",
        "status.both_ok": "✓ Both installed and running",
        "library.heading": "LIBRARIES",
        "library.add_folder": "＋ Add folder",
        "library.remove": "Remove",
        "library.images": "%1 images",
        "library.images_one": "%1 image",
        "library.videos": "%1 videos",
        "library.videos_one": "%1 video",
        "library.empty": "No libraries.\nClick Add folder.",
        "library.preview": "preview",
        "library.picker_title": "Choose wallpaper folder",
        "display.heading": "DISPLAYS",
        "display.library": "Library",
        "display.omarchy_global": "Omarchy global (not per-display)",
        "display.change_wallpaper": "Change wallpaper",
        "display.seconds_short_min": "60s",
        "display.seconds_short_max": "300s",
        "display.seconds": "%1 seconds",
        "display.play_videos": "Play videos",
        "display.hyprpaper_missing": "hyprpaper not installed — will use omarchy global (same image on all monitors)",
        "display.mpvpaper_missing": "mpvpaper not installed — videos disabled",
        "display.empty": "No monitor detected.\nCheck hyprctl.",
        "error.unknown_command": "Unknown command",
        "error.invalid_path": "Invalid library path",
        "error.folder_exists": "Folder already added",
        "error.invalid_id_path": "Invalid library id/path",
        "error.display_empty": "Display object is empty",
        "error.serialize_status": "Could not serialize status"
    })

    // --- Language detection -------------------------------------------------

    // detectLanguage returns the first available catalog tag, or "en".
    // Priority:
    //   1. WALLWEAVE_LANG env override (dev/testing without system locale)
    //   2. Qt.locale().uiLanguages (OS locale, preference order)
    function detectLanguage() {
        // Dev override: WALLWEAVE_LANG=pt (or pt-BR, zh, ...)
        var override = ""
        try { override = Quickshell.env("WALLWEAVE_LANG") || "" } catch (e) { override = "" }
        if (override) {
            var t = String(override).toLowerCase()
            if (root.available.indexOf(t) >= 0) return t
            var p = t.split(/[-_]/)[0]
            if (root.available.indexOf(p) >= 0) return p
        }

        var prefs = []
        try { prefs = Qt.locale().uiLanguages || [] } catch (e) { prefs = [] }
        if (!prefs || prefs.length === 0) {
            // Older Qt: fall back to the locale name (e.g. "pt_BR").
            var n = String(Qt.locale().name || "en").toLowerCase()
            if (n) prefs = [n]
        }
        for (var i = 0; i < prefs.length; i++) {
            var tag = String(prefs[i]).toLowerCase()
            // Exact catalog match (e.g. "pt" when uiLanguages has "pt").
            if (root.available.indexOf(tag) >= 0) return tag
            // Primary subtag only: "pt-BR" -> "pt", "zh-Hans-CN" -> "zh".
            var primary = tag.split(/[-_]/)[0]
            if (root.available.indexOf(primary) >= 0) return primary
        }
        return "en"
    }

    // --- Catalog loading ----------------------------------------------------

    // fsPath turns a URL relative to this file into a plain filesystem path
    // for FileView (which expects a path, not a file:// URL).
    function fsPath(name) {
        var s = Qt.resolvedUrl(name).toString()
        if (s.startsWith("file://"))
            s = decodeURIComponent(s.slice("file://".length))
        return s
    }

    // Loads <lang>.json. English uses only the embedded enCatalog (path
    // empty → no file load). FileView is a named property because QtObject
    // has no default child property. blockLoading makes text() block until
    // the file is ready, which ensureCatalog() relies on.
    property FileView catalogFile: FileView {
        path: root.lang === "en" ? "" : root.fsPath(root.lang + ".json")
        blockLoading: true
        printErrors: false
        onLoaded: {
            if (root.lang === "en") return
            try {
                root.catalog = JSON.parse(text())
            } catch (e) {
                console.warn("[wallweave i18n] failed to parse", root.lang + ".json:", e)
                root.catalog = ({})
            }
        }
        onLoadFailed: {
            if (root.lang !== "en")
                console.warn("[wallweave i18n] could not load", root.lang + ".json — falling back to English")
        }
    }

    // ensureCatalog forces the non-English catalog to be ready before the
    // first translation. blockLoading makes text() wait for the load, so
    // UI strings created in Component.onCompleted are already translated.
    // Returns the active catalog object (English or loaded JSON).
    function ensureCatalog() {
        if (root.lang === "en") return root.enCatalog
        if (Object.keys(root.catalog).length > 0) return root.catalog
        try {
            var raw = root.catalogFile.text()
            if (raw && raw.length > 0) {
                root.catalog = JSON.parse(raw)
                return root.catalog
            }
        } catch (e) {
            console.warn("[wallweave i18n] failed to load", root.lang + ".json:", e)
        }
        return root.catalog
    }

    // --- Lookup -------------------------------------------------------------

    // tr returns the translated string for key. args is an optional array
    // substituted into %1, %2, ... placeholders. Missing keys fall back to
    // English, then to the key itself (so bugs stay visible).
    function tr(key, args) {
        var cat = root.ensureCatalog()
        var s = undefined
        if (root.lang !== "en" && cat[key] !== undefined)
            s = cat[key]
        if (s === undefined)
            s = root.enCatalog[key]
        if (s === undefined) {
            console.warn("[wallweave i18n] missing key:", key)
            return key
        }
        if (args && args.length > 0) {
            for (var i = 0; i < args.length; i++)
                s = s.split("%" + (i + 1)).join(String(args[i]))
        }
        return s
    }

    // trCount picks the singular form (<key>.one) when n is 1, else the
    // plural form (<key>). Languages that do not inflect can map both to
    // the same text.
    function trCount(key, n) {
        return tr(n === 1 ? key + "_one" : key, [n])
    }
}
