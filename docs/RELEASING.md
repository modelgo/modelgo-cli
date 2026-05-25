# Releasing modelgo-cli

## Cutting a release

1. Make sure `main` is green and ahead of the last tag.
2. Tag: `git tag v0.1.0 && git push origin v0.1.0`
3. The `Release` workflow runs:
   - GoReleaser builds 6 platform archives + `checksums.txt`, creates a GitHub Release.
   - `checksums.txt` is committed back to `main` so the next `npm publish` ships with it.
   - `package.json` version is synced to the tag.
   - `npm publish` pushes `@modelgo/cli@<version>` to **GitHub Packages** (npm.pkg.github.com). Authenticates via the built-in `GITHUB_TOKEN`; no separate npm token needed.

> While modelgo-cli is in pre-release, the npm package is internal-only. See [INTERNAL_TESTING.md](./INTERNAL_TESTING.md) for the one-time `.npmrc` setup testers need. When the package goes public, switch the release workflow's `registry-url` to `registry.npmjs.org` (see the "Going public later" section of INTERNAL_TESTING.md).

## Manual smoke test (required after every release)

In a clean environment (e.g. fresh Docker container or a machine without modelgo-cli installed). Note: during the GitHub Packages phase, the container needs a `.npmrc` with the GitHub Packages scope + PAT (see [INTERNAL_TESTING.md](./INTERNAL_TESTING.md)).

```bash
docker run --rm -it -v ~/.npmrc:/root/.npmrc:ro node:20 bash
# inside the container:
npx @modelgo/cli@latest install --lang=en
which modelgo-cli && modelgo-cli --version       # expect the just-released version
modelgo-cli hello --name smoketest               # expect "Hello, smoketest!"
```

On a developer machine that has Claude Code installed:

```bash
ls ~/.claude/skills/modelgo-shared/SKILL.md      # expect file to exist
ls ~/.claude/skills/modelgo-hello/SKILL.md       # expect file to exist
```

Open a new Claude Code session and ask: "Have modelgo-cli say hello to me." Expect the agent to call `modelgo-cli hello` and report the greeting.

If any step fails, file an issue and consider yanking the release. On GitHub Packages, delete the bad version under https://github.com/modelgo/modelgo-cli/pkgs/npm/cli/versions.
