package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/selimserbes/routurn/internal/cli"
)

var version = "dev"

func resolvedVersion() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return version
}

func main() {
	if err := cli.Execute(resolvedVersion()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
