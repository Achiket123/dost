package main

import (
	"dost/cmd/app"
	"os"
)

func main() {
	if app.Execute() != nil {
		os.Exit(1)
	}
}

/
