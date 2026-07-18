# stockagent 开发工作总结

> 记录本项目从选题到上线的完整过程（2026-07-18，Kimi Code CLI 协助完成）。

## 一、项目定位

**stockagent**：A 股 AI 投研多智能体 CLI —— 一条命令，4 位 AI 分析师并行工作，输出中文投资研究报告。

- 选题背景：用户技术栈 Go、目标"几周业余时间 + 数据好看（star/下载量）"
- 第一轮提案（AI 代码评审 / 周报生成器 / RAG 问答）被否，用户指定 **AI + 金融**方向
- 灵感来源：Python 版 ai-hedge-fund（数万 star），差异化 = **A 股 + Go 单二进制 + 数据免 key + 中文报告**

## 二、数据源验证（方案前提）

实测结论（决定架构选型）：

| 数据源 | 状态 | 说明 |
| --- | --- | --- |
| 东方财富 K线 `push2his` | ✅ 可用 | 免 key，前复权日线 |
| 东方财富快照 `push2` | ✅ 可用 | 免 key，PE/PB/市值等（×100 缩放整数编码） |
| 东方财富公告 `np-anotice-stock` | ✅ 可用 | 免 key，公告标题+日期 |
| Yahoo Finance / Stooq / Google News | ❌ 不可达 | 限流或网络阻断，不作为依赖 |

## 三、技术方案评估结论

逐层重新评估后的最终选型（直接依赖仅 4 个）：

| 层 | 选择 | 放弃 | 理由 |
| --- | --- | --- | --- |
| CLI 框架 | cobra | flag / Kong | 多子命令扩展，生态事实标准 |
| 配置 | yaml.v3 + 环境变量 | Viper | 仅 3 个配置项，Viper 依赖过重 |
| HTTP/JSON | 标准库 | resty / jsoniter | 数据量小，无性能诉求 |
| LLM client | 自写 ~150 行 | go-openai SDK | 只用 chat/completions 一个端点 |
| 并发 | x/sync errgroup | 手写 WaitGroup | 数据拉取与 4 agent 并行，错误聚合 |
| 技术指标 | 自研 | TA-Lib 移植库 | MA/MACD/RSI 等每个仅几十行，可单测 |
| 终端美化 | tablewriter v0.0.5 + 自写 ANSI | lipgloss | 静态报告非 TUI；A股红涨绿跌配色 |

## 四、环境搭建

- 安装 **Go 1.26.5** 至 `/usr/local`（官方 tar 包，go.dev → golang.google.cn → 阿里云镜像回退策略）
- GOPROXY 使用预设腾讯镜像 `mirrors.tencent.com/go`
- 安装 **git 2.43.7**（dnf，OpenCloudOS 9）

## 五、核心开发（约 2500 行 Go，40 个文件）

```
main.go                     入口
internal/cli/               cobra 命令（root/analyze/version）
internal/config/            YAML + env + flag 三级优先级配置
internal/eastmoney/         东财 client：SecID 推断 / K线 / 快照缩放解析 / 公告
internal/indicator/         SMA/EMA/MACD(×2)/RSI(Wilder)/年化波动率/最大回撤
internal/llm/               OpenAI 兼容 client（json_mode、429/5xx 重试一次）
internal/agent/             技术面/基本面/消息面/风控 4 分析师 + 组合经理
internal/report/            终端（红涨绿跌）/ Markdown / JSON 渲染
```

关键设计：

- **指标本地确定性计算，LLM 只做解读** —— 幻觉不污染数据
- **全链路降级容错** —— 公告失败 / 单分析师失败 / 组合经理 LLM 失败均不阻断，自动降级为本地置信度加权聚合
- **LLM 输出双保险** —— `response_format: json_object` + 解析端容错提取（截取首个 `{` 至末个 `}`）

## 六、测试与工程化

- 单元测试：指标手工核算向量（Wilder RSI 等）、解析 fixture **取自东财真实报文**、LLM httptest 重试/鉴权
- Mock 端到端测试：`internal/cli/analyze_test.go` 全链路（mock 东财 + mock LLM）无需 key
- Live 回归测试：`STOCKAGENT_LIVE=1 go test ./internal/eastmoney/ -run Live`（默认跳过）
- CI：GitHub Actions（gofmt + vet + test + build）
- 发版：goreleaser 配置（linux/darwin/windows × amd64/arm64）
- 文档：README 中英双语、stockagent.yaml.example、MIT LICENSE

## 七、开发中修复的问题

1. tablewriter v1.1.4 API 不兼容 → 降级 v0.0.5 经典 API
2. `go vet` 报 markdown.go 冗余换行 → 修复
3. go.mod go 指令定为 1.25（依赖最低要求）
4. 校验顺序：股票代码合法性检查移至 API key 检查之前
5. 开启 `SilenceErrors`，消除错误信息重复打印

## 八、发布过程

- git 身份：`deng23yu <939265649@qq.com>`（仓库级配置）
- module 路径 `stockagent` → `github.com/deng23yu/stockagent`（14 文件），README OWNER 占位符同步替换
- 提交记录：
  - `88b82bd` feat: initial implementation of stockagent CLI
  - `bad7762` chore: rename module to github.com/deng23yu/stockagent
- 通过 **HTTPS + Personal Access Token** 推送至 `github.com/deng23yu/stockagent`，临时凭证一次性使用、零落盘

## 九、当前状态与遗留事项

- ✅ 代码已上线，CI 已自动触发
- ✅ 新增同花顺数据源 (`--source ths`): K线/快照已实测通过，东财限流时可直接切换；公告仍走东财 (该子域未被限流)
- ⚠️ 东财 push2/push2his 子域在开发后期对本机 IP 限流（curl 同样 000，非代码问题），live 测试待限流解除后重跑
- 📋 待办：
  1. 吊销本次使用的 PAT（GitHub → Settings → Developer settings → Tokens）
  2. 配置 DeepSeek key 跑真实分析，用真实输出截图替换 README 演示样例
  3. 打 tag `v0.1.0` 并添加 release workflow，自动产出三平台二进制
  4. Roadmap：北交所、F10 深度基本面、watch 批量分析、新闻舆情
