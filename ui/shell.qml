// shell.qml
// Omarchy bar widget entry point: a thin layer around the isolated layout
// in Main.qml. It shows a bar button (icon) and opens a PopupCard with
// the full WallWeave UI when the button is clicked.
import QtQuick
import Quickshell
import qs.Commons
import qs.Ui

// Root widget registered with omarchy's bar.
BarWidget {
    id: root
    // Unique module id used by omarchy to load this widget.
    moduleName: "audryus.wallweave"

    // The widget is exactly as big as its button.
    implicitWidth: button.implicitWidth
    implicitHeight: button.implicitHeight

    // --- Popup open/close helpers ---
    readonly property bool opened: wallweavePopup.open
    function open() { wallweavePopup.open = true }
    function close() { wallweavePopup.open = false }
    // Toggle the popup on left/middle click (right click is ignored).
    function toggle() { if (wallweavePopup.open) close(); else open() }

    // Bar button: shows the wallpaper icon and opens/closes the popup.
    BarIconButton {
        id: button
        bar: root.bar
        // nf-md-wallpaper — Material Design "wallpaper" glyph
        text: ""
        tooltipText: "Wall Weave"
        // Left/middle click toggles the popup; right click does nothing.
        onPressed: function(b) { if (b === Qt.RightButton) return; root.toggle() }
    }

    // Popup card anchored to the button; contains the full Main UI.
    PopupCard {
        id: wallweavePopup
        anchorItem: button
        owner: root
        bar: root.bar
        // Keep the popup open automatically during development.
        open: Quickshell.env("WALLWEAVE_DEV") === "1" ? true : false
        // Size: fit the content width and its natural height.
        contentWidth: fittedContentWidth(Style.space(720))
        contentHeight: fittedContentHeight(content.implicitHeight + Style.space(8))

        // The WallWeave content itself (header, menu, panels).
        Main {
            id: content
            bar: root.bar
            width: parent.width
        }
    }
}
