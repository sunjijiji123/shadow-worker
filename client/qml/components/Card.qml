// Card.qml - matches HTML .card.
// Usage: Card { title: "..."; description: "..."; <children> }

import QtQuick
import QtQuick.Layouts
import ShadowWorker

Rectangle {
    id: root

    property string title: ""
    property string description: ""
    property bool showHeader: title !== "" || description !== ""

    // optional top-right extra (e.g. a "Manage" button)
    property alias headerExtra: headerExtraRow.children

    // default body children
    default property alias body: bodyColumn.children

    color: Theme.bg3
    border.color: Theme.rule
    border.width: 1
    radius: Theme.radiusCard
    implicitHeight: contentLayout.implicitHeight + Theme.cardPadding * 2

    ColumnLayout {
        id: contentLayout
        anchors.fill: parent
        anchors.margins: Theme.cardPadding
        spacing: 8

        RowLayout {
            Layout.fillWidth: true
            visible: root.showHeader
            spacing: 12

            ColumnLayout {
                Layout.fillWidth: true
                spacing: 2
                Text {
                    visible: root.title !== ""
                    text: root.title
                    color: Theme.ink
                    font.pixelSize: Theme.fontCardTitle
                    font.weight: Font.DemiBold
                }
                Text {
                    visible: root.description !== ""
                    text: root.description
                    color: Theme.muted
                    font.pixelSize: Theme.fontSmall
                    wrapMode: Text.WordWrap
                    Layout.fillWidth: true
                }
            }

            Row {
                id: headerExtraRow
                spacing: 8
            }
        }

        ColumnLayout {
            id: bodyColumn
            Layout.fillWidth: true
            Layout.fillHeight: true
            spacing: 12
        }
    }
}
