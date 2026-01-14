package main

import (
	"gunp/cmd"
	logger "gunp/internal/log"
)

func main() {
	logger.Initialize()

	logger.Get().Print("gunp - Git Unpushed")

	logger.Get().Print("By running gunp it will recursively explore all folders starting from the current, and count the unpushed commits of your git repositories.")

	cmd.Execute()
}
