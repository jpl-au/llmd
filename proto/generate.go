// Package proto contains the protocol buffer definitions for llmd plugins.
//
// To regenerate the Go code from the .proto files:
//
//	go generate ./proto
//
// Prerequisites:
//   - protoc: apt install -y protobuf-compiler
//   - protoc-gen-go-plugin: go install github.com/knqyf263/go-plugin/cmd/protoc-gen-go-plugin@latest
package proto

//go:generate protoc --go-plugin_out=. --go-plugin_opt=paths=source_relative plugin/plugin.proto
//go:generate protoc --go-plugin_out=. --go-plugin_opt=paths=source_relative host/host.proto
