import QtQuick
import Quickshell
import Quickshell.Io
import QtQuick.Controls
import qs.Commons
import qs.Ui


Column {
    id: root
    width: parent.width - Style.space(32)
    spacing: Style.space(8)
    visible: active
    opacity: active ? 1 : 0

    Component.onCompleted: if (backend) Qt.callLater(() => backend.send({cmd: "browse_displays"}))

    // ----- props injetadas pelo pai -----
    property var backend: null
    property var bar: null
    readonly property color foreground: bar ? bar.foreground : Color.foreground
    readonly property string fontFamily: bar ? bar.fontFamily : Style.font.family
    // visibilidade controlada pelo Menu de Main, não aqui dentro
    property bool active: true

    // ----- estado local -----
    property var libraries: []
    property var displays: []
    property var monitorState: ({})
    property var status: null

    function getLibraries() {
        let tmp = [{value: 0, label: "Omarchy global (not per-display)"}]

        for (let i=0; i<libraries.length; i++) {
            tmp.push({value: libraries[i].id, label: libraries[i].path})
        }
        return tmp || []
    }

    function stateForMonitor(name) {
        var s = monitorState[String(name)]
        if (s) return s
        return { theme: 0, timer: 60, video: false, seed: 0 }
    }

    function ensureState() {
        if (displays.length === 0) return
        var changed = false
        var next = {}
        for (var k in monitorState) next[k] = monitorState[k]
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

    function updateMonitor(name, patch) {
        var n = String(name)
        var cur = stateForMonitor(n)
        var merged = {}
        for (var k in cur) merged[k] = cur[k]
        for (var pk in patch) merged[pk] = patch[pk]
        merged.name = n
        var next = {}
        for (var k in monitorState) next[k] = monitorState[k]
        next[n] = merged
        monitorState = next

        if (backend) backend.send({cmd: "edit_display", display: merged})
    }

    // escuta SÓ o sinal tipado — não statusReceived
    Connections {
        target: backend
        function onDisplaysReceived(data) {
            if (Array.isArray(data)) root.displays = data
            else if (data && Array.isArray(data.displays)) root.displays = data.displays
            else { console.log("displays payload inesperado", JSON.stringify(data)); return }
            root.ensureState()
        }
        function onErrorReceived(msg) { console.warn("Display erro backend:", msg) }
    }

    Behavior on opacity { NumberAnimation { duration: 120 } }

    Text {
        text: "DISPLAYS"
        color: Qt.darker(root.bar ? root.bar.foreground : Color.foreground, 1.5)
        font.family: root.bar ? root.bar.fontFamily : Style.font.family
        font.pixelSize: Style.font.caption
        font.letterSpacing: 1
        font.bold: true
    }

    Flickable {
        width: parent.width
        height: Math.min(monitorsCol.implicitHeight, Style.space(420))
        contentWidth: width
        contentHeight: monitorsCol.implicitHeight
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        interactive: contentHeight > height

        Column {
            id: monitorsCol
            width: parent.width
            spacing: Style.space(12)

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
                    readonly property var mon: modelData
                    readonly property var st: root.stateForMonitor(mon.name)

                    Column {
                        id: monCard
                        width: parent.width - Style.space(20)
                        anchors.horizontalCenter: parent.horizontalCenter
                        anchors.top: parent.top
                        anchors.topMargin: Style.space(10)
                        spacing: Style.space(8)

                        Row {
                            width: parent.width
                            spacing: Style.space(6)
                            Text {
                                text: "🖥 " + mon.name
                                color: root.bar ? root.bar.foreground : Color.foreground
                                font.family: root.bar ? root.bar.fontFamily : Style.font.family
                                font.pixelSize: Style.font.body
                                font.bold: true
                                elide: Text.ElideRight
                                width: parent.width - resLabel2.width - Style.space(8)
                            }
                            Text {
                                id: resLabel2
                                anchors.verticalCenter: parent.verticalCenter
                                text: mon.width + "×" + mon.height
                                color: Qt.darker(root.bar ? root.bar.foreground : Color.foreground, 1.4)
                                font.family: root.bar ? root.bar.fontFamily : Style.font.family
                                font.pixelSize: Style.font.bodySmall
                            }
                        }

                        Text {
                            text: "Library"
                            color: Qt.darker(root.bar ? root.bar.foreground : Color.foreground, 1.4)
                            font.family: root.bar ? root.bar.fontFamily : Style.font.family
                            font.pixelSize: Style.font.caption
                        }

                        Column {
                            width: parent.width
                            spacing: Style.space(4)
                            opacity: 1
                            Dropdown {
                                width: parent.width
                                value: st.theme
                                options: root.getLibraries()
                                foreground: root.bar ? root.bar.foreground : Color.foreground
                                fontFamily: root.bar ? root.bar.fontFamily : Style.font.family
                                enabled: true
                                onChanged: function(v) { root.updateMonitor(mon.name, { theme: Number(v) }) }
                            }
                            Text {
                                visible: !root.status.hyprpaper
                                width: parent.width
                                wrapMode: Text.WordWrap
                                text: "hyprpaper não instalado — usará omarchy global (mesma imagem em todos monitores)"
                                color: "#f1c40f"
                                font.family: root.bar ? root.bar.fontFamily : Style.font.family
                                font.pixelSize: Style.font.caption
                                font.italic: true
                            }
                        }

                        Text {
                            text: "Trocar wallpaper"
                            color: Qt.darker(root.bar ? root.bar.foreground : Color.foreground, 1.4)
                            font.family: root.bar ? root.bar.fontFamily : Style.font.family
                            font.pixelSize: Style.font.caption
                            topPadding: Style.space(4)
                        }

                        Column {
                            width: parent.width
                            spacing: Style.space(2)
                            Row {
                                width: parent.width
                                spacing: Style.space(8)
                                Text { id: t60b; text: "60s"; color: Qt.darker(root.bar ? root.bar.foreground : Color.foreground, 1.4); font.family: root.bar ? root.bar.fontFamily : Style.font.family; font.pixelSize: Style.font.caption }
                                Item { width: parent.width - t60b.width - t300b.width - parent.spacing*2; height: 1 }
                                Text { id: t300b; text: "300s"; color: Qt.darker(root.bar ? root.bar.foreground : Color.foreground, 1.4); font.family: root.bar ? root.bar.fontFamily : Style.font.family; font.pixelSize: Style.font.caption }
                            }
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
                            Text {
                                width: parent.width
                                horizontalAlignment: Text.AlignHCenter
                                text: Math.round(intervalSlider.value) + " segundos"
                                color: root.bar ? root.bar.foreground : Color.foreground
                                font.family: root.bar ? root.bar.fontFamily : Style.font.family
                                font.pixelSize: Style.font.caption
                                font.bold: true
                            }
                        }

                        Row {
                            width: parent.width
                            spacing: Style.space(8)
                            opacity: root.status.mpvpaper ? 1 : 0.45
                            Text {
                                anchors.verticalCenter: parent.verticalCenter
                                text: "Reproduzir vídeos"
                                color: root.bar ? root.bar.foreground : Color.foreground
                                font.family: root.bar ? root.bar.fontFamily : Style.font.family
                                font.pixelSize: Style.font.bodySmall
                                width: parent.width - toggleRect2.width - parent.spacing
                                elide: Text.ElideRight
                            }
                            Rectangle {
                                id: toggleRect2
                                anchors.verticalCenter: parent.verticalCenter
                                width: Style.space(44)
                                height: Style.space(24)
                                radius: height/2
                                color: st.video ? Color.accent : Qt.rgba((root.bar ? root.bar.foreground : Color.foreground).r, (root.bar ? root.bar.foreground : Color.foreground).g, (root.bar ? root.bar.foreground : Color.foreground).b, 0.18)
                                border.width: Style.spacing.hairline
                                border.color: Qt.rgba((root.bar ? root.bar.foreground : Color.foreground).r, (root.bar ? root.bar.foreground : Color.foreground).g, (root.bar ? root.bar.foreground : Color.foreground).b, 0.12)
                                Rectangle {
                                    width: Style.space(18); height: Style.space(18); radius: width/2
                                    anchors.verticalCenter: parent.verticalCenter
                                    x: st.video ? parent.width - width - 3 : 3
                                    color: Color.background
                                    Behavior on x { NumberAnimation { duration: 120; easing.type: Easing.OutCubic } }
                                }
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
                        Text {
                            visible: !root.status.mpvpaper
                            width: parent.width
                            wrapMode: Text.WordWrap
                            text: "mpvpaper não instalado — vídeos desabilitados"
                            color: "#f1c40f"
                            font.family: root.bar ? root.bar.fontFamily : Style.font.family
                            font.pixelSize: Style.font.caption
                            font.italic: true
                        }
                    }
                }
            }

            Text {
                visible: root.displays.length === 0
                width: parent.width
                wrapMode: Text.WordWrap
                horizontalAlignment: Text.AlignHCenter
                text: "Nenhum monitor detectado.\nVerifique hyprctl."
                color: Qt.darker(root.bar ? root.bar.foreground : Color.foreground, 1.6)
                font.family: root.bar ? root.bar.fontFamily : Style.font.family
                font.pixelSize: Style.font.caption
                font.italic: true
            }
        }
    }
    }