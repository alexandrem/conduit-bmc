module bmc-test

go 1.25.1

replace local-agent => ../../local-agent

require (
	github.com/rs/zerolog v1.34.0
	local-agent v0.0.0
)

require (
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/spf13/cobra v1.10.1 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	golang.org/x/sys v0.36.0 // indirect
)
