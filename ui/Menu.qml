import QtQuick
import qs.Commons
import qs.Ui

Column {
    id: root
    width: Style.space(170)
    spacing: Style.space(20)
    topPadding: Style.space(16)
    leftPadding: Style.space(12)
    rightPadding: Style.space(12)
    bottomPadding: Style.space(12)

    property int selected: 1 // 0 = Bibliotecas, 1 = Monitores

    Rectangle {
        width: parent.width - Style.space(20)
        height: Style.space(40)
        radius: Style.cornerRadius
        color: selected === 0
            ? Style.selectedStateColor(root.bar ? root.bar.foreground : Color.foreground, Color.accent)
            : (bibMouse.containsMouse ? Style.hoverFillFor(root.bar ? root.bar.foreground : Color.foreground, Color.accent) : "transparent")
        border.width: root.selected === 0 ? 0 : Style.spacing.hairline
        border.color: Qt.rgba((root.bar ? root.bar.foreground : Color.foreground).r, (root.bar ? root.bar.foreground : Color.foreground).g, (root.bar ? root.bar.foreground : Color.foreground).b, 0.12)
        Row {
            anchors.fill: parent
            anchors.leftMargin: Style.space(10)
            spacing: Style.space(8)
            anchors.verticalCenter: parent.verticalCenter
            Text { text: "▦"; color: root.selected===0?Color.background:(root.bar?root.bar.foreground:Color.foreground); font.pixelSize: Style.font.body; anchors.verticalCenter: parent.verticalCenter }
            Text {
                text: "Libraries"
                color: selected === 0 ? Color.background : (root.bar ? root.bar.foreground : Color.foreground)
                font.family: root.bar ? root.bar.fontFamily : Style.font.family
                font.pixelSize: Style.font.bodySmall
                font.bold: selected === 0
                anchors.verticalCenter: parent.verticalCenter
            }
        }
        MouseArea { id: bibMouse; anchors.fill: parent; hoverEnabled: true; cursorShape: Qt.PointingHandCursor; onClicked: selected = 0 }
    }

    Rectangle {
        width: parent.width - Style.space(20)
        height: Style.space(40)
        radius: Style.cornerRadius
        color: selected === 1
            ? Style.selectedStateColor(root.bar ? root.bar.foreground : Color.foreground, Color.accent)
            : (monMouse.containsMouse ? Style.hoverFillFor(root.bar ? root.bar.foreground : Color.foreground, Color.accent) : "transparent")
        border.width: selected === 1 ? 0 : Style.spacing.hairline
        border.color: Qt.rgba((root.bar ? root.bar.foreground : Color.foreground).r, (root.bar ? root.bar.foreground : Color.foreground).g, (root.bar ? root.bar.foreground : Color.foreground).b, 0.12)
        Row {
            anchors.fill: parent
            anchors.leftMargin: Style.space(10)
            spacing: Style.space(8)
            anchors.verticalCenter: parent.verticalCenter
            Text { text: "▣"; color: selected===1?Color.background:(root.bar?root.bar.foreground:Color.foreground); font.pixelSize: Style.font.body; anchors.verticalCenter: parent.verticalCenter }
            Text {
            text: "Displays"
            color: selected === 1 ? Color.background : (root.bar ? root.bar.foreground : Color.foreground)
            font.family: root.bar ? root.bar.fontFamily : Style.font.family
            font.pixelSize: Style.font.bodySmall
            font.bold: selected === 1
            anchors.verticalCenter: parent.verticalCenter
            }
        }
        MouseArea { id: monMouse; anchors.fill: parent; hoverEnabled: true; cursorShape: Qt.PointingHandCursor; onClicked: selected = 1 }
    }
}