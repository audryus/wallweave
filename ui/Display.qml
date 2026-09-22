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
    
    // ----- props injetadas pelo pai -----
    property var backend: null
    property var bar: null
    readonly property color foreground: bar ? bar.foreground : Color.foreground
    readonly property string fontFamily: bar ? bar.fontFamily : Style.font.family
    // visibilidade controlada pelo Menu de Main, não aqui dentro
    property bool active: true

    // ----- estado local -----
    property var displays: []

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
            model: root.monitors
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

                // Próximo wallpaper — tempo fixo até próxima iteração e arquivo
                Column {
                id: nextCol
                visible: st.library !== ""
                width: parent.width
                spacing: Style.space(2)
                // intervalo fixo da iteração atual — só muda após trocar wallpaper
                property int currentInterval: {
                    var v = Number(st.intervalAtLastChange)
                    if (isNaN(v) || v < 5 || v > 300) v = Number(st.interval)
                    return isNaN(v) ? 5 : v
                }
                property int timeLeft: 0
                property string nextFileName: {
                    if (st.library === "") return ""
                    var files = root.getFilteredFilesForMonitor(st.library, st.seed, st.playVideo, root.widgetStatus.videoEnabled)
                    if (files.length === 0) return "—"
                    var idx = Number(st.currentIndex)
                    if (isNaN(idx) || idx < 0 || idx >= files.length) idx = 0
                    var full = String(files[idx] || "")
                    var base = full.split("/").pop()
                    return base || full
                }
                function recalc() {
                    var last = Number(st.lastChange) || 0
                    if (last === 0) { timeLeft = currentInterval; return }
                    var elapsed = Math.floor((Date.now() - last)/1000)
                    var left = currentInterval - elapsed
                    timeLeft = left < 0 ? 0 : left
                }
                onCurrentIntervalChanged: recalc()
                Component.onCompleted: recalc()
                onVisibleChanged: if (visible) recalc()
                // reage a qualquer mudança no st deste monitor (lastChange, interval, currentIndex, seed, library)
                Connections {
                    target: root
                    function onMonitorStateChanged() { nextCol.recalc() }
                }
                // força recalc quando st muda (lastChange etc.)
                property var _stTrigger: st
                on_StTriggerChanged: recalc()
                Timer {
                    interval: 1000
                    repeat: true
                    running: st.library !== ""
                    onTriggered: parent.recalc()
                    Component.onCompleted: parent.recalc()
                }
                Row {
                    width: parent.width
                    spacing: Style.space(6)
                    Text {
                    text: "Próximo em: " + parent.parent.timeLeft + "s"
                    color: Qt.darker(root.bar ? root.bar.foreground : Color.foreground, 1.3)
                    font.family: root.bar ? root.bar.fontFamily : Style.font.family
                    font.pixelSize: Style.font.caption
                    font.bold: true
                    }
                    Text {
                    text: "•"
                    color: Qt.darker(root.bar ? root.bar.foreground : Color.foreground, 1.6)
                    font.family: root.bar ? root.bar.fontFamily : Style.font.family
                    font.pixelSize: Style.font.caption
                    }
                    Text {
                    text: parent.parent.nextFileName
                    color: root.bar ? root.bar.foreground : Color.foreground
                    font.family: root.bar ? root.bar.fontFamily : Style.font.family
                    font.pixelSize: Style.font.caption
                    elide: Text.ElideMiddle
                    width: parent.width - 120
                    }
                }
                }

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
                text: "Biblioteca"
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
                    value: st.library
                    options: root.themeOptions
                    foreground: root.bar ? root.bar.foreground : Color.foreground
                    fontFamily: root.bar ? root.bar.fontFamily : Style.font.family
                    enabled: true
                    onChanged: function(v) { root.updateMonitor(mon.name, { library: String(v) }) }
                }
                Text {
                    visible: !root.widgetStatus.hyprpaper
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
                    Text { id: t60b; text: "5s"; color: Qt.darker(root.bar ? root.bar.foreground : Color.foreground, 1.4); font.family: root.bar ? root.bar.fontFamily : Style.font.family; font.pixelSize: Style.font.caption }
                    Item { width: parent.width - t60b.width - t300b.width - parent.spacing*2; height: 1 }
                    Text { id: t300b; text: "300s"; color: Qt.darker(root.bar ? root.bar.foreground : Color.foreground, 1.4); font.family: root.bar ? root.bar.fontFamily : Style.font.family; font.pixelSize: Style.font.caption }
                }
                Slider {
                    id: intervalSlider
                    width: parent.width
                    from: 5
                    to: 300
                    stepSize: 1
                    snapMode: Slider.SnapOnRelease
                    value: st.interval
                    onMoved: root.updateMonitor(mon.name, { interval: Math.round(value) })
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
                opacity: root.widgetStatus.videoEnabled ? 1 : 0.45
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
                    color: st.playVideo && root.widgetStatus.videoEnabled ? Color.accent : Qt.rgba((root.bar ? root.bar.foreground : Color.foreground).r, (root.bar ? root.bar.foreground : Color.foreground).g, (root.bar ? root.bar.foreground : Color.foreground).b, 0.18)
                    border.width: Style.spacing.hairline
                    border.color: Qt.rgba((root.bar ? root.bar.foreground : Color.foreground).r, (root.bar ? root.bar.foreground : Color.foreground).g, (root.bar ? root.bar.foreground : Color.foreground).b, 0.12)
                    Rectangle {
                    width: Style.space(18); height: Style.space(18); radius: width/2
                    anchors.verticalCenter: parent.verticalCenter
                    x: (st.playVideo && root.widgetStatus.videoEnabled) ? parent.width - width - 3 : 3
                    color: Color.background
                    Behavior on x { NumberAnimation { duration: 120; easing.type: Easing.OutCubic } }
                    }
                    MouseArea {
                    anchors.fill: parent
                    enabled: root.widgetStatus.videoEnabled
                    cursorShape: enabled ? Qt.PointingHandCursor : Qt.ArrowCursor
                    onClicked: {
                        if (!root.widgetStatus.videoEnabled) return
                        root.updateMonitor(mon.name, { playVideo: !st.playVideo })
                    }
                    }
                }
                }
                Text {
                visible: !root.widgetStatus.videoEnabled
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
            visible: root.monitors.length === 0
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