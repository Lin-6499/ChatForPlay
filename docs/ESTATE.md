# 地产大亨：使用与接口

## 本地交互预览

`node scripts/estate-preview.cjs`启动隔离测试服务，输出可直接访问的预览地址。
两名测试同伴在回合中短暂停顿约1.5秒后，使用普通玩家接口自动掷骰、购地、建设和偿债，
不替人类预览账号操作；正式游戏规则及45秒期限不变。陪玩仅属于预览脚本，不是正式AI玩家功能。
`node --test scripts/estate-preview-bots.test.cjs`验证决策、真人隔离及真实服务器及时行动。

本功能包含于1.2.15及更新版本的内置服务端；是否在线可用取决于所连接远程服务器是否部署了包含本功能的版本。
详细房规和地图见 [首版设计](ESTATE_V1.md)。

## 使用

1. 登录后打开娱乐模式，选择“地产大亨 · 多人棋盘”。
2. 创建2–4人的房间，朋友从同服务器的房间列表加入。
3. 每位玩家选择不同编号及对应配色的圆形棋子并准备；房主点击“开始对局”。编号在对局中固定，不随行动顺序改变。
4. 当前玩家在45秒内管理资产并掷骰；落在无主地产时在20秒内购买或放弃。
5. 现金不足时选择自己的地产拆建或抵押；60秒到期按既定顺序清算。
6. 12轮结束或仅剩一名未破产玩家时结算。再来一局创建新房，历史对局不覆盖。

多人状态、骰子、牌堆、资产和期限保存在服务器 SQLite。动画不决定结果，刷新不会重新投骰。
房间聊天使用现有群聊接口，普通聊天不触发游戏操作。
准备房公开列在同服务器登录用户的列表中；开始后的快照和记录只允许参赛玩家读取。
每位房主最多保留3个未结束房间；房间列表返回最近200个记录中可见的房间。
开局后没有中途补位，离线计时继续；全员离线也按规则完成对局。

## HTTP 认证与写请求

使用真人账号登录 Cookie `linchat_session`；不接受剧情战役的主持令牌代替玩家。
所有写请求包含 `X-LinChat: 1`、`Content-Type: application/json`。

```json
{"client_id":"a2d74a60-5cbf-4db4-a7ee-6cfe6d170dcc","version":8}
```

- `client_id`遵循现有16–80位请求编号规则，推荐UUID。
- 请求失败且结果未知时，原请求体及编号一起重试；不生成新编号。
- 同一账号复用编号但改变路径或内容返回409。
- `version`来自最新快照；创建房间可以省略。版本冲突应刷新，不能自动用新版本重放购买。
- 重试成功返回原回执，之后重新GET最新快照，因为其他玩家可能已经继续操作。

## 房间

| 接口 | 作用 |
|---|---|
| `GET /api/v1/estate/rooms` | 可加入的准备房和本人参与的对局摘要 |
| `POST /api/v1/estate/rooms` | 创建独立对局及聊天群 |
| `POST /api/v1/estate/rooms/{id}/join` | 加入未开始且未满的房间 |
| `POST /api/v1/estate/rooms/{id}/ready` | 设置准备状态 |
| `POST /api/v1/estate/rooms/{id}/token` | 选择棋子，同时取消自己的准备 |
| `POST /api/v1/estate/rooms/{id}/start` | 房主开始，要求至少2人且全员准备 |
| `POST /api/v1/estate/rooms/{id}/leave` | 离开准备房；房主移交最早加入的剩余玩家 |

首版一个房间对应一个对局，room id与match id相同。

创建：
```json
{"client_id":"a2d74a60-5cbf-4db4-a7ee-6cfe6d170dcc","title":"周末环城局","capacity":4}
```

准备：
```json
{"client_id":"bb9b1a69-7782-4bec-8e0c-63d4a150d549","version":3,"ready":true}
```

棋子字段`token`取0–3，分别对应1–4号圆形棋子，使用独立的蓝、橙、紫、绿实色及白色数字辅助识别，不随界面强调色变化。选中别人占用的编号返回409。
棋盘的彩色“地主：1号”等标签表示地产主人编号；独立底部停靠区的数字圆形表示当前停留的玩家，二者不混用。
点击玩家姓名即可在资产列表查看其全部地产；点击地产查看详情。实线框表示选中查看，
虚线框表示当前行动玩家的权威落点。小窗口操作区保持置顶，聊天在棋盘之后展开；
完整棋盘支持内部横向滚动，不再通过隐藏地名、租金或地主解决小屏遮挡。
最后一人离开准备房后状态为`closed`，不再显示在列表。

## 快照与动作

`GET /api/v1/estate/matches/{id}`，及所有成功写请求，返回：

```text
{
  game: {
    id, conversation_id, owner_id, title, capacity, ruleset,
    version, phase, round, current, deadline,
    players: [...], properties: [...], dice: [d1,d2],
    last_card, debt, creditor, upgraded, events: [...]
  },
  tiles: [...], rents: [...], quotes: [...], scores: [...], server_time
}
```

- `current`是`players`数组中的索引，不是用户ID。
- `deadline`、`server_time`均为Unix毫秒；无期限时deadline为0。
- `properties`固定24项，以格号索引；owner为用户ID或0。
- `quotes`固定24项，提供服务器整数计算的`redeem_cost`、`sell_income`、`mortgage_income`及拆建/抵押影响的全部地产租金`{tile,before,after}`；界面直接使用，避免小数舍入差异和遗漏成组租金影响。
- 牌堆次序、抽牌游标不会进入公开快照、WebSocket通知或操作回执。
- `scores`含排名、现金、土地、建筑清算额、抵押剩余价值和净资产；对局中可查看实时估值，结束后作为结算。

动作统一发送到`POST /api/v1/estate/matches/{id}/actions`：

```json
{"client_id":"8f95df2e-7561-4021-bf54-a9b3569ab138","version":8,"action":"upgrade","tile":3}
```

| action | 所需阶段 | 额外字段 |
|---|---|---|
| `roll` | management | 无 |
| `buy` / `decline` | purchase_decision | 无，使用服务器当前落点 |
| `upgrade` | management | tile，且每回合最多一次 |
| `sell` / `mortgage` | management或debt | tile |
| `redeem` | management | tile |
| `check_release` / `check_wait` | check_decision | 无 |
| `market_buy` / `market_leave` | market_decision | 无 |

所有动作要求当前玩家身份。金额、点数、目的格、其他玩家余额不作为客户端输入。
同一次落点的租金、免租券、税费与强制事件均由服务器自动结算。

## 推送、历史与恢复

成员WebSocket接收：
```json
{"type":"estate.changed","room_id":12,"version":37}
```

收到后读取快照。前端同时以3秒轮询兜底；断线不影响服务器期限执行。
`GET /api/v1/estate/matches/{id}/events?after_seq=20`读取序号20之后的事件。
完整对局历史也包含在快照中；聊天历史通过快照中的`conversation_id`使用既有聊天接口分页读取。

期限处理与玩家写入使用数据库事务串行结算。处理完成即写新状态和期限，重启扫描到期记录继续处理，
不会以客户端动画结束或浏览器计时回调为权威。

## 错误

| 状态码 | 典型情况 |
|---|---|
| 400 | JSON、人数、请求编号、棋子或动作无效 |
| 401 | 未登录或会话失效 |
| 403 | 非参赛玩家、越权开始、不是当前玩家或管理他人地产 |
| 404 | 对局不存在 |
| 409 | 旧版本、超时、阶段不符、现金不足、房满、重复编号内容不同 |
| 429 | 写入过快 |

事务失败不写半条扣款或回执。未知网络失败保留原请求，界面提供明确重试。

## 验收

```text
go test ./...
npm run build
npm run test:estate
npm run test:entertainment
```

`test:estate`使用隔离本机服务、四个账号与无头Chromium，走实际HTTP、WebSocket及界面。
覆盖准备开局、聊天、掷骰/购买、响应丢失重试、完整12轮、四端结算一致和再开一局。
截图、结果与自动对局过程记录在`dist/verification/estate/`。不是线上负载或经济平衡测试。

另外，`scripts/estate-edge-check.cjs`验证110/220精确余额赎回、成组地产与双枢纽的清算租金影响、
真实偿债动作以及棋子位置的可访问名称。专用Go测试只允许向`dist/verification/campaign-demo-*`
隔离目录创建边界场景，通过真实规则进入债务；生产服务不包含测试后门。
`results.json`和`edge-results.json`分别记录8项完整流程与4项边界验收；`build-binding.json`记录对应源码和构建校验值。
