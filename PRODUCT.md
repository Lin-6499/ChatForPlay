# LinChat

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

LinChat serves people chatting with friends in a shared cloud or Radmin space, and players running text campaigns together. These workflows are documented in `README.md`; no additional audience claims are assumed.

## Product Purpose

One Vue interface provides real-time chat in browsers and an Electron desktop client. Users connect to a server, sign in or join by invitation, and participate in public, direct, group, or campaign conversations.

## Operating Context

- Existing stack: Vue 3, TypeScript, Vite, Electron, and a Go server.
- Desktop connection center supports saved servers and starting a local/Radmin host.
- Each separately deployed server owns its accounts, messages, files, and campaigns.
- Mobile web uses a single-panel navigation flow.

## Capabilities and Constraints

- Preserve the existing authentication, messaging, attachments, mentions, unread states, notifications, server management, and entertainment workflows.
- Campaign actions, audited rolls, characters, host controls, private material, and external-agent integration remain functional.
- User-confirmed scope: improve chat, entertainment, and the desktop connection center.
- User-confirmed appearance requirement: premium, minimal, dark by default, with presets and custom accent, background, and panel colors saved locally.
- Appearance preferences are device-local; cross-device account synchronization is not a requested capability.

## Brand Commitments

Keep the LinChat name, current logo assets, and Chinese interface. The user explicitly requested a restrained dark visual system and complete user-defined colors.

## Evidence on Hand

`README.md`, `docs/ENTERTAINMENT.md`, existing Vue/Electron code, desktop smoke checks, entertainment fixtures, and prior rendered captures under `.impeccable/review/`.

## Product Principles

- Keep conversations and the current action legible.
- Preserve working, familiar navigation across desktop and mobile.
- Make user preferences persistent and reversible.
- Distinguish public conversation, private material, and server state clearly.
