// _preview.qml
// Isolated preview window for development, launched with:
//   qs -p ui/_preview.qml
// or:
//   QML_IMPORT_PATH=/usr/share/omarchy/shell qs -p ui/_preview.qml
// It reuses Main.qml without BarWidget/PopupCard, using a fake bar object.
import Quickshell
import QtQuick
import QtQuick.Controls
import qs.Commons
import qs.Ui

// Root of the preview shell (one floating window).
ShellRoot {
    id: root

    // Colors/fonts taken from the shared theme (mirrors GalleryPanel's fakeBar).
    readonly property color foreground: Color.foreground
    readonly property color background: Color.background
    readonly property string fontFamily: Style.font.family

    // Mock bar object passed to Main, imitating a real omarchy bar so the
    // component can resolve colors/fonts without a real bar.
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

    // Floating window that hosts the preview content.
    FloatingWindow {
        id: win
        title: "WallWeave — isolated preview"
        color: root.background
        implicitWidth: 740
        implicitHeight: 560
        minimumSize: Qt.size(560, 420)
        visible: true

        // Hot-reload during development already works when editing via
        // symlink or calling `omarchy-shell shell rescanPlugins` — here it
        // is not needed: qs -p reloads on F5.
        ScrollView {
            anchors.fill: parent
            anchors.margins: Style.space(18)
            clip: true

            // main.qml is lower-case, so the QML type would be `main`; but
            // since Main.qml also exists, the type `Main` works — we use
            // Main here.
            Main {
                id: content
                bar: root.fakeBar
                width: win.width - Style.space(36)
            }
        }
    }
}
