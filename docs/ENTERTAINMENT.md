# LinChat 娱乐模式

娱乐模式把群聊、角色卡、服务器掷骰和持久化战役放在同一个空间。可以由房主手动主持，也可以使用 Grok / OpenAI 兼容模型，或连接独立的主持智能体。

## 5 分钟开始一场战役

1. 登录后点击左侧的**骰子图标 → 娱乐模式 → 新建战役**。
2. 填写名称、选择伙伴、固定规则版本（5e 2014、5e 2024 或自定义），写下第一幕。规则资料由房主提供，应用不内置受版权限制的完整规则书。
3. 每位玩家在**角色卡**中建立自己的角色。记录六项属性、等级、HP、AC、状态、装备、法术与资源；私密背景只对本人、房主及明确获得秘密权限的智能体开放。
4. 房主在**主持台**选择主持方式，保存，然后点击**继续战役**。
5. 玩家描述行动并点击**提交行动**，或在对应群聊用成员选择器 **@主持人**。开启自动排队时，每个行动依次交给主持处理；普通聊天不会触发模型。
6. 点击**掷骰**进行检定。服务器生成并保存每颗骰子、保留结果、修正值和总点数。
7. 房主结束本次时，可以生成 AI 摘要，或在日志编辑框选择**结束并保存摘要**，手动保存。战役暂停，角色资源、公开场景、日志和累计摘要保留，下次继续使用。

战役页面支持 `/#campaign=战役编号` 链接。新成员可由房主从右上角**群聊 → 添加成员**加入。

在当前页面内切换群聊或战役时，未发送的文字、公开／私密发送方式与接收者会保留。草稿只保存在当前页面内存中，刷新页面、退出账号或关闭窗口后会清除。

### 骰式

**属性／技能检定分级房规**：新增「行动检定」页，先由主持锁定角色、DC、修正与各档后果，再由软件投掷 d20 并结算。天然 20 大成功、1 大失败；轻度失败的代价只由角色所属玩家确认。详见 [规则与检定 API](ABILITY_CHECKS.md)。该规则不是原版 5e，不适用于攻击、豁免、伤害或死亡豁免。

| 骰式 | 含义 |
|---|---|
| `1d20+5` | 一颗 d20，加 5 |
| `2d6+3` | 两颗 d6 之和，加 3 |
| `2d20kh1+4` | 优势：保留较高一颗，加 4 |
| `2d20kl1+4` | 劣势：保留较低一颗，加 4 |

每次最多 50 颗骰子，每颗 2–1000 面，修正值范围 ±10000。房主也可以进行仅主持可见的秘密检定。

### 掷骰动画

点击**掷骰**，填写检定目的和骰式，再点击**掷骰并记录**。骰子会短暂抛起、翻滚并落定，随后显示服务器返回的每颗点数和合计。优势／劣势会分别标注「保留」和「舍弃」。点击**完成**回到日志，或点击**再掷一次**准备下一次检定。

- 动画约 0.82 秒；点数始终来自服务端，不在浏览器中重新随机。
- 网络响应较慢时，骰子停止运动并显示「等待服务器结果」。可按 Esc 关闭弹窗，已完成的结果仍会写入日志；确认失败时，重试复用同一次请求。
- 骰盘最多同时绘制六枚骰子；更多点数可在「查看全部点数」中展开，仍按全部服务器结果计算合计。
- 外形支持 D4、D6、D8、D10、D12、D20，D100 沿用十面外形；其他合法面数使用通用多面外形。骰盘继承本机的自定义配色。
- 系统开启「减少动态效果」时，取消抛起和翻滚，用短暂淡入呈现结果。历史日志保持静态。

## 内置 Grok / 兼容模型

服务器管理员打开**娱乐模式 → 模型设置**：

- API 基础地址：Grok 使用 `https://api.x.ai/v1`；兼容服务填写其包含 `/v1` 的基础地址。
- 模型 ID：填写服务商当前提供、支持 **Chat Completions + function calling** 的模型。
- API 密钥：填写该服务商的有效密钥，保存，点击**测试已保存的连接**。
- 开启内置 AI 后，房主将战役主持方式设为**内置 AI 主持**。

服务端发送请求到 `<基础地址>/chat/completions`，不经过玩家浏览器。密钥用 AES-GCM 加密保存；读取设置接口不会返回密钥。更换服务商地址时必须重新填写对应密钥。支持 HTTPS；本机兼容服务可使用 `http://localhost:端口/v1`。

一次回合最多 8 个模型工具步骤，执行期限 150 秒；每个战役每小时最多启动 60 个内置主持回合。模型需按顺序调用 `roll_dice` 或 `finish_turn`。错误、余额不足、网络中断等会显示在日志上方，由房主重试或跳过。已完成的骰子和工具记录会复用，状态变化与叙述在同一事务中提交。

**真实服务的可用性取决于管理员填写的密钥、模型权限和余额。** 仓库集成测试使用隔离的本机工具调用服务，不代表已验证某个付费账户。

### 公开与秘密

- `/context` 以及内置公开主持只接收公开场景、公开角色字段、公开日志和摘要。
- 主持笔记、私密背景、私信和秘密掷骰由服务端查询权限隔离，不会附带在玩家数据或公开模型请求中。
- 房主希望揭露秘密时，可以将线索提交为场景提案并采纳，或明确写入公开叙述。
- 外部智能体令牌可由房主额外授予秘密权限。取得这类令牌的智能体属于受信任主持方，能够读取秘密和发送获准私信；LinChat 无法控制外部程序取得数据之后如何使用。
- 公开叙述、摘要和公开骰子同步到战役群聊。私密事件只留在有权限的战役日志。
- 修改角色既有背景、房规等通过**设定提案**交由房主确认；模型的回合结果只能更新当前场景、HP、状态、物品和资源。

## 外部自动主持

1. 房主打开**智能体接入**，创建战役专用令牌，复制令牌及战役 API 地址。令牌仅显示一次，30 天后过期，可随时撤销。
2. 主持方式选择**外部智能体**，继续战役。
3. 在运行独立机器人或 MCP 客户端的电脑上配置环境变量。令牌和模型密钥不要放进群消息或提交到 Git。

### 可运行的机器人示例（Node.js 22+）

```powershell
$env:LINCHAT_CAMPAIGN_URL = "https://chat.linblog.site/api/v1/campaigns/1"
$env:LINCHAT_AGENT_TOKEN = "<在娱乐模式中创建的令牌>"
$env:MODEL_BASE_URL = "https://api.x.ai/v1"
$env:MODEL_ID = "<支持工具调用的模型 ID>"
$env:MODEL_API_KEY = "<服务商密钥>"
node scripts/campaign-agent.cjs
```

加 `--once` 只处理一次领取结果。脚本持续运行时轮询战役队列，普通群聊不会调用模型。外部示例默认只使用公开上下文；想使用主持秘密需要扩展自己的机器人流程并显式授权。

### stdio MCP 连接器

`scripts/campaign-mcp.cjs` 提供独立的 MCP JSON-RPC stdio 连接器（协议版本 `2024-11-05`），启动命令：

```text
node E:\LinChat\scripts\campaign-mcp.cjs
```

支持 stdio MCP 的客户端可以使用以下配置形状；字段外层名称以该客户端文档为准：

```json
{
  "mcpServers": {
    "linchat-campaign": {
      "command": "node",
      "args": ["E:\\LinChat\\scripts\\campaign-mcp.cjs"],
      "env": {
        "LINCHAT_CAMPAIGN_URL": "https://chat.linblog.site/api/v1/campaigns/1",
        "LINCHAT_AGENT_TOKEN": "<战役专用令牌>"
      }
    }
  }
}
```

工具包括公开上下文、角色卡、规则、获准的主持笔记、领取／完成／报告失败回合、服务器掷骰、公开／私密叙述、场景更新和故事提案。连接器不会获得服务器管理权限。

MCP 是**按需调用接口**，不会自动把群聊 @ 变成当前聊天智能体的唤醒事件。实时自动主持需要内置 API 主持，或持续运行的外部机器人。

## HTTP 接口约定

可直接交给外部主持智能体的独立文档：[战役主持 HTTP API 接入文档](CAMPAIGN_API.md)，包含认证、请求／响应字段、权限、租约、版本与完整调用顺序。

基础路径：`/api/v1/campaigns/{id}`。

- 玩家浏览器使用已有登录 Cookie；外部智能体带 `Authorization: Bearer lcgame_…`。
- 写入带 `Content-Type: application/json` 和 `X-LinChat: 1`；外部进程不发送伪造的跨站 Origin。
- 幂等写入使用 `client_id`：16–80 位字母、数字、`_`、`-`，推荐 UUID。**网络重试必须复用原编号和原内容**；同编号不同内容返回 409。
- 更新共享状态、保存角色、接受提案和发布叙述带当前 `version`。冲突返回 409，不覆盖新状态。
- 规则版本创建后不可修改。所有接口始终校验战役成员身份或当前战役令牌。

| 方法与路径 | 用途 |
|---|---|
| `GET /context` | 公开场景、角色、近期日志；活跃回合上下文不包含排在其后的玩家行动 |
| `GET /state`、`PATCH /state` | 当前状态／版本；更新场景、阶段，房主可改主持模式 |
| `GET /rules` | 固定规则版本、房规和房主提供的规则资料 |
| `GET /characters`、`POST /characters` | 读取角色；真人玩家修改自己的角色，房主可管理 |
| `GET /dm-notes`、`PUT /dm-notes` | 秘密笔记，要求房主或秘密授权；保存带 `note_version` |
| `GET /events?before=事件编号` | 按权限分页读取日志，每页至多 80 条 |
| `POST /actions` | 真人玩家提交公开行动 |
| `POST /rolls` | 服务器掷骰，支持 `public` / `dm` |
| `POST /messages` | 主持叙述／摘要或获准的私密消息 |
| `GET /story-proposals`、`POST /story-proposals` | 草稿查询／提交 |
| `POST /story-proposals/{proposal}/decide` | 房主采纳或拒绝，带 `accept` 和战役版本 |
| `GET /turns` | 最近 40 个回合状态，普通读取不返回租约 |
| `POST /turns/claim` | 外部主持领取最早可处理回合；3 分钟租约 |
| `POST /turns/{turn}/complete` | 使用租约和版本，原子提交结果 |
| `POST /turns/{turn}/fail` | 外部主持报告失败，释放租约供房主处理 |
| `POST /turns/{turn}/retry` | 房主重试，或 `skip: true` 跳过失败／超时回合 |
| `POST /host` | 房主主动继续主持：`kind: "continue"`；结束：`kind: "summary"` |
| `GET/POST /agent-tokens`、`DELETE /agent-tokens/{token}` | 房主管理专用令牌 |

服务器级接口：`GET/POST /api/v1/campaigns` 查询／创建战役；`GET/PUT /api/v1/entertainment/ai` 查询／管理模型配置；`POST /api/v1/entertainment/ai/test` 测试已保存的配置（仅管理员）。

### 领取 → 掷骰 → 完成

领取：`POST /turns/claim`，请求体 `{}`。

```json
{
  "turn": {
    "id": 12, "event_id": 35, "kind": "action", "status": "running",
    "base_version": 7, "started_at": 1795590000000, "lease": "<临时租约>"
  },
  "context": { "state": {}, "characters": [], "events": [] }
}
```

无任务时 `turn` 为 `null`。暂停、已有失败回合或其他主持已领取时返回 409。仅外部模式可以使用外部令牌领取；相同令牌重试领取返回同一租约。

掷骰：`POST /rolls`。

```json
{
  "client_id": "agent-turn-12-roll-01",
  "formula": "1d20+5",
  "reason": "调查石门",
  "visibility": "public",
  "turn_id": 12,
  "lease": "<领取时返回的租约>"
}
```

完成：`POST /turns/12/complete`。

```json
{
  "client_id": "agent-turn-12-complete",
  "version": 7,
  "lease": "<领取时返回的租约>",
  "outcome": {
    "narration": "根据刚才的检定，你辨认出了石门上的徽记。你接下来准备做什么？",
    "phase": "awaiting_action",
    "scene": "石门的徽记已经被辨认，门后仍一片安静。",
    "changes": [{ "id": 3, "hp": 10, "resources": "一环法术位：1/2" }]
  }
}
```

`changes` 为当前资源的**绝对新值**，省略字段保留原值。无变化传空数组。阶段只能为 `awaiting_action` / `awaiting_roll`；`kind: "summary"` 回合必须填写 `outcome.summary`，成功后服务端自动暂停。叙述和各资源更新一起成功或一起回滚。服务重启／房主暂停会使旧租约失效，旧主持结果不能覆盖新状态。

发送私信：`POST /messages`，`{client_id, content, visibility:"private", recipient_id}`。普通玩家只能私信房主；主持可以发给本战役真人成员；外部智能体需要秘密权限。

提案：`POST /story-proposals`，`{client_id, kind:"scene"|"house_rules"|"background", content, character_id}`；只有背景提案需要角色 ID。草稿不进入公开上下文，房主采纳后才写入正式设定和公开日志。

## 数据与部署

- 本版 SQLite 迁移版本为 `4`，包含战役、角色、事件、回合、工具步骤、幂等回执、提案、令牌、模型设置和锁定检定表；原聊天数据保留。
- 数据位于原有服务端数据目录，云端与 Radmin 主机各自保存。
- **备份完整数据目录，包含 `ai-secret.key`。** 缺少此文件时已有模型密钥不能解密。建议停服后备份完整目录，恢复后再启动。
- `deploy/update-cloud.sh` 会在升级前停服、备份数据及旧程序，并校验新服务健康状态。
- 外部连接器没有 NPM 运行依赖；使用 Node.js 22 或更高版本。

验证命令：

```powershell
go test ./...
go vet ./...
npm run test:features
npm run build:web
npm run build:server
npm run test:entertainment
npm run test:dice
npm run test:desktop
```

`test:entertainment` 使用隔离测试数据库和本机模型替身，验证桌面／移动界面、角色、骰子、私信、提案、令牌、模型配置与完整工具调用流程。

`test:dice` 自动启动并清理独立的本机预览和测试数据，使用无头浏览器验证服务器点数、优势／劣势、幂等重试、慢网关闭、私密权限、窄屏布局与减少动态效果。需要先准备 `dist/server/linchat-windows-amd64.exe`；也可通过 `LINCHAT_TEST_SERVER` 指向单独构建的验收服务端。结果位于 `dist/verification/dice/results.json`。
# 主持流式输出（1.2.7）

内置主持与新版外部主持示例支持真实流式公开正文；旧接口调用者需接入新的 stream 接口。
临时正文只用于预览，最终仍由 complete 确认。完整协议见 [HOST_STREAMING.md](HOST_STREAMING.md)。

## 剧本与记忆（1.2.10）

战役新增文字资料页，支持TXT／Markdown导入预览、章节／NPC／任务／线索／事实分类、
公开与秘密权限、长期保存、版本历史及归档。房主可选择有效公开资料自动供主持参考。
详见 [文字剧本与长期记忆](CAMPAIGN_LORE.md)。不会自动把AI推测写成既定事实，也不会自动把秘密发送给内置公开模型。

### 剧情进度（1.2.11）

资料可关联章节并分别记录任务／线索进度；房主可一键选择本幕公开参考资料。
主持完成时可提出待房主核对的记忆建议，日志提供只读的前情回顾。
见 [剧情进度与记忆整理](MEMORY_PROGRESS.md)。
