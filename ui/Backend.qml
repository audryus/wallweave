import QtQuick
import Quickshell.Io

QtObject {
    id: backend

    signal statusReceived(var status)
    signal librariesReceived(var libraries)
    signal libraryReceived(var library)
    signal errorReceived(string message)
    signal displaysReceived(var displays)

    // Backend.qml fica em <pluginRoot>/ui/ — root = pai deste arquivo.
    // Evita path absoluto: funciona com qualquer id/usuário de plugin.
    readonly property string pluginRoot: {
        let s = Qt.resolvedUrl("..").toString()
        if (s.startsWith("file://"))
            s = decodeURIComponent(s.slice("file://".length))
        if (s.endsWith("/"))
            s = s.slice(0, -1)
        return s
    }

    property var proc: Process {
        id: proc
        // go run . — precisa de Go no PATH do shell (mise/shims ou go install)
        command: ["go", "run", "."]
        workingDirectory: backend.pluginRoot
        running: true
        stdinEnabled: true

        stdout: SplitParser {
            onRead: data => backend.handleLine(data)
        }
        stderr: SplitParser {
            onRead: data => console.warn("[wallweave]", data)
        }
        onExited: (code, status) => console.warn("[wallweave] exited", code, status)
    }

    function handleLine(data) {
        if (!data || data.length === 0) return
        let msg
        try {
            msg = JSON.parse(data)
        } catch (e) {
            // linha não-JSON (log poluindo stdout) — ignora mas registra
            console.warn("[wallweave] stdout não-JSON:", data)
            return
        }

        // message pode ser JSON string (payload) ou texto puro (erro)
        let payload = msg.message
        if (typeof payload === "string" && payload.length > 0) {
            try { payload = JSON.parse(payload) } catch (e) { /* texto cru, ok */ }
        }

        switch (msg.type) {
            case "get_status": backend.statusReceived(payload); break
            case "browse_libraries": backend.librariesReceived(payload); break
            case "add_library": backend.libraryReceived(payload); break
            case "del_library": break
            case "browse_displays": backend.displaysReceived(payload); break
            case "edit_display": break
            case "error":
                backend.errorReceived(typeof payload === "string" ? payload : JSON.stringify(payload))
                break
            default:
                console.warn("Backend tipo desconhecido:", msg.type, msg)
        }
    }

    function send(obj) {
        if (!proc.running) {
            console.warn("[wallweave] send ignorado — processo não está rodando")
            return
        }
        proc.write(JSON.stringify(obj) + "\n")
    }
}
