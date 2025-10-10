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
	github.com/gorilla/mux v1.8.1
	github.com/gorilla/websocket v1.5.3
	github.com/rs/zerolog v1.34.0
	golang.org/x/net v0.44.0
	google.golang.org/protobuf v1.36.9
)

require (
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	golang.org/x/sys v0.36.0 // indirect
	golang.org/x/text v0.29.0 // indirect
)
