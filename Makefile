GITHUB_URL := git@github.com:modelgo/modelgo-cli.git
GITHUB_REMOTE := github
BRANCH := main

.PHONY: help github-remote push push-tags release test build skills clean install-local uninstall-local

help:
	@echo "modelgo-cli — common dev targets"
	@echo ""
	@echo "  make push                  Push $(BRANCH) to GitHub (creates '$(GITHUB_REMOTE)' remote if missing)"
	@echo "  make push-tags             Push all tags to GitHub"
	@echo "  make release VERSION=0.1.0 Bump version + skills → commit → push main → tag → triggers release"
	@echo "  make test                  go test + npm test + lint:skills"
	@echo "  make build                 Build local Go binary into bin/"
	@echo "  make skills                Regenerate skill reference + sync skill versions"
	@echo "  make install-local         Build, pack, install locally (npm + binary + skills)"
	@echo "  make uninstall-local       Remove global @model-go/cli"
	@echo "  make clean                 Remove bin/ dist/ node_modules/"

github-remote:
	@if ! git remote get-url $(GITHUB_REMOTE) >/dev/null 2>&1; then \
		echo "Adding remote $(GITHUB_REMOTE) → $(GITHUB_URL)"; \
		git remote add $(GITHUB_REMOTE) $(GITHUB_URL); \
	fi

push: github-remote
	git push $(GITHUB_REMOTE) $(BRANCH)

push-tags: github-remote
	git push $(GITHUB_REMOTE) --tags

release: github-remote
	@if [ -z "$(VERSION)" ]; then echo "Usage: make release VERSION=0.1.0"; exit 1; fi
	@git diff --quiet && git diff --cached --quiet || { echo "working tree/index not clean; commit or stash unrelated changes first"; exit 1; }
	@echo "Checking $(GITHUB_REMOTE)/$(BRANCH) is fast-forwardable from HEAD..."
	git fetch $(GITHUB_REMOTE) $(BRANCH)
	@git merge-base --is-ancestor $(GITHUB_REMOTE)/$(BRANCH) HEAD || { \
		echo ""; \
		echo "ERROR: $(GITHUB_REMOTE)/$(BRANCH) is not an ancestor of HEAD — histories have diverged."; \
		echo "'git push $(GITHUB_REMOTE) $(BRANCH)' would be rejected (non-fast-forward)."; \
		echo "Align the two remotes once (see CLAUDE.md → 双 remote 同步), then re-run."; \
		exit 1; }
	@echo "Bumping package.json + skill assets to v$(VERSION)..."
	npm version $(VERSION) --no-git-tag-version --allow-same-version
	$(MAKE) skills
	npm run lint:skills
	@echo "Cross-compile smoke test (build only, nothing published)..."
	goreleaser build --snapshot --clean
	git add package.json package-lock.json skills
	git diff --cached --quiet -- package.json package-lock.json skills || \
		git commit -m "chore: release v$(VERSION) (package.json, lock, skills)" -- package.json package-lock.json skills
	git push $(GITHUB_REMOTE) $(BRANCH)
	git push origin $(BRANCH)
	git tag v$(VERSION)
	git push $(GITHUB_REMOTE) v$(VERSION)
	@echo "Pushed tag v$(VERSION). GitHub Actions release workflow should now run."

test:
	go test -race ./...
	npm test
	npm run lint:skills

# Regenerate the per-skill reference/ docs from the built binary's --help, then
# sync every SKILL.md version: from package.json. Run before committing skill or
# command changes (and automatically as part of `make release`).
skills: build
	npm run sync:skill-assets

build:
	mkdir -p bin
	go build -ldflags "-X github.com/modelgo/modelgo-cli/internal/version.Version=v0.0.0-dev" \
		-o bin/modelgo ./cmd/modelgo-cli
	@echo "Built bin/modelgo — try: ./bin/modelgo hello"

clean:
	rm -rf bin/ dist/ node_modules/

VERSION_JS := $(shell node -p "require('./package.json').version")

install-local: build
	npm pack
	npm install -g ./model-go-cli-$(VERSION_JS).tgz --ignore-scripts
	@mkdir -p "$(shell npm root -g)/@model-go/cli/bin"
	cp bin/modelgo "$(shell npm root -g)/@model-go/cli/bin/modelgo"
	npx -y skills add . -y -g
	@echo "✓ Local install complete — try: modelgo --version"

uninstall-local:
	npm uninstall -g @model-go/cli
	@echo "✓ Uninstalled — restore stable with: npx @model-go/cli@latest install"
