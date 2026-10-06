module webtyp.com/agenteval

go 1.26.8

require (
	webtyp.com/agent v1.1.0
	webtyp.com/agentworker v0.1.5
	webtyp.com/artifacts v0.1.1
	webtyp.com/components v0.8.6
	webtyp.com/context v0.0.23
	webtyp.com/dom v0.13.21
	webtyp.com/fmt v1.0.0
	webtyp.com/html v0.0.26
	webtyp.com/lfm v0.1.3
	webtyp.com/llm v0.2.3
	webtyp.com/model v0.2.2
	webtyp.com/qwen v0.4.7
	webtyp.com/server v0.2.74
	webtyp.com/sitec v0.2.47
	webtyp.com/unixid v0.2.29
)

require (
	github.com/HugoSmits86/nativewebp v1.2.1 // indirect
	github.com/disintegration/imaging v1.6.2 // indirect
	github.com/smallstep/truststore v0.13.0 // indirect
	github.com/tdewolff/minify/v2 v2.24.8 // indirect
	github.com/tdewolff/parse/v2 v2.8.5 // indirect
	golang.org/x/crypto v0.53.0 // indirect
	golang.org/x/image v0.45.0 // indirect
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	howett.net/plist v1.0.0 // indirect
	webtyp.com/agentcontext v0.3.1 // indirect
	webtyp.com/await v0.1.2 // indirect
	webtyp.com/base64 v0.0.6 // indirect
	webtyp.com/color v0.1.2 // indirect
	webtyp.com/css v0.4.28 // indirect
	webtyp.com/decoder v0.5.2 // indirect
	webtyp.com/device v0.1.0 // indirect
	webtyp.com/escape v0.1.0 // indirect
	webtyp.com/fetch v0.1.29 // indirect
	webtyp.com/filepath v0.1.0 // indirect
	webtyp.com/files v0.0.4 // indirect
	webtyp.com/font v0.0.5 // indirect
	webtyp.com/image v0.1.16 // indirect
	webtyp.com/js v0.1.1 // indirect
	webtyp.com/json v0.5.29 // indirect
	webtyp.com/lang v0.1.2 // indirect
	webtyp.com/mcp v0.2.40 // indirect
	webtyp.com/modfind v0.0.10 // indirect
	webtyp.com/nn v0.4.2 // indirect
	webtyp.com/opfs v0.1.3 // indirect
	webtyp.com/pwa v0.1.1 // indirect
	webtyp.com/router v0.3.2 // indirect
	webtyp.com/svg v0.3.14 // indirect
	webtyp.com/time v0.5.7 // indirect
	webtyp.com/tinygo v1.0.1 // indirect
	webtyp.com/tokenizer v0.4.2 // indirect
	webtyp.com/vector v0.1.1 // indirect
	webtyp.com/weights v0.3.0 // indirect
	webtyp.com/widget v0.6.36 // indirect
)

replace (
	webtyp.com/components => ../components
	// dom v0.13.19 made Show lazy and components is not migrated yet: back to ../dom once it is.
	webtyp.com/html => ../html
)
