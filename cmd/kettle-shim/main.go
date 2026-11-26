/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package main

import (
	"os"

	"kettle/cmd/kettle-shim/cmd"

	"github.com/opencontainers/runc/libcontainer"
	_ "github.com/opencontainers/runc/libcontainer/nsenter"
)

func init() {
	// The shim creates containers, so libcontainer re-executes this binary
	// as "kettle-shim init" to set up each container's init process (the
	// equivalent of `runc init`).
	if len(os.Args) > 1 && os.Args[1] == "init" {
		libcontainer.Init()
	}
}

func main() {
	cmd.Execute()
}
