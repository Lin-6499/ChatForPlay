---
version: 1
slug: "web-src-entertainment-vue"
primary_target: "web/src/Entertainment.vue"
related_targets: ["web/src/entertainment.css","web/src/game-types.ts","web/src/DiceRoll.vue","web/src/DiceDie.vue","web/src/dice.css","web/src/dice-geometry.ts"]
---

# Entertainment mode — existing LinChat surface

## Progress and memory organization · 1.2.11

Operate extension, same visual identity: chapter focus is an inline explicit action; progress is a field independent of archive. Pending memory suggestions sit behind a counted disclosure with authoritative source text, never mixed into saved facts. A collapsed read-only recap joins the journal, bounded to42vh so the composer remains reachable on mobile.

Four new headless real-server scenarios plus eight incumbent lore, six realtime and thirteen entertainment scenarios pass. Backend tests verify chapter focus isolation, progress, recap filtering and atomic owner-confirmed suggestions with duplicate retry protection. Rendered review at1264/390/320 shows readable sources and reachable confirmation controls; the320px recap retains journal and composer. Mechanical detector: no findings. Verdict: ship, limited to phase1; phase2/3 are not claimed delivered.

## Script and durable memory extension · direction

- Operate mode, existing visual identity unchanged. The user selected TXT/Markdown and pasted text, not PDF/Word.
- Extend the campaign desk with a “剧本与记忆” tab. Searchable category list joins a plain-text reading/editor area; mobile stacks list then detail without adding modals.
- Entries cover chapters, NPCs, quests, clues and confirmed facts. Owner-controlled public/secret scope is explicit before save. Only selected active public records enter bounded model context; no automatic private-model injection or AI inference becoming canon.
- New imports are preview-only until saved. Version history, archive/restore and conflict-preserved drafts provide durable control without destructive deletion.
- Inherit canvas-local colors, warning tokens, system type, full borders and restrained spacing. Keep reading comfortable and actions reachable in the existing scroll region.
- Finish with isolated real-server permission/context tests and headless rendered review at desktop/mobile widths.

### Finish evidence

Eight isolated UI scenarios pass: import preview, context selection, unsaved edit retention, revision restore, secret isolation, conflict preservation, file limit rejection, archive/restore and responsive/theme coverage. Backend tests verify token permissions, secret-history filtering, context budgets and running-turn protection.
Rendered review initially found unthemed list filters; fixed using existing field/focus/selection tokens. Refreshed dark captures at1264/768/390/320 and light1000 show usable editors and reachable save controls; player reader390 contains no secret labels or editing actions. Verdict: ship, scoped to this feature. Mechanical check only notes the intentional custom palette in the test fixture. No new visual system or raster assets introduced.

## Host streaming · 2026-11-25

- Extend the established journal, with one temporary host entry marked “正在生成 · 尚未入档”; use inherited surface warning/muted tokens, a full border, 16px padding and 9px radius. No typewriter effect or token announcements.
- Preserve the existing journal anchor/follow behavior. Final narration replaces the temporary entry; errors and expired leases cannot masquerade as saved outcomes.
- Verified with isolated, deliberately held-open SSE providers for builtin and external hosts: preview before completion, reload recovery, exactly one final entry. Six rendered captures cover both hosts at 1264, 390 and 320px.
- In-thread visual review initially found unpadded draft borders; corrected padding and canvas-local semantic colors, rebuilt and recaptured. Final narrow-screen review: ship. Composer and content remain reachable; detector reports no findings.

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

## Dice motion · 2026-11-25

- Focal motion: one bounded 820ms lift/tumble/contact sequence, followed by the actual server receipt; the rest of the campaign desk stays still.
- Continuity: prepared dice become the recorded dice in the same tray; kept/discarded labels explain advantage and disadvantage. No outcome is shown before the server responds.
- Feedback: stable pending and failure states, idempotent retry, explicit next-roll action, keyboard dismissal and restored focus.
- Budget: six projected solids maximum, no new dependency, no continuous animation while waiting or hidden. Reduced motion uses a 100ms opacity-only result reveal.
- Evidence: `.impeccable/review/dice/{normal,reduced}/` contains the skill's CSS-timeline captures; `{normal-live,reduced-live}/` supplements them with real-time frame series and capture limitations. Layout captures cover 1264×811, 390×780 and 320×568. Functional tests cover eight dice scenarios and thirteen incumbent entertainment scenarios, all headless.
- Motion review: initial `fix` identified the waiting label's missing semantic amber; resolved using the panel-scoped warning token, measured at 8.07:1 dark / 5.14:1 custom light. Post-fix verdict: `ship`, scoped to the refreshed 1000×760 real-time frame sequences and both waiting-palette states. This is not a claim of cross-device frame-rate certification. Review record: `.impeccable/review/dice/verdict.md`.

## Dice shape correction · 2026-11-25

- User requested corrected shapes with a front-facing or overhead view. The chosen resting view is face-on, with zero residual pitch/yaw and uniform orthographic scaling.
- D20 retains an equilateral front triangle and a symmetric silhouette. D10/D100 have an upright kite with the label at its incenter. Small-face typography and long labels fit within the actual face.
- Eight headless dice scenarios passed again. Geometry checks cover regular face edges, reflection symmetry, eight shape mappings, three die indices and 101 animation positions; dark/light component specimens verify ready and maximum-value ink bounds.
- Rendered evidence: `.impeccable/review/dice-shape/`, including 1264×811, 768×900, 390×780 and 320×568 layouts, normal/reduced-motion sequences and the in-thread review. Production build and mechanical detector passed.
