# Releasing modelgo-cli

## Cutting a release

1. Make sure `main` is green and ahead of the last tag.
2. Tag and push: `make release VERSION=0.1.0` (or `git tag v0.1.0 && git push github v0.1.0`).
3. The `Release` workflow runs:
   - GoReleaser builds 6 platform archives + `checksums.txt`, creates a GitHub Release.
   - `checksums.txt` is committed back to `main` so the next `npm publish` ships with it.
   - `package.json` version is synced to the tag.
   - `npm publish --access public` pushes `@model-go/cli@<version>` to npmjs.org. Authenticates via the `NPM_TOKEN` repo secret (npm automation token, publish scope).

## Manual smoke test (required after every release)

In a clean environment (e.g. fresh Docker container or a machine without modelgo installed):

```bash
docker run --rm -it node:20 bash
# inside the container:
npx @model-go/cli@latest install --lang=en
which modelgo && modelgo --version               # expect the just-released version
modelgo hello --name smoketest                   # expect "Hello, smoketest!"
```

On a developer machine that has Claude Code installed:

```bash
ls ~/.claude/skills/modelgo-shared/SKILL.md      # expect file to exist
ls ~/.claude/skills/modelgo-hello/SKILL.md       # expect file to exist
```

Open a new Claude Code session and ask: "Have modelgo say hello to me." Expect the agent to call `modelgo hello` and report the greeting.

If any step fails, file an issue and yank the release:

```bash
npm deprecate @model-go/cli@<version> "yanked: <reason>"
# Or, within 72 hours of publish, fully remove:
npm unpublish @model-go/cli@<version>
```

## Reverting to GitHub Packages (if ever needed)

Pre-release builds were published to GitHub Packages. To switch back:

1. Edit `.github/workflows/release.yml`:
   - `registry-url: "https://registry.npmjs.org"` → `"https://npm.pkg.github.com"`
   - `NODE_AUTH_TOKEN: ${{ secrets.NPM_TOKEN }}` → `${{ secrets.GITHUB_TOKEN }}`
   - Remove `--access public` from the `npm publish` line
   - Add `permissions: packages: write`

2. Internal testers must configure `~/.npmrc`:
   ```
   @model-go:registry=https://npm.pkg.github.com
   //npm.pkg.github.com/:_authToken=ghp_YOUR_PAT
   ```
   PAT needs `read:packages` scope.
