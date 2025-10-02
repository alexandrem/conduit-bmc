module local-agent

go 1.25.1

replace (
	core => ../core
	gateway => ../gateway
	manager => ../manager
)

require (
	connectrpc.com/connect v1.19.0
	core v0.0.0-00010101000000-000000000000
	gateway v0.0.0-00010101000000-000000000000
	github.com/bougou/go-ipmi v0.7.8
	github.com/gorilla/mux v1.8.1
	github.com/gorilla/websocket v1.5.3
	github.com/rs/zerolog v1.34.0
	golang.org/x/net v0.44.0
)

require (
	github.com/fatih/color v1.15.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/olekukonko/errors v1.1.0 // indirect
	github.com/olekukonko/ll v0.0.9 // indirect
	github.com/olekukonko/tablewriter v1.0.9 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/rogpeppe/go-internal v1.9.0 // indirect
	golang.org/x/sys v0.36.0 // indirect
	golang.org/x/text v0.29.0 // indirect
	google.golang.org/protobuf v1.36.9 // indirect
)
