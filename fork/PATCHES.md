# ag/fixes 修复记录

本分支 = 官方 `Wei-Shaw/sub2api` main + 下表的修复。每个修复是一个独立提交，官方合并或修好以后，变基时把对应提交丢掉。

- 基线：`origin/main` 62cebadf5（2026-11-20）
- 远程：`origin` = 官方，`fork` = `Warren-W/sub2api`
- 不含数据库迁移，可以随时退回官方镜像 `weishaw/sub2api:<同版本>`

## 修复表

「来源」列由 `fork/check-upstream.sh` 读取：`PR #N` 查 PR 状态，`Issue #N` 查 issue 状态，`自研` 不查。「提交标题」列是提交标题里的一段原文，脚本用它核对分支上的提交是否都有记录。

| 提交标题 | 来源 | 类别 | 内容 | 与官方 PR 的差异 / 备注 |
|---|---|---|---|---|
| add Gemini 4 Neon and Hydrogen | PR #8952 | 模型 | 新增 Gemini 4 Neon、Hydrogen，裸 Gemini 4 名映射到档位名 | 原样 |
| 无重置时间的裸 429 只冷却失败的模型 | PR #8836 | 限流 | 没有重置时间的裸 429 只冷却失败的模型 | 原样 |
| fail over on per-account 404 | PR #8540 | 切号 | 单账号 404「Requested entity was not found」切号 | 原样 |
| 容量 503 重试两次 | PR #8974 | 切号 | 容量 503 重试两次后切号 | 原样 |
| 空流不再冷却共享账号 | PR #8975 | 限流 | 空流不再冷却共享账号 | 原样 |
| fail over on upstream first-byte timeout | PR #8744 | 切号 | 首字节超时切号 | 原样 |
| strip the Claude Code dual-use | PR #8829 | 假 429 | 去掉触发假 429 的 dual-use 安全提示句 | 原样 |
| keep mid-conversation system messages | PR #7534 | 缓存 | 对话中途的 system 消息留在 contents，保住前缀缓存 | 原样 |
| 请求级 403 不再逐账号停用和换号重放 | PR #8730 | 切号 | 请求级 403 不再逐个停用账号、换号重放 | 原样 |
| also strip the Claude Code pasted-content | PR #8894 | 假 429 | 去掉触发假 429 的 pasted-content 句子 | 与 #8829 冲突，两种匹配都保留 |
| upstream 计费按实际发往上游的模型计价 | PR #6516 | 计费 | `billing_model_source=upstream` 时按实际上游模型计价 | 适配：main 已重构到 `recordUsageBillingModel`，按新结构移植。只在渠道把计费模型来源设为 upstream 时生效 |
| preserve fractional quota utilization | PR #8244 | 额度显示 | 额度百分比保留小数，小用量显示 `<0.01%` | 改动：撤回它对标签宽度 class 的修改（破坏 main 的 labelWidth 测试） |
| show fractional utilization on quota group bars | 自研 | 额度显示 | 补 #8244 漏掉的额度分组条（5h / 周），之前永远显示 0% | #8244 合并后检查官方是否也覆盖了分组条 |
| 保留并还原 Gemini 思考签名 | PR #8545 | 协议 | `/v1/responses` 保留并还原 Gemini 思考签名 | 收窄：原 PR 对所有 `claude-*` 生效、`signature_delta` 无条件透传，会改变官方 Claude 行为；改为仅 `gemini-*` 和原有签名模型 |
| compare fractional quota utilization in the forward-endpoint test | 自研 | 测试 | #8244 后配额测试改用 InDelta 比较小数 | 跟随 #8244，一起丢 |
| Responses 兼容链路默认输出上限 | PR #8164 | 截断 | Codex 不传 `max_output_tokens` 时用 64000，不再截在 8192 | 只取输出上限部分，PR 其余部分未合 |
| Gemini 3.5 Flash 档位名按基础模型计费 | PR #5935 | 计费 | 3.5 Flash 档位名（-high/-low/-extra-low/-medium）原来计 $0 | 只取 PR 的计费部分并自己实现，白名单/映射部分未合；PR 合并后比对 |
| gpt-oss-120b 不再按 GPT-5.4 计费 | 自研 | 计费 | gpt-oss-120b 原来回退到 GPT-5.4 价；改为 $0.15/$0.60 静态价，远端价格表有则用远端 | 价格取自 Vertex AI MaaS 公开价 |
| 非流式聚合带上最后一个分片的 | 自研 | 计费 | 非流式请求取末块的 usage 和 finishReason，原来少计输出 token、丢截断原因 | |
| toolUsePromptTokenCount 计入输入 | 自研 | 计费 | 内置工具（搜索）结果的 token 计入输入 | |
| QUOTA_EXHAUSTED 立即切号 | Issue #8978 | 切号 | 额度耗尽立即限流并切号，不在同账号退避重试 3 次 | 只修缺陷 1；缺陷 2（裸 429 误锁积分）未修 |
| 使用记录延迟列显示缓存命中率 | 自研 | 前端 | 管理端使用记录的延迟列加一行「缓存命中率」= 缓存读取 ÷（输入 + 缓存读取 + 缓存写入），与 Token 趋势图口径一致；无用量显示 — | 官方无对应 PR；官方若加同类显示，以官方为准丢掉本提交 |

## 看过但没改的

| 项目 | 原因 |
|---|---|
| PR #8316 | 与 #8545 重叠，其重复输出防护有误伤风险 |
| PR #8240（下线旧默认模型） | 冲突多，不影响 Antigravity 使用 |
| #8851 缺陷 2：`/antigravity/v1/messages` 的 gemini 模型走旧转换链路 | 只影响用 Anthropic 协议直连 Gemini 的客户端，改动面大 |
| #8851 缺陷 3：Responses effort low/none 映射成 -high | main 已修好（实测 none/minimal/low → low） |
| 粘性会话切号后输入按缓存价计费 | 上游有意设计（72b08f9cc），切号导致的缓存失效不让用户买单 |
| Argon/Carbon 优惠价到期 | 远端价格表优先，改代码无效；到期前后留意价格表或设渠道价 |
| 输出上限按模型读取价格表 | 决定保持写死 64000 |
| Issue #8926：删掉的 Antigravity 映射仍出现在导出的 codex-models.json | `ensureAntigravityDefaultPassthroughs` 有意把默认直通模型补回映射，且同一映射用于路由；只影响导出的模型目录 |
| PR #8730 是否作用于 Antigravity | 只改 Gemini 平台（`gemini_*` 服务），对 Antigravity 无影响也无副作用 |

## 合并官方前的检查流程

1. `git fetch origin`
2. 运行 `fork/check-upstream.sh`，它会输出：
   - 每个 `PR #N` 是否已被官方合并，每个 `Issue #N` 是否已关闭；
   - 分支上有没有提交不在表里（表要补）；
   - 官方新提交里，改过本分支同一批文件的有哪些（可能已修好或会冲突）。
3. 逐条处理：
   - PR 已合并：变基时丢掉对应提交。标了「适配 / 改动 / 收窄」的，先比对官方合并的版本是否覆盖我们的改动，没覆盖的部分改成自研提交。
   - Issue 已关闭、或官方新提交改了同一处：看官方的修法，等价就丢掉我们的提交。
   - 自研：看第 2 步列出的官方提交里有没有修同一问题的。
4. 变基：`git rebase -i origin/main`，丢掉已被官方覆盖的提交，处理冲突。
5. 跑测试（后端 `go test -tags unit ./...`，前端 vitest），更新本表：基线提交、删掉已丢弃的行，被官方修掉的写进下面的「已回归官方」。

## 已回归官方

（暂无）
