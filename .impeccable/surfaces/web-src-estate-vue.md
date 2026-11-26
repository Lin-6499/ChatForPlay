---
version: 1
slug: "web-src-estate-vue"
primary_target: "web/src/Estate.vue"
related_targets: ["web/src/EstateBoard.vue","web/src/EstateToken.vue","web/src/estate.css","web/src/estate-types.ts"]
---

# 地产大亨 · Operate

Confirmed: desktop-first, 2–4 real players, 24 tiles and 12 rounds. Full rule and interaction brief: docs/ESTATE_V1.md.

## Direction contract

THESIS: A shared table keeps the property board, everyone's cash and the current decision visible together; players can chat without losing their turn.

OWN-WORLD: Inherit LinChat's ink canvas, adaptive user themes, system typography and restrained full borders. Property groups use letter and theme-derived colour; user-requested pawns share a circular shape with stable numbers1–4 and independent saturated blue/orange/purple/green fills with white numbers (user rejected the muted inherited pawn colours). Land ownership uses explicit 主1 text, not a second indistinguishable pawn.

STORY: Create or join, choose a pawn and ready, roll and buy, manage assets and debts, then inspect the final asset breakdown.

FIRST VIEWPORT: Player order left, square 7×7 perimeter board centrally, persistent decision above bounded chat at right. On compact windows decision precedes board and chat collapses. The centre is selected-property information, never an unrelated marketing panel.

FORM: seed: none (pinned). User approved desktop-first initial design and requested implementation of that design.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance.

No generated or third-party raster art. Verify isolated authoritative-server scenarios and desktop/mobile screenshots headlessly; never open a visible client.

## Ownership and compact-layout refinement

User approved prominent owner colours, reserved pawn docks and compact operation/board/chat ordering. Tiles now have separate information and fixed-height four-pawn rows, explicit coloured 地主：N号 labels; player-name buttons filter the holdings list. Solid selection and dashed authoritative landing are labelled separately. Compact action remains sticky, chat expands after the board; internal horizontal board scrolling preserves all tile text at small sizes rather than hiding names/rents.
Six real-server edge scenarios pass, including four occupants non-overlap at1440/1280/820/390 and chat expansion preserving board offset. New evidence: `dist/verification/estate/ownership-*.png`. Go tests/build pass; hop regression passes. Local preview only, not deployed.

## Pawn colour correction

User screenshot rejected muted inherited pawn colours. Replaced with independent blue/orange/purple/green fills and white numbers; measured number contrast5.17/5.18/5.70/5.02. Targeted real-server checks assert actual SVG fill values; desktop/mobile captures visibly distinguish blue1 and orange2. Fresh independent review: ship for visible distinction, numeral readability and separate 主N owner labels; green4, active purple and light-theme visuals require the separate four-user run.

## Finish evidence · 2026-11-26

- Local implementation includes authoritative Go rules, persistent deadlines, operation receipts, member-only snapshots, embedded group chat and desktop-first Vue board. No release or production deployment performed.
- Go tests and vet pass. Four actual users complete twelve rounds through the real server; eight end-to-end scenarios cover ready/start, chat, reload, responsive keyboard access, lost-response retry, complete results, custom light appearance and rematch.
- Four additional edge checks verify exact-balance110/220 redemption, real debt payment via demolition/mortgage with all affected group/station rents, and accessible authoritative pawn positions.
- Independent first review: fix (redemption float rounding, accessible occupant locations, debt consequences and metadata order). Server integer quotes and UI disclosures corrected all four. Scoped verdict: **ship**, all four resolved; not an assistive-technology certification or economic-balance claim.
- Current captures and both passing reports: `dist/verification/estate/`; source/server/web checksums recorded in `build-binding.json` and both reports. General UI captures were refreshed after the fixes. Desktop1440/1280, compact820, mobile390, management/debt/redemption/results and light appearance inspected.
- Mechanical findings are limited to deliberately custom light-theme colours in the isolated test fixture. Established DESIGN.md remains the visual authority; no global theme or shipping raster changes.

## Pawn motion refinement · 2026-11-26

- User reported that dice-driven pawn jumps were too fast to perceive. Preserve the established board, palette, numbered pieces and authoritative accessible position labels.
- Normal motion waits for the existing800ms dice reveal, then uses300ms per tile: continuous transform arc, modest14px lift and brief landing compression. Duplicate snapshots do not cancel the journey. A changed authoritative location, room switch or deactivation cancels stale work.
- Reduced motion displays the final authoritative position immediately without a spatial animation. No dependency, decorative board animation or server-rule change.
- `node scripts/estate-motion-check.cjs --capture` validates eight visible steps through a corner under100ms snapshot refreshes, interruption,390px layout and reduced motion. Actual component wall-clock frames and reports: `dist/verification/estate-motion/realtime-normal/` and `realtime-reduced/`. Skill recorder evidence is also retained in `normal/` and `reduced/`, but replay of separately created WAAPI effects cannot establish the timer-driven total journey; use wall-clock frames for trajectory and timing.
- Motion review was performed inline using the skill's degraded reviewer because nested-agent depth prevented an independent reviewer. Scoped component verdict: ship; low-end hardware performance and complete multiplayer motion sequencing were not measured. Web/server builds and real-server estate edge checks pass. Detector only reports pre-existing isolated light-theme fixture colours.
