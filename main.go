package main

import (
	"github.com/spf13/cobra"
	"github.com/trknhr/ghosttype/cmd"
)

func main() {
	cobra.CheckErr(cmd.Execute())
}
