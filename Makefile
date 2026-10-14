.PHONY: build dist dev test desktop

build:
	pnpm -C web install
	pnpm -C web build
	go build -o roundtrip .

# Cross-compile release binaries (no cgo, so this works from any OS)
dist:
	pnpm -C web install
	pnpm -C web build
	GOOS=darwin GOARCH=arm64 go build -o dist/roundtrip-macos-arm64 .
	GOOS=darwin GOARCH=amd64 go build -o dist/roundtrip-macos-amd64 .
	GOOS=windows GOARCH=amd64 go build -o dist/roundtrip-windows-amd64.exe .

# Go API on :8787 using ./workspace, Vite dev server (with /api proxy) on :5173
dev:
	@mkdir -p web/dist && touch web/dist/.keep
	@trap 'kill 0' EXIT; go run . -no-open workspace & pnpm -C web dev

test:
	go test ./...
	pnpm -C web test

# Native macOS app (dist/Roundtrip.app). Needs Xcode command line tools.
desktop:
	pnpm -C web install
	pnpm -C web build
	mkdir -p dist/Roundtrip.app/Contents/MacOS dist/Roundtrip.app/Contents/Resources
	cp build/darwin/Info.plist dist/Roundtrip.app/Contents/Info.plist
	cp build/darwin/icon.icns dist/Roundtrip.app/Contents/Resources/icon.icns
	CGO_ENABLED=1 CGO_CFLAGS="-mmacosx-version-min=10.13" \
	CGO_LDFLAGS="-framework UniformTypeIdentifiers -mmacosx-version-min=10.13" \
	go build -tags desktop,production -ldflags "-w -s" \
		-o dist/Roundtrip.app/Contents/MacOS/Roundtrip ./cmd/roundtrip-desktop
