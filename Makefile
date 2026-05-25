GITHUB_URL := git@github.com:modelgo/modelgo-cli.git
GITHUB_REMOTE := github
BRANCH := main

.PHONY: help github-remote push push-tags release test build clean

help:
	@echo "modelgo-cli — common dev targets"
	@echo ""
	@echo "  make push                  Push $(BRANCH) to GitHub (creates '$(GITHUB_REMOTE)' remote if missing)"
	@echo "  make push-tags             Push all tags to GitHub"
	@echo "  make release VERSION=0.1.0 Tag v<VERSION> and push tag → triggers release workflow"
	@echo "  make test                  go test + npm test + lint:skills"
	@echo "  make build                 Build local Go binary into bin/"
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
		-o bin/modelgo-cli ./cmd/modelgo-cli
	@echo "Built bin/modelgo-cli — try: ./bin/modelgo-cli hello"

clean:
	rm -rf bin/ dist/ node_modules/
