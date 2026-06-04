# modelgo-cli — Agent 指南

> 面向 modelgo 外部用户的 AI agent 友好型 CLI：一行 `npx @model-go/cli@latest install` 装齐 Go 二进制 + 跨多个 AI agent 的 skill bundle，体验对标 lark-cli。
> 完整 CICD / 内测设计见飞书文档《Modelgo-cli CICD流程》。

## 速览

| 项 | 值 |
|-|-|
| GitHub | [github.com/modelgo/modelgo-cli](https://github.com/modelgo/modelgo-cli)（public） |
| npm 包 | `@model-go/cli`（scope `model-go`，因 `modelgo` 已被占用） |
| CLI 命令 | `modelgo` |
| 安装 | `npx @model-go/cli@latest install` |

## 开发纪律

- **v0 阶段直接 commit 到 `main`**：单人维护、无并发开发，不开 feature 分支也不用 worktree。团队扩张后重新评估。
- 流程：brainstorming → spec → plan → subagent-driven implementation → final review。
- 测试：`internal/*` 走 TDD；npm wrapper 用 `node:test`；SKILL.md 用 `npm run lint:skills` 校验。

## 内测与渐进式发布流程（三段式）

任何代码 / skill 改动都按 **本地自测 → rc 内测 → 正式发布** 推进，避免把未验证的版本直接推给真实用户。

| 阶段 | 谁参与 | 分发渠道 | 命令 |
|-|-|-|-|
| ① 本地自测 | 开发者本人 | 本地二进制 + 本机 `npm pack` tarball + 本地 skills | `make build` / `npm install -g ./model-go-cli-x.y.z.tgz` / `npx -y skills add . -y -g` |
| ② RC 内测 | QA / 早期用户 | npm `rc` dist-tag（公开但 opt-in） | `npx @model-go/cli@rc install` |
| ③ 正式发布 | 所有用户 | npm `latest` dist-tag | `npx @model-go/cli@latest install` |

**核心路由规则**：tag 名决定频道，由 `release.yml` 自动判断——
- 带 `-` 后缀（如 `v0.1.2-rc.1`）→ 发到 `rc` dist-tag，**不动 `latest`**，公开用户无感知。
- 不带 `-` 后缀（如 `v0.1.2`）→ `npm publish --access public`，默认走 `latest`。

### ① 本地自测（不动 npm registry，无需 token）

本地自测分三层，目的不同，不要混成一步：

1. **验证本地 Go 改动**

```bash
cd ~/code/modelgo/modelgo-cli
make build
./bin/modelgo --version
./bin/modelgo hello
# 有鉴权改动时继续跑：
# ./bin/modelgo auth login
```

2. **验证 npm wrapper / postinstall 安装链路**

```bash
npm pack                                # 产出 model-go-cli-0.1.x.tgz
npm install -g ./model-go-cli-0.1.x.tgz
modelgo --version
```

注意：
- 这一步**不会**使用工作区里的 `./bin/modelgo`。
- `postinstall` 会按 `package.json` 的版本号下载远端 release binary，所以验证的是 npm tarball + `scripts/install.js`，**不是**本地 Go 改动。
- 这一步也**不会**安装 `skills/`，因为 npm tarball 的 `files` 白名单不包含 `skills/`。

3. **验证本地 skills**

```bash
npx -y skills add . -y -g
# 在 AI agent 新会话里验证：modelgo-* skill 出现、AI 能主动调用 `modelgo hello`
```

恢复公开版本：

```bash
npm uninstall -g @model-go/cli
npx @model-go/cli@latest install
```

### ② 发 rc 给 QA 内测

维护者发版（任何带 `-` 后缀的版本号都走 rc）：
```bash
make release VERSION=0.1.2-rc.1
npm view @model-go/cli dist-tags        # 例：{ latest: '0.1.1', rc: '0.1.2-rc.1' }
```
QA 安装并验证（主动 opt-in rc 频道）：
```bash
npx @model-go/cli@rc install
modelgo --version                       # 应为 rc 版而非 latest
# 在 AI agent 新会话里验证：modelgo-* skill 出现、AI 能主动调用 modelgo hello
```
迭代：修代码 → `make release VERSION=0.1.2-rc.2` → QA `npx @model-go/cli@rc install`（wizard 自动升级 rc.1→rc.2）。
QA 反馈建议用 GitHub issue label `rc-feedback` 收集。

### ③ 正式发布到 latest

```bash
make release VERSION=0.1.2              # 不带 - 后缀 → 自动走 latest
```
QA 从 rc 切回 stable：
- **Case A（rc 顺利转 stable）**：直接 `npx @model-go/cli@latest install`。wizard 判定 `0.1.2-rc.2 < 0.1.2`（pre-release < release）自动升级，无需 uninstall。
- **Case B（rc 被砍 / 想装更旧的 stable）**：wizard 不允许自动降级，必须先 `npm uninstall -g @model-go/cli` 再 `npx @model-go/cli@latest install`。
- **保险做法**：每次正式发布后统一清理重装回干净 stable：
  ```bash
  npm uninstall -g @model-go/cli && npx @model-go/cli@latest install && modelgo --version
  ```

### 时间线示例（v0.1.1 → v0.1.2）

| 时间 | 维护者 | QA / 用户 |
|-|-|-|
| D-7 | 改代码 + `make build` + `npm pack` + 本地装验证 | — |
| D-5 | `make release VERSION=0.1.2-rc.1` | `npx @model-go/cli@rc install` |
| D-3 | 修 bug + `make release VERSION=0.1.2-rc.2` | `npx @model-go/cli@rc install`（rc.1→rc.2） |
| D-1 | QA 通过 | — |
| D-0 | `make release VERSION=0.1.2` | `npx @model-go/cli@latest install`；QA 清理重装回 stable |

## 发布前置条件（一次性配置）

- npm Org `model-go`（Free 计划即可）。
- npm **Granular Access Token**，**必须勾选 "Bypass two-factor authentication when publishing"**（npm 强制 2FA-on-writes，CI 无法输 OTP）；scope 选 `@model-go` Read and write；Allowed IP ranges 留空。
- 把 token 存为 GitHub Secret `NPM_TOKEN`。

| Token 类型 | CI 能用 | 说明 |
|-|-|-|
| Classic / Automation | ✅ | 专为 CI 设计 |
| Classic / Publish | ❌ | 被 2FA 拦截（403 Two-factor authentication required） |
| Granular + 勾 bypass 2FA | ✅ | 推荐 |
| Granular，没勾 bypass 2FA | ❌ | 同 Classic Publish 错误 |

> **CDN 同步延迟**：`npm publish` 成功后立刻 `npm view` 可能 404，属正常，等 30s–2min。

## Makefile 速查

| 命令 | 作用 |
|-|-|
| `make build` | 编译本地 Go 二进制到 `bin/modelgo` |
| `make test` | `go test -race ./...` + `npm test` + `npm run lint:skills` |
| `make push` / `make push-tags` | push main / 所有 tag 到 GitHub |
| `make release VERSION=x.y.z` | 稳定版：tag `vx.y.z` → CI 走 `@latest` |
| `make release VERSION=x.y.z-rc.N` | 内测版：tag `vx.y.z-rc.N` → CI 走 `@rc` |
| `make clean` | 清除 `bin/` `dist/` `node_modules/` |

## 扩展 Skill

1. 加 `skills/modelgo-<name>/SKILL.md`，frontmatter `name` 必须与目录名一致、`description` 单行。
2. `npm run lint:skills` 本地校验。
3. 发版同代码：先 `make release VERSION=x.y.z-rc.N` 给 QA 验触发效果，通过后发 stable。已安装用户再跑 `npx @model-go/cli@latest install`，wizard 自动 sync 新 skill。
