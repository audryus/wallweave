// Backend.qml
// Non-visual bridge between the QML UI and the Go backend process.
// It starts "go run ." as a child process, writes one JSON command per
// line to its stdin, reads one JSON response per line from its stdout,
// and emits typed signals (statusReceived, librariesReceived, ...) that
// the other components listen to.
import QtQuick
import Quickshell.Io

QtObject {
    id: backend

    // --- Signals emitted when a response arrives from Go ---
    signal statusReceived(var status)          // health status (get_status)
    signal librariesReceived(var libraries)    // full library list
    signal libraryReceived(var library)        // one library was added
    // An error from Go: code is a stable i18n key (e.g. "folder_exists")
    // when present; message is the technical text for the console.
    signal errorReceived(string message, string code)
    signal displaysReceived(var displays)      // display/monitor list

    // Backend.qml lives in <pluginRoot>/ui/ — the plugin root is the
    // parent folder of this file. Resolving it relatively (instead of a
    // hard-coded absolute path) makes the plugin work for any user/id.
    //
    // NOTE: Qt.resolvedUrl("..") is blackholed by Quickshell's URL
    // interceptor (returns qrc:/qs-blackhole), so resolve "." (this
    // file's folder) and take its parent instead.
    readonly property string pluginRoot: {
        let s = Qt.resolvedUrl(".").toString()
        if (s.startsWith("file://"))
            s = decodeURIComponent(s.slice("file://".length))
        if (s.endsWith("/"))
            s = s.slice(0, -1)
        // Parent of ui/ → plugin root.
        let i = s.lastIndexOf("/")
        return i > 0 ? s.slice(0, i) : s
    }

    // Prefer the prebuilt binary (make build → bin/wallweave) so the UI
    // does not need Go on qs's PATH; fall back to `go run .` otherwise.
    readonly property string backendCommand:
        "if [ -x ./bin/wallweave ]; then exec ./bin/wallweave; " +
        "else exec go run .; fi"

    // The Go child process. It runs continuously while the UI is open.
    property var proc: Process {
        id: proc
        command: ["/bin/sh", "-c", backend.backendCommand]
        // Run from the plugin root so the database is created next to main.go.
        workingDirectory: backend.pluginRoot
        running: true
        stdinEnabled: true

        // One line of stdout = one complete JSON response from Go.
        stdout: SplitParser {
            onRead: data => backend.handleLine(data)
        }
        // Forward Go's stderr (logs/diagnostics) to the console.
        stderr: SplitParser {
            onRead: data => console.warn("[wallweave]", data)
        }
        // Log when the backend process exits unexpectedly.
        onExited: (code, status) => console.warn("[wallweave] exited", code, status)
    }

    // handleLine parses one line of stdout and re-emits it as a typed
    // signal. Steps:
    //  1. Ignore empty lines.
    //  2. Parse the line as JSON; ignore non-JSON lines (logs on stdout).
    //  3. Parse msg.message again when it is a JSON string (payload);
    //     plain text (errors) is kept as-is.
    //  4. Emit the signal that matches msg.type.
    function handleLine(data) {
        // Step 1: nothing to do with an empty line.
        if (!data || data.length === 0) return
        // Step 2: parse the line as JSON.
        let msg
        try {
            msg = JSON.parse(data)
        } catch (e) {
            // Non-JSON line (log polluting stdout) — ignore but log it.
            console.warn("[wallweave] non-JSON stdout:", data)
            return
        }

        // Step 3: message may be a JSON string (payload) or plain text (error).
        let payload = msg.message
        if (typeof payload === "string" && payload.length > 0) {
            try { payload = JSON.parse(payload) } catch (e) { /* plain text, fine */ }
        }

        // Step 4: emit the signal for this response type.
        switch (msg.type) {
            case "get_status": backend.statusReceived(payload); break
            case "browse_libraries": backend.librariesReceived(payload); break
            case "add_library": backend.libraryReceived(payload); break
            case "del_library": break
            case "browse_displays": backend.displaysReceived(payload); break
            case "edit_display": break
            case "error":
                backend.errorReceived(
                    typeof payload === "string" ? payload : JSON.stringify(payload),
                    msg.code || ""
                )
                break
            default:
                console.warn("Backend unknown type:", msg.type, msg)
        }
    }

    // send writes one JSON command to the Go process (one line = one
    // command). It does nothing when the process is not running.
    function send(obj) {
        if (!proc.running) {
            console.warn("[wallweave] send ignored — process is not running")
            return
        }
        proc.write(JSON.stringify(obj) + "\n")
    }
}
