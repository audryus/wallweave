import QtQuick
import QtQuick.Layouts
import qs.Commons
import qs.Ui


// Conteúdo isolado do wallweave — sem BarWidget, sem PopupCard.
// Usado tanto em shell.qml (dentro do PopupCard do omarchy)
// quanto em _preview.qml (via qs -p).
Column {
    id: root

    // Injetado pelo host. Em _preview é fakeBar, no omarchy é root.bar
    property var bar: null
    readonly property color foreground: bar ? bar.foreground : Color.foreground
    readonly property color background: bar ? bar.background : Color.background
    readonly property string fontFamily: bar ? bar.fontFamily : Style.font.family

    // Backend não-visual — QtObject, não entra no layout do Column
    Backend { id: backend }

    // Largura vem do parent (PopupCard) ou do FloatingWindow no preview
    width: parent ? parent.width : 680
    spacing: Style.space(14)

    // Header
    Row {
        width: parent.width
        height: Style.space(36)
        leftPadding: Style.space(16)
        rightPadding: Style.space(16)
        spacing: Style.space(8)

        Text {
            anchors.verticalCenter: parent.verticalCenter
            text: "Wall Weave"
            color: root.foreground
            font.family: root.fontFamily
            font.pixelSize: Style.font.title
            font.bold: true
        }
        Item { width: parent.width - 140 - statusItem.width; height: 1 } // spacer

        Status {
            id: statusItem
            backend: backend
            anchors.verticalCenter: parent.verticalCenter
        }
    }

    // — Inline details: largura = Main (não Status), fora do Row de 36px
    //   fica abaixo do header e expande o PopupCard via implicitHeight
    Rectangle {
        id: statusDetailsCard
        width: parent.width - Style.space(32)
        anchors.horizontalCenter: parent.horizontalCenter
        // visível enquanto hover no Status OU no próprio card (sem flicker)
        visible: opacity > 0
        opacity: statusItem.showDetails ? 1 : 0
        height: statusItem.showDetails ? statusDetailsCol.implicitHeight + Style.space(16) : 0
        clip: true
        color: Qt.rgba(root.foreground.r, root.foreground.g, root.foreground.b, 0.06)
        border.width: Style.spacing.hairline
        border.color: Qt.rgba(root.foreground.r, root.foreground.g, root.foreground.b, 0.12)
        radius: Style.cornerRadius
        Behavior on height { NumberAnimation { duration: 120; easing.type: Easing.OutCubic } }
        Behavior on opacity { NumberAnimation { duration: 120 } }

        // ponte: quando mouse entra no card, mantém Status.showDetails
        HoverHandler {
            id: detailsHover
            onHoveredChanged: statusItem.detailsHovered = hovered
        }

        Column {
            id: statusDetailsCol
            width: parent.width - Style.space(20)
            anchors.centerIn: parent
            spacing: Style.space(4)
            Text {
                text: "Connected: " + (statusItem.widgetStatus.connected ? "Yes" : "No")
                color: statusItem.widgetStatus.connected ? "#2ecc71" : "#e74c3c"
                font.family: root.fontFamily
                font.pixelSize: Style.font.caption
                font.bold: !statusItem.widgetStatus.mpvpaper
                width: parent.width
                wrapMode: Text.WordWrap
            }
            Text {
                text: "mpvpaper: " + (statusItem.widgetStatus.mpvpaper ? "installed" : "not installed")
                color: statusItem.widgetStatus.mpvpaper ? "#2ecc71" : "#e74c3c"
                font.family: root.fontFamily
                font.pixelSize: Style.font.caption
                font.bold: !statusItem.widgetStatus.mpvpaper
                width: parent.width
                wrapMode: Text.WordWrap
            }
            Text {
                visible: !statusItem.widgetStatus.mpvpaper
                text: "→ mpvpaper not installed"
                color: "#e74c3c"
                font.family: root.fontFamily
                font.pixelSize: Style.font.caption
                font.italic: true
            }
            Text {
                text: "hyprpaper: " + (statusItem.widgetStatus.hyprpaper ? "installed" : "not installed")
                color: statusItem.widgetStatus.hyprpaper ? "#2ecc71" : "#f1c40f"
                font.family: root.fontFamily
                font.pixelSize: Style.font.caption
                font.bold: !statusItem.widgetStatus.hyprpaper
                width: parent.width
                wrapMode: Text.WordWrap
            }
            Text {
                visible: !statusItem.widgetStatus.hyprpaper
                text: statusItem.widgetStatus.Image ? statusItem.widgetStatus.Image : "→ Will use Omarchy (global, not per-monitor)"
                color: "#f1c40f"
                font.family: root.fontFamily
                font.pixelSize: Style.font.caption
                font.italic: true
            }
            Text {
                // Go manda color:"green" -> Backend converte para #2ecc71 + label Running
                visible: statusItem.widgetStatus.color === "#2ecc71"
                text: "✓ Both installed and running"
                color: "#2ecc71"
                font.family: root.fontFamily
                font.pixelSize: Style.font.caption
                font.bold: true
                topPadding: Style.space(4)
            }
        }
    }

    PanelSeparator { width: parent.width; foreground: root.foreground }

    Row {
        id: contentRow
        width: parent.width
        spacing: 0

        Menu {
            id: menu
        }

        Rectangle {
          width: Style.spacing.hairline
          height: contentRow.height
          color: Qt.rgba((root.bar ? root.bar.foreground : Color.foreground).r, (root.bar ? root.bar.foreground : Color.foreground).g, (root.bar ? root.bar.foreground : Color.foreground).b, 0.12)
        }

        Column {
          id: rightCol
          width: parent.width - menu.width - 1
          leftPadding: Style.space(16)
          rightPadding: Style.space(16)
          topPadding: Style.space(12)
          bottomPadding: Style.space(12)
          spacing: Style.space(12)

          Library {
            id: libraryCol
            backend: backend
            bar: root.bar
            active: menu.selected === 0
          }

          Display {
            id: displayCol
            backend: backend
            bar: root.bar
            active: menu.selected === 1
            libraries: libraryCol.libraries
            status: statusItem.widgetStatus
          }
        }
    }

}
