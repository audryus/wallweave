import QtQuick
import Quickshell.Io

QtObject {
    id: backend

    signal statusReceived(var status)
    signal librariesReceived(var libraries)
    signal libraryReceived(var library)
    signal errorReceived(string message)

    // Processo é filho, exposto via property para controle externo se precisar
    property var proc: Process {
        id: proc
        command: ["./bin/wallweave"]
        running: true
        stdinEnabled: true
        stdout: SplitParser {
            onRead: data => {
                const msg = JSON.parse(data)
                let payload = msg
                try {
                    payload = JSON.parse(msg.message)
                } catch (e) {}
                
                switch (msg.type) {
                    case "status": backend.statusReceived(payload); break
                    case "libraries": backend.librariesReceived(payload); break
                    case "library": backend.libraryReceived(payload); break
                    case "error": backend.errorReceived(payload); break
                    default: console.warn("Backend tipo desconhecido:", msg.type, msg)
                }
            }
        }
    }

    function send(obj) {
        proc.write(JSON.stringify(obj) + "\n")
    }
}