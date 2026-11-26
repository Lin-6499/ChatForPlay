# 主持接入统一清单（1.2.12）

先确认你使用的是内置模型、常驻外部Bot，还是外部AI客户端中的MCP工具。
**仅创建令牌或配置MCP不会自动启动机器人，也不会自动唤醒外部AI会话。**

## 接入前检查

战役专用令牌访问：

```http
GET /api/v1/campaigns/1/capabilities
Authorization: Bearer <战役令牌>
```

返回服务器版本、`protocol:{min:1,max:1}`、支持的features、规则编号、限制及文档链接。
协议编号是主持HTTP协议，不是MCP协议版本；当前MCP stdio仍使用2024-11-05。
成员及本战役令牌可读，非成员403、匿名401，不返回API Key、令牌、lease或秘密内容。

内置功能包括：
`atomic_complete`、`stream_snapshots`、`reply_choices`、`d20_graded_v1`、
`lore`、`memory_suggestions`、`recap`、`group_actions`、`action_supplements`。
`d20-graded-v1`是属性／技能分级房规，不是D&D原版规则。

## 新版外部Bot

配置 `LINCHAT_CAMPAIGN_URL` 与 `LINCHAT_AGENT_TOKEN` 后：

```powershell
node scripts/campaign-agent.cjs --check
```

只检查访问权限、协议范围和必要能力，不领取回合、不调用模型，不需要MODEL_API_KEY。
启动正常主持仍需MODEL_BASE_URL/MODEL_ID/MODEL_API_KEY，见[启动步骤](ENTERTAINMENT.md)。
正常启动也会先执行检查，旧服务器缺少端点／协议不兼容时会明确停止，而非拿旧规则继续操作。
检查通过不代表已验证供应商额度、模型输出质量或网络长期可用性。

## 协议阅读顺序

1. [主HTTP协议与租约、幂等](CAMPAIGN_API.md)
2. [分级检定、玩家代价选择、资源结算](ABILITY_CHECKS.md)
3. [实时公开草稿与最终确认](HOST_STREAMING.md)
4. [文字资料与秘密权限](CAMPAIGN_LORE.md)
5. [本幕资料、任务进度、记忆建议](MEMORY_PROGRESS.md)
6. [行动队列、联合回合、行动补充](ACTION_FLOW.md)

主持必须使用服务器骰子；公开草稿不是已入档结果；状态变化由complete事务确认。
联合回合从context.turn_actions读取每位玩家与event_id，不将全部检定归到第一位。
记忆建议由房主审核，不能擅自改写既定事实。秘密只通过显式授权接口读取，不复制到公开输出。

## MCP

新版连接器提供 `campaign_capabilities`。开始前调用它并读取相应文档。
外部客户端应重新启动／加载新版连接器，服务端发布不会自动刷新客户端已缓存的工具清单。
Bot启动检查也不能替其他客户端升级其MCP配置或旧文档。

## 诊断

房主在“智能体接入 → 主持接入自检”查看版本、协议、内置配置是否就绪，
以及最近联系／领取／流式更新／完成／失败时间。
`GET /host-diagnostics`仅限真人房主，不含密钥、token、lease或正文。
最近联系记录有效身份的领取请求，**不是在线保证**，Bot退出后时间不会自动消失。
模型未调用流式或返回整段JSON时，流式时间可能为空；旧回合在升级前没有诊断时间也属正常。
