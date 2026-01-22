module github.com/jpl-au/llmd/plugins/core

go 1.25.5

require github.com/jpl-au/llmd/sdk v0.0.0

require (
	github.com/jpl-au/llmd v0.0.0 // indirect
	github.com/knqyf263/go-plugin v0.9.0 // indirect
	github.com/tetratelabs/wazero v1.11.0 // indirect
	golang.org/x/sys v0.40.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

// For local development - remove when SDK is published
replace (
	github.com/jpl-au/llmd => ../..
	github.com/jpl-au/llmd/sdk => ../../sdk
)
