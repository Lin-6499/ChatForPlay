---
version: 1
slug: "web-src-entertainment-vue"
primary_target: "web/src/Entertainment.vue"
related_targets: ["web/src/entertainment.css","web/src/game-types.ts"]
---

# Entertainment mode — existing LinChat surface

Mode: Operate. Extend the shared LinChat Electron/Vue interface for friends running text campaigns. Confirmed: explicit player actions or @主持人 trigger AI; ordinary chat stays conversational; support built-in Grok/OpenAI-compatible APIs and campaign-scoped external-agent tokens. Visual authority is now `DESIGN.md` and the user-approved appearance redesign in `web-src-app-vue.md`.

## Direction contract

THESIS: A shared campaign desk makes current fiction, player choice and durable outcomes legible. The primary action is submitting a character's action.

OWN-WORLD: Inherit LinChat's system typography, ink canvas, gray-green panels, contrast-safe user palette, full borders and restrained rounding. Amber identifies private material or pending decisions, not decoration. The shared theme runtime controls all surface and state colors.

STORY: Choose a campaign, see its scene and turn state, act or roll, then read the resulting journal. Characters, secrets, proposals and connection settings are explicit secondary tabs.

FIRST VIEWPORT: Campaign list at left, room header and state at top, readable journal centrally, composer anchored below. A compact scene strip remains discoverable. On narrow screens use one panel with a back button; dialogs and banners occupy bounded space.

FORM: Established application extension, `seed: none (pinned)` — “融入到当前LinChat做完娱乐模式”. Clear empty states guide creation and API configuration. No model credential is sent back by read endpoints.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance.

Scope: text rooms, role cards/resources, audited server rolls, state/versioned history, owner-approved story changes, scoped private notes, summaries, persistent serialized turns and agent integration. The initial feature extension retained its incumbent system; the subsequent user-approved appearance redesign replaces that visual world while retaining this task structure.

## Finish evidence · 1.2.0

- Rendered review: 1264×811, 390×780, and 800×654; additional hit testing at 768px and 320px. The journal scrolls independently and the complete composer remains within the viewport.
- Initial review disposition: `fix`. Addressed draft continuity on chat round trips and visible mobile campaign-load recovery. Draft state also retains public/private mode and recipient when selecting a campaign again.
- Post-fix verdict: `ship`, scoped to those fixes and regressions introduced by the fix batch. Secondary screens received source review and functional verification, not a complete screenshot review.
- Functional evidence: 13 entertainment scenarios and 11 incumbent Electron scenarios passed. Model and external-agent integration used an isolated local tool-calling provider; paid model accounts were not exercised.
- Mechanical detector: no findings. Generic blank-composer action-click checks report no effect because Submit is intentionally disabled; the dedicated end-to-end flow fills text, clicks Submit, and verifies the persisted action.
- Local evidence: `.impeccable/review/entertainment-review.md`, `entertainment-verdict.md`, `capture-results.json`, screenshots; `dist/verification/entertainment-check-results.json` and `desktop-smoke-results.json`. Generated evidence is excluded from Git.
