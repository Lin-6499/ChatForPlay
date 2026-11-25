---
name: LinChat
description: 墨色书页 — 克制、清晰、可自定义的深色交流空间
colors:
  accent: "#a8c7b5"
  on-accent: "#14251b"
  canvas: "#111615"
  panel: "#1b2220"
  text: "#edf2ef"
  muted: "#999e9c"
  selection: "#262f2b"
typography:
  display:
    fontFamily: '"Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif'
    fontSize: "46px"
    fontWeight: 550
    lineHeight: 1.45
    letterSpacing: "-0.8px"
  headline:
    fontFamily: '"Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif'
    fontSize: "23px"
    fontWeight: 600
    letterSpacing: "-0.4px"
  title:
    fontFamily: '"Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif'
    fontSize: "16px"
    fontWeight: 600
  body:
    fontFamily: '"Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif'
    fontSize: "14px"
    fontWeight: 400
    lineHeight: 1.85
  label:
    fontFamily: '"Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif'
    fontSize: "12px"
    fontWeight: 400
rounded:
  field: "7px"
  button: "8px"
  item: "9px"
  composer: "10px"
  message: "12px"
  dialog: "14px"
spacing:
  xs: "4px"
  sm: "8px"
  compact: "12px"
  md: "16px"
  lg: "24px"
  xl: "32px"
components:
  button-primary:
    backgroundColor: "{colors.accent}"
    textColor: "{colors.on-accent}"
    rounded: "{rounded.button}"
    padding: "12px 20px"
    height: "44px"
  button-secondary:
    rounded: "{rounded.field}"
    padding: "9px 14px"
    height: "36px"
  text-field:
    textColor: "{colors.text}"
    rounded: "{rounded.field}"
    padding: "10px 12px"
    height: "44px"
  conversation-item:
    rounded: "{rounded.item}"
    padding: "15px 11px"
  status-tag:
    rounded: "4px"
    padding: "3px 6px"
  message:
    textColor: "{colors.text}"
    rounded: "{rounded.message}"
    padding: "11px 16px"
---

# Design System: LinChat

## Overview

**Creative North Star: "墨色书页"**

用深墨色与灰绿层次给对话留出安静的阅读空间。视觉表达来自清晰的排版、稳定的间距和明确的状态，而非装饰性效果。沿用熟悉的通讯界面，让消息、会话与下一步操作容易辨认。

默认深色是用户确认的产品要求；同一套组件也必须适配用户自定义的背景、面板和强调色。现有 LinChat 名称与图标保留。

**Key Characteristics:**
- 深墨画布与略亮面板区分工作区域。
- 强调色用于主操作、选中状态和必要提示。
- 系统字体、细完整边框与克制圆角。
- 配色预览与实际界面共享同一套计算逻辑。

## Colors

Frontmatter records the **default** theme. The normative runtime is `desktop/theme.mjs`; `desktop/theme.css` supplies the boot palette and shared component styling.

### Primary

**柔和灰绿** is the default accent. Primary buttons use the supplied on-accent ink; action labels use a separately contrast-adjusted accent color. The selected-surface tint is derived from the current background and accent.

### Neutral

**深墨画布** holds messages and the primary workspace. **灰绿面板** holds conversation lists, dialogs and connection-center introduction. **浅墨文字** and **次级墨色** establish hierarchy without reducing body legibility.

**The Independent Surface Rule.** Canvas, panel and navigation each derive their own text, muted, border, focus, selected and semantic colors. A light panel on a dark canvas is valid; do not assume that all regions share one foreground.

**The Semantic Color Rule.** Amber identifies private material, mentions and paused/pending states; red identifies errors; the online marker uses a separately adapted success color. User accent changes do not replace these meanings.

Four presets ship: 墨色, 夜蓝, 暖灰 and 黑曜. User colors accept three- or six-digit HEX; saved values are normalized to six digits. Text and state colors are calculated, not user-authored. New components must use the semantic CSS variables, including the appropriate surface scope.

## Typography

One system UI stack supports Chinese copy without external font requests. The desktop connection center uses the same Segoe UI/Microsoft YaHei language. Monospace is limited to editable HEX values and actual code.

- Authentication headline: display role; the connection-center equivalent is 40px, reducing to 34px at its compact desktop breakpoint.
- Workspace heading: headline role; dialog headings are 22px and campaign titles are 20px.
- Conversation titles: title role; message and journal prose use body role.
- Supporting forms and secondary actions use 12–13px. Compact timestamps, presence and badges use 10–11px.
- Messages wrap at up to 620px; campaign prose uses a maximum 74ch measure.
- Counts, times, dice and character numbers use tabular numerals.

**The Reading Priority Rule.** Keep message prose larger and more open than timestamps, receipts and navigation metadata.

## Layout

The chat workspace uses a 72px navigation rail, a 310px conversation column and a flexible reading area. At 1550px the first columns become 82px and 340px; at 1050px they become 64px and 280px. The entertainment index follows the same shell with its own 280px campaign list.

At 760px, navigation becomes a 58px rail plus a list. Opening a conversation or campaign switches to a single content panel with a back control. Preserve this structural change rather than shrinking desktop columns into the phone viewport.

The message/journal region scrolls independently. Composers remain at the bottom of the available height; update and connection banners participate in layout rather than covering the primary action.

Dialogs are bounded to 90dvh (94dvh on mobile). Appearance settings use a scrolling control area and a fixed status/action footer. Forms use tight label/control groups and more space between sections. Desktop authentication and connection-center screens use two columns, collapsing to the form at 760px.

## Elevation & Depth

Tonal surfaces and complete borders provide everyday depth. Shadows are reserved for overlays: dialogs use `0 24px 80px #00000040`, popovers `0 10px 32px #00000040`, and toast feedback `0 8px 30px #00000040`. The dialog backdrop is a flat black scrim. The interface has no decorative blur or glowing edges.

## Shapes

Fields and small controls have restrained corners; conversation rows use the item radius; the composer and dialogs receive slightly larger radii. Message bubbles retain one small leading or trailing corner to clarify authorship. Avatars are compact rounded squares. Selection is expressed by a complete border and tint.

## Components

### Buttons

Primary actions are solid accent with contrast-safe on-accent text. Secondary actions use a raised surface and full border. Text actions remain quiet, with an underline on hover. Disabled actions use opacity and native disabled semantics.

Hover color transitions last 160ms with ease-out; there is no page-entry choreography. Reduced-motion mode disables transitions and minimizes loading animation.

### Inputs / Fields

Fields use a low-contrast surface and a stronger complete boundary. Focus is a 2px contrasting outline with 3px offset, or a visible focus-within border on compound search/composer controls. Carets and text selection use theme roles.

### Navigation

Selected navigation and conversation rows use the derived selection tint, matching border and contrast-safe selected text. Unread counts remain explicit. Hover does not shift layout. All icon-only actions have labels.

### Tags and States

Tags are compact secondary information. Preserve semantic warning/error text in addition to color. Paused campaign states, mentions, private material and errors have separate adapted foreground/background pairs.

### Messages and Containers

Received messages use the raised surface; sent messages use the selected surface. File links wrap long names. Campaign character panels use fine borders and the same surface language. Long prose wraps without forcing viewport overflow.

### Appearance Editor

One shared DOM component serves Vue and Electron. It presents a conversation preview, four presets, native color inputs and HEX fields for accent/background/panel. Preview changes do not affect the app until Apply. Invalid HEX shows inline feedback and disables Apply. Reset loads the default draft, then Apply commits it.

The web saves per browser origin in local storage. Electron also saves a validated device preference through its existing restricted IPC boundary, so the connection center and selected server share the palette. Escape closes web dialogs and restores trigger focus; native desktop dialogs use the platform dialog behavior.

## Do's and Don'ts

### Do:
- **Do** use derived semantic variables for every new surface and state.
- **Do** keep primary actions reachable while message or settings content scrolls.
- **Do** preserve readable Chinese body copy and the current responsive navigation.
- **Do** preview custom colors with the same runtime used by the application.

### Don't:
- **Don't** hard-code a light panel or dark foreground inside a themed component.
- **Don't** use the user accent to replace error, private or connection semantics.
- **Don't** add ornamental gradients, glow, large promotional labels or entrance animations to the workspace.
- **Don't** interpret a dark default as a restriction on user-selected colors.
