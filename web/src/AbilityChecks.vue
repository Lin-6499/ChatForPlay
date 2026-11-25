<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { abilityNames, type Campaign, type Character, type GameAPI, type GameEvent, type GameUser } from './game-types'
import { checkGrades, type AbilityCheck, type CheckEffect } from './check-types'
import CheckEffectFields from './CheckEffectFields.vue'
const props = defineProps<{ campaign: Campaign; characters: Character[]; events: GameEvent[]; members: GameUser[]; me: GameUser; api: GameAPI; revision: number }>()
const emit = defineEmits<{ changed: [] }>()
const checks = ref<AbilityCheck[]>([]), busy = ref(false), loading = ref(true), error = ref(''), editing = ref(false)
const owner = computed(() => props.campaign.owner_id === props.me.id)
const blocked = computed(() => !!props.campaign.active_turn || props.campaign.phase === 'paused')
const effect = (description = ''): CheckEffect => ({ description, hp_delta: 0 })
const form = ref({ event_id: 0, kind: 'ability', ability: 'str', skill: '', reason: '', dc: 15, bonus: 0, bonus_reason: '',
  cost: effect('无额外消耗'), outcomes: Object.fromEntries(Object.keys(checkGrades).map(key => [key, effect()])) })
const offerEnabled = ref(false), offer = ref(effect())
const actions = computed(() => props.events.filter(e => e.kind === 'action' && e.visibility === 'public' &&
  props.characters.some(ch => ch.user_id === e.actor_id) && !checks.value.some(c => c.event_id === e.id)))
const action = computed(() => actions.value.find(e => e.id === form.value.event_id))
const character = computed(() => props.characters.find(ch => ch.user_id === action.value?.actor_id))
const modifier = computed(() => character.value ? Math.floor((character.value.abilities[form.value.ability] - 10) / 2) + Number(form.value.bonus) : 0)
const player = (id: number) => props.members.find(m => m.id === id)?.nickname || `玩家 #${id}`
const status = (check: AbilityCheck) => ({ prepared: '条件已锁定，等待投骰', awaiting_choice: '等待玩家选择', resolved: '已结算', cancelled: '已取消' } as Record<string, string>)[check.status] || check.status
function describe(e: CheckEffect) {
  return [e.description, e.hp_delta ? `HP ${e.hp_delta > 0 ? '+' : ''}${e.hp_delta}` : '',
    ...(['conditions', 'inventory', 'resources'] as const).filter(k => e[k] !== undefined).map(k => `${({ conditions: '状态', inventory: '物品', resources: '资源' })[k]}：${e[k] || '清空'}`)].filter(Boolean).join(' · ')
}
let generation = 0, stopped = false
const operations = new Map<string, string>()
async function load() {
  const version = ++generation, cid = props.campaign.id
  try {
    const result = await props.api<AbilityCheck[]>(`/v1/campaigns/${cid}/checks`)
    if (!stopped && version === generation) { checks.value = result; error.value = '' }
  } catch (e) { if (version === generation) error.value = e instanceof Error ? e.message : '检定暂时无法读取' }
  finally { if (version === generation) loading.value = false }
}
async function write(suffix: string, body: Record<string, unknown>) {
  if (busy.value) return
  busy.value = true; error.value = ''
  const path = `/v1/campaigns/${props.campaign.id}/checks${suffix}`, key = path + JSON.stringify(body)
  if (!operations.has(key)) operations.set(key, Array.from(crypto.getRandomValues(new Uint8Array(18)), n => n.toString(16).padStart(2, '0')).join(''))
  try {
    await props.api(path, 'POST', { ...body, client_id: operations.get(key) })
    operations.delete(key); editing.value = false
    await load(); emit('changed')
  } catch (e) { error.value = e instanceof Error ? e.message : '操作失败，请重试' }
  finally { busy.value = false }
}
function prepare() {
  if (!character.value) return
  void write('', { ...form.value, character_id: character.value.id, version: props.campaign.version, ...(offerEnabled.value ? { offer: offer.value } : {}) })
}
watch(() => props.revision, () => { if (!busy.value) void load() })
watch(() => props.campaign.id, () => { checks.value = []; editing.value = false; loading.value = true; void load() }, { immediate: true })
watch(() => form.value.event_id, () => { form.value.reason = action.value?.content.slice(0, 200) || '' })
onBeforeUnmount(() => { stopped = true; generation++ })
</script>
<template>
  <section class="ability-checks">
    <div class="game-section-heading"><div><h3>属性与技能检定</h3><p>标准 d20 范围＋分级结果房规 · 非 D&D 5e 原版规则</p></div><button v-if="owner" class="game-button" :disabled="busy || blocked" @click="editing = !editing">{{ editing ? '收起表单' : '创建检定' }}</button></div>
    <details class="check-rules"><summary>查看判定规则与边界</summary><p>天然 20 大成功、天然 1 大失败，优先于总点数。其余按总点数减 DC：≥5 出色成功，0～4 普通成功，-4～-1 轻度失败，≤-5 明显失败。</p><p>仅用于属性／技能检定。攻击、豁免、伤害骰和死亡豁免按战役所选规则处理。大成功不能完成不可能的事，大失败不能凭空即死或替玩家决定行动。</p></details>
    <p v-if="blocked" class="game-helper">{{ campaign.phase === 'paused' ? '战役暂停中，请房主继续后再投骰或选择代价。' : '主持正在处理回合；完成后玩家才能确认代价。' }}</p>
    <p v-if="error" class="game-error" role="alert">{{ error }} <button class="text-button" :disabled="busy" @click="load">重新读取检定</button></p>
    <form v-if="editing && owner" class="game-form check-create" @submit.prevent="prepare">
      <label>玩家行动<select v-model.number="form.event_id" required aria-label="检定对应的玩家行动"><option :value="0" disabled>选择已提交的行动</option><option v-for="e in actions" :key="e.id" :value="e.id">{{ player(e.actor_id) }}：{{ e.content }}</option></select></label>
      <p v-if="!actions.length" class="game-helper">暂无可检定的行动。请玩家先创建角色并提交行动；同一行动只能创建一次检定。</p>
      <p v-if="character" class="game-helper">角色：{{ character.name }} · 玩家：{{ player(character.user_id) }} · HP {{ character.hp }}/{{ character.max_hp }}</p>
      <div class="game-form-row"><label>检定类型<select v-model="form.kind"><option value="ability">属性检定</option><option value="skill">技能检定</option></select></label><label>检定属性<select v-model="form.ability"><option v-for="(name, key) in abilityNames" :key="key" :value="key">{{ name }}</option></select></label><label v-if="form.kind === 'skill'">技能名称<input v-model="form.skill" maxlength="60" required /></label></div>
      <label>检定目的<input v-model="form.reason" maxlength="200" required /></label>
      <div class="game-form-row"><label>难度 DC<input v-model.number="form.dc" type="number" min="1" max="40" required /></label><label>额外修正<input v-model.number="form.bonus" type="number" min="-20" max="20" required /></label></div>
      <label v-if="form.bonus">额外修正依据<input v-model="form.bonus_reason" maxlength="200" required placeholder="例如：熟练加值 +2" /></label>
      <p class="game-helper">属性修正自动读取角色卡，当前总修正 {{ modifier >= 0 ? '+' : '' }}{{ modifier }}。先锁定下面的全部条件，再由软件投骰。</p>
      <CheckEffectFields :effect="form.cost" label="尝试消耗（投骰时必付）" />
      <CheckEffectFields v-for="(name, grade) in checkGrades" :key="grade" :effect="form.outcomes[grade]" :label="name" />
      <label class="game-check"><input v-model="offerEnabled" type="checkbox" />轻度失败时允许玩家付出代价换取成功</label>
      <CheckEffectFields v-if="offerEnabled" :effect="offer" label="接受代价后的成功结果" />
      <p class="game-helper">接受代价后应用替代成功结果；拒绝则应用轻度失败后果。不会同时应用两项结果。物品和资源请填写变化后的完整清单。</p>
      <button class="primary" :disabled="busy || blocked || !character">{{ busy ? '正在锁定…' : '锁定条件' }}</button>
    </form>
    <p v-if="loading" class="game-helper" role="status">正在读取检定…</p>
    <p v-else-if="!checks.length && !editing && !error" class="game-empty-small">还没有检定。玩家提交行动后，由房主或主持智能体确定属性、DC 和后果。</p>
    <article v-for="check in checks" :key="check.id" class="check-record" :data-check-id="check.id">
      <header><div><h4>{{ check.reason }}</h4><p>{{ check.character_name }} · 玩家：{{ player(check.user_id) }}<span v-if="check.user_id === me.id"> · 我的角色</span></p></div><span>{{ status(check) }}</span></header>
      <p>{{ abilityNames[check.ability] }}{{ check.skill ? `（${check.skill}）` : '' }} · DC {{ check.dc }} · 属性修正 {{ check.ability_modifier >= 0 ? '+' : '' }}{{ check.ability_modifier }} · 额外修正 {{ check.bonus >= 0 ? '+' : '' }}{{ check.bonus }}{{ check.bonus_reason ? `（${check.bonus_reason}）` : '' }}</p>
      <p v-if="check.natural" class="check-result"><strong>{{ checkGrades[check.grade] }}</strong> 原始 {{ check.natural }} + 修正 {{ check.modifier }} = {{ check.total }}，差值 {{ check.margin >= 0 ? '+' : '' }}{{ check.margin }}</p>
      <p v-if="check.resolution">{{ check.resolution }}</p>
      <p v-if="check.natural" class="game-helper">HP：{{ check.before.hp }} → {{ check.after.hp }}；资源变化已保存，重复请求不会再次扣除。</p>
      <details><summary>已锁定的消耗与后果</summary><dl class="check-consequences"><dt>尝试消耗</dt><dd>{{ describe(check.cost) }}</dd><template v-for="(name, grade) in checkGrades" :key="grade"><dt>{{ name }}</dt><dd>{{ describe(check.outcomes[grade]) }}</dd></template><template v-if="check.offer"><dt>可选的代价成功</dt><dd>{{ describe(check.offer) }}</dd></template></dl></details>
      <div v-if="check.status === 'prepared'" class="game-actions">
        <button v-if="owner || check.user_id === me.id" class="game-button" :disabled="busy || blocked || !!check.turn_id" @click="write(`/${check.id}/roll`, {})">投掷 d20 并结算</button>
        <button v-if="owner" class="text-button" :disabled="busy || !!campaign.active_turn" @click="write(`/${check.id}/cancel`, { version: campaign.version })">取消未投骰检定</button>
        <span v-if="check.turn_id" class="game-helper">由领取此回合的主持智能体投骰。</span>
      </div>
      <div v-if="check.status === 'awaiting_choice' && check.offer" class="check-choice">
        <p><b>接受代价：</b>{{ describe(check.offer) }}</p><p><b>拒绝代价：</b>{{ describe(check.outcomes.minor_failure) }}</p>
        <div v-if="check.user_id === me.id" class="game-actions"><button class="game-button" :disabled="busy || blocked" @click="write(`/${check.id}/choose`, { accept: true })">接受代价，换取成功</button><button class="game-button" :disabled="busy || blocked" @click="write(`/${check.id}/choose`, { accept: false })">拒绝，保留失败结果</button></div>
        <p v-else class="game-helper">仅 {{ player(check.user_id) }} 可以作出选择。</p>
      </div>
    </article>
  </section>
</template>
