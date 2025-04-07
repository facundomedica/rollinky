package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"

	svrcmd "github.com/cosmos/cosmos-sdk/server/cmd"

	"rollinky/app"
	"rollinky/cmd/rollinkyd/cmd"
)

func main() {
	go func() {
		http.ListenAndServe("localhost:6060", nil)
	}()

	rootCmd := cmd.NewRootCmd()
	if err := svrcmd.Execute(rootCmd, "", app.DefaultNodeHome); err != nil {
		fmt.Fprintln(rootCmd.OutOrStderr(), err)
		os.Exit(1)
	}
}
