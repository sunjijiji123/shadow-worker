// TextArea.qml - labeled multiline text area (HTML .field + .label + .textarea).
// Mirrors TextField styling: bg fill, rule border, 6px radius, 13px text.
// .textarea CSS: min-height 80px, resize: vertical.
//
// A visible drag handle lets the user resize vertically, matching the
// browser's `resize: vertical` affordance.
//   resizeEdge: "bottom" (default) -> handle on bottom-right, drag DOWN to grow
//               "top"             -> handle on top-right,    drag UP to grow
// The "top" mode is for contexts where growing downward would overlap content
// below (e.g. the result bubble sitting above the recording pill).

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import ShadowWorker

ColumnLayout {
    id: root

    property string label: ""
    property string text: ""
    property string placeholder: ""
    property bool readOnly: false
    // .textarea CSS min-height = 80px (content area, excl. label)
    property int minHeight: 80
    // upper bound the user can drag to
    property int maxHeight: 480
    // frame fill color. Default Theme.bg (.textarea); .prompt-textarea uses bg2.
    property color frameColor: Theme.bg
    // which edge the resize handle sits on: "bottom" | "top"
    property string resizeEdge: "bottom"

    signal textEdited(string newText)

    spacing: 6
    Layout.fillWidth: true

    Text {
        visible: root.label !== ""
        text: root.label
        color: Theme.muted
        font.pixelSize: 12
    }

    // user-controlled height (0 = follow content until maxHeight). Once the
    // user drags the handle, this is pinned and the area no longer auto-grows.
    property int userHeight: 0

    Rectangle {
        id: frame
        Layout.fillWidth: true
        // effective height = user-pinned height, else content-follow with cap.
        Layout.preferredHeight: {
            if (root.userHeight > 0) return root.userHeight
            var content = edit.implicitHeight + 16
            return content < root.minHeight ? root.minHeight
                                            : Math.min(content, root.maxHeight)
        }
        Layout.minimumHeight: root.minHeight
        Layout.maximumHeight: root.maxHeight
        color: root.frameColor
        border.color: edit.activeFocus ? Theme.accent : Theme.rule
        border.width: 1
        radius: Theme.radiusInput
        clip: true

        // QtQuick.Controls TextArea inside a Flickable so long content scrolls.
        Flickable {
            id: flick
            anchors.fill: parent
            anchors.margins: 1   // keep border visible
            anchors.bottomMargin: 1
            contentWidth: width
            contentHeight: edit.implicitHeight + 16
            flickableDirection: Flickable.VerticalFlick
            clip: true
            boundsBehavior: Flickable.StopAtBounds
            ScrollBar.vertical: ScrollBar { policy: ScrollBar.AsNeeded }

            TextArea {
                id: edit
                width: flick.width
                // padding matches .textarea CSS (8px 10px)
                topPadding: 8
                bottomPadding: 8
                leftPadding: 10
                rightPadding: 10
                text: root.text
                placeholderText: root.placeholder
                readOnly: root.readOnly
                color: Theme.ink
                placeholderTextColor: Theme.muted
                font.pixelSize: 13
                wrapMode: TextArea.Wrap
                selectByMouse: true
                // flat look: no background (the parent Rectangle is the bg)
                background: null
                onTextChanged: root.textEdited(text)
            }
        }

        // ---- vertical resize handle ----
        // bottom-edge: anchors bottom-right, drag DOWN grows (delta positive)
        // top-edge:    anchors top-right,    drag UP grows (delta negative -> next = startH - delta)
        Rectangle {
            id: handle
            anchors.right: parent.right
            anchors.top: root.resizeEdge === "top" ? parent.top : undefined
            anchors.bottom: root.resizeEdge === "bottom" ? parent.bottom : undefined
            anchors.topMargin: 0
            width: 16
            height: 16
            color: "transparent"

            // diagonal "grip" lines. For the top edge, mirror vertically so the
            // glyph reads as "grab here to pull up".
            Canvas {
                id: grip
                anchors.centerIn: parent
                width: 8; height: 8
                rotation: root.resizeEdge === "top" ? 180 : 0
                onPaint: {
                    var ctx = getContext("2d")
                    ctx.reset()
                    ctx.strokeStyle = handleMa.containsMouse ? Theme.accent : Theme.muted
                    ctx.lineWidth = 1
                    // two diagonal lines: bottom-left to top-right
                    ctx.beginPath(); ctx.moveTo(7, 1); ctx.lineTo(1, 7); ctx.stroke()
                    ctx.beginPath(); ctx.moveTo(7, 4); ctx.lineTo(4, 7); ctx.stroke()
                }
            }

            MouseArea {
                id: handleMa
                anchors.fill: parent
                cursorShape: Qt.SizeVerCursor
                hoverEnabled: true
                // Critical: once pressed, keep ALL subsequent mouse events routed
                // to this handle (even outside its 16x16 box) so the parent
                // Flickable can't steal the drag and scroll the page instead.
                preventStealing: true

                property real startY: 0
                property int startH: 0
                onPressed: function(mouse) {
                    // IMPORTANT: use SCREEN coordinates (mapToGlobal), not local.
                    // In a floating window the window itself moves during the
                    // resize (the result window slides up as the textarea grows),
                    // so a local/mapToItem reference frame drifts and produces
                    // inverted/jumpy drag direction. Screen coords are stable.
                    var g = handleMa.mapToGlobal(mouse.x, mouse.y)
                    startY = g.y
                    startH = frame.height
                    handleMa.grabMouse()
                }
                onPositionChanged: function(mouse) {
                    // hoverEnabled makes this fire on mere hover movement too;
                    // only resize while the mouse button is actually held down.
                    if (!handleMa.pressed) return
                    var g = handleMa.mapToGlobal(mouse.x, mouse.y)
                    var delta = g.y - startY
                    // top edge: dragging the handle DOWN (delta+) SHRINKS the box;
                    // dragging UP (delta-) GROWS it.
                    var next = root.resizeEdge === "top"
                               ? Math.round(startH - delta)
                               : Math.round(startH + delta)
                    // clamp to [minHeight, maxHeight]
                    root.userHeight = Math.max(root.minHeight,
                                               Math.min(next, root.maxHeight))
                }
                onReleased: handleMa.ungrabMouse()
                onContainsMouseChanged: grip.requestPaint()
            }
        }
    }
}
