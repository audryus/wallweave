import QtQuick
import Quickshell
import qs.Commons
import qs.Ui

// Widget omarchy — fina camada ao redor do layout isolado em main.qml/Main.qml
BarWidget {
    id: root
    moduleName: "audryus.wallweave"

    implicitWidth: button.implicitWidth
    implicitHeight: button.implicitHeight

    readonly property bool opened: wallweavePopup.open
    function open() { wallweavePopup.open = true }
    function close() { wallweavePopup.open = false }
    function toggle() { if (wallweavePopup.open) close(); else open() }

    BarIconButton {
        id: button
        bar: root.bar
        text: "󰸞"
        tooltipText: "Wall Weave"
        onPressed: function(b) { if (b === Qt.RightButton) return; root.toggle() }
    }

    PopupCard {
        id: wallweavePopup
        anchorItem: button
        owner: root
        bar: root.bar
        open: Quickshell.env("WALLWEAVE_DEV") === "1" ? true : false
        contentWidth: fittedContentWidth(Style.space(720))
        contentHeight: fittedContentHeight(content.implicitHeight + Style.space(8))

        Main {
            id: content
            bar: root.bar
            width: parent.width
        }
    }
}
