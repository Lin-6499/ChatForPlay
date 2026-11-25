export interface GameUser { id: number; username: string; nickname: string; admin: boolean }
export interface Campaign {
  id: number; conversation_id: number; owner_id: number; bot_id: number; title: string; ruleset: string
  house_rules: string; scene: string; summary: string; phase: string; mode: string; auto_respond: boolean
  version: number; active_turn: number; created_at: number; updated_at: number
}
export interface Character {
  id: number; campaign_id: number; user_id: number; name: string; role: string; level: number
  hp: number; max_hp: number; ac: number; abilities: Record<string, number>; conditions: string
  inventory: string; resources: string; background: string; private_notes?: string; version: number
}
export interface DiceInput { formula: string; reason: string; visibility: string }
export interface DiceResult { formula: string; dice: number[]; kept: number[]; modifier: number; total: number }
export interface DiceReceipt { event_id: number; turn_id: number; roll: DiceResult }
export interface GameEvent {
  id: number; actor_id: number; actor: string; kind: string; visibility: string; recipient_id: number; content: string; created_at: number
  payload: { roll?: DiceResult; turn_id?: number; reply_prompt?: ReplyPrompt; reply_to_event_id?: number; option_id?: string }
}
export interface ReplyOption { id: string; label: string; content: string }
export interface ReplyPrompt { target_user_id: number; options: ReplyOption[] }
export interface ReplyPromptView extends ReplyPrompt { event_id: number; content: string; answered: boolean; can_reply: boolean }
export interface Turn { id: number; event_id: number; kind: string; status: string; base_version: number; started_at: number; error: string }
export interface Proposal { id: number; author_id: number; kind: string; content: string; character_id: number; status: string; version: number }
export interface AgentToken { id: number; label: string; secrets: boolean; expires_at: number; revoked: boolean }
export interface AISettings { enabled: boolean; has_key: boolean; base_url?: string; model?: string }
export type GameAPI = <T = any>(path: string, method?: string, body?: unknown, timeout?: number) => Promise<T>
export const phases: Record<string, string> = { paused: '已暂停', awaiting_action: '等待玩家行动', awaiting_roll: '等待掷骰', narrating: '主持叙述中' }
export const rulesets: Record<string, string> = { '5e-2024': 'D&D 5e · 2024', '5e-2014': 'D&D 5e · 2014', custom: '自定义规则' }
export const modes: Record<string, string> = { manual: '手动主持', builtin: '内置 AI 主持', external: '外部智能体' }
export const abilityNames: Record<string, string> = { str: '力量', dex: '敏捷', con: '体质', int: '智力', wis: '感知', cha: '魅力' }
