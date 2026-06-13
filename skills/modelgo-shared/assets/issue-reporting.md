# Report a CLI bug (GitHub Issue)

> Hand-maintained. Entry point: [SKILL.md → Report a CLI bug](../SKILL.md#report-a-cli-bug).

When `modelgo` fails, first help the user fix the problem. If the failure looks
like a **CLI bug** (not usage, auth, permission, quota, network, or a
service-side business error), ask **once** whether to open a GitHub issue for the
modelgo CLI team.

**Issue tracker:** https://github.com/modelgo/modelgo-cli/issues

modelgo uses a small exit-code set (`0` success, `1` runtime error, `2` usage
error) and prints plain-text errors to **stderr** — there is no JSON error
envelope and no `--verbose`/`--dry-run`. So classification relies mostly on the
**stderr message text**, not on a rich exit-code taxonomy.

---

## Decision flow

```pseudocode
function shouldOfferIssueReport(exitCode, stderrText):

  # Step 1: usage errors are never CLI bugs
  if exitCode == 2:
    return EXCLUDE                       # bad flag / unknown subcommand / missing arg

  # Step 2: exit 1 is shared by user/service errors AND CLI bugs —
  #         inspect the stderr message to disambiguate
  if exitCode == 1:
    if matchesExcludePatterns(stderrText):
      return EXCLUDE                     # auth / permission / network / service business error

  # Step 3: INCLUDE criteria (CLI defect)
  if matchesIncludeCriteria(exitCode, stderrText):
    return INCLUDE                       # ask once → collect → submit

  # Step 4: ambiguous — default to EXCLUDE
  return EXCLUDE


function matchesExcludePatterns(stderrText):
  EXCLUDE_PATTERNS = [
    /session expired/i,                         # 401 → re-login
    /permission denied/i,                       # 403 → check `modelgo permissions`
    /cannot reach server at/i,                  # network (self-service)
    /\(code \d+\)/,                             # APIError: service business error
    /HTTP 4\d\d/,                               # client-side / service 4xx
    /load config|no such file|permission denied|ENOENT|EACCES/i,  # local FS / config
    /is required|mutually exclusive|must be|unknown (\w+ )?(sub)?command|unexpected argument/i,  # usage (matches "unknown command:", "unknown pay subcommand:", "unknown auth command:")
    /no payment profile set|--token is required/i,  # pay setup
  ]
  return any(p.test(stderrText) for p in EXCLUDE_PATTERNS)


function matchesIncludeCriteria(exitCode, stderrText):
  return any of:
    - panic / goroutine stack trace / "runtime error:" in output (unhandled crash)
    - exit 0 but output is empty or malformed when data was expected
    - `--json` emits invalid or truncated JSON
    - same gateway request works via curl but `modelgo pay request` fails to relay/parse it
    - regression right after `npx @model-go/cli@latest install`
    - HTTP 5xx that persists across retries and tenants (gateway/CLI defect — report so the team can trace)
    - stderr message contradicts the actual behavior or exit code
```

> **Key point:** exit code **1** covers both service-passthrough errors and CLI
> bugs. Always check the stderr text against the EXCLUDE patterns before
> considering INCLUDE.

---

## EXCLUDE — do not offer issue reporting

These are user, environment, or service-business errors. Give a fix hint; do not
ask to file an issue.

| Category | Signal (exit code + stderr) | Fix hint |
| --- | --- | --- |
| **Usage / args** | exit `2`; `unknown command`, `is required`, `mutually exclusive`, `unexpected argument` | Correct the command; run `modelgo <cmd> --help` |
| **Auth** | exit `1`; `session expired` (HTTP 401) | `modelgo auth login` to re-authenticate |
| **Permission** | exit `1`; `permission denied` (HTTP 403) | `modelgo permissions`; switch tenant with `modelgo tenant use` |
| **Service business error** | exit `1`; `<msg> (code N)` from the gateway | The gateway rejected the request; the message explains why |
| **Service 4xx** | exit `1`; `HTTP 4xx` | Client-side/service error, not a CLI defect |
| **Network (self-service)** | exit `1`; `cannot reach server at <url>` | Check DNS/proxy, `modelgo env current`, `--env`, base URL |
| **Local env / config** | exit `1`; `load config`, `ENOENT`, `EACCES` | Fix the path/permissions; re-run |
| **Payment not configured** | exit `1`; `error: no payment profile set` (`pay header`). Note: `pay status` prints the same text to **stdout** with exit `0` — informational, not a failure | `modelgo pay set --token <token>` |
| **Payment usage error** | exit `2`; `error: --token is required` (`pay set`) | Supply `--token`: `modelgo pay set --token <token>` |

> Note: the friendly `session expired` / `permission denied` wording is produced
> only by apiclient-based commands (`auth`, `balance`, `permissions`, `logs`,
> `tenant`). `pay request` is a raw HTTP relay — a 401/403 there prints
> `pay request: HTTP 401` / `pay request: HTTP 403` (plus body) and is caught by
> the **Service 4xx** row, not the Auth/Permission rows.

**Rule:** if the authoritative source of the error is the **service response** or
**user input**, it is non-reportable.

---

## INCLUDE — offer issue reporting

Offer reporting when **none** of EXCLUDE applies **and** any of the following holds:

| Category | Signal | Example |
| --- | --- | --- |
| **Unhandled crash** | panic / stack trace / `runtime error:` | Nil deref, index out of range |
| **Success but wrong output** | exit `0`, empty/malformed result | `balance` prints nothing; `logs --json` returns `[]` when calls exist |
| **Malformed `--json`** | invalid/truncated JSON on stdout | Breaks agent/CI parsing |
| **API works, `modelgo` fails** | curl succeeds, CLI errors | Request build, 402 parsing, or response relay bug in `pay request` |
| **Regression** | breaks right after `npx @model-go/cli@latest install` | Worked on the prior version |
| **Persistent 5xx** | `HTTP 5xx` across retries and tenants | Possible gateway/CLI defect — report for tracing |
| **Contradictory output** | message vs behavior vs exit code disagree | Misleading auth/usage signal from the CLI itself |

### Before offering to report

1. Align versions: [SKILL.md → Skill / CLI version check](../SKILL.md#skill--cli-version-check-agent--do-first). Run `npx @model-go/cli@latest install` if mismatched, then retry.
2. Confirm auth is healthy for commands that need it: `modelgo auth status`.
3. Re-run once and capture full stderr; add `--json` if the command supports it.

If it still fails with INCLUDE signals → offer reporting.

---

## Agent constraints

| Situation | Behavior |
| --- | --- |
| **CI / non-interactive** | Do not ask proactively. Only report if the user explicitly requests it. |
| **Same error in one session** | Ask **at most once** per distinct failure. |
| **User declines** | Stop asking; continue troubleshooting or alternate tools. |
| **Secrets** | Never paste credentials into the issue (see [Redaction](#redaction)). |

---

## User prompt (ask once)

When INCLUDE matches, ask in **Chinese** (adjust if the user prefers English):

> `modelgo` 命令出现了疑似 CLI 自身的问题。
> 是否需要帮你整理信息，向 modelgo CLI 团队提交 GitHub Issue？
> 提交前会自动脱敏凭证；你也可以只复制模板自行提交。

If the user agrees → [Collect information](#collect-information) → [Submit](#submit).

---

## Collect information

Run these and paste the results into the template (redact first).

| Field | How to obtain |
| --- | --- |
| CLI version | `modelgo --version` |
| Skill version | `version` in the installed `SKILL.md` frontmatter |
| Node version | `node --version` |
| OS | `uname -a` (Linux/macOS) or `sw_vers` (macOS) |
| Active env | `modelgo env current` |
| Auth status | `modelgo auth status` (redacted) |
| Command | The exact command the user ran (redacted) |
| stderr | The original failure output (capture full text) |
| JSON output | Re-run with `--json` if the command supports it |
| request_id | For API failures: `modelgo logs` (recent) or `modelgo logs <request-id>` — **keep** this, it helps tracing |
| Repro steps | Numbered 1-2-3 |
| Expected vs actual | One sentence each |
| Frequency | Always / sometimes / once |

### Redaction

Before any paste or `gh issue create`:

- Replace session tokens / `Authorization: Bearer ...` → `[REDACTED]`
- Replace any API key → `[REDACTED]`
- Replace `X-PAYMENT` header values and `modelgo pay set --token <...>` → `[REDACTED]` (these are payment credentials)
- Replace `X-ModelGo-Anonymous-ID` → `[REDACTED]`
- Replace `account_id` / `tenant_id` from `modelgo auth status` → `[REDACTED]` (the only account-identifying fields it prints)
- Redact `--data` / `--prompt` request bodies that contain business content → summarize as `[user request about <topic>]`
- Redact custom `--base-url` / internal endpoints → `[REDACTED_URL]`
- **Keep** `request_id` — it lets the team trace gateway logs
- Local paths may stay or be generalized (`~/path/to/file`)

**Principle:** anything that could identify the account, credentials, internal
infrastructure, or business content must be redacted. When in doubt, redact.

---

## Issue template

**Title format:** `[bug] <command> <short symptom>`

Example: `[bug] pay request relays empty body after 402 handoff`

Copy into the issue body (or pass to `gh issue create --body-file`):

````markdown
## Environment

- CLI: modelgo vX.Y.Z
- Skill: X.Y.Z
- Node: vXX.X.X
- OS: ...
- Env: cn | intl | <custom>

## Reproduce

```bash
modelgo ...   # credentials redacted
```

## Expected

What should have happened.

## Actual

What happened instead.

## Full output

```
<stderr text>
Exit code: ...
```

## JSON output (if the command supports --json)

```json
...
```

## request_id (for API failures)

```
request_id: ...
status: ...
```

## Already tried

- `npx @model-go/cli@latest install` and skill version aligned with the CLI
- `modelgo auth status` OK for this command
- Different env / network — still reproduces

## Notes

- Frequency: always / intermittent / once
- Invoked via: terminal / agent
````

---

## Check for duplicates

Before submitting, search existing issues:

```bash
# If gh is available:
gh issue list --repo modelgo/modelgo-cli --search "<error keyword or command>" --state open --limit 10
```

Or search manually: [open issues](https://github.com/modelgo/modelgo-cli/issues?q=is%3Aissue+is%3Aopen)

If a matching open issue exists, tell the user its URL and offer to add a comment
with their repro details instead of creating a duplicate.

---

## Submit

### Pre-submit confirmation

Always show the redacted issue body to the user and ask for confirmation first:

> 以下是即将提交的 Issue 内容（已脱敏），请确认是否提交：
> show body

Only proceed after the user confirms.

### Option A — GitHub CLI (`gh`)

Preferred when `gh` is installed and authenticated (`gh auth status` succeeds).

```bash
gh issue create \
  --repo modelgo/modelgo-cli \
  --title "[bug] <command> <short symptom>" \
  --body-file /path/to/redacted-issue.md
```

Tell the user the issue URL returned by `gh`.

> Do not pass `--label` unless you've confirmed it exists (`gh label list --repo modelgo/modelgo-cli`); `gh issue create` fails on an unknown label.

### Option B — Browser

1. Open https://github.com/modelgo/modelgo-cli/issues/new
2. Paste the redacted body from the template above
3. Submit

### Fallback — `gh` not available

1. Write the complete redacted issue body to a local file (e.g. `./modelgo-bug-report.md`)
2. Print the file content to the user
3. Provide the URL: https://github.com/modelgo/modelgo-cli/issues/new
4. Instruct: "请在浏览器中打开上面的链接，将内容粘贴到 issue body 中提交。"

Do **not** block on `gh` — always provide a manual path.

---

## Exit codes (reference)

| Code | Meaning | Usually reportable? |
| --- | --- | --- |
| 0 | Success | Only if output is wrong/empty when data was expected |
| 1 | Runtime error (auth / permission / network / API / CLI) | Sometimes — only if stderr shows a CLI defect, not a service/user error |
| 2 | Usage error | No |
