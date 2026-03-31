package main

import "github.com/janpgu/kronk/cmd"

// version is set at build time via -ldflags "-X main.version=vX.Y.Z".
var version = "dev"

func main() {
	cmd.Execute(version)
}
