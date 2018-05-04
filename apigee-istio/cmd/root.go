// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"flag"

	"github.com/apigee/istio-mixer-adapter/apigee-istio/cmd/bindings"
	"github.com/apigee/istio-mixer-adapter/apigee-istio/cmd/provision"
	"github.com/apigee/istio-mixer-adapter/apigee-istio/shared"
	"github.com/spf13/cobra"
)

// GetRootCmd returns the root of the cobra command-tree.
func GetRootCmd(args []string, printf, fatalf shared.FormatFn) *cobra.Command {
	rootArgs := &shared.RootArgs{}

	c := &cobra.Command{
		Use:   "apigee-istio",
		Short: "Utility to work with Apigee and Istio.",
		Long:  "This command lets you interact with Apigee and Istio.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			rootArgs.Resolve()
			return nil
		},
	}
	c.SetArgs(args)
	c.PersistentFlags().AddGoFlagSet(flag.CommandLine)

	c.PersistentFlags().StringVarP(&rootArgs.RouterBase, "routerBase", "r",
		shared.DefaultRouterBase, "Apigee router base")
	c.PersistentFlags().StringVarP(&rootArgs.ManagementBase, "managementBase", "m",
		shared.DefaultManagementBase, "Apigee management base")
	c.PersistentFlags().BoolVarP(&rootArgs.Verbose, "verbose", "v",
		false, "verbose output")

	c.PersistentFlags().StringVarP(&rootArgs.Org, "org", "o",
		"", "Apigee organization name")
	c.PersistentFlags().StringVarP(&rootArgs.Env, "env", "e",
		"", "Apigee environment name")
	c.PersistentFlags().StringVarP(&rootArgs.Username, "username", "u",
		"", "Apigee username")
	c.PersistentFlags().StringVarP(&rootArgs.Password, "password", "p",
		"", "Apigee password")

	c.MarkPersistentFlagRequired("org")
	c.MarkPersistentFlagRequired("env")

	c.AddCommand(provision.Cmd(rootArgs, printf, fatalf))
	c.AddCommand(bindings.Cmd(rootArgs, printf, fatalf))

	//c.AddCommand(version.CobraCommand())

	return c
}
