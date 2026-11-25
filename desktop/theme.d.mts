export interface Appearance { version: number; accent: string; background: string; panel: string }
export interface AppearanceBridge {
  getAppearance?(): Promise<Appearance | null>
  setAppearance?(value: Appearance): Promise<Appearance>
}
export const APPEARANCE_KEY: string
export const DEFAULT_APPEARANCE: Readonly<Appearance>
export const APPEARANCE_PRESETS: readonly (Appearance & { id: string; name: string })[]
export function normalizeColor(value: unknown): string | null
export function normalizeAppearance(value: unknown): Appearance
export function mix(a: string, b: string, amount: number): string
export function contrast(a: string, b: string): number
export function readableColor(background: string, preferred: string, minimum?: number): string
export function appearanceTokens(value: Appearance): Record<string, string>
export function getAppearance(): Appearance
export function applyAppearance(value: Appearance): Appearance
export function onAppearanceChange(listener: (value: Appearance) => void): () => void
export function initializeAppearance(bridge?: AppearanceBridge): Promise<void>
export function saveAppearance(value: Appearance): Promise<Appearance>
