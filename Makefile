# Project tasks. CI runs format-dry-run, build, test and release.

NAME := timesheet
RELEASE_TARGETS := linux-arm32 linux-arm64 linux-x64 macOS-intel macOS-apple-silicon

.PHONY: build format format-dry-run test release

build:
	go build -o build/$(NAME) ./cmd

format:
	gofmt -w .

format-dry-run:
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "These files need make format:" >&2; \
		echo "$$unformatted" >&2; \
		exit 1; \
	fi

# the plugin's own tests run beside timesheet's
test:
	go test ./...
	@for t in tests/*.sh; do $$t || exit 1; done

release:
	./scripts/release.sh $(NAME) $(RELEASE_TARGETS)
