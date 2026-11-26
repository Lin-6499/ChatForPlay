---
version: 1
slug: "web-src-app-vue"
primary_target: "web/src/App.vue"
related_targets: ["web/src/style.css", "web/src/entertainment.css", "desktop/index.html", "desktop/launcher.css", "desktop/theme.mjs", "desktop/theme.css", "desktop/appearance.mjs"]
---

# LinChat appearance

Mode: Operate. User confirmed full palette customization and selected 墨色书页 from the visual decision page, code-led.

## Direction contract

THESIS: Conversations have the reading priority of a well-set page. Keep the familiar rail, conversation list, message canvas, and anchored composer; remove promotional ornaments from the working shell.

OWN-WORLD: Matte ink #111615, gray-green panels #1b2220, sage #a8c7b5 for primary actions, light ink #ecf0ed, system UI typography, fine full borders and restrained corners. The user's dark preference fits friends chatting and running campaigns over long evening sessions. Color strategy: Restrained.

STORY: Choose a conversation, read and send; enter entertainment through its existing navigation. Open Appearance from settings or the palette control, preview a real conversation fragment, edit accent/background/panel, apply, and retain the choice on reopening.

FIRST VIEWPORT: 72px navigation, 310px conversation list, flexible message area. Compact header above readable messages; composer fixed at the bottom. Mobile keeps the existing list-to-conversation flow. Signature: a live conversation preview makes custom colors understandable before applying.

FORM: Candidate 6, 墨色书页, seed:33374462513d47e5. Retain consistent timestamps, primary reading space and semantic states from the challenger disciplines. Chosen sketch: `.impeccable/mocks/decision/bound.html`.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

Scope: shared theme runtime and palette controls; chat, auth, entertainment, desktop connection center; responsive and keyboard verification. Existing logo assets retained.

## Finish evidence

- Verdict: `ship`, scoped to the rendered surfaces and interaction evidence in `.impeccable/review/appearance-finish-review.md`. Nested subagent creation was blocked by the harness depth limit, so the shipped degraded reviewer and documenter procedures ran in-thread.
- Captures: chat, authentication and appearance at 1264×811 / 390×780; entertainment also at 800×654 / 1858×903; desktop launcher and editor through the real Electron smoke test. Apply remains hit-testable at 320×640.
- Functional checks: 13 desktop scenarios, 13 entertainment scenarios, 7 existing feature tests, and 2 theme tests covering malformed values and 104 contrasting palette samples. Web typecheck/Vite and both Go server builds passed.
- First-viewport checks confirm the composer lies inside the viewport. Its generic click-only check reports the intentional empty-draft disabled state; typed message/action tests verify submission.
- Mechanical detector ran once; its one desktop small-copy warning was resolved. No new visual raster was introduced; retained logo and tray PNGs carry source provenance, preserved by the icon build.
- Documentation: token-bearing `DESIGN.md` and `.impeccable/design.json`.

## Reading refinement — 2026-11-26

- Preserve the established Operate shell and system font stack. Chat prose and composer use `--reading-font-size: 1.143rem` (16px at the existing 14px UI root) and 1.85 leading. Sender labels use 13px semibold; timestamps, receipts and header metadata use 12px.
- Chat gutters grow from 20px to 40px, then center a 960px reading region on wide canvases. Individual message content remains capped at 640px; mobile retains the existing single-panel navigation and safe-area-aware composer.
- Conversation names truncate within their flex track, retaining their complete text and a native title. Preview text uses 13px with a tighter 4px relationship to its title.
- Verification: `npm run build:web` passed; all 11 `test:reading-position` scenarios passed using an isolated in-memory server and headless browser. Desktop (1264×811), mobile (390×780), and empty-state captures are in `dist/verification/reading-position/`. Desktop and mobile captures were visually reviewed; mobile overflow and reading-position restoration passed. Mechanical changed-file scan returned no findings.
