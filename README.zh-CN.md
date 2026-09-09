[English](./README.md) | **简体中文**

<div align="center">

<img src="https://readme-typing-svg.demolab.com/?font=Fira+Code&weight=600&size=20&pause=1500&color=318347&center=true&vCenter=true&random=false&width=760&lines=lastword+verdict+session.jsonl;lead%3A++the+final+answer%2C+pinned;changes%3A++7+files%C2%A0%C2%A0%2B43+%E2%88%9215;results%3A++go+test+./...+ok;pointers%3A++%238+%23500+--+proof%2C+one+turn+away" alt="lastword verdict session.jsonl — lead: the final answer, pinned; changes: 7 files +43 −15; results: go test ./... ok; pointers: #8 #500 — proof, one turn away">

# LastWord

**把 Coding Agent 的最终答案钉在会话记录之上的那一层。**

数小时的 Claude Code 会话会把真正的答案埋进几百轮叙述里。LastWord 读取跑完的会话记录，把结论钉住——最终决定、改了什么、以及指向产生它们的确切轮次的可机校验指针——让一场 500 轮的运行以一屏可见的答案收尾，而不是再来一次滚动。

[![CI](https://github.com/SuperMarioYL/lastword/actions/workflows/ci.yml/badge.svg)](https://github.com/SuperMarioYL/lastword/actions/workflows/ci.yml)
[![Version](https://img.shields.io/badge/version-0.1.0-318347)](https://github.com/SuperMarioYL/lastword/releases)
[![Go](https://img.shields.io/badge/go-1.24%2B-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-318347)](./LICENSE)
![Local](https://img.shields.io/badge/local_only-no_telemetry-318347)

<picture>
  <source media="(max-width: 640px) and (prefers-color-scheme: dark)" srcset="assets/presentation/hero-mobile-dark.svg">
  <source media="(max-width: 640px)" srcset="assets/presentation/hero-mobile-light.svg">
  <source media="(prefers-color-scheme: dark)" srcset="assets/presentation/hero-dark.svg">
  <img src="assets/presentation/hero-light.svg" width="960" alt="LastWord — 散落在 500 轮会话里的论断，汇聚成一个钉住的结论点">
</picture>

</div>

---

## <img src="assets/icons/rocket.svg" width="22" alt=""> 快速开始

需要 Go 1.24+。全部本地运行——没有 API key、没有模型调用、没有网络。

```bash
git clone https://github.com/SuperMarioYL/lastword.git
cd lastword
go build -o /tmp/lastword ./cmd/lastword
```

先在内置的 500 轮会话上试一次（一份按真实 Claude Code 会话形状生成的脱敏合成示例）：

```bash
/tmp/lastword verdict testdata/sessions/session-500turn.jsonl --format text
```

然后在你自己的机器上跑——Claude Code 早已替你把会话写好了：

```bash
/tmp/lastword latest    # ~/.claude/projects 下最新的主会话
/tmp/lastword verdict   # 它的 verdict span，自动发现
```

v0.1 的完整闭环就这些：一条命令进，一份结论出。上面三行同样放在 [examples/quickstart.sh](examples/quickstart.sh) 里。

## <img src="assets/icons/video.svg" width="22" alt=""> 运行记录

以下命令与输出逐字复制自 `0.1.0` 的真实运行——完整记录在 [docs/demo-results.json](docs/demo-results.json)。被测会话是内置的脱敏示例（`testdata/sessions/session-500turn.jsonl`，500 个计数轮次），布置成一个演示 projects 目录；在你自己的机器上，同样的命令会以完全相同的代码路径读取 `~/.claude/projects` 里你最新的会话。

![Before: 503 lines of raw session JSONL. After: lastword verdict pins the answer, changes and command outcomes with pointers to the turns that produced them — the click-to-jump viewer lands with milestone m3.](docs/demo-500turn.gif)

`lastword verdict --format text` 打出的结论面板：

```text
LastWord — verdict-span/v0
session 7f3d2c8a-1e94-4b67-9c02-5a8d3f6b1e47

ANSWER
  The login flake was a stale refresh-token cache key: rotating a session
  changed the in-memory key but never invalidated the old entry, so roughly
  one in five refreshes read a token the issuer had already rejected.

CHANGES (7)
  src/auth/token.go               +6   -6
  src/auth/session.go             +5   -3
  internal/api/middleware.go      +8   -5
  tests/refresh_rotation_test.go  +13  -0
  internal/api/server.go          +1   -1
  docs/auth.md                    +5   -0
  README.md                       +5   -0

RESULTS (6)
  [ok] go test ./...
  [ok] gofmt -w .
  [ok] go vet ./...
  [FAIL (exit 2)] npm --prefix web run build
  [ok] git log --oneline -3 -- web/
  [ok] go test ./... -run Auth

POINTERS (14)
  #500  final   answer.lead
  #8    edit    answer.changes[src/auth/token.go]
  #8    edit    answer.changes[src/auth/session.go]
  #11   edit    answer.changes[internal/api/middleware.go]
  #13   edit    answer.changes[tests/refresh_rotation_test.go]
  #37   edit    answer.changes[internal/api/server.go]
  #40   edit    answer.changes[docs/auth.md]
  #40   edit    answer.changes[README.md]
  #16   result  answer.results[go test ./...]
  #39   result  answer.results[gofmt -w .]
  #44   result  answer.results[go vet ./...]
  #45   result  answer.results[npm --prefix web run build]
  #47   result  answer.results[git log --oneline -3 -- web/]
  #48   result  answer.results[go test ./... -run Auth]

checks: not evaluated (verify arrives with milestone m2)
```

注意原始会话记录里看不出来的细节：`go test ./...` 在会话早期失败过一次，修复后才通过——span 只主张**最后一次**结果，即第 16 轮那次通过。`npm` 构建失败则带着它打印出的退出码被如实主张。同一份结论，以公开的 JSON 文档输出（此处为开头部分；完整文档见运行记录）：

```json
{
  "schema": "verdict-span/v0",
  "session": "7f3d2c8a-1e94-4b67-9c02-5a8d3f6b1e47",
  "answer": {
    "lead": "The login flake was a stale refresh-token cache key: rotating a session changed the in-memory key but never invalidated the old entry, so roughly one in five refreshes read a token the issuer had already rejected.",
    "changes": [
      {
        "file": "src/auth/token.go",
        "added": 6,
        "removed": 6
      },
      {
        "file": "src/auth/session.go",
        "added": 5,
        "removed": 3
      },
      {
        "file": "internal/api/middleware.go",
        "added": 8,
        "removed": 5
      },
      {
        "file": "tests/refresh_rotation_test.go",
        "added": 13,
        "removed": 0
      }
      /* ……其余 3 个文件、6 条命令结果、14 个轮次指针，
         以及 "checks": null——完整文档见 docs/demo-results.json */
    ]
  }
}
```

## <img src="assets/icons/bulb.svg" width="22" alt=""> 为什么需要 LastWord

四十五分钟的 Claude Code 运行结束，你回到终端。500 轮工具调用、重试与叙述里，藏着你真正要的东西：它决定了什么、改了什么、测试过没过。最后一条消息往往仍是叙述，于是你只能滚屏——而滚屏搜索找到的是*提及*，不是*结论*。

当前生态的解法都差着一层：

- **行为技能**（[i-have-ADHD](https://github.com/ayghri/i-have-adhd)、[caveman](https://github.com/JuliusBrussee/caveman)）*要求* agent 把答案放最前面、说话精简。提示词是请求，不是契约：任何一次运行仍可能把答案埋掉；没有任何机制校验一场跑完的会话；没装技能的那场运行更是无人能读。
- **会话渲染器**（[claude-code-log](https://github.com/daaain/claude-code-log)）让滚屏更好受。但在格式层面，答案与叙述无法区分；它们的会话摘要是生成出来的复述——不解析到具体轮次，也无法校验。

LastWord 用推导出的契约替代请求。抽取是确定性的——绝不调用模型——两个新元素让它成为原语而非又一份摘要：

- **指针**：答案里的每个论断都带一个可解析、带逐字引用的引用，指向产生它的那一轮。`answer.changes[src/auth/token.go]` 不是感觉，是第 8 轮，附原文。
- **校验语义**：规范定义了 span 何为有效——每个指针必须解析到真实轮次，主张的 diff 必须与会话里的实际编辑事件一致。`lastword verify`（里程碑 m2）将带着通过/失败退出码强制执行这一切。

## <img src="assets/icons/schema.svg" width="22" alt=""> 工作原理

一个 Go 二进制、一个进程，没有守护进程、没有服务器、没有网络调用。

```
~/.claude/projects/<project-dir>/<session-uuid>.jsonl
   （主会话，仅一层深；嵌套的子代理记录被排除）
      |
      v
 transcript/claude.go    JSONL -> 轮次 {uuid, role, text, toolUses, toolResults}
                        （sidechain 条目与簿记类型被跳过）
 transcript/discover.go  最新主会话查找
      |
      v
 verdictspan/extract.go  确定性规则 -> VerdictSpan
        answer.lead = 最后一条助手消息的首段
        changes     = 聚合的 Edit/Write 工具事件
        results     = 每条命令的最后结果：ok 镜像 is_error，
                      退出码仅在出现 "Exit code N" 文本时解析
 verdictspan/schema.go  verdict-span/v0 类型 + 公开格式参考
      |
      v
 render/terminal.go     `lastword verdict` 标准输出——规范 JSON，
                        或 --format text 的可读面板
```

<picture>
  <source media="(max-width: 640px) and (prefers-color-scheme: dark)" srcset="assets/presentation/architecture-mobile-dark.svg">
  <source media="(max-width: 640px)" srcset="assets/presentation/architecture-mobile-light.svg">
  <source media="(prefers-color-scheme: dark)" srcset="assets/presentation/architecture-dark.svg">
  <img src="assets/presentation/architecture-light.svg" width="960" alt="架构蓝图：会话 JSONL 经 transcript 解析、verdictspan 抽取、render 输出">
</picture>

每份 span 背后的抽取流水线：

<picture>
  <source media="(max-width: 640px) and (prefers-color-scheme: dark)" srcset="assets/presentation/process-mobile-dark.svg">
  <source media="(max-width: 640px)" srcset="assets/presentation/process-mobile-light.svg">
  <source media="(prefers-color-scheme: dark)" srcset="assets/presentation/process-dark.svg">
  <img src="assets/presentation/process-light.svg" width="960" alt="抽取循环：发现最新会话、解析轮次、推导答案与改动、为每个论断锚定指针">
</picture>

## <img src="assets/icons/file-text.svg" width="22" alt=""> verdict-span 格式

span 是一份公开的开放格式——[docs/verdict-span-v0.md](docs/verdict-span-v0.md)——其他工具可以同样地生成与消费它。格式本身与具体 agent 无关；本 CLI 的 v0.1 只读取 Claude Code 会话 JSONL。

```json
{
  "schema": "verdict-span/v0",
  "session": "<session uuid>",
  "answer": {
    "lead": "leading paragraph of the final assistant turn",
    "changes": [{"file": "src/auth.go", "added": 12, "removed": 4}],
    "results": [{"kind": "command", "cmd": "go test ./...", "ok": true, "exit_code": null}]
  },
  "pointers": [
    {"turn": 412, "uuid": "<message uuid>", "kind": "final | edit | result",
     "snippet": "verbatim 1-2 lines from that turn",
     "provenance": "which claim this turn backs"}
  ],
  "checks": [
    {"name": "pointer_resolution", "pass": true},
    {"name": "answer_diff", "pass": true, "detail": "3 files claimed, 3 edited, 0 unclaimed"}
  ]
}
```

结果条目是推导出来的，不是读出来的：Claude Code 的 tool_result 块没有结构化退出码，因此 `cmd` 来自按 `tool_use_id` 配对的 tool_use，`ok` 镜像 `is_error`，`exit_code` 只在失败输出 `Exit code N` 文本时解析。规范还固定了全部确定性规则——什么算一轮、行数怎么数、重复命令主张哪次结果——让独立实现产出同一份 span。

## <img src="assets/icons/terminal-2.svg" width="22" alt=""> 命令行

| 命令 | 作用 |
| --- | --- |
| `lastword verdict [file]` | 抽取并打印 verdict span；不带文件时自动发现最新主会话 |
| `lastword latest` | 打印最新主会话记录的路径 |

| 选项 | 默认值 | 说明 |
| --- | --- | --- |
| `--format` | `json` | `json`——规范 verdict-span/v0 文档；`text`——可读面板 |
| `--projects-dir` | `~/.claude/projects` | Claude Code projects 根目录，供发现使用 |

主会话是项目目录一层的 `*.jsonl` 文件；更深层的（子代理记录）一律排除。自动发现时，解析出的路径打印到 stderr，stdout 保持机器可读。

## <img src="assets/icons/chart-bar.svg" width="22" alt=""> 能力与集成

<picture>
  <source media="(max-width: 640px) and (prefers-color-scheme: dark)" srcset="assets/presentation/integrations-mobile-dark.svg">
  <source media="(max-width: 640px)" srcset="assets/presentation/integrations-mobile-light.svg">
  <source media="(prefers-color-scheme: dark)" srcset="assets/presentation/integrations-dark.svg">
  <img src="assets/presentation/integrations-light.svg" width="960" alt="输入与输出：Claude Code 会话 JSONL、Edit/Write 事件、Bash 结果进；verdict-span/v0 JSON 出">
</picture>

| 组件 | 当前支持 |
| --- | --- |
| 输入 | 跑完的 Claude Code 会话 JSONL（`~/.claude/projects` 下的文件），事后读取 |
| 改动主张 | `Edit` 与 `Write` 工具事件，逐文件聚合 +/− 行数 |
| 结果主张 | 按 `tool_use_id` 配对的 `Bash` 调用与结果；每条不同命令的最后结果 |
| 指针 | 轮次号 + 消息 uuid + 逐字引用，覆盖答案、每个文件、每条命令 |
| 输出 | `verdict-span/v0` JSON（默认）与终端面板（`--format text`） |
| 示例 | `testdata/sessions/` 下的脱敏合成会话，含一场 500 轮运行 |

## <img src="assets/icons/settings.svg" width="22" alt=""> 配置与边界

- **v0.1 只读取跑完的 Claude Code 会话。** 不支持 Codex 与 Antigravity 会话，不做会话中的实时钩子，不做跨会话搜索或索引。
- **校验已定义、尚未执行。** 规范中的指针解析与答案 diff 语义随 `docs/verdict-span-v0.md` 发布；带通过/失败退出码的 `lastword verify` 是里程碑 m2，因此 v0.1 输出 `"checks": null`。
- **还没有查看器。** 带点击跳转溯源的钉住式 HTML 面板是里程碑 m3；v0.1 输出 JSON 与终端面板。
- **设计上就是确定性的。** 没有 LLM 生成的总结、没有解读——span 只主张会话中客观存在的内容。`Write` 事件只计新增行、不主张删除行（旧内容不在会话记录里）。
- **没有配置文件、没有遥测、没有模型调用。** 只有命令行参数；一切都在本地。

## <img src="assets/icons/route.svg" width="22" alt=""> 路线图

- [x] **v0.1（当前，m1）**——轮次解析、最新会话发现、确定性 verdict-span 抽取（答案、改动、结果、指针）、JSON + 终端输出、公开格式规范、500 轮示例
- [ ] **m2**——`lastword verify`：指针解析与答案 diff 机器校验，带通过/失败退出码，并在十个真实会话上测量
- [ ] **m3**——`lastword view`：自包含 HTML 查看器，结论钉住、点击论断跳转到产生它的轮次，以及前后对比的演示录制

v0.1 不做、也不急着做：实时运行强制、Codex/Antigravity 会话支持、LLM 摘要、暴露 span 的 MCP 服务、IDE 扩展、遥测。格式与 CLI 先行；地基（开放的 JSONL 会话记录）早已存在。

## <img src="assets/icons/license.svg" width="22" alt=""> 开发与许可

```bash
go build ./... && go vet ./...
go test ./...
```

测试套件覆盖 JSONL 解析（块形状、sidechain 跳过、工具配对）、发现逻辑、每条抽取规则与全部示例。演示图稿用 `python3 docs/render_presentation.py --repo .` 重新生成。

[MIT](./LICENSE) © 2026 SuperMarioYL

---

<p align="center"><sub><a href="./LICENSE">MIT</a> © 2026 SuperMarioYL</sub></p>
