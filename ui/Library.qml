import QtQuick
import Quickshell.Io
import qs.Commons
import qs.Ui

// Bibliotecas — botão adicionar pasta + cards com carrossel e remover
// Reusa o MESMO backend de Main.qml — não cria Process próprio
Column {
    id: root
    width: parent.width - Style.space(20)
    spacing: Style.space(10)
    visible: active
    opacity: active ? 1 : 0
    // chama assim que backend for injetado e no onCompleted (cobre ambas ordens)
    onBackendChanged: if (backend) Qt.callLater(() => backend.send({cmd: "libraries"}))
    Component.onCompleted: if (backend) Qt.callLater(() => backend.send({cmd: "libraries"}))

    // ----- props injetadas pelo pai -----
    property var backend: null
    property var bar: null
    readonly property color foreground: bar ? bar.foreground : Color.foreground
    readonly property string fontFamily: bar ? bar.fontFamily : Style.font.family
    // visibilidade controlada pelo Menu de Main, não aqui dentro
    property bool active: true

    // ----- estado local -----
    property var libraries: []

    function removeLibrary(id) {
        // otimista local + avisa backend
        libraries = libraries.filter(l => String(l.path) !== String(id))
        if (backend) backend.send({cmd: "library_remove", path: String(id)})
    }

    

    Process { id: folderPickerProc; command: ["zenity", "--file-selection", "--directory", "--title=Escolher pasta de wallpapers"]; stdout: StdioCollector { waitForEnd: true; onStreamFinished: { var p = String(text||"").trim(); if (p.length>0 && backend) backend.send({cmd: "library", path: p}) } } }

    // escuta SÓ o sinal tipado — não statusReceived
    Connections {
        target: backend
        function onLibrariesReceived(data) {
            // Go envia {type:"libraries", message:"[...]"} -> Backend já fez JSON.parse
            if (Array.isArray(data)) root.libraries = data
            else if (data && Array.isArray(data.libraries)) root.libraries = data.libraries
            else console.log("libraries payload inesperado", JSON.stringify(data))
        }
        function onLibraryReceived(data) {
            // resposta a library add/remove — atualiza lista
            if (backend) backend.send({cmd: "libraries"})
        }
        function onErrorReceived(msg) { console.warn("Library erro backend:", msg) }
    }

    Behavior on opacity { NumberAnimation { duration: 120 } }

    Text {
        text: "LIBRARIES"
        color: Qt.darker(root.foreground, 1.5)
        font.family: root.fontFamily
        font.pixelSize: Style.font.caption
        font.letterSpacing: 1
        font.bold: true
    }

    Button {
        width: parent.width
        text: "＋ Add folder"
        foreground: root.foreground
        onClicked: folderPickerProc.running = true
    }

    Flickable {
        width: parent.width
        height: Math.min(libCol.implicitHeight, Style.space(380))
        contentWidth: width
        contentHeight: libCol.implicitHeight
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        interactive: contentHeight > height

        Column {
        id: libCol
        width: parent.width
        spacing: Style.space(10)

        Repeater {
            model: root.libraries
            Rectangle {
            required property var modelData
            width: libCol.width
            height: Style.space(150)
            radius: Style.cornerRadius
            color: modelData.selected ? Style.selectedFillFor(root.bar?root.bar.foreground:Color.foreground, Color.accent) : Qt.rgba((root.bar?root.bar.foreground:Color.foreground).r,(root.bar?root.bar.foreground:Color.foreground).g,(root.bar?root.bar.foreground:Color.foreground).b,0.04)
            border.width: Style.spacing.hairline
            border.color: modelData.selected ? Color.accent : Qt.rgba((root.bar?root.bar.foreground:Color.foreground).r,(root.bar?root.bar.foreground:Color.foreground).g,(root.bar?root.bar.foreground:Color.foreground).b,0.12)

            Column {
                anchors.fill: parent
                anchors.margins: Style.space(10)
                spacing: Style.space(6)

                // carrossel preview — swipe horizontal
                Flickable {
                width: parent.width
                height: Style.space(60)
                contentWidth: previewRow.width
                contentHeight: height
                clip: true
                flickableDirection: Flickable.HorizontalFlick
                boundsBehavior: Flickable.StopAtBounds

                Row {
                    id: previewRow
                    spacing: Style.space(6)
                    height: parent.height

                    Repeater {
                    model: (modelData.thumbs && modelData.thumbs.length > 0) ? modelData.thumbs : [""]
                    Rectangle {
                        required property var modelData
                        required property int index
                        width: Style.space(80)
                        height: Style.space(60)
                        radius: Style.cornerRadius
                        color: Qt.rgba((root.bar?root.bar.foreground:Color.foreground).r,(root.bar?root.bar.foreground:Color.foreground).g,(root.bar?root.bar.foreground:Color.foreground).b,0.06)
                        border.width: Style.spacing.hairline
                        border.color: Qt.rgba((root.bar?root.bar.foreground:Color.foreground).r,(root.bar?root.bar.foreground:Color.foreground).g,(root.bar?root.bar.foreground:Color.foreground).b,0.08)
                        clip: true

                        Image {
                        anchors.fill: parent
                        source: modelData && String(modelData).length > 0 ? "file://" + String(modelData) : ""
                        fillMode: Image.PreserveAspectCrop
                        asynchronous: true
                        visible: String(modelData).length > 0
                        }
                        Text {
                        anchors.centerIn: parent
                        visible: !modelData || String(modelData).length === 0
                        text: "preview"
                        color: Qt.darker(root.bar?root.bar.foreground:Color.foreground,1.6)
                        font.family: root.bar?root.bar.fontFamily:Style.font.family
                        font.pixelSize: Style.font.caption
                        font.italic: true
                        }
                    }
                    }
                }
                }

                Row {
                width: parent.width
                spacing: Style.space(6)
                Text {
                    text: modelData.path
                    color: root.bar?root.bar.foreground:Color.foreground
                    font.family: root.bar?root.bar.fontFamily:Style.font.family
                    font.pixelSize: Style.font.bodySmall
                    font.bold: true
                    elide: Text.ElideRight
                    width: parent.width - removeBtn.width - Style.space(8)
                    anchors.verticalCenter: parent.verticalCenter
                }
                Button {
                    id: removeBtn
                    text: "Remover"
                    foreground: Color.urgent
                    fontSize: Style.font.caption
                    horizontalPadding: Style.space(8)
                    verticalPadding: Style.space(4)
                    onClicked: root.removeLibrary(modelData.path)
                }
                }

                Row {
                width: parent.width
                spacing: Style.space(8)
                Text {
                    text: modelData.count.images + " images"
                    color: Qt.darker(root.bar?root.bar.foreground:Color.foreground,1.4)
                    font.family: root.bar?root.bar.fontFamily:Style.font.family
                    font.pixelSize: Style.font.caption
                }
                Text {
                    visible: modelData.count.videos > 0
                    text: "• " + modelData.count.videos + " videos"
                    color: Qt.darker(root.bar?root.bar.foreground:Color.foreground,1.4)
                    font.family: root.bar?root.bar.fontFamily:Style.font.family
                    font.pixelSize: Style.font.caption
                }
                }
            }
            }
        }

        Text {
            visible: root.libraries.length === 0
            width: parent.width
            wrapMode: Text.WordWrap
            horizontalAlignment: Text.AlignHCenter
            text: "No libraries.\nClick Add folder."
            color: Qt.darker(root.foreground, 1.6)
            font.family: root.fontFamily
            font.pixelSize: Style.font.caption
            font.italic: true
            topPadding: Style.space(12)
        }
        }
    }
}