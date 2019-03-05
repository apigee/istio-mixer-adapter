package main

import (
	"fmt"
	coreLog "log"
	"os"

	"github.com/apigee/istio-mixer-adapter/adapter"
	"github.com/spf13/cobra"
	"istio.io/istio/pkg/log"
)

var address string

func main() {
	options := log.DefaultOptions()

	rootCmd := &cobra.Command{
		Run: func(cmd *cobra.Command, args []string) {

			if err := log.Configure(options); err != nil {
				coreLog.Fatal(err)
			}

			s, err := adapter.NewGRPCAdapter(address)
			if err != nil {
				fmt.Printf("unable to start server: %v", err)
				os.Exit(-1)
			}

			shutdown := make(chan error, 1)
			go func() {
				s.Run(shutdown)
			}()
			_ = <-shutdown
		},
	}
	rootCmd.Flags().StringVarP(&address, "address", "a", ":5000", `Address to use for Adapter's gRPC API`)

	options.AttachCobraFlags(rootCmd)
	rootCmd.SetArgs(os.Args[1:])
	rootCmd.Execute()
}
