# 属性／技能检定分级房规与 API

规则编号：`d20-graded-v1`。这是 LinChat 新增房规，**不是 D&D 5e 原版规则**。软件保留战役所选规则版本；攻击、豁免、伤害骰、死亡豁免仍使用通用骰子并按所选规则处理，不套用本分级。

## 玩家与房主如何使用

1. 玩家创建自己的角色卡，在「冒险日志」提交行动。
2. 房主进入「行动检定 → 创建检定」，选择行动。软件按行动的 `actor_id` 匹配角色 `user_id`，显示角色和所属玩家。
3. 确定属性／技能、DC、额外修正及依据、尝试消耗、全部六档后果。可为轻度失败增加“付出代价换取成功”。
4. 点击「锁定条件」。角色属性修正由软件从角色卡计算；条件不可修改。尚未投骰时房主可以取消，取消不扣资源。再次尝试需玩家提交新行动。
5. 手动主持时房主或该角色的玩家点击「投掷 d20 并结算」。外部／内置主持的检定由领取回合的主持方调用投骰接口。
6. 软件随机生成 1～20，计算等级并保存资源变化。出现可选代价时，只有角色所属的真人玩家能在「行动检定」接受或拒绝，房主与智能体都不能代选。

所有战役成员可以查看公开检定条件及结果。日志和群聊记录条件锁定、投骰结果和玩家选择；角色的私密笔记不包含在检定中。

## 判定顺序

| 条件 | 等级 |
|---|---|
| 原始骰子 = 20 | 大成功 `critical_success` |
| 原始骰子 = 1 | 大失败 `critical_failure` |
| 其余，总点数 − DC ≥ 5 | 出色成功 `great_success` |
| 差值 0～4 | 普通成功 `success` |
| 差值 -4～-1 | 轻度失败 `minor_failure` |
| 差值 ≤ -5 | 明显失败 `failure` |

原始点数优先于差值；16 加 4 得到 20 不算天然 20。属性修正为 `floor((属性值 - 10) / 2)`，负数向下取整。最终修正 = 属性修正 + 预先声明的额外修正（例如熟练加值）。

大成功不能完成不可能的事情；大失败不能凭空即死或代玩家决定行动。主持方在投骰前提供符合这些边界的具体剧情后果；软件执行已声明的数值和资源变化，不自行判断叙事是否“可能”，也不自动解析自然语言中的扣血描述。

## 保存与资源规则

- 一项玩家行动只能创建一个检定，数据库按战役和行动事件编号唯一约束；更换 `client_id` 也不能重建或重投同一项检定。
- `prepared`：已锁定、尚未投骰，未扣资源。
- `awaiting_choice`：轻度失败且有代价选项。尝试消耗已付，只等待选择后的后果。
- `resolved`：已结算。`accepted` 表示是否接受代价；原始 `grade` 仍保留轻度失败，`resolution` 记录实际采用的替代成功／失败结果。
- `cancelled`：房主取消未投骰检定，不生成骰子、不扣资源。
- `hp_delta` 是生命值增减；`conditions`、`inventory`、`resources` 是完整新值，省略保留，空字符串清空。文本资源适用于现有角色卡，不会从“箭 -1”自动解析出数量。
- 先应用尝试消耗，再应用命中的等级后果。轻度失败有 offer 时，接受应用 offer，拒绝应用 minor_failure，两者不会同时执行。
- 所有分支在锁定时检查，HP 不得低于 0 或高于上限；不隐式截断数值。不能通过这套接口直接修改背景、规则、等级、属性或最大生命值。
- 检定准备中或等待选择时，角色卡资源被锁定，避免其他写入覆盖它。软件按角色版本原子更新。
- 投骰与当次资源变化、审计日志、幂等回执在同一事务提交。重复投骰接口返回已保存结果，即使换请求编号或换成另一位有权调用者，也不会重投。
- 数据库迁移版本为 4，新增 `campaign_checks` 表，旧数据保留。

## API

基础地址及认证同 [CAMPAIGN_API.md](CAMPAIGN_API.md)，以下路径相对 `/api/v1/campaigns/{id}`。成功返回 HTTP 200，错误为 `{ "error": "原因" }`。

| 接口 | 权限与用途 |
|---|---|
| `GET /checks` | 战役成员／本战役令牌。最多 80 项，未结算项优先，其次 ID 降序 |
| `GET /checks/{check_id}` | 读取单个检定最新状态 |
| `POST /checks` | 房主／主持智能体锁定条件 |
| `POST /checks/{check_id}/roll` | 房主／主持智能体／角色本人投骰；活跃回合仍必须验证所属租约 |
| `POST /checks/{check_id}/choose` | 仅角色所属真人玩家选择，Bearer 令牌不允许 |
| `POST /checks/{check_id}/cancel` | 仅房主取消未投骰检定；需无活跃回合及当前战役版本 |

所有写入携带 `client_id`，规则与现有 API 一致。请求不接受客户端指定的原始点数、结果等级或最终修正。

### 1. 锁定条件

`POST /checks`：

```json
{
  "client_id": "example-check-prepare-001",
  "version": 7,
  "event_id": 35,
  "character_id": 3,
  "kind": "skill",
  "ability": "str",
  "skill": "运动",
  "reason": "尝试踹开木门",
  "bonus": 2,
  "bonus_reason": "运动熟练加值 +2",
  "dc": 15,
  "cost": { "description": "无额外尝试消耗", "hp_delta": 0 },
  "outcomes": {
    "critical_success": { "description": "木门打开，并发现门闩可保留再用", "hp_delta": 0 },
    "great_success": { "description": "迅速开门，动静较小", "hp_delta": 0 },
    "success": { "description": "木门打开", "hp_delta": 0 },
    "minor_failure": { "description": "木门未开，可考虑其他方法", "hp_delta": 0 },
    "failure": { "description": "木门未开，附近守卫听到了声音", "hp_delta": 0 },
    "critical_failure": { "description": "木门未开，踢击造成擦伤并发出响声", "hp_delta": -1 }
  },
  "offer": { "description": "承担擦伤，强行挤开木门", "hp_delta": -2 }
}
```

`event_id` 必须是公开 `action` 日志，作者须与角色所属玩家一致。普通消息、房主继续请求、其他玩家行动不能作为这张角色卡的行动。

- `kind` 只允许 `ability`、`skill`；技能检定需 `skill`。
- `ability` 只允许 `str`、`dex`、`con`、`int`、`wis`、`cha`。
- DC 1～40，额外修正 -20～20，非零额外修正必须有依据。
- 检定目的／修正依据最多 200 字，技能名最多 60 字；后果说明 1～1000 字；资源文本每项最多 6000 字。
- 外部／内置主持在执行中的回合还要带 `turn_id`、`lease`，且 `event_id` 必须是该回合的行动事件。不在回合中的手动检定使用当前战役 `version`。
- 六档后果和 cost 必填；offer 可省略。锁定后没有编辑接口。

响应为检定对象，包含 `id`、原输入的锁定条件，以及软件读取的 `user_id`、`character_name`、`ability_score`、`ability_modifier`、`modifier`、`before`、`after`、`character_version`、时间戳和 `status`。

`before`／`after` 结构为 `{hp, max_hp, conditions, inventory, resources}`。未投骰时 `natural`、`total`、`margin` 为 0，`grade` 为空。

### 2. 投骰并结算

`POST /checks/8/roll`：

```json
{
  "client_id": "example-check-roll-0001",
  "turn_id": 12,
  "lease": "领取回合返回的租约"
}
```

手动检定省略 `turn_id`、`lease`。不得追加 `natural`、`dc` 等字段。

响应为更新后的完整检定对象，包含 `natural`、`modifier`、`total`、`dc`、`margin`、`grade`、资源前后快照和 `resolution`。未投骰的请求才检查当前租约／阶段；已投骰的重试在鉴权后读取原结果，不执行第二次。

### 3. 玩家选择代价

角色所属玩家使用自己的登录 Cookie：

```http
POST /checks/8/choose
Content-Type: application/json
X-LinChat: 1
```

```json
{ "client_id": "example-check-choice-01", "accept": true }
```

`false` 表示拒绝。必须在战役继续且无活跃主持回合时选择。重复同一选择返回已结算结果；试图改变已经作出的选择返回 409。

自动主持开启时，玩家选择会排队一个后续主持回合，继续叙述结果；等待玩家选择期间新的主持回合不会被领取。

### 4. 取消未投骰检定

```json
{ "client_id": "example-check-cancel-01", "version": 7 }
```

向 `POST /checks/8/cancel` 提交。取消不会删除审计记录，也不会允许对同一行动重建检定。失败／超时回合留下未投骰检定时，可重试原回合继续投骰；若放弃，由房主先暂停／结束活跃状态，再取消未投骰检定。

## 主持智能体接入

`GET /context` 及领取响应的公开上下文增加：

- `check_rule: "d20-graded-v1"`
- `checks`：当前公开检定状态，供恢复和继续叙述。

推荐流程：

```text
claim 回合 → 根据行动选择对应角色
POST /checks 锁定全部条件 → POST /checks/{id}/roll
读取软件等级和资源结果 → complete 仅提交叙述／场景
若 awaiting_choice：complete 后等待玩家确认
```

执行中检定保留当前回合 `base_version`，完成回合再递增战役版本。手动检定／玩家选择直接递增战役版本。`complete.outcome.changes` 不能再次修改本回合已结算检定的角色，也不能修改任何仍被检定锁定的角色。

检定资源在投骰事务中已经生效，后续主持失败、超时或跳过不会撤销已保存结果。重试原回合时读取原检定并继续叙述，不能重新扣资源。

内置模型增加 `prepare_check`、`roll_check` 工具；外部机器人示例同样支持。MCP 增加 `campaign_checks`、`campaign_check_get`、`campaign_prepare_check`、`campaign_check_roll`，不向智能体提供代替玩家选择的工具。现有 `/rolls` 继续用于通用骰子，不执行本分级规则。
