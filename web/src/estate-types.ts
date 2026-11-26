export interface EstatePlayer {
  id: number; name: string; token: number; ready: boolean; cash: number; position: number
  bankrupt: boolean; checked: boolean; coupon: boolean; rent_earned: number; max_payment: number; turns: number
}
export interface EstateProperty { owner: number; level: number; mortgaged: boolean }
export interface EstateTile { id: number; name: string; kind: string; group: string; price: number; upgrade: number; rents: number[] }
export interface EstateGame {
  id: number; conversation_id: number; owner_id: number; title: string; capacity: number; ruleset: string
  version: number; phase: string; round: number; current: number; deadline: number; players: EstatePlayer[]
  properties: EstateProperty[]; dice: number[]; last_card: string; debt: number; creditor: number; upgraded: boolean
  events: { seq: number; at: number; text: string }[]
}
export interface EstateScore { id: number; name: string; cash: number; land: number; buildings: number; equity: number; total: number; properties: number; bankrupt: boolean; turns: number; rank: number }
export interface EstateRentChange { tile: number; before: number; after: number }
export interface EstateQuote { redeem_cost: number; sell_income: number; mortgage_income: number; sell_rents: EstateRentChange[]; mortgage_rents: EstateRentChange[] }
export interface EstateView { game: EstateGame; tiles: EstateTile[]; rents: number[]; quotes: EstateQuote[]; scores: EstateScore[]; server_time: number }
export interface EstateRoom { id: number; title: string; capacity: number; count: number; phase: string; joined: boolean; version: number }
export const tokenNames = ['1号 · 蓝色', '2号 · 橙色', '3号 · 紫色', '4号 · 绿色']
export const phaseNames: Record<string, string> = {
  lobby: '等待准备', management: '管理资产 / 掷骰', purchase_decision: '选择是否购买', market_decision: '市集',
  check_decision: '处理检查', debt: '偿还债务', end_turn: '回合结算', finished: '已结束', closed: '已关闭',
}
