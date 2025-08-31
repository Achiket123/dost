package main

import (
	"dost/cmd/app"
	"os"
)

var Environment = "dev"

func main() {

	if app.Execute() != nil {
		os.Exit(1)
	}
}
