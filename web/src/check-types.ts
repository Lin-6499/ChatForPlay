export const checkGrades: Record<string, string> = {
  critical_success: '大成功', great_success: '出色成功', success: '普通成功',
  minor_failure: '轻度失败', failure: '明显失败', critical_failure: '大失败',
}
export interface CheckEffect { description: string; hp_delta: number; conditions?: string; inventory?: string; resources?: string }
export interface CheckSnapshot { hp: number; max_hp: number; conditions: string; inventory: string; resources: string }
export interface AbilityCheck {
  id: number; event_id: number; character_id: number; user_id: number; character_name: string; turn_id: number
  kind: string; ability: string; skill: string; reason: string; ability_score: number; ability_modifier: number; bonus: number; bonus_reason: string
  modifier: number; dc: number; cost: CheckEffect; outcomes: Record<string, CheckEffect>; offer?: CheckEffect
  status: string; natural: number; total: number; margin: number; grade: string; accepted?: boolean; resolution: string
  before: CheckSnapshot; after: CheckSnapshot
}
