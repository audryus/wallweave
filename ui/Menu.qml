// Menu.qml
// Left-side navigation menu of the popup. It has two buttons —
// "Libraries" and "Displays" — and exposes which one is selected so the
// right column can show the matching panel. Labels come from I18n.
import QtQuick
import qs.Commons
import qs.Ui
import "./i18n"

// Root column that lays out the two menu buttons vertically.
Column {
    id: root
    width: Style.space(170)
    spacing: Style.space(20)
    topPadding: Style.space(16)
    leftPadding: Style.space(12)
    rightPadding: Style.space(12)
    bottomPadding: Style.space(12)

    // Which menu item is active: 0 = Libraries, 1 = Displays.
    property int selected: 1 // 0 = Libraries, 1 = Displays

    // --- "Libraries" menu button ---
    // Highlighted when selected; shows a subtle fill on hover otherwise.
    Rectangle {
        width: parent.width - Style.space(20)
        height: Style.space(40)
        radius: Style.cornerRadius
        color: selected === 0
            ? Style.selectedStateColor(root.bar ? root.bar.foreground : Color.foreground, Color.accent)
            : (bibMouse.containsMouse ? Style.hoverFillFor(root.bar ? root.bar.foreground : Color.foreground, Color.accent) : "transparent")
        border.width: root.selected === 0 ? 0 : Style.spacing.hairline
        border.color: Qt.rgba((root.bar ? root.bar.foreground : Color.foreground).r, (root.bar ? root.bar.foreground : Color.foreground).g, (root.bar ? root.bar.foreground : Color.foreground).b, 0.12)

        // Icon + label row inside the button.
        Row {
            anchors.fill: parent
            anchors.leftMargin: Style.space(10)
            spacing: Style.space(8)
            anchors.verticalCenter: parent.verticalCenter
            // Grid icon for the Libraries section.
            Text { text: ""; color: root.selected===0?Color.background:(root.bar?root.bar.foreground:Color.foreground); font.pixelSize: Style.font.body; anchors.verticalCenter: parent.verticalCenter }
            // "Libraries" label.
            Text {
                text: I18n.tr("menu.libraries")
                color: selected === 0 ? Color.background : (root.bar ? root.bar.foreground : Color.foreground)
                font.family: root.bar ? root.bar.fontFamily : Style.font.family
                font.pixelSize: Style.font.bodySmall
                font.bold: selected === 0
                anchors.verticalCenter: parent.verticalCenter
            }
        }
        // Click/hover area: clicking selects the Libraries panel.
        MouseArea { id: bibMouse; anchors.fill: parent; hoverEnabled: true; cursorShape: Qt.PointingHandCursor; onClicked: selected = 0 }
    }

    // --- "Displays" menu button ---
    // Highlighted when selected; shows a subtle fill on hover otherwise.
    Rectangle {
        width: parent.width - Style.space(20)
        height: Style.space(40)
        radius: Style.cornerRadius
        color: selected === 1
            ? Style.selectedStateColor(root.bar ? root.bar.foreground : Color.foreground, Color.accent)
            : (monMouse.containsMouse ? Style.hoverFillFor(root.bar ? root.bar.foreground : Color.foreground, Color.accent) : "transparent")
        border.width: selected === 1 ? 0 : Style.spacing.hairline
        border.color: Qt.rgba((root.bar ? root.bar.foreground : Color.foreground).r, (root.bar ? root.bar.foreground : Color.foreground).g, (root.bar ? root.bar.foreground : Color.foreground).b, 0.12)

        // Icon + label row inside the button.
        Row {
            anchors.fill: parent
            anchors.leftMargin: Style.space(10)
            spacing: Style.space(8)
            anchors.verticalCenter: parent.verticalCenter
            // Monitor icon for the Displays section.
            Text { text: ""; color: selected===1?Color.background:(root.bar?root.bar.foreground:Color.foreground); font.pixelSize: Style.font.body; anchors.verticalCenter: parent.verticalCenter }
            // "Displays" label.
            Text {
            text: I18n.tr("menu.displays")
            color: selected === 1 ? Color.background : (root.bar ? root.bar.foreground : Color.foreground)
            font.family: root.bar ? root.bar.fontFamily : Style.font.family
            font.pixelSize: Style.font.bodySmall
            font.bold: selected === 1
            anchors.verticalCenter: parent.verticalCenter
            }
        }
        // Click/hover area: clicking selects the Displays panel.
        MouseArea { id: monMouse; anchors.fill: parent; hoverEnabled: true; cursorShape: Qt.PointingHandCursor; onClicked: selected = 1 }
    }
}
