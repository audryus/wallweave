import QtQuick
import Quickshell.Io

QtObject {
    id: backend

    signal statusReceived(var status)
    signal librariesReceived(var libraries)
    signal libraryReceived(var library)
    signal errorReceived(string message)
    signal displaysReceived(var displays)
    
    // Processo é filho, exposto via property para controle externo se precisar
    property var proc: Process {
        id: proc
        command: ["./bin/wallweave"]
        running: true
        stdinEnabled: true
        stdout: SplitParser {
            onRead: data => {
                try {
                    const msg = JSON.parse(data)
                    
                    const payload = JSON.parse(msg.message)
                
                    switch (msg.type) {
                        case "get_status": backend.statusReceived(payload); break
                        case "browse_libraries": backend.librariesReceived(payload); break
                        case "add_library": backend.libraryReceived(payload); break
                        case "del_library": break
                        case "browse_displays": backend.displaysReceived(payload); break
                        case "edit_display": break
                        case "error": backend.errorReceived(payload); break
                        default: console.warn("Backend tipo desconhecido:", msg.type, msg)
                    }
                } catch (e) {}
            }
        }
    }

    function send(obj) {
        proc.write(JSON.stringify(obj) + "\n")
    }
}