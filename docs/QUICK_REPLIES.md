# 主持人快捷回复 API

主持人发布问题时可以附带 1–6 个建议选项；玩家可以选择一个，也可以自由输入。选项只是回复建议，不会自动掷骰、扣资源或代替玩家作决定。

本功能需要部署包含此接口的新版服务端与 Web 页面；旧客户端可继续通过原有 `/actions` 提交普通行动。

## 1. 认证与通用约定

基础路径：`/api/v1/campaigns/{campaign_id}`。

- 玩家与手动主持使用登录会话 Cookie `linchat_session`。
- 外部主持使用 `Authorization: Bearer <战役令牌>`，只能访问令牌所属战役。
- 写请求必须包含 `Content-Type: application/json` 和 `X-LinChat: 1`。
- 写请求的 `client_id` 为 16–80 字符的唯一请求编号，推荐 UUID。网络重试必须复用原编号及原请求体；相同编号改内容返回 `409`。
- 玩家账号必须是战役成员。智能体令牌只能发布选项，不能替玩家回复。
- `version` 从 `GET /state` 获取；外部回合完成使用领取时的 `base_version`。

完整认证、回合租约和错误协议见 [CAMPAIGN_API.md](CAMPAIGN_API.md)。

## 2. 选项数据结构

```json
{
  "target_user_id": 2,
  "options": [
    {"id": "accept", "label": "接下委托", "content": "我接下委托，询问出发前还需要准备什么。"},
    {"id": "ask_pay", "label": "先询问报酬", "content": "我想先问清这份委托的报酬和风险。"},
    {"id": "leave", "label": "暂时离开", "content": "我暂时不接委托，先与同伴商量。"}
  ]
}
```

| 字段 | 类型 | 规则 |
|---|---|---|
| `target_user_id` | integer | `0` 表示所有成员各自可回复一次；正数指定一名战役成员，不能指定主持机器人 |
| `options` | array | 1–6 个选项 |
| `options[].id` | string | 同组内唯一，匹配 `[A-Za-z0-9_-]{1,40}` |
| `options[].label` | string | 按钮文字，1–40 字 |
| `options[].content` | string | 完整回复，1–4000 字；选择后由服务器从存档取出 |

自由输入始终可用，没有 `allow_custom=false` 或强迫玩家只能选按钮的模式。此字段及选项均为公开资料，不要放入主持秘密。指定回复者只限制谁能关联回答，不会把问题变成私信。

## 3. 手动主持／外部工具直接发布

`POST /messages`

```json
{
  "client_id": "b21ba85c-70af-44d3-a56c-04f8b1352909",
  "version": 17,
  "kind": "narration",
  "visibility": "public",
  "content": "露娜把委托书推到你面前：你打算怎样回应？",
  "reply_prompt": {
    "target_user_id": 2,
    "options": [
      {"id": "accept", "label": "接下委托", "content": "我接下委托，准备出发。"},
      {"id": "ask_pay", "label": "询问报酬", "content": "我想先了解这份委托的报酬。"}
    ]
  }
}
```

成功：

```json
{"event_id": 351}
```

要求：

- 当前账号为房主或获授权的主持智能体。
- 战役处于 `awaiting_action`，没有正在运行的主持回合，`version` 与当前存档一致。
- `reply_prompt` 只允许附在公开 `narration` 上，不允许用于私信或结束摘要。
- 新叙述即使不带 `reply_prompt`，也会撤下上一组快捷回复。历史日志仍保留原选项。

界面上房主选择“发送为 → 主持叙述”，展开“添加快捷回复”，选择回复者并按行填写选项。界面以每行文字同时作为按钮标题和回复内容；API 可以分别设置简短标题与长回复。

## 4. 外部智能体在完成回合时发布

`POST /turns/{turn_id}/complete`

```json
{
  "client_id": "7d798285-e93d-4b7e-9766-334c897ec837",
  "version": 17,
  "lease": "领取回合时返回的租约",
  "outcome": {
    "narration": "你找到了委托登记处。接待员问你是否愿意接下调查任务。",
    "phase": "awaiting_action",
    "changes": [],
    "reply_prompt": {
      "target_user_id": 2,
      "options": [
        {"id": "accept", "label": "接受", "content": "我愿意接下调查任务。"},
        {"id": "details", "label": "了解详情", "content": "我先询问失踪者和最后出现的地点。"}
      ]
    }
  }
}
```

选项与叙述在同一事务中存档，完成回合后一起对玩家可用。已有检定需要玩家选择代价时，不能用快捷回复替代 `/checks/{check}/choose`；若完成结果被要求等待检定，附 `reply_prompt` 会被拒绝，应先完成检定流程。

内置 AI 的 `finish_turn`、外部示例 `scripts/campaign-agent.cjs`、MCP 的 `campaign_complete` 和 `campaign_message` 均支持这个可选字段。MCP `campaign_reply_prompt` 可查询当前选项；未提供替玩家点击选项的主持工具。

## 5. 查询当前问题与回复状态

`GET /reply-prompt`

```json
{
  "prompt": {
    "target_user_id": 2,
    "options": [
      {"id": "accept", "label": "接受", "content": "我愿意接下调查任务。"}
    ],
    "event_id": 351,
    "content": "你愿意接下调查任务吗？",
    "answered": false,
    "can_reply": true
  }
}
```

无当前问题时：`{"prompt":null}`。

`answered` 表示当前调用者是否已回复这组问题；`can_reply` 同时考虑身份、指定回复者、是否已回复、战役阶段和正在运行的回合。智能体令牌的 `can_reply` 始终为 `false`。服务端仍会在写入时重新检查这些条件，不能把查询结果当作永久授权。

选项单独查询，不依赖当前已加载的日志页。收到 `campaign.changed` WebSocket 事件后重新读取此接口即可刷新按钮。

## 6. 玩家选择快捷回复

`POST /actions`，使用**玩家登录 Cookie**：

```json
{
  "client_id": "58986bfe-8aa2-428a-a57b-0eb70a635cd6",
  "reply_to_event_id": 351,
  "option_id": "accept"
}
```

不要同时传 `content`。服务端使用已存档的选项内容生成行动，避免客户端伪造选项文字。

## 7. 玩家自由输入或修改建议

仍然使用 `POST /actions`：

```json
{
  "client_id": "819b951a-97ed-485b-97a1-329882739082",
  "reply_to_event_id": 351,
  "content": "我暂不接受，先询问能否带同伴一起调查。"
}
```

省略 `option_id`。自由回复为 1–4000 字。界面点击按钮只会填入输入框；文字未经修改时提交 `option_id`，修改后则提交自定义 `content`，最后仍需玩家点击“提交行动”。

两种提交方式的成功响应相同：

```json
{"event_id": 352, "turn_id": 48}
```

`turn_id=0` 表示手动主持或未启用自动响应；否则进入既有主持队列。快捷回复与普通行动遵循同样的排队和后果处理流程。

行动日志 `payload` 会记录：

```json
{"reply_to_event_id": 351, "option_id": "accept"}
```

自定义回复的 `option_id` 为 `""`。主持智能体通过回合上下文中的行动事件即可知道玩家回答了哪一组问题。

原有普通行动 `{"client_id":"…","content":"…"}` 不受影响；不带 `reply_to_event_id` 的行动不计为该组问题的已回答记录，也不会限制玩家后续行动。

## 8. 失效、重复与错误

- 每位玩家对同一个问题只能关联回复一次。换 `client_id` 再回复也会被拒绝；网络重试复用同一编号和内容会返回原成功结果，不重复记日志或排队。
- `target_user_id=0` 时各成员独立回复。有人回复或掷骰不会单独撤下其他人的选项；主持回合运行期间暂不可回答，下一条公开叙述／摘要替换原问题。
- 新的公开叙述（含不带选项的叙述）或摘要出现后，旧 `reply_to_event_id` 返回 `409`；客户端应刷新问题，保留草稿供玩家自行确认，不能自动改选新问题。
- 普通行动、私信和掷骰不会替换当前问题。暂停时不能回答；恢复到 `awaiting_action` 且没有运行中回合后才能回答。

| HTTP | 常见原因 |
|---|---|
| 400 | 选项数／长度／ID 不合法、指定成员不存在、未知 `option_id`、同时传选项和内容、选项没有问题 ID、把选项附在私信／摘要上 |
| 401 | 未登录、登录失效或令牌无效 |
| 403 | 非成员、非指定回复者、普通玩家发布主持叙述、智能体代替玩家回答 |
| 409 | 问题被替换、已经回答、战役暂停／主持中、版本或租约冲突、复用请求编号却改变内容 |
| 429 | 触发请求频率限制或主持队列已满 |

错误体沿用 `{"error":"可读原因"}`。失败事务不会留下半条行动或已回答标记；排队失败也会整体回滚。
