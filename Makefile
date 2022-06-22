all: out/reversi.wasm out/wasm_exec.js

out/wasm_exec.js:
	cp `go env GOROOT`/misc/wasm/wasm_exec.js $@

out/reversi.wasm: main.go pattern.go utility.go
	GOOS=js GOARCH=wasm go build -o $@