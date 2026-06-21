// Command togo is the CLI for the togo framework — a Laravel-artisan-like
// developer experience for the Go + sqlc + Atlas + GraphQL/OpenAPI + Next.js stack.
//
// Install:
//
//	go install github.com/togo-framework/cli/cmd/togo@latest
//
// The main package lives in this `togo` directory so `go install` names the
// binary `togo` (Go derives the binary name from the directory of the main package).
package main

import "github.com/togo-framework/cli/cmd"

func main() {
	cmd.Execute()
}
