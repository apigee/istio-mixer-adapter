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
	"github.com/apigee/istio-mixer-adapter/apigee-istio/cmd/token"
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

	var addCommand = func(cmds ...*cobra.Command) {
		for _, subC := range cmds {
			// add general flags
			subC.PersistentFlags().StringVarP(&rootArgs.RouterBase, "routerBase", "r",
				shared.DefaultRouterBase, "Apigee router base")
			subC.PersistentFlags().StringVarP(&rootArgs.ManagementBase, "managementBase", "m",
				shared.DefaultManagementBase, "Apigee management base")
			subC.PersistentFlags().BoolVarP(&rootArgs.Verbose, "verbose", "v",
				false, "verbose output")
			subC.PersistentFlags().StringVarP(&rootArgs.NetrcPath, "netrc", "n",
				"", "Path to a .netrc file to use (default is $HOME/.netrc")

			subC.PersistentFlags().StringVarP(&rootArgs.Org, "org", "o",
				"", "Apigee organization name")
			subC.PersistentFlags().StringVarP(&rootArgs.Env, "env", "e",
				"", "Apigee environment name")
			subC.PersistentFlags().StringVarP(&rootArgs.Username, "username", "u",
				"", "Apigee username")
			subC.PersistentFlags().StringVarP(&rootArgs.Password, "password", "p",
				"", "Apigee password")

			subC.MarkPersistentFlagRequired("org")
			subC.MarkPersistentFlagRequired("env")

			c.AddCommand(subC)
		}
	}

	addCommand(provision.Cmd(rootArgs, printf, fatalf))
	addCommand(bindings.Cmd(rootArgs, printf, fatalf))
	addCommand(token.Cmd(rootArgs, printf, fatalf))

	c.AddCommand(version(printf))

	return c
}

func version(printf shared.FormatFn) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Prints build version information",
		Run: func(cmd *cobra.Command, args []string) {
			printf("apigee-istio version %s %s [%s]",
				shared.BuildInfo.Version, shared.BuildInfo.Date, shared.BuildInfo.Commit)
		},
	}
}
