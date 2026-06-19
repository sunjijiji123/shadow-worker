// PlaceholderLabel.qml - muted hint text used by M2 tabs; M3 replaces with real content.

import QtQuick
import ShadowWorker

Text {
    color: Theme.muted
    font.pixelSize: Theme.fontSmall
    text: ""
    wrapMode: Text.WordWrap
}
