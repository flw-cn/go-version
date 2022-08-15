package main

import (
	"os"

	"github.com/flw-cn/go-version"
)

func main() {
	version.PrintVersion(os.Stderr, "", "")
}
