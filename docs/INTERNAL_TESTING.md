# Internal testing for modelgo-cli

While `@modelgo/cli` is pre-release, it is published to **GitHub Packages** (private to the modelgo org), not to public npm. Internal testers need a one-time `.npmrc` setup to install it.

After that one-time setup, the install flow is identical to lark-cli:

```bash
npx @modelgo/cli@latest install   # first install
npx @modelgo/cli@latest install   # also: detects and upgrades to the latest version
```

## One-time setup

1. Create a GitHub Personal Access Token (classic): https://github.com/settings/tokens
   - Scopes required: `read:packages`
   - Expiration: as you see fit; longer = fewer renewals
   - Copy the token (starts with `ghp_...`)

2. Append two lines to your `~/.npmrc` (create the file if missing):

   ```ini
   @modelgo:registry=https://npm.pkg.github.com
   //npm.pkg.github.com/:_authToken=ghp_YOUR_TOKEN_HERE
   ```

   Replace `ghp_YOUR_TOKEN_HERE` with the token from step 1.

   This config only routes `@modelgo/*` packages to GitHub Packages — every other package (`@larksuite/*`, `express`, etc.) keeps using the default npm registry. Your other projects are unaffected.

3. Verify the auth + scope routing works:

   ```bash
   npm view @modelgo/cli version
   ```

   Expected: prints the latest published version (e.g. `0.1.0-rc.1`). If you see `404` or `Unauthorized`, the `.npmrc` is not in effect — check the file path and token.

## Install

```bash
npx @modelgo/cli@latest install
```

The wizard runs through:

1. `npm install -g @modelgo/cli` (downloads the Go binary from GitHub Releases, SHA-256 verified)
2. `npx skills add modelgo/modelgo-cli -y -g` (drops `modelgo-*` skill bundles into every AI agent's skills directory)

After install, open a new AI agent session and ask:

> "Have modelgo-cli say hello to me."

The agent should auto-discover the `modelgo-hello` skill and run `modelgo-cli hello`.

## Upgrade

Re-run the same command:

```bash
npx @modelgo/cli@latest install
```

The wizard detects the installed version, queries the registry for the latest, and upgrades in place. No need to uninstall first.

## Where things live

| Artifact | Hosted at | Auth needed |
|---|---|---|
| `@modelgo/cli` npm package | GitHub Packages (`npm.pkg.github.com`) | Yes — PAT, via `~/.npmrc` |
| `modelgo-cli` binary | GitHub Releases | No (public repo) |
| `modelgo-*` skills | GitHub raw files in this repo | No (public repo) |

## Going public later

When `@modelgo/cli` graduates to public npm, two edits in `.github/workflows/release.yml`:

1. `registry-url: "https://npm.pkg.github.com"` → `registry-url: "https://registry.npmjs.org"`
2. `NODE_AUTH_TOKEN: ${{ secrets.GITHUB_TOKEN }}` → `NODE_AUTH_TOKEN: ${{ secrets.NPM_TOKEN }}` (and add `--access public` to `npm publish`)

Plus add the `NPM_TOKEN` repo secret. Users then drop the `.npmrc` scope config and `npx @modelgo/cli@latest install` works without any setup.
