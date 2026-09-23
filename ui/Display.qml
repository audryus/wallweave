// Display.qml
// "Displays" panel: one card per monitor. Each card lets the user pick
// which library (wallpaper folder) that monitor rotates through, change
// the rotation interval with a slider, and enable/disable video
// wallpapers. It also shows warnings when hyprpaper or mpvpaper are not
// installed. All labels go through I18n.
import QtQuick
import Quickshell
import Quickshell.Io
import QtQuick.Controls
import qs.Commons
import qs.Ui
import "./i18n"

// Root column that stacks the heading and the scrollable monitor list.
Column {
    id: root
    width: parent.width - Style.space(32)
    spacing: Style.space(8)
    // Visibility is controlled by the Menu of Main (active flag), not here.
    visible: active
    opacity: active ? 1 : 0

    // Ask for the display list as soon as the component is completed.
    Component.onCompleted: if (backend) Qt.callLater(() => backend.send({cmd: "browse_displays"}))

    // ----- Properties injected by the parent (Main.qml) -----
    property var backend: null
    property var bar: null
    readonly property color foreground: bar ? bar.foreground : Color.foreground
    readonly property string fontFamily: bar ? bar.fontFamily : Style.font.family
    // Visibility is controlled by the Menu of Main, not inside this file.
    property bool active: true

    // ----- Local state -----
    // Libraries available to choose from (passed in from Library.qml).
    property var libraries: []
    // The monitors/displays returned by the backend.
    property var displays: []
    // Editable settings per monitor, keyed by monitor name.
    property var monitorState: ({})
    // Health status (mpvpaper/hyprpaper installed?), passed from Main.
    property var status: null

    // getLibraries builds the dropdown options: always offer "Omarchy
    // global" (value 0 = no per-display library) followed by every library.
    // The global label is translated at call time via I18n.
    function getLibraries() {
        let tmp = [{value: 0, label: I18n.tr("display.omarchy_global")}]

        for (let i=0; i<libraries.length; i++) {
            tmp.push({value: libraries[i].id, label: libraries[i].path})
        }
        return tmp || []
    }

    // stateForMonitor returns the saved settings for one monitor, or safe
    // defaults (theme 0, 60s timer, video off) when nothing is saved yet.
    function stateForMonitor(name) {
        var s = monitorState[String(name)]
        if (s) return s
        return { theme: 0, timer: 60, video: false, seed: 0 }
    }

    // ensureState fills monitorState with an entry for every display that
    // does not have one yet (used after the display list arrives).
    function ensureState() {
        if (displays.length === 0) return
        var changed = false
        // Copy the current state into a new object.
        var next = {}
        for (var k in monitorState) next[k] = monitorState[k]
        // Add missing monitors with defaults taken from their data.
        for (var i = 0; i < displays.length; i++) {
            var m = displays[i]
            var n = String(m.name)
            if (!next[n]) {
                next[n] = { theme: Number(m.theme) || 0, timer: Number(m.timer) || 60, video: !!m.video, seed: Number(m.seed) || 0 }
                changed = true
            }
        }
        if (changed || Object.keys(monitorState).length === 0) monitorState = next
    }

    // updateMonitor merges a patch into one monitor's local state, then
    // sends the full updated display to the backend ("edit_display").
    function updateMonitor(name, patch) {
        var n = String(name)
        // Merge: current state + the changed fields.
        var cur = stateForMonitor(n)
        var merged = {}
        for (var k in cur) merged[k] = cur[k]
        for (var pk in patch) merged[pk] = patch[pk]
        merged.name = n
        // Replace the entry in the state map (immutably, so bindings fire).
        var next = {}
        for (var k in monitorState) next[k] = monitorState[k]
        next[n] = merged
        monitorState = next

        // Persist the change in the backend/database.
        if (backend) backend.send({cmd: "edit_display", display: merged})
    }

    // Listen to ONLY the typed display signals — not statusReceived.
    Connections {
        target: backend
        // The display list arrived after browse_displays.
        function onDisplaysReceived(data) {
            if (Array.isArray(data)) root.displays = data
            else if (data && Array.isArray(data.displays)) root.displays = data.displays
            else { console.log("displays unexpected payload", JSON.stringify(data)); return }
            root.ensureState()
        }
        // An error came from Go — log it (code is available for i18n later).
        function onErrorReceived(msg, code) { console.warn("Display backend error:", code || "", msg) }
    }

    // Fade the panel in/out when it becomes active/inactive.
    Behavior on opacity { NumberAnimation { duration: 120 } }

    // Section heading above the monitor list.
    Text {
        text: I18n.tr("display.heading")
        color: Qt.darker(root.bar ? root.bar.foreground : Color.foreground, 1.5)
        font.family: root.bar ? root.bar.fontFamily : Style.font.family
        font.pixelSize: Style.font.caption
        font.letterSpacing: 1
        font.bold: true
    }

    // Scrollable area for the list of monitor cards.
    Flickable {
        width: parent.width
        height: Math.min(monitorsCol.implicitHeight, Style.space(420))
        contentWidth: width
        contentHeight: monitorsCol.implicitHeight
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        interactive: contentHeight > height

        // Vertical stack of monitor cards.
        Column {
            id: monitorsCol
            width: parent.width
            spacing: Style.space(12)

            // One card per display/monitor.
            Repeater {
                model: root.displays
                Rectangle {
                    required property var modelData
                    width: monitorsCol.width
                    implicitHeight: monCard.implicitHeight + Style.space(20)
                    radius: Style.cornerRadius
                    color: Qt.rgba((root.bar ? root.bar.foreground : Color.foreground).r, (root.bar ? root.bar.foreground : Color.foreground).g, (root.bar ? root.bar.foreground : Color.foreground).b, 0.06)
                    border.width: Style.spacing.hairline
                    border.color: Qt.rgba((root.bar ? root.bar.foreground : Color.foreground).r, (root.bar ? root.bar.foreground : Color.foreground).g, (root.bar ? root.bar.foreground : Color.foreground).b, 0.12)
                    // This monitor's data and its editable local state.
                    readonly property var mon: modelData
                    readonly property var st: root.stateForMonitor(mon.name)

                    // Content of one monitor card: header, library picker,
                    // interval slider, video toggle.
                    Column {
                        id: monCard
                        width: parent.width - Style.space(20)
                        anchors.horizontalCenter: parent.horizontalCenter
                        anchors.top: parent.top
                        anchors.topMargin: Style.space(10)
                        spacing: Style.space(8)

                        // Header row: monitor name + resolution.
                        Row {
                            width: parent.width
                            spacing: Style.space(6)
                            // Monitor name (with a small screen emoji).
                            Text {
                                text: "🖥 " + mon.name
                                color: root.bar ? root.bar.foreground : Color.foreground
                                font.family: root.bar ? root.bar.fontFamily : Style.font.family
                                font.pixelSize: Style.font.body
                                font.bold: true
                                elide: Text.ElideRight
                                width: parent.width - resLabel2.width - Style.space(8)
                            }
                            // Resolution label (e.g. 1920×1080).
                            Text {
                                id: resLabel2
                                anchors.verticalCenter: parent.verticalCenter
                                text: mon.width + "×" + mon.height
                                color: Qt.darker(root.bar ? root.bar.foreground : Color.foreground, 1.4)
                                font.family: root.bar ? root.bar.fontFamily : Style.font.family
                                font.pixelSize: Style.font.bodySmall
                            }
                        }

                        // Label above the library dropdown.
                        Text {
                            text: I18n.tr("display.library")
                            color: Qt.darker(root.bar ? root.bar.foreground : Color.foreground, 1.4)
                            font.family: root.bar ? root.bar.fontFamily : Style.font.family
                            font.pixelSize: Style.font.caption
                        }

                        // Library picker area: dropdown + optional warning.
                        Column {
                            width: parent.width
                            spacing: Style.space(4)
                            opacity: 1
                            // Dropdown to choose which library this monitor uses.
                            Dropdown {
                                width: parent.width
                                value: st.theme
                                options: root.getLibraries()
                                foreground: root.bar ? root.bar.foreground : Color.foreground
                                fontFamily: root.bar ? root.bar.fontFamily : Style.font.family
                                enabled: true
                                onChanged: function(v) { root.updateMonitor(mon.name, { theme: Number(v) }) }
                            }
                            // Warning when hyprpaper is not installed: every
                            // monitor will share the same global image.
                            Text {
                                visible: !root.status.hyprpaper
                                width: parent.width
                                wrapMode: Text.WordWrap
                                text: I18n.tr("display.hyprpaper_missing")
                                color: "#f1c40f"
                                font.family: root.bar ? root.bar.fontFamily : Style.font.family
                                font.pixelSize: Style.font.caption
                                font.italic: true
                            }
                        }

                        // Label above the rotation interval slider.
                        Text {
                            text: I18n.tr("display.change_wallpaper")
                            color: Qt.darker(root.bar ? root.bar.foreground : Color.foreground, 1.4)
                            font.family: root.bar ? root.bar.fontFamily : Style.font.family
                            font.pixelSize: Style.font.caption
                            topPadding: Style.space(4)
                        }

                        // Interval slider area: min/max labels, slider, value.
                        Column {
                            width: parent.width
                            spacing: Style.space(2)
                            // Row with the range labels (60s … 300s).
                            Row {
                                width: parent.width
                                spacing: Style.space(8)
                                // Minimum value label (60 seconds).
                                Text { id: t60b; text: I18n.tr("display.seconds_short_min"); color: Qt.darker(root.bar ? root.bar.foreground : Color.foreground, 1.4); font.family: root.bar ? root.bar.fontFamily : Style.font.family; font.pixelSize: Style.font.caption }
                                // Spacer that pushes the max label to the right.
                                Item { width: parent.width - t60b.width - t300b.width - parent.spacing*2; height: 1 }
                                // Maximum value label (300 seconds).
                                Text { id: t300b; text: I18n.tr("display.seconds_short_max"); color: Qt.darker(root.bar ? root.bar.foreground : Color.foreground, 1.4); font.family: root.bar ? root.bar.fontFamily : Style.font.family; font.pixelSize: Style.font.caption }
                            }
                            // Slider: how often the wallpaper changes (60–300s).
                            // onMoved fires only when the user actually drags/releases.
                            Slider {
                                id: intervalSlider
                                width: parent.width
                                from: 60
                                to: 300
                                stepSize: 1
                                snapMode: Slider.SnapOnRelease
                                value: st.timer
                                onMoved: root.updateMonitor(mon.name, { timer: Math.round(value) })
                            }
                            // Current interval shown under the slider.
                            Text {
                                width: parent.width
                                horizontalAlignment: Text.AlignHCenter
                                text: I18n.tr("display.seconds", [Math.round(intervalSlider.value)])
                                color: root.bar ? root.bar.foreground : Color.foreground
                                font.family: root.bar ? root.bar.fontFamily : Style.font.family
                                font.pixelSize: Style.font.caption
                                font.bold: true
                            }
                        }

                        // Video toggle row: label + custom switch. Dimmed
                        // when mpvpaper is not installed.
                        Row {
                            width: parent.width
                            spacing: Style.space(8)
                            opacity: root.status.mpvpaper ? 1 : 0.45
                            // "Play videos" label.
                            Text {
                                anchors.verticalCenter: parent.verticalCenter
                                text: I18n.tr("display.play_videos")
                                color: root.bar ? root.bar.foreground : Color.foreground
                                font.family: root.bar ? root.bar.fontFamily : Style.font.family
                                font.pixelSize: Style.font.bodySmall
                                width: parent.width - toggleRect2.width - parent.spacing
                                elide: Text.ElideRight
                            }
                            // Custom on/off switch for video wallpapers.
                            Rectangle {
                                id: toggleRect2
                                anchors.verticalCenter: parent.verticalCenter
                                width: Style.space(44)
                                height: Style.space(24)
                                radius: height/2
                                color: st.video ? Color.accent : Qt.rgba((root.bar ? root.bar.foreground : Color.foreground).r, (root.bar ? root.bar.foreground : Color.foreground).g, (root.bar ? root.bar.foreground : Color.foreground).b, 0.18)
                                border.width: Style.spacing.hairline
                                border.color: Qt.rgba((root.bar ? root.bar.foreground : Color.foreground).r, (root.bar ? root.bar.foreground : Color.foreground).g, (root.bar ? root.bar.foreground : Color.foreground).b, 0.12)
                                // The sliding circle inside the switch.
                                Rectangle {
                                    width: Style.space(18); height: Style.space(18); radius: width/2
                                    anchors.verticalCenter: parent.verticalCenter
                                    x: st.video ? parent.width - width - 3 : 3
                                    color: Color.background
                                    Behavior on x { NumberAnimation { duration: 120; easing.type: Easing.OutCubic } }
                                }
                                // Click area: toggles video on/off (only when
                                // mpvpaper is installed).
                                MouseArea {
                                    anchors.fill: parent
                                    enabled: root.status.mpvpaper
                                    cursorShape: enabled ? Qt.PointingHandCursor : Qt.ArrowCursor
                                    onClicked: {
                                        if (!root.status.mpvpaper) return
                                        root.updateMonitor(mon.name, { video: !st.video })
                                    }
                                }
                            }
                        }
                        // Warning when mpvpaper is not installed: videos are
                        // disabled.
                        Text {
                            visible: !root.status.mpvpaper
                            width: parent.width
                            wrapMode: Text.WordWrap
                            text: I18n.tr("display.mpvpaper_missing")
                            color: "#f1c40f"
                            font.family: root.bar ? root.bar.fontFamily : Style.font.family
                            font.pixelSize: Style.font.caption
                            font.italic: true
                        }
                    }
                }
            }

            // Shown when no monitors were detected.
            Text {
                visible: root.displays.length === 0
                width: parent.width
                wrapMode: Text.WordWrap
                horizontalAlignment: Text.AlignHCenter
                text: I18n.tr("display.empty")
                color: Qt.darker(root.bar ? root.bar.foreground : Color.foreground, 1.6)
                font.family: root.bar ? root.bar.fontFamily : Style.font.family
                font.pixelSize: Style.font.caption
                font.italic: true
            }
        }
    }
    }
