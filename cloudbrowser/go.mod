module github.com/browserscale/browserscale-net-go/cloudbrowser

go 1.25

require (
	github.com/browserscale/browserscale-go v0.0.0
	github.com/browserscale/browserscale-net-go v0.0.0
)

require (
	golang.org/x/net v0.31.0 // indirect
	golang.org/x/sys v0.27.0 // indirect
	golang.org/x/text v0.20.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20241015192408-796eee8c2d53 // indirect
	google.golang.org/grpc v1.69.2 // indirect
	google.golang.org/protobuf v1.36.1 // indirect
)

// Monorepo-local resolution; replace with tagged versions when published.
replace (
	github.com/browserscale/browserscale-go => ../../sdks/browserscale-go
	github.com/browserscale/browserscale-net-go => ../
)
