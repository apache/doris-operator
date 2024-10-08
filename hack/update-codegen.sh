#!/usr/bin/env bash
# Licensed to the Apache Software Foundation (ASF) under one
# or more contributor license agreements.  See the NOTICE file
# distributed with this work for additional information
# regarding copyright ownership.  The ASF licenses this file
# to you under the Apache License, Version 2.0 (the
# "License"); you may not use this file except in compliance
# with the License.  You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.

#
# Copyright 2017 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.


set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
CODEGEN_PKG=${CODEGEN_PKG:-$(cd "${SCRIPT_ROOT}"; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../hack)}

source "${CODEGEN_PKG}/kube_codegen.sh"

THIS_PKG="github.com/apache/doris-operator"

kube::codegen::gen_helpers \
    --boilerplate "${SCRIPT_ROOT}/hack/boilerplate.go.txt" \
    "${SCRIPT_ROOT}/pkg/apis"

kube::codegen::gen_client \
    --with-watch \
    --output-dir "${SCRIPT_ROOT}/client" \
    --output-pkg "${THIS_PKG}/client" \
    --boilerplate "${SCRIPT_ROOT}/hack/boilerplate.go.txt" \
    "${SCRIPT_ROOT}/api"

#set -o errexit
#set -o nounset
#set -o pipefail
#
#
#SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
#
#source "kube_codegen.sh"
#kube::codegen::gen_helpers \
#    --input-pkg-root github.com/intelligentfu8/doris-operator/api \
#    --output-base "$(dirname "${BASH_SOURCE[0]}")/../../../.." \
#    --boilerplate "${SCRIPT_ROOT}/hack/boilerplate.go.txt"
#
#kube::codegen::gen_client \
#    --with-watch \
#    --input-pkg-root github.com/intelligentfu8/doris-operator/api \
#    --output-pkg-root github.com/intelligentfu8/doris-operator/client \
#    --output-base "$(dirname "${BASH_SOURCE[0]}")/../../../.." \
#    --boilerplate "${SCRIPT_ROOT}/hack/boilerplate.go.txt"
#
#kube::codegen::gen_helpers \
#    --input-pkg-root github.com/apache/doris-operator/api \
#    --output-base "$(dirname "${BASH_SOURCE[0]}")/../../../.." \
#    --boilerplate "${SCRIPT_ROOT}/hack/boilerplate.go.txt"
#
#kube::codegen::gen_client \
#    --with-watch \
#    --input-pkg-root github.com/selectdb/doris-operator/api \
#    --output-pkg-root github.com/selectdb/doris-operator/client \
#    --output-base "$(dirname "${BASH_SOURCE[0]}")/../../../.." \
#    --boilerplate "${SCRIPT_ROOT}/hack/boilerplate.go.txt"
