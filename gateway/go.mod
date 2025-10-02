module gateway

go 1.25.1

replace (
	core => ../core
	manager => ../manager
)

require (
	connectrpc.com/connect v1.19.0
	core v0.0.0-00010101000000-000000000000
	github.com/golang-jwt/jwt/v5 v5.3.0
	github.com/gorilla/mux v1.8.1
	github.com/gorilla/websocket v1.5.3
	github.com/rs/zerolog v1.34.0
	github.com/stretchr/testify v1.11.1
	golang.org/x/net v0.44.0
	google.golang.org/protobuf v1.36.9
	manager v0.0.0-00010101000000-000000000000
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rogpeppe/go-internal v1.9.0 // indirect
	golang.org/x/sys v0.36.0 // indirect
	golang.org/x/text v0.29.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
