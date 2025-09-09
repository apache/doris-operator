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
package get

import (
    "encoding/json"
    "fmt"
    "github.com/apache/doris-operator/pkg/common/cmd/templates"
    cmdtypes "github.com/apache/doris-operator/pkg/common/cmd/types"
    "github.com/apache/doris-operator/pkg/common/cmd/util"
    "github.com/spf13/cobra"
    "github.com/tidwall/gjson"
    "io"
    "strings"
)

//GetOptions contains the input to the get command.
type GetOptions struct {
    out          io.Writer
    CmdParent    string
    dc           *cmdutil.DorisConfig
    OutputFormat string
}

//NewGetOptions returns  GetOptions.
func NewGetOptions(parent string, dc *cmdutil.DorisConfig, out io.Writer) *GetOptions {
    return &GetOptions{
        CmdParent: parent,
        out:       out,
        dc:        dc,
    }
}

//Run performs the get operation from doris.
func (o *GetOptions) Run(args []string) {
    if len(args) != 2 {
        fmt.Fprintf(o.out, "You must specify the type of resource to get. %s\n", cmdutil.SuggestAPIResources(o.CmdParent))
        return
    }

    metaType := args[0]
    node := args[1]
    switch metaType {
    case "node":
        o.getNode(node)
        return
    case "computegroup":

    default:
        fmt.Fprintf(o.out, "The type %s not supported.\n", metaType)
        return
    }
}

func (o *GetOptions) getComputeGroup(computeGroupName string) {

}

//getNode get the node details information.
func (o *GetOptions) getNode(node string) {
    c, err := cmdutil.NewDorisClient(o.dc)
    if err != nil {
        fmt.Fprintf(o.out, "%s\n", err.Error())
        return
    }

    frontends, err := c.ShowFrontends()
    if err != nil {
        fmt.Fprintf(o.out, "%s\n", err.Error())
        return
    }
    for _, f := range frontends {
        if f.Host == node {
            o.print(f)
            return
        }
    }

    backends, err := c.ShowBackends()
    if err != nil {
        fmt.Fprintf(o.out, "%s\n", err.Error())
        return
    }

    for _, b := range backends {
        if b.Host == node {
            o.print(b)
            return
        }
    }
}

func (o *GetOptions) print(i interface{}) {
    bytes, err := json.Marshal(i)
    if err != nil {
        fmt.Fprintf(o.out, "%s\n", err.Error())
        return
    }

    if strings.Contains(o.OutputFormat, "custom-columns") {
        a := strings.Split(o.OutputFormat, "=")
        if len(a) != 2 {
            fmt.Fprintf(o.out, "custom-columns not follow the format \"custom-columns=tags.compute_group_name\"%s\n", o.OutputFormat)
            return
        }

        column := a[1]
        //the tag is string, unmarshal to struct.
        if strings.Contains(column, "tag") {
            b := i.(*cmdtypes.Backend)
            tv := b.Tag
            var t cmdtypes.Tag
            err := json.Unmarshal([]byte(tv), &t)
            if err != nil {
                fmt.Fprintf(o.out, "unmarshal tag failed,%s\n", err.Error())
                return
            }
            subColumn,_ := strings.CutPrefix(column, "tag.")
            if subColumn != "" {
                t_str, _ := json.Marshal(&t)
                subV := gjson.GetBytes(t_str, subColumn)
                fmt.Fprintf(o.out, "%s\n", subV)
                return
            }
        } else {
            v := gjson.GetBytes(bytes, column)
            fmt.Fprintf(o.out, "%s\n", v.String())
        }
        return
    }

    f_bytes, err := json.MarshalIndent(i, "", "    ")
    if err != nil {
        fmt.Fprintf(o.out, "%s\n", err.Error())
        return
    }
    fmt.Fprintf(o.out, "%s\n", f_bytes)
}

var (
    getLong = templates.LongDesc(`
        Display one or many resource's meta information.

        Prints a table of the most information  about the specified resources. 
    `)

    getExample = templates.Examples(`
        # get compute group name.
        dorisctl get backend xxx  -o custom-columns=tags.compute_group_name
    `)
)

//NewCmdGet creates a command object for the generic "get" action.
func NewCmdGet(parent string, dc *cmdutil.DorisConfig, out io.Writer) *cobra.Command {
    o := NewGetOptions(parent, dc, out)
    cmd := &cobra.Command{
        Use:                   fmt.Sprintf("get [(-o|--output=)%s] (TYPE[.GROUP][.VERSION] [NAME| -l label]) [flags]", "json|yaml"),
        DisableFlagsInUseLine: true,
        Short:                 "Display one or many resources.",
        Long:                  getLong + "\n\n" + cmdutil.SuggestAPIResources(parent),
        Example:               getExample,
        Run: func(_ *cobra.Command, args []string) {
            o.Run(args)
        },
    }

    cmd.Flags().StringVarP(&o.OutputFormat, "output", "o", o.OutputFormat, "output format.")

    return cmd
}
