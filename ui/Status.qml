// Status.qml
// Small status badge shown in the header: a colored dot plus a label
// (Running / Degraded / Loading...). It asks the backend for the health
// status and exposes hover state so Main can keep a details card open.
// The badge text is translated: Go sends a stable code (running/degraded),
// which I18n maps to the current language.
import QtQuick
import qs.Commons
import qs.Ui
import "./i18n"

// Root item of the status badge; its width hugs the content.
Item {
    id: root
    width: statusRow.width
    height: parent.height

    // Injected by the parent (Main.qml / shell.qml). E.g.: Status { backend: backend }
    property var backend: null

    // Initial state until the first response arrives from Go. label is
    // empty on purpose — the badge shows the translated "Loading..." then.
    property var widgetStatus: ({ color: '#ff0000', label: "", mpvpaper: false, hyprpaper: false, connected: false })

    // Single place that sends the status request: covers late injection
    // (Main creates the backend after this component) and normal creation.
    Component.onCompleted: if (backend) Qt.callLater(() => backend.send({cmd: "get_status"}))

    // Update the badge whenever the backend emits statusReceived.
    Connections {
        target: backend
        function onStatusReceived(s) {
            root.widgetStatus = s
            root.widgetStatus.connected = true
        }
    }

    // The visible badge: a colored dot + the status label, centered.
    Row {
        id: statusRow
        anchors.centerIn: parent
        spacing: Style.space(6)

        // Small colored circle indicating the status color.
        Rectangle {
            anchors.verticalCenter: parent.verticalCenter
            width: Style.space(10); height: Style.space(10); radius: 5
            color: root.widgetStatus.color || "#666"
            border.width: Style.spacing.hairline
            border.color: Qt.rgba(0,0,0,0.12)
        }

        // Text label next to the dot (translated status code, or Loading...).
        Text {
            anchors.verticalCenter: parent.verticalCenter
            text: {
                var code = root.widgetStatus.label
                if (!code) return I18n.tr("status.loading")
                return I18n.tr("status." + code)
            }
            color: root.widgetStatus.color || "#666"
            font.family: Style.font.family
            font.pixelSize: Style.font.caption
            font.bold: true
        }
    }

    // Expose hover to the parent (Main.qml) so the details card stays open
    // while the mouse moves from the badge to the card.
    readonly property bool hovered: statusHover.hovered
    // Keep hover also when the mouse is over the outer card, via a property
    // injected by the parent.
    property bool detailsHovered: false
    // Show the details card when either the badge or the card is hovered.
    readonly property bool showDetails: hovered || detailsHovered

    // Detects when the mouse is over the badge (and shows a pointer cursor).
    HoverHandler {
        id: statusHover
        cursorShape: Qt.PointingHandCursor
    }
}
