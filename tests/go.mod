module tests

go 1.25.1

replace (
	cli => ../cli
	core => ../core
	gateway => ../gateway
	local-agent => ../local-agent
	manager => ../manager
	tests/synthetic => ./synthetic
)

require (
	connectrpc.com/connect v1.19.0
	gateway v0.0.0-00010101000000-000000000000
	github.com/gorilla/websocket v1.5.3
	github.com/stretchr/testify v1.11.1
	gopkg.in/yaml.v3 v3.0.1
	local-agent v0.0.0-00010101000000-000000000000
	manager v0.0.0-00010101000000-000000000000
	tests/synthetic v0.0.0-00010101000000-000000000000
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/mux v1.8.1 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rs/zerolog v1.34.0 // indirect
	golang.org/x/sys v0.36.0 // indirect
	google.golang.org/protobuf v1.36.9 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
)
