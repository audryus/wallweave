import QtQuick
import qs.Commons
import qs.Ui

Item {
    id: root
    width: statusRow.width
    height: parent.height

    // injetado pelo pai (Main.qml / shell.qml). Ex: Status { backend: backend }
    property var backend: null

    // estado inicial até chegar resposta do Go
    property var widgetStatus: ({ color: '#ff0000', label: "Loading...", mpvpaper: false, hyprpaper: false, connected: false })

    // único ponto de envio: cobre injeção tardia (Main cria backend depois) e criação
    //onBackendChanged: if (backend) Qt.callLater(() => backend.send({cmd: "status"}))
    Component.onCompleted: if (backend) Qt.callLater(() => backend.send({cmd: "get_status"}))

    // atualiza quando Backend emite statusReceived
    Connections {
        target: backend
        function onStatusReceived(s) { 
            root.widgetStatus = s 
            root.widgetStatus.connected = true
        }
    }

    Row {
        id: statusRow
        anchors.centerIn: parent
        spacing: Style.space(6)
        Rectangle {
            anchors.verticalCenter: parent.verticalCenter
            width: Style.space(10); height: Style.space(10); radius: 5
            color: root.widgetStatus.color || "#666"
            border.width: Style.spacing.hairline
            border.color: Qt.rgba(0,0,0,0.12)
        }
        Text {
            anchors.verticalCenter: parent.verticalCenter
            text: root.widgetStatus.label || "..."
            color: root.widgetStatus.color || "#666"
            font.family: Style.font.family
            font.pixelSize: Style.font.caption
            font.bold: true
        }
    }

    // expõe hover para o pai (Main.qml) manter o details aberto ao mover para o card
    readonly property bool hovered: statusHover.hovered
    // mantém hover também quando mouse está sobre o card externo via propriedade injetada
    property bool detailsHovered: false
    readonly property bool showDetails: hovered || detailsHovered

    HoverHandler {
        id: statusHover
        cursorShape: Qt.PointingHandCursor
    }
}