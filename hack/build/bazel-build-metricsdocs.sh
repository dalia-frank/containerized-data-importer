#!/bin/bash
#
# This file is part of the KubeVirt project
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
#
# Copyright 2022 Red Hat, Inc.
#

set -e

source hack/build/common.sh
source hack/build/config.sh

rm -rf ${CMD_OUT_DIR}
mkdir -p ${CMD_OUT_DIR}/dump

# Build all binaries for amd64
bazel build \
    --verbose_failures \
    --config=${ARCHITECTURE} \
    //pkg/monitoring/tools/metricsdocs/...

rm -rf _out/pkg/monitoring/tools/metricsdocs
mkdir -p _out/pkg/monitoring/tools/metricsdocs
cp ./bazel-bin/pkg/monitoring/tools/metricsdocs/metricsdocs_/metricsdocs _out/pkg/monitoring/tools/metricsdocs/

bazel clean
