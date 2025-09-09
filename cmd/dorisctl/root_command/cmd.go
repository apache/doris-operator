// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
package root_command

import (
	"io"

	"github.com/apache/doris-operator/pkg/common/cmd/get"
	"github.com/apache/doris-operator/pkg/common/cmd/templates"
	cmdutil "github.com/apache/doris-operator/pkg/common/cmd/util"
	"github.com/spf13/cobra"
)

func NewDorisctlCommand(out io.Writer) (*cobra.Command, error) {
	cmds := &cobra.Command{
		Use:   "dorisctl",
		Short: "dorisctl controls the doris cluster manager",
		Long: templates.LongDesc(`
        dorisctl controls the doris cluster manager.
        `),
	}

	var dc cmdutil.DorisConfig

	flags := cmds.PersistentFlags()
	flags.StringVar(&dc.FeHost, "fe-host", "", "The fe access address.")
	flags.StringVar(&dc.User, "user", "", "The name of user to access doris.")
	flags.StringVar(&dc.Password, "password", "", "The password of login in doris.")
	flags.IntVar(&dc.QueryPort, "query-port", 9030, "The FE mysql protocol listen port")
	flags.StringVar(&dc.SSLCaPath, "ssl-ca", "", "the root certificate path.")
	flags.StringVar(&dc.SSLCrtPath, "ssl-cert", "", "the client certificate path.")
	flags.StringVar(&dc.SSLKeyPath, "ssl-key", "", "the client private key path")
	groups := templates.CommandGroups{
		{
			Message: "Basic Commands (Beginner):",
			Commands: []*cobra.Command{
				get.NewCmdGet("dorisctl", &dc, out),
			},
		},
	}

	groups.Add(cmds)
	return cmds, nil
}
