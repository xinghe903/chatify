module access

go 1.24.7

replace (
	api => ./api
	pkg => ../pkg
)

require (
	github.com/go-kratos/kratos/v2 v2.9.1
	github.com/google/uuid v1.6.0
	github.com/google/wire v0.6.0
	github.com/gorilla/websocket v1.5.3
	go.uber.org/automaxprocs v1.5.1
	google.golang.org/genproto/googleapis/api v0.0.0-20251007200510-49b9836ed3ff
	google.golang.org/grpc v1.76.0
	google.golang.org/protobuf v1.36.10
)

require (
	api v0.0.0-00010101000000-000000000000 // indirect
	dario.cat/mergo v1.0.0 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/go-kratos/aegis v0.2.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-playground/form/v4 v4.2.1 // indirect
	github.com/gorilla/mux v1.8.1 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/otel v1.37.0 // indirect
	go.opentelemetry.io/otel/metric v1.37.0 // indirect
	go.opentelemetry.io/otel/sdk v1.37.0 // indirect
	go.opentelemetry.io/otel/trace v1.37.0 // indirect
	golang.org/x/net v0.42.0 // indirect
	golang.org/x/sync v0.16.0 // indirect
	golang.org/x/sys v0.34.0 // indirect
	golang.org/x/text v0.27.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251002232023-7c0ddcbb5797 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
