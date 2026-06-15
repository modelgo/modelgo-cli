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

本地自测分两层，目的不同，不要混成一步：

1. **验证本地 Go 改动**

```bash
make build
./bin/modelgo --version
./bin/modelgo hello
# 有鉴权改动时继续跑：
# ./bin/modelgo auth login
```

2. **验证 npm wrapper + skills 全链路**（一行搞定）

```bash
make install-local
modelgo --version
```

`make install-local` 会依次执行：`make build` → `npm pack` → `npm install -g ... --ignore-scripts` → 拷贝本地 binary → `npx skills add . -y -g`。

注意：
- `--ignore-scripts` 跳过了 postinstall 的远端 binary 下载，使用的是本地编译的 `./bin/modelgo`。
- npm tarball 的 `files` 白名单不包含 `skills/`，所以 skills 是从本地工作目录独立安装的。

恢复公开版本：

```bash
make uninstall-local
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

## 发布流程细节

`make release VERSION=x.y.z` 完整步骤：

1. **校验可快进** — `git fetch github main` 后确认 `github/main` 是 HEAD 的祖先；分叉则**提前报错**（不再 bump 后半途失败）。
2. **bump + 资产** — `npm version` 写 `package.json`，`make skills` 重新生成 reference 并同步 SKILL.md 版本，`npm run lint:skills` 校验。
3. **编译冒烟** — `goreleaser build --snapshot --clean` 仅交叉编译六平台，**不产出归档/checksums、不发布**，纯粹提前发现编译错误。
4. **commit** — 提交 `package.json`/`package-lock.json`/`skills`（**不再有 checksums.txt**）。
5. **push main → github + origin** — 两个 remote 都推。
6. **打 tag + push tag 到 github** — 触发 CI `release.yml` 正式构建、发布 GitHub Release、`npm publish`。

CI（`release.yml`）只负责：
- GoReleaser 正式构建 + 发布 GitHub Release（产物 `modelgo-<ver>-<os>-<arch>.*` + `checksums.txt`）
- `npm publish`

> **checksums 真源 = GitHub Release**。npm 包**不再内置** `checksums.txt`；`scripts/install.js` 在安装时从 `releases/download/v<ver>/checksums.txt` 下载权威校验值（mirror 兜底），再校验下载的二进制。**绝不能**回退到"本地 snapshot 生成 checksums 塞进 npm 包"——snapshot 的产物名/SHA 永远对不上 CI 真实发布，会导致全员 `Checksum entry not found`（v0.1.1–0.1.3 即因此损坏并已 deprecate）。

> **本地自测的盲区**：`make install-local` 用 `--ignore-scripts` + 本地二进制，**不会**跑真实下载/校验路径。验证发布链路必须做一次真实安装：`npm install --prefix /tmp/x @model-go/cli@<rc或latest>` 看 postinstall 是否成功 + 二进制能否运行。

### 双 remote 同步（dev=GitLab / release=GitHub）

- `origin` = GitLab（`git@gitlab.ops.modelgo.com:backend/modelgo-cli.git`，日常开发）。
- `github` = GitHub（public，`make release` 推这里触发 CI）。
- 两边 `main` 历史曾在 `v0.1.2-rc.1` 之后分叉成**不同 hash 的平行历史**（内容相同），导致 `git push github main` 非快进失败。优化后的 `make release` 会在 bump 前用 `merge-base --is-ancestor` 提前拦截并提示。
- **彻底解决**：做一次性对齐让两个 remote 共享历史（二选一 force-push），之后 `make release` 的 push 永远是干净快进。未对齐时，临时发布的手法是基于 `github/main` cherry-pick 新提交后推送 + 打 tag（见 git 历史 v0.1.3/v0.1.4）。

前置依赖：
- 本机安装 [GoReleaser](https://goreleaser.com/install/)（`brew install goreleaser`）。
- `make release` 内用 `goreleaser build --snapshot`，**不需要** GitHub Token，不发布 Release。

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
| `make skills` | build → 生成各 skill 的 `reference/`（scrape `--help`）→ 从 `package.json` 同步 SKILL.md 版本 |
| `make install-local` | 一键本地安装：build → pack → npm install → 拷贝 binary → 安装 skills |
| `make uninstall-local` | 清理全局 `@model-go/cli` |
| `make test` | `go test -race ./...` + `npm test` + `npm run lint:skills`（含版本一致性校验） |
| `make push` / `make push-tags` | push main / 所有 tag 到 GitHub |
| `make release VERSION=x.y.z` | 校验可快进 → bump package.json + 重新生成 skill 资产 → 编译冒烟 → commit → push main(github+origin) → 打 stable tag → CI 走 `@latest` |
| `make release VERSION=x.y.z-rc.N` | 内测版：同上，打 rc tag → CI 走 `@rc` |
| `make clean` | 清除 `bin/` `dist/` `node_modules/` |

> **Skill 版本真源 = `package.json` 的 `version`**（npm 发布版 = git tag = `npx skills add` 分发版）。三个 SKILL.md 的 `version:` 由 `npm run sync:skills` 自动写入，**不要手改**；`npm run lint:skills` 会在脱节时报错。`reference/` 由 `npm run generate:reference` 从二进制 `--help` 生成，标了 "Do not edit by hand"。

## 扩展 Skill

1. 加 `skills/modelgo-<name>/SKILL.md`，frontmatter `name` 必须与目录名一致、`description` 单行；`version:` 随便填（会被同步覆盖）。
2. 若新 skill 含命令：在 `scripts/generate-reference.mjs` 的 `PAGES` 里登记命令组归属，跑 `make skills` 生成 `reference/`。
3. `make skills` 同步版本 + 生成 reference；`npm run lint:skills` 校验（name/description/version 一致性）。复杂用法/故障协议放 `assets/`，SKILL.md 只留触发词 + 路由 + 指针（参考 `modelgo-shared`）。
4. 发版同代码：先 `make release VERSION=x.y.z-rc.N` 给 QA 验触发效果（release 会自动 bump 版本 + 重新生成 skill 资产并提交），通过后发 stable。已安装用户再跑 `npx @model-go/cli@latest install`，wizard 自动 sync 新 skill。
