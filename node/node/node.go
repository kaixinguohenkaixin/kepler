package node

import (
	"fmt"
	"github.com/op/go-logging"
	"github.com/spf13/cobra"
)

var logger = logging.MustGetLogger("node")

func Cmd() *cobra.Command {
	nodeCmd.AddCommand(startCmd())

	return nodeCmd
}

var nodeCmd = &cobra.Command{
	Use:   "node",
	Short: fmt.Sprint("node operation(start)"),
	Long:  fmt.Sprint("node operation(start)"),
}
