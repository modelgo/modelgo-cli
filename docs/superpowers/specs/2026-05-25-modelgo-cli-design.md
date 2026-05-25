# modelgo-cli v0 框架设计

**Date**: 2026-05-25
**Status**: Approved (brainstorming)
**Scope**: 仅 v0 框架；不含任何业务接口

## 1. 目标与非目标

### 目标

搭建 `modelgo-cli` 的最小可用框架，让外部用户（modelgo 的客户）能用一条命令完成安装：

```
npx @modelgo/cli@latest install
```

安装完毕后用户机器上具备：

1. 全局可执行的 `modelgo-cli` 二进制
2. 一组 `SKILL.md` 文件，已落到本机所有支持的 AI agent（Claude Code、Codex、Gemini CLI 等）各自的 skills 目录
3. 一个可立刻演示的端到端回路：让 AI 说一句话 → AI 自动调用 `modelgo-cli hello`

### 非目标（v0 显式不做）

- 任何 modelgo 业务接口的封装（API key 管理、用量查询、模型路由等）
- 用户认证（OAuth / API key / SSO）
- `config init` / `auth login` 子命令
- MCP server 模式
- 国际化（除安装向导支持 zh/en 文案外）

## 2. 总体方案

**Approach: 单仓库 monorepo（lark-cli 同构）**。`github.com/modelgo/modelgo-cli` 一个仓库承载：Go 二进制源码、npm wrapper 脚本、bundled skills、Release 流水线。npm 包名 `@modelgo/cli` 公开发布到 registry.npmjs.org。Go 二进制公开发布到 GitHub Releases。Skills 直接以源文件形式存在仓库 `skills/` 目录，由用户机器上的 `npx skills add modelgo/modelgo-cli` 同步到各 agent 的 skills 目录。

替代方案及为何拒绝：

- **三仓库拆分**（`modelgo-cli` / `modelgo-cli-npm` / `modelgo-skills`）：v0 没有独立维护团队，多仓库带来的版本对齐和跨仓 PR 开销大于其灵活性收益。
- **npm `optionalDependencies` 全托管**（无 GitHub Releases）：能省 GoReleaser，但失去 lark-cli 的成熟参照、且 skills 仍需 GitHub 公网可读源，反而更绕。

## 3. 命名与许可

| 项 | 值 |
|---|---|
| GitHub 仓库 | `github.com/modelgo/modelgo-cli`（已存在，公开） |
| Go module | `github.com/modelgo/modelgo-cli` |
| npm 包 | `@modelgo/cli`（scoped，对标 `@larksuite/cli`） |
| 二进制名 | `modelgo-cli` |
| 安装入口 | `npx @modelgo/cli@latest install` |
| License | MIT |

GitLab 内部仓 `gitlab.ops.modelgo.com/backend/modelgo-cli` 与 GitHub 仓的关系：v0 阶段以 GitHub 为单一 source of truth，GitLab 仓暂留作历史 remote，后续团队可决定是否做双向同步。

## 4. 仓库布局

```
modelgo-cli/
├── cmd/modelgo-cli/main.go            # Go 二进制入口
├── internal/
│   ├── version/version.go             # -ldflags 注入版本号
│   └── hello/hello.go                 # v0 唯一 demo 业务
├── scripts/
│   ├── run.js                         # npm bin 入口 → exec 真正的二进制
│   ├── install.js                     # postinstall：下载 + SHA-256 校验
│   ├── install-wizard.js              # `install` 子命令的交互向导
│   └── __tests__/                     # node:test 单测
├── skills/
│   ├── modelgo-shared/SKILL.md        # 占位 setup/排错 skill
│   └── modelgo-hello/SKILL.md         # demo skill
├── bin/                               # 运行时目录，二进制下载到这里（.gitignore）
├── checksums.txt                      # release CI 自动生成并 commit 回主分支
├── package.json
├── .goreleaser.yaml                   # GoReleaser：6 平台矩阵
├── .github/workflows/
│   ├── release.yml                    # tag → GoReleaser → npm publish
│   └── ci.yml                         # PR：go test + lint
├── LICENSE                            # MIT
└── README.md
```

平台矩阵：`darwin-amd64`、`darwin-arm64`、`linux-amd64`、`linux-arm64`、`windows-amd64`、`windows-arm64`。

## 5. 组件职责

### 5.1 Go 二进制（`cmd/modelgo-cli`）

v0 子命令：

| 命令 | 行为 |
|---|---|
| `modelgo-cli --version` | 打印通过 `-ldflags "-X .../version.Version=v0.1.0"` 注入的版本号 |
| `modelgo-cli --help` | 顶层帮助 |
| `modelgo-cli hello [--name NAME]` | 打印 `Hello, <name>!`（默认 `world`） |

Go 端**不**实现 `install` / `config` / `auth`——这些属于 npm wrapper 层。

### 5.2 npm wrapper（`scripts/`）

| 文件 | 触发时机 | 职责 |
|---|---|---|
| `run.js` | `package.json` bin 指向，用户执行 `modelgo-cli ...` | 定位 `bin/modelgo-cli` → `execFileSync` 透传 argv。第一个参数是 `install` 时直接 `require("./install-wizard.js")`，不走二进制。 |
| `install.js` | `package.json` postinstall 钩子 | 平台检测 → 下载 `https://github.com/modelgo/modelgo-cli/releases/download/v<ver>/modelgo-cli-<ver>-<os>-<arch>.<ext>` → SHA-256 校验（对 `checksums.txt`）→ 解压到 `bin/`。失败时 fallback 到 `registry.npmmirror.com`。`npm_command=exec`（npx 临时执行）时 `exit(0)` 跳过下载。 |
| `install-wizard.js` | `npx @modelgo/cli@latest install` 或 `modelgo-cli install` | 交互向导，详见 §6.1 |

v0 向导只做两个实际安装动作（npm 全局装 + skills 安装）。lark-cli 后续的「`config init` / `auth login`」步骤在 v0 不存在；待业务接入再补。

### 5.3 Skills

每个 skill 一个目录，结构与 lark 系列 skills 一致：

**`skills/modelgo-shared/SKILL.md`**（meta skill，按 `lark-shared` 模板）：YAML frontmatter 描述「使用 modelgo-cli 遇到 setup/排错/升级时查这个」。v0 正文先写「modelgo-cli 还没接入业务接口；要升级到最新版执行 `npx @modelgo/cli@latest install`」。

**`skills/modelgo-hello/SKILL.md`**（demo skill）：YAML frontmatter `description` 用强触发词「打招呼 / 测试 modelgo-cli / hello world」。正文教 AI：「调用 `modelgo-cli hello` 即可；可传 `--name <name>` 自定义」。

### 5.4 Release 流水线

- `.goreleaser.yaml`：6 平台目标、归档名 `modelgo-cli-{{.Version}}-{{.Os}}-{{.Arch}}.{tar.gz|zip}`、生成 `checksums.txt`、上传 GitHub Release。
- `.github/workflows/release.yml`：监听 `v*` tag → GoReleaser → 把 `checksums.txt` commit 回主分支 → `npm publish --access public`。所需 secrets：`NPM_TOKEN`（npm automation token，限定 publish 权限）、`GITHUB_TOKEN`（默认提供）。
- Skills 文件就在主仓 `skills/`，无需独立发版动作。

## 6. 数据流

### 6.1 安装流（`npx @modelgo/cli@latest install`）

```
[npx]  拉 @modelgo/cli 到临时 cache → postinstall = install.js
  └─ install.js 检测 npm_command=exec → exit 0 跳过下载
[npx]  执行 package.json bin → run.js install
  └─ run.js 看到 argv[2]==="install" → require("./install-wizard.js")

[install-wizard]
  preamble: 交互模式下选语言（zh/en），非交互默认 en
  step1: npm install -g @modelgo/cli
         └─ 这次 postinstall 真跑：install.js 从 GitHub Releases 下二进制
             └─ 写 npm_global_prefix/.../bin/modelgo-cli
  step2: npx -y skills add modelgo/modelgo-cli -y -g
         └─ skills CLI 扫描本机已装 agent（claude-code/codex/gemini/...）
         └─ 把 skills/modelgo-* 软链到各 agent 的 skills 目录：
              ~/.claude/skills/modelgo-shared/
              ~/.codex/skills/modelgo-shared/
              ...
  outro: 提示「跟 AI 说『用 modelgo-cli 跟我打个招呼』」
```

安装产物：
1. `modelgo-cli` 二进制在 npm global prefix `bin/`，已加入 PATH。
2. `SKILL.md` 文件在每个 agent 的 skills 目录。
3. **不**修改 `settings.json`，**不**装 plugin，**不**起任何后台进程。下次开新会话 agent 自动扫到。

### 6.2 运行流

```
用户："让 modelgo-cli 跟我打个招呼"
  ↓
Claude Code 会话启动时已加载所有 ~/.claude/skills/*/SKILL.md frontmatter
（modelgo-hello 的 description 出现在 system prompt 里）
  ↓
模型匹配 modelgo-hello → Skill 工具加载完整正文
  ↓
模型用 Bash 工具：modelgo-cli hello --name 渭哲
  ↓
modelgo-cli 输出 "Hello, 渭哲!" → 模型转述
```

整条链路是纯文件契约：无 MCP、无 plugin、无自定义协议。同一份 `skills/` 目录因此能跨 50+ agent 工具复用。

## 7. 错误处理

### 7.1 二进制下载（`install.js`）

| 错误 | 处理 |
|---|---|
| 平台不在白名单 | `exit(1)` 并打印支持矩阵 |
| 主源 GitHub 网络失败 | 自动重试 `registry.npmmirror.com/-/binary/modelgo-cli/v<ver>/...` |
| 所有源均失败 | `exit(1)`，提示设置 `https_proxy` 或自定义 `--registry` |
| SHA-256 不匹配 | 抛 `[SECURITY] Checksum mismatch`，**不**降级；临时归档目录在 `finally` 强删 |
| 主机白名单 | 仅放行 `github.com` / `objects.githubusercontent.com` / `registry.npmmirror.com` + 用户配置的 `npm_config_registry`。`curl --max-redirs 3` 防链式重定向到非白名单 |

### 7.2 向导步骤（`install-wizard.js`）

每个 step 失败只让该 step 报错，并打印手工重试命令：

| 失败 step | 提示 |
|---|---|
| step1 全局安装失败 | `npm install -g @modelgo/cli` |
| step2 skills 装失败 | `npx skills add modelgo/modelgo-cli -y -g` |

step2 先跑 `npx -y skills ls -g` 解析输出，发现已有 `modelgo-` 开头的 skill 就跳过，避免重复安装抖动（`skillsAlreadyInstalled()` 同 lark-cli）。

### 7.3 非交互（CI、容器、非 TTY）

向导检测 `process.stdin.isTTY === false` 时跳过所有交互确认；v0 没有 config/auth 步骤，所以非交互模式下与交互模式行为基本一致，仅省去语言选择（默认 en）。

### 7.4 运行时错误（`run.js`）

| 错误 | 处理 |
|---|---|
| `bin/modelgo-cli` 不存在 | 提示「请重新安装：`npm install -g @modelgo/cli`」 |
| 二进制非零退出 | 透传退出码与 stderr |

### 7.5 安全边界

- npm wrapper 脚本只用 `execFileSync`，不用 `execSync`，避免 shell 注入。
- 所有 URL 拼接走 `URL()` 构造器解析。
- `checksums.txt` 由 GoReleaser 在 release CI 内生成并与二进制同 release 产出；不允许人工编辑。

### 7.6 显式不处理

- 用户禁用 `curl` 的环境（lark-cli 也假设 curl 存在）。
- npm prefix 不可写——抛 npm 原生权限错误。
- `npx skills` 装出的 skill 文件被第三方污染——上游包责任。

## 8. 测试策略

### 8.1 Go 单元测试（`.github/workflows/ci.yml`）

| 范围 | 关键用例 |
|---|---|
| `internal/version` | 默认未注入时 `"dev"`；`-ldflags` 注入后能读出 |
| `internal/hello` | 默认 `world`；`--name 渭哲` 输出含中文 |

CI 跑 `go test ./... && go vet ./...`。

### 8.2 npm wrapper 单测（`scripts/__tests__/`，Node `node:test`）

**纯函数**（无网络/FS）：
- `resolveMirrorUrls(env, archive, ver)`：默认链条、`npm_config_registry` 注入、http 协议拒绝、`registry.npmjs.org` 跳过
- `isValidDownloadBase`、`isDefaultNpmjsRegistry`、`assertAllowedHost`
- `getExpectedChecksum(archiveName, dir)`：fixture `checksums.txt` 命中/未命中
- `verifyChecksum(path, hash)`：tmp 文件一致/不一致
- `semverLessThan`：边界

`download()` 和 `install()` 不写单测——几行 `execFileSync("curl")` + `fs.copyFileSync`，跑通即可，靠 release smoke test 兜底。

### 8.3 Skills lint（`scripts/lint-skills.mjs`）

CI 跑约 30 行脚本扫 `skills/*/SKILL.md`：
- frontmatter 是合法 YAML
- 必需字段：`name`、`description`、`version`
- `name` 与目录名一致
- `description` 非空、单行

### 8.4 Release smoke test（手动 checklist，不入 CI）

`release.yml` 跑完后由 release 负责人在干净环境跑：

```bash
docker run --rm -it node:20 bash
npx @modelgo/cli@latest install
which modelgo-cli && modelgo-cli --version
modelgo-cli hello
ls ~/.claude/skills/modelgo-*   # 在装了 claude code 的 dev 机上验
```

v0 不写自动 E2E：`npx skills` 行为依赖外部网络和 50+ agent 适配层，自动化收益低。

### 8.5 显式不测

- 跨平台二进制下载——靠 GoReleaser 自身 matrix 兜底。
- `npx skills` 内部行为——上游责任。
- 各家 AI agent 是否能识别新 skill——agent 自己协议，不在可控范围（lark-cli 也不测）。

## 9. 前置阻塞项（开工前必须解决）

- **npm scope `@modelgo` 所有权**：`npm view @modelgo/cli` 当前 404。需要在 npmjs.com 创建 `modelgo` 组织（或确认已存在），并把 release CI 用的 npm automation token 加入仓库 secrets。如组织无法在公共 npm 注册（已被他人占用），改用无 scope 包名 `modelgo-cli`，本设计其余部分不变（仅替换包名）。
- **GitHub repo 写权限 + Actions 启用**：`github.com/modelgo/modelgo-cli` 当前空仓库，需确认 release CI 用的账号有 push tag 权限和 Release 创建权限。

## 10. 未来工作（v1+，不在 v0 范围）

- 接入 modelgo 业务接口：API key 管理、用量查询、模型路由配置等。
- 认证体系：OAuth 浏览器流程 + 本地 token cache（参考 lark-cli 的 `auth login`）。
- `config init` 引导用户创建第一个 app 配置。
- 国际化扩展（除安装向导外，二进制输出也按语言切换）。
- MCP server 模式（供不支持 skills 的 agent 使用）。
- 内部 GitLab 仓与 GitHub 仓的同步策略（如有需要）。
