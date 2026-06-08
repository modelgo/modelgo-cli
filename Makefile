GITHUB_URL := git@github.com:modelgo/modelgo-cli.git
GITHUB_REMOTE := github
BRANCH := main

.PHONY: help github-remote push push-tags release test build clean install-local uninstall-local

help:
	@echo "modelgo-cli — common dev targets"
	@echo ""
	@echo "  make push                  Push $(BRANCH) to GitHub (creates '$(GITHUB_REMOTE)' remote if missing)"
	@echo "  make push-tags             Push all tags to GitHub"
	@echo "  make release VERSION=0.1.0 Bump checksums → commit → push main → tag → triggers release"
	@echo "  make test                  go test + npm test + lint:skills"
	@echo "  make build                 Build local Go binary into bin/"
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
	@echo "Generating checksums for v$(VERSION)..."
	goreleaser release --snapshot --clean
	cp dist/checksums.txt checksums.txt
	git add checksums.txt
	git diff --cached --quiet -- checksums.txt || git commit -m "chore: update checksums.txt for v$(VERSION)"
	git push $(GITHUB_REMOTE) $(BRANCH)
	git tag v$(VERSION)
	git push $(GITHUB_REMOTE) v$(VERSION)
	@echo "Pushed tag v$(VERSION). GitHub Actions release workflow should now run."

test:
	go test -race ./...
	npm test
	npm run lint:skills

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
