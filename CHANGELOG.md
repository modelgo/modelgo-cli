# Changelog

本文件记录 `@model-go/cli`（CLI 命令 `modelgo`）的所有重要变更。

格式参考 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

> 发布渠道：带 `-rc.N` 后缀的版本走 npm `rc` dist-tag（QA 内测，不动 `latest`）；不带后缀的版本走 `latest`。详见 `CLAUDE.md` 的「内测与渐进式发布流程」。
>
> 变更条目从 git tag 间的 conventional commits 归类生成。

## [Unreleased] — v0.1.6 内测中

> rc 线（`v0.1.6-rc.1` … `v0.1.6-rc.6`，2026-06-23）仍在 QA 内测，尚未转正到 `latest`。当前 npm `latest` = `0.1.4`。
> 自 `v0.1.4` 以来累积的功能变更如下（即将构成 v0.1.5 / v0.1.6 稳定版）：

### 新增
- **模型调用命令**：`modelgo chat` / `models` / `embeddings` / `call`（OpenAI 兼容 `/v1/*`），配套 `modelgo key` 管理每个 env 的 API key（`feat(modelcmd)`）。
- **多租户支持增强**：`tenant` 信息处理与 JSON 输出改进（`feat(auth, tenantcmd)`）。

## [0.1.4] — 2026-06-15

> 修复了 `v0.1.1`–`v0.1.3` 的 checksums 损坏问题。这些早期版本已在 npm 上 **deprecate**，请勿使用。

### 修复
- **安装校验**：`scripts/install.js` 改为从 GitHub Release 下载权威 `checksums.txt`，不再把本地 snapshot 的 checksums 打进 npm 包——后者的产物名/SHA 永远对不上 CI 真实发布，会导致全员 `Checksum entry not found`（`fix(install)`）。

### 构建 / 文档
- 修复 `make release` 以适配双 remote（GitLab dev / GitHub release），checksums 真源统一为 GitHub Release（`docs+build`）。

## [0.1.3] — 2026-06-12

> ⚠️ 已 deprecate（checksums 损坏，请升级到 `0.1.4`）。

### 新增
- **x402 按次付费**：`modelgo pay` 命令 + x402 pay-per-call 客户端原语，支持匿名按调用付费（`feat(pay)`）。
- **余额命令**：`balance` 金额字段改用 `apiclient.Decimal` 精确表示货币（`feat(balance)`）。
- **查询能力增强**：`balance` / `logs` / `permissions` 响应结构与数据处理改进；`logs stats` / `logs usage` 的日期输入归一化（`feat(balance, logs, permissions)`、`feat(logs)`）。
- **鉴权健壮性**：`FetchTenants` 能正确处理业务错误信封并保持向后兼容（`feat(auth)`）。
- **Skill 管理**：skill 管理与版本同步增强（`feat(cli)`）。

### 变更
- 移除演示用的 `hello` 命令并更新文档（`refactor(cli)`）。

### 文档
- 更新发布流程文档与 Makefile 说明（`docs`）。

## [0.1.2-rc.1] — 2026-06-11

> 仅 rc 内测版，未转正到 `latest`；其内容已并入 `0.1.3`。

## [0.1.1] — 2026-06-10

> ⚠️ 已 deprecate（checksums 损坏，请升级到 `0.1.4`）。

### 新增
- **安装向导**：postinstall wizard，支持版本比较与操作系统检测（`feat`，#1）。
- 集成 `AGENTS.md` / `CLAUDE.md`，纳入 skill bundle（`feat(cli)`）。

## [0.1.0] — 2026-06-09

首个版本。

### 新增
- modelgo CLI 脚手架，含 `hello` 命令与版本上报（`feat`，#2）。
- modelgo-cli skill bundle（`feat`，#3）。

### 修复
- 修正 `package.json` 中 skill 文件路径，CLI 命令从 `mg` 改为 `modelgo`（`fix`）。

[Unreleased]: https://github.com/modelgo/modelgo-cli/compare/v0.1.4...HEAD
[0.1.4]: https://github.com/modelgo/modelgo-cli/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/modelgo/modelgo-cli/compare/v0.1.1...v0.1.3
[0.1.2-rc.1]: https://github.com/modelgo/modelgo-cli/compare/v0.1.1...v0.1.2-rc.1
[0.1.1]: https://github.com/modelgo/modelgo-cli/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/modelgo/modelgo-cli/releases/tag/v0.1.0
