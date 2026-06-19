// Theme.qml - global color singleton, maps v2 HTML :root CSS vars.
// Source of truth: docs/ui-spec-v2.md section 1.
// NOTE: category/event Chinese display names are NOT here - they go through i18n .ts files.
//       This file only holds colors/sizes (language-neutral).
// Singleton type declared in CMakeLists (QT_QML_SINGLETON_TYPE TRUE).

import QtQuick

QtObject {
    // ---- base palette (matches :root) ----
    readonly property color bg:        "#18181B"   // app main bg (near-black)
    readonly property color bg2:       "#232327"   // sidebar / between-cards bg
    readonly property color bg3:       "#2C2C31"   // card bg
    readonly property color rule:      "#3F3F46"   // border / divider

    // ---- text layer ----
    readonly property color ink:       "#F4F4F5"   // primary text (bright)
    readonly property color muted:     "#9CA3AF"   // secondary text

    // ---- accent ----
    readonly property color accent:    "#10B981"   // green (UI action color: selected/primary/ready)
    readonly property color accentDim: "#059669"
    readonly property color danger:    "#EF4444"   // red (delete/error)

    // accent translucent bg (used by active/hover states)
    readonly property color accentBg:  "rgba(16,185,129,0.10)"
    readonly property color accentBg2: "rgba(16,185,129,0.15)"

    // ---- fixed category colors (timeline/stats/cards unified) ----
    // NOTE: chat category color equals accent value, but roles differ (category vs UI action)
    readonly property var categoryColor: ({
        "coding":  "#3B82F6",
        "office":  "#8B5CF6",
        "browser": "#F59E0B",
        "chat":    "#10B981",
        "other":   "#6B7280",
        "idle":    "#9CA3AF"
    })

    // event type colors (timeline markers + event list)
    readonly property var eventTypeColor: ({
        "voice":          "#10B981",
        "prompt_inject":  "#F59E0B",
        "screenshot":     "#3B82F6",
        "vlm_summary":    "#8B5CF6"
    })

    // ---- sizes / radius (ui-spec section 9) ----
    readonly property int sidebarWidth: 180
    readonly property int contentPadding: 20
    readonly property int cardPadding: 16
    readonly property int cardSpacing: 16
    readonly property int radiusCard: 10
    readonly property int radiusInput: 6
    readonly property int radiusBubble: 12

    // ---- font sizes ----
    readonly property int fontStat: 28      // stat big number
    readonly property int fontTitle: 20     // page title
    readonly property int fontCardTitle: 15
    readonly property int fontBody: 14
    readonly property int fontSmall: 13
    readonly property int fontTiny: 12

    // safe category color lookup (unknown -> other color)
    function colorOf(category) {
        return categoryColor[category] || "#6B7280"
    }
}
