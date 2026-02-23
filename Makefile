MODULE := github.com/miyamo2/phasedchecker
DEST   := internal/x/tools

.PHONY: sync-x-tools
sync-x-tools:
	@set -e; \
	XTOOLS_VERSION=$$(awk '/golang.org\/x\/tools /{print $$NF}' go.mod); \
	echo "golang.org/x/tools version: $$XTOOLS_VERSION"; \
	go mod download "golang.org/x/tools@$$XTOOLS_VERSION"; \
	SRC=$$(go env GOMODCACHE)/golang.org/x/tools@$$XTOOLS_VERSION; \
	\
	rm -rf $(DEST)/diff $(DEST)/driverutil $(DEST)/free; \
	mkdir -p $(DEST)/diff/lcs $(DEST)/driverutil $(DEST)/free; \
	\
	find "$$SRC/internal/diff" -maxdepth 1 -name '*.go' ! -name '*_test.go' -exec cp {} $(DEST)/diff/ \; ; \
	find "$$SRC/internal/diff/lcs" -maxdepth 1 -name '*.go' ! -name '*_test.go' -exec cp {} $(DEST)/diff/lcs/ \; ; \
	find "$$SRC/internal/analysis/driverutil" -maxdepth 1 -name '*.go' ! -name '*_test.go' -exec cp {} $(DEST)/driverutil/ \; ; \
	find "$$SRC/internal/astutil/free" -maxdepth 1 -name '*.go' ! -name '*_test.go' -exec cp {} $(DEST)/free/ \; ; \
	\
	chmod -R u+w $(DEST)/diff $(DEST)/driverutil $(DEST)/free; \
	\
	find $(DEST) -name '*.go' -exec sed -i \
		-e 's|golang.org/x/tools/internal/diff/lcs|$(MODULE)/$(DEST)/diff/lcs|g' \
		-e 's|golang.org/x/tools/internal/diff|$(MODULE)/$(DEST)/diff|g' \
		-e 's|golang.org/x/tools/internal/astutil/free|$(MODULE)/$(DEST)/free|g' \
		-e 's|golang.org/x/tools/internal/analysis/driverutil|$(MODULE)/$(DEST)/driverutil|g' \
		{} +; \
	\
	{ \
	echo '# Vendored from golang.org/x/tools'; \
	echo ''; \
	echo "- **Source**: \`golang.org/x/tools\`"; \
	echo "- **Version**: \`$$XTOOLS_VERSION\`"; \
	echo "- **Copy date**: $$(date +%Y-%m-%d)"; \
	echo '- **License**: BSD-3-Clause (see LICENSE file in this directory)'; \
	echo ''; \
	echo '## Copied packages'; \
	echo ''; \
	echo '| Original path | Local path |'; \
	echo '|---|---|'; \
	echo '| `internal/diff/` | `diff/` |'; \
	echo '| `internal/diff/lcs/` | `diff/lcs/` |'; \
	echo '| `internal/astutil/free/` | `free/` |'; \
	echo '| `internal/analysis/driverutil/` | `driverutil/` |'; \
	echo ''; \
	echo '## Import path rewriting'; \
	echo ''; \
	echo '| Original import | Rewritten import |'; \
	echo '|---|---|'; \
	echo '| `golang.org/x/tools/internal/diff` | `$(MODULE)/$(DEST)/diff` |'; \
	echo '| `golang.org/x/tools/internal/diff/lcs` | `$(MODULE)/$(DEST)/diff/lcs` |'; \
	echo '| `golang.org/x/tools/internal/astutil/free` | `$(MODULE)/$(DEST)/free` |'; \
	echo '| `golang.org/x/tools/internal/analysis/driverutil` | `$(MODULE)/$(DEST)/driverutil` |'; \
	echo ''; \
	echo 'Public imports (`golang.org/x/tools/go/analysis`, `golang.org/x/tools/go/ast/astutil`, etc.) remain unchanged.'; \
	} > $(DEST)/VENDORED.md
