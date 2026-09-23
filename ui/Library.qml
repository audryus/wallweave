// Library.qml
// "Libraries" panel: an "Add folder" button plus a scrollable list of
// wallpaper folders. Each folder shows a thumbnail carousel, its path, a
// Remove button, and image/video counts. It reuses the SAME backend
// created by Main.qml — no Process of its own. All labels go through I18n.
import QtQuick
import Quickshell.Io
import qs.Commons
import qs.Ui
import "./i18n"

// Root column that stacks the heading, the add button, and the list.
Column {
    id: root
    width: parent.width - Style.space(20)
    spacing: Style.space(10)
    // Visibility is controlled by the Menu of Main (active flag), not here.
    visible: active
    opacity: active ? 1 : 0
    // Ask for the library list as soon as the backend is available
    // (covers both injection orders: backend before or after onCompleted).
    Component.onCompleted: if (backend) Qt.callLater(() => backend.send({cmd: "browse_libraries"}))

    // ----- Properties injected by the parent (Main.qml) -----
    property var backend: null
    property var bar: null
    readonly property color foreground: bar ? bar.foreground : Color.foreground
    readonly property string fontFamily: bar ? bar.fontFamily : Style.font.family
    // Visibility is controlled by the Menu of Main, not inside this file.
    property bool active: true

    // ----- Local state -----
    // The list of libraries currently shown (array of Library objects).
    property var libraries: []

    // removeLibrary deletes a library: it removes it from the local list
    // right away (optimistic update) and tells the backend to delete it
    // (prefers id, falls back to path).
    function removeLibrary(lib) {
        libraries = libraries.filter(l => l.id !== lib.id)
        if (backend) {
            if (lib.id)
                backend.send({cmd: "del_library", id: lib.id})
            else
                backend.send({cmd: "del_library", path: String(lib.path)})
        }
    }

    // Folder picker: opens zenity's directory chooser; when the user picks
    // a folder, the path is sent to Go as an "add_library" command.
    Process {
        id: folderPickerProc
        command: ["zenity", "--file-selection", "--directory", "--title=" + I18n.tr("library.picker_title")]
        stdout: StdioCollector {
            waitForEnd: true
            onStreamFinished: {
                var p = String(text||"").trim()
                if (p.length>0 && backend) backend.send({cmd: "add_library", path: p})
            }
        }
    }

    // Listen to ONLY the typed signals for libraries — not statusReceived.
    Connections {
        target: backend
        // Full list arrived (after browse_libraries).
        function onLibrariesReceived(data) {
            // Go sends {type:"browse_libraries", message:"[...]"} — Backend
            // already ran JSON.parse on the message.
            if (Array.isArray(data)) root.libraries = data
            else if (data && Array.isArray(data.libraries)) root.libraries = data.libraries
            else console.log("libraries unexpected payload", JSON.stringify(data))
        }
        // A library was added — refresh the whole list.
        function onLibraryReceived(data) {
            backend.send({cmd: "browse_libraries"})
        }
        // An error came from Go — log it (code is available for i18n later).
        function onErrorReceived(msg, code) { console.warn("Library backend error:", code || "", msg) }
    }

    // Fade the panel in/out when it becomes active/inactive.
    Behavior on opacity { NumberAnimation { duration: 120 } }

    // Section heading above the list.
    Text {
        text: I18n.tr("library.heading")
        color: Qt.darker(root.foreground, 1.5)
        font.family: root.fontFamily
        font.pixelSize: Style.font.caption
        font.letterSpacing: 1
        font.bold: true
    }

    // "Add folder" button — opens the zenity folder picker.
    Button {
        width: parent.width
        text: I18n.tr("library.add_folder")
        foreground: root.foreground
        onClicked: folderPickerProc.running = true
    }

    // Scrollable area for the list of libraries (stops at the bounds).
    Flickable {
        width: parent.width
        height: Math.min(libCol.implicitHeight, Style.space(380))
        contentWidth: width
        contentHeight: libCol.implicitHeight
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        interactive: contentHeight > height

        // Vertical stack of library cards.
        Column {
        id: libCol
        width: parent.width
        spacing: Style.space(10)

        // One card per library in the list.
        Repeater {
            model: root.libraries
            Rectangle {
            required property var modelData
            width: libCol.width
            height: Style.space(150)
            radius: Style.cornerRadius
            // Slightly different fill when this library is "selected".
            color: modelData.selected ? Style.selectedFillFor(root.bar?root.bar.foreground:Color.foreground, Color.accent) : Qt.rgba((root.bar?root.bar.foreground:Color.foreground).r,(root.bar?root.bar.foreground:Color.foreground).g,(root.bar?root.bar.foreground:Color.foreground).b,0.04)
            border.width: Style.spacing.hairline
            border.color: modelData.selected ? Color.accent : Qt.rgba((root.bar?root.bar.foreground:Color.foreground).r,(root.bar?root.bar.foreground:Color.foreground).g,(root.bar?root.bar.foreground:Color.foreground).b,0.12)

            // Content of one library card: thumbs on top, path+remove in
            // the middle, counts at the bottom.
            Column {
                anchors.fill: parent
                anchors.margins: Style.space(10)
                spacing: Style.space(6)

                // Thumbnail carousel — swipe horizontally to see more thumbs.
                Flickable {
                width: parent.width
                height: Style.space(60)
                contentWidth: previewRow.width
                contentHeight: height
                clip: true
                flickableDirection: Flickable.HorizontalFlick
                boundsBehavior: Flickable.StopAtBounds

                // Horizontal row of thumbnail tiles.
                Row {
                    id: previewRow
                    spacing: Style.space(6)
                    height: parent.height

                    // One tile per thumbnail (placeholder when there is none).
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

                        // The actual thumbnail image (loaded from the file path).
                        Image {
                        anchors.fill: parent
                        source: modelData && String(modelData).length > 0 ? "file://" + String(modelData) : ""
                        fillMode: Image.PreserveAspectCrop
                        asynchronous: true
                        visible: String(modelData).length > 0
                        }
                        // Placeholder text when there is no thumbnail yet.
                        Text {
                        anchors.centerIn: parent
                        visible: !modelData || String(modelData).length === 0
                        text: I18n.tr("library.preview")
                        color: Qt.darker(root.bar?root.bar.foreground:Color.foreground,1.6)
                        font.family: root.bar?root.bar.fontFamily:Style.font.family
                        font.pixelSize: Style.font.caption
                        font.italic: true
                        }
                    }
                    }
                }
                }

                // Middle row: the folder path + the Remove button.
                Row {
                width: parent.width
                spacing: Style.space(6)
                // The library folder path (elided if too long).
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
                // Remove button (red) — deletes this library.
                Button {
                    id: removeBtn
                    text: I18n.tr("library.remove")
                    foreground: Color.urgent
                    fontSize: Style.font.caption
                    horizontalPadding: Style.space(8)
                    verticalPadding: Style.space(4)
                    onClicked: root.removeLibrary(modelData)
                }
                }

                // Bottom row: image count and (optional) video count.
                Row {
                width: parent.width
                spacing: Style.space(8)
                // Number of images in this library.
                Text {
                    text: I18n.trCount("library.images", modelData.count.images)
                    color: Qt.darker(root.bar?root.bar.foreground:Color.foreground,1.4)
                    font.family: root.bar?root.bar.fontFamily:Style.font.family
                    font.pixelSize: Style.font.caption
                }
                // Video count — only shown when the library has videos.
                Text {
                    visible: modelData.count.videos > 0
                    text: "• " + I18n.trCount("library.videos", modelData.count.videos)
                    color: Qt.darker(root.bar?root.bar.foreground:Color.foreground,1.4)
                    font.family: root.bar?root.bar.fontFamily:Style.font.family
                    font.pixelSize: Style.font.caption
                }
                }
            }
            }
        }

        // Shown when there are no libraries yet.
        Text {
            visible: root.libraries.length === 0
            width: parent.width
            wrapMode: Text.WordWrap
            horizontalAlignment: Text.AlignHCenter
            text: I18n.tr("library.empty")
            color: Qt.darker(root.foreground, 1.6)
            font.family: root.fontFamily
            font.pixelSize: Style.font.caption
            font.italic: true
            topPadding: Style.space(12)
        }
        }
    }
}
