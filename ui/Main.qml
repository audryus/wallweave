// Main.qml
// The complete WallWeave popup content: header (title + status badge),
// an expandable status details card, a separator, and a two-column body
// with the left menu and the right panel (Libraries or Displays).
// Used both inside shell.qml (within the omarchy PopupCard) and inside
// _preview.qml (standalone preview window). No BarWidget/PopupCard here.
import QtQuick
import QtQuick.Layouts
import qs.Commons
import qs.Ui
import "./i18n"

// Root column that stacks: header, status card, separator, content row.
Column {
    id: root

    // Injected by the host: fakeBar in _preview, root.bar in omarchy.
    property var bar: null
    readonly property color foreground: bar ? bar.foreground : Color.foreground
    readonly property color background: bar ? bar.background : Color.background
    readonly property string fontFamily: bar ? bar.fontFamily : Style.font.family

    // Non-visual backend — a QtObject, so it does not take layout space.
    Backend { id: backend }

    // Width comes from the parent (PopupCard) or the FloatingWindow preview.
    width: parent ? parent.width : 680
    spacing: Style.space(14)

    // --- Header row: the app title on the left, the status badge on the right ---
    Row {
        width: parent.width
        height: Style.space(36)
        leftPadding: Style.space(16)
        rightPadding: Style.space(16)
        spacing: Style.space(8)

        // Application title.
        Text {
            anchors.verticalCenter: parent.verticalCenter
            text: "Wall Weave"
            color: root.foreground
            font.family: root.fontFamily
            font.pixelSize: Style.font.title
            font.bold: true
        }
        // Flexible spacer that pushes the status badge to the right.
        Item { width: parent.width - 140 - statusItem.width; height: 1 } // spacer

        // Status badge (dot + label) that queries the backend for health.
        Status {
            id: statusItem
            backend: backend
            anchors.verticalCenter: parent.verticalCenter
        }
    }

    // --- Inline status details card ---
    // Width matches Main (not Status), sits below the header (outside the
    // 36px header row), and expands the PopupCard via implicitHeight.
    Rectangle {
        id: statusDetailsCard
        width: parent.width - Style.space(32)
        anchors.horizontalCenter: parent.horizontalCenter
        // Visible while hovering the Status badge OR this card itself
        // (no flicker when moving between them).
        visible: opacity > 0
        opacity: statusItem.showDetails ? 1 : 0
        height: statusItem.showDetails ? statusDetailsCol.implicitHeight + Style.space(16) : 0
        clip: true
        color: Qt.rgba(root.foreground.r, root.foreground.g, root.foreground.b, 0.06)
        border.width: Style.spacing.hairline
        border.color: Qt.rgba(root.foreground.r, root.foreground.g, root.foreground.b, 0.12)
        radius: Style.cornerRadius
        // Animate height and opacity when the card opens/closes.
        Behavior on height { NumberAnimation { duration: 120; easing.type: Easing.OutCubic } }
        Behavior on opacity { NumberAnimation { duration: 120 } }

        // Bridge: when the mouse enters the card, keep Status.showDetails
        // true so the card does not close while the user is reading it.
        HoverHandler {
            id: detailsHover
            onHoveredChanged: statusItem.detailsHovered = hovered
        }

        // Vertical stack of status lines inside the card.
        Column {
            id: statusDetailsCol
            width: parent.width - Style.space(20)
            anchors.centerIn: parent
            spacing: Style.space(4)
            // Connection state of the backend (translated Yes/No).
            Text {
                text: I18n.tr("status.connected", [
                    statusItem.widgetStatus.connected ? I18n.tr("common.yes") : I18n.tr("common.no")
                ])
                color: statusItem.widgetStatus.connected ? "#2ecc71" : "#e74c3c"
                font.family: root.fontFamily
                font.pixelSize: Style.font.caption
                font.bold: !statusItem.widgetStatus.mpvpaper
                width: parent.width
                wrapMode: Text.WordWrap
            }
            // Whether mpvpaper (video wallpapers) is installed.
            Text {
                text: I18n.tr("status.mpvpaper", [
                    statusItem.widgetStatus.mpvpaper ? I18n.tr("status.installed") : I18n.tr("status.not_installed")
                ])
                color: statusItem.widgetStatus.mpvpaper ? "#2ecc71" : "#e74c3c"
                font.family: root.fontFamily
                font.pixelSize: Style.font.caption
                font.bold: !statusItem.widgetStatus.mpvpaper
                width: parent.width
                wrapMode: Text.WordWrap
            }
            // Extra hint when mpvpaper is missing.
            Text {
                visible: !statusItem.widgetStatus.mpvpaper
                text: I18n.tr("status.mpvpaper_missing")
                color: "#e74c3c"
                font.family: root.fontFamily
                font.pixelSize: Style.font.caption
                font.italic: true
            }
            // Whether hyprpaper (per-monitor images) is installed.
            Text {
                text: I18n.tr("status.hyprpaper", [
                    statusItem.widgetStatus.hyprpaper ? I18n.tr("status.installed") : I18n.tr("status.not_installed")
                ])
                color: statusItem.widgetStatus.hyprpaper ? "#2ecc71" : "#f1c40f"
                font.family: root.fontFamily
                font.pixelSize: Style.font.caption
                font.bold: !statusItem.widgetStatus.hyprpaper
                width: parent.width
                wrapMode: Text.WordWrap
            }
            // Extra hint when hyprpaper is missing: Go sends the
            // "omarchy_fallback" code, which I18n turns into a sentence.
            Text {
                visible: !statusItem.widgetStatus.hyprpaper
                text: statusItem.widgetStatus.image
                    ? I18n.tr("status." + statusItem.widgetStatus.image)
                    : I18n.tr("status.omarchy_fallback")
                color: "#f1c40f"
                font.family: root.fontFamily
                font.pixelSize: Style.font.caption
                font.italic: true
            }
            // Happy path: both tools installed and running.
            Text {
                // Go sends color "#2ecc71" + label "running" when all is well.
                visible: statusItem.widgetStatus.color === "#2ecc71"
                text: I18n.tr("status.both_ok")
                color: "#2ecc71"
                font.family: root.fontFamily
                font.pixelSize: Style.font.caption
                font.bold: true
                topPadding: Style.space(4)
            }
        }
    }

    // Thin horizontal separator between the header area and the content.
    PanelSeparator { width: parent.width; foreground: root.foreground }

    // --- Content row: menu on the left, panel on the right ---
    Row {
        id: contentRow
        width: parent.width
        spacing: 0

        // Left navigation menu (Libraries / Displays).
        Menu {
            id: menu
        }

        // Vertical hairline separating the menu from the panel.
        Rectangle {
          width: Style.spacing.hairline
          height: contentRow.height
          color: Qt.rgba((root.bar ? root.bar.foreground : Color.foreground).r, (root.bar ? root.bar.foreground : Color.foreground).g, (root.bar ? root.bar.foreground : Color.foreground).b, 0.12)
        }

        // Right column: holds the two panels, only one visible at a time.
        Column {
          id: rightCol
          width: parent.width - menu.width - 1
          leftPadding: Style.space(16)
          rightPadding: Style.space(16)
          topPadding: Style.space(12)
          bottomPadding: Style.space(12)
          spacing: Style.space(12)

          // Libraries panel — visible when menu index is 0.
          Library {
            id: libraryCol
            backend: backend
            bar: root.bar
            active: menu.selected === 0
          }

          // Displays panel — visible when menu index is 1.
          // It receives the library list and the health status from here.
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
