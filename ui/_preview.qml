import Quickshell
import QtQuick
import QtQuick.Controls
import qs.Commons
import qs.Ui

// Janela isolada para `qs -p ui/_preview.qml` ou
// `QML_IMPORT_PATH=/usr/share/omarchy/shell qs -p ui/_preview.qml`
// Reusa ui/main.qml sem BarWidget/PopupCard.
ShellRoot {
    id: root

    // Mock do bar igual GalleryPanel.qml:73 fakeBar
    readonly property color foreground: Color.foreground
    readonly property color background: Color.background
    readonly property string fontFamily: Style.font.family

    readonly property var fakeBar: QtObject {
        readonly property color foreground: root.foreground
        readonly property color background: root.background
        readonly property color barForeground: root.foreground
        readonly property color urgent: Color.urgent
        readonly property string fontFamily: root.fontFamily
        readonly property string position: "top"
        readonly property bool vertical: false
        readonly property int barSize: Style.bar.sizeHorizontal
    }

    FloatingWindow {
        id: win
        title: "WallWeave — preview isolado"
        color: root.background
        implicitWidth: 740
        implicitHeight: 560
        minimumSize: Qt.size(560, 420)
        visible: true

        // Hot-reload em dev já funciona se editar via symlink ou chamar
        // `omarchy-shell shell rescanPlugins` — aqui não precisa, qs -p recarrega no F5
        ScrollView {
            anchors.fill: parent
            anchors.margins: Style.space(18)
            clip: true

            // main.qml lower-case -> tipo é `main` em qml, mas como temos Main.qml
            // o tipo `Main` funciona. Usamos Main aqui.
            Main {
                id: content
                bar: root.fakeBar
                width: win.width - Style.space(36)
            }
        }
    }
}
