// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
package dorisctl

import (
    "github.com/apache/doris-operator/pkg/common/utils/templates"
    "github.com/spf13/cobra"
    "k8s.io/cli-runtime/pkg/genericclioptions"
    "k8s.io/kubectl/pkg/util/i18n"
)

type DorisctlOptions struct {
    Arguments []string
    genericclioptions.IOStreams
}

func NewDorisctlCommand(o DorisctlOptions) *cobra.Command {
    cmds := &cobra.Command{
        Use: "dorisctl",
        Short: i18n.T("dorisctl controls the doris cluster manager"),
        Long: templates.LongDesc(`
        dorisctl controls the doris cluster manager.
        `),
        Run: runHelp,

    }

    return cmds
}

func runHelp(cmd *cobra.Command, args []string) {
    cmd.Help()
}
