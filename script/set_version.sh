#!/bin/bash

# Licensed to the Apache Software Foundation (ASF) under one or more
# contributor license agreements.  See the NOTICE file distributed with
# this work for additional information regarding copyright ownership.
# The ASF licenses this file to You under the Apache License, Version 2.0
# (the "License"); you may not use this file except in compliance with
# the License.  You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e

if [ "$#" -lt 2 ] || [ "$#" -gt 3 ]; then
    echo "usage: $0 version snapshot_version [image_name]"
    exit 1
fi

location=$(dirname $0)
version=$1
snapshot_version=$2
image_name=${3:-docker.io\/yaks\/yaks}
sanitized_image_name=${image_name//\//\\\/}

# Update olm-catalog

for f in $(find $location/../deploy -type f -name "*.yaml" | grep -v olm-catalog);
do
  if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    sed -i -r "s/docker.io\/yaks\/yaks:([0-9]+[a-zA-Z0-9\-\.].*).*/${sanitized_image_name}:${version}/" $f
  elif [[ "$OSTYPE" == "darwin"* ]]; then
    # Mac OSX
    sed -i '' -E "s/docker.io\/yaks\/yaks:([0-9]+[a-zA-Z0-9\-\.].*).*/${sanitized_image_name}:${version}/" $f
  fi
done

# Update Java sources

java_sources=${location}/../java

blacklist=("./java/.mvn/wrapper" "./java/.idea" ".DS_Store" "/target/")

find ${java_sources} -type f -print0 | while IFS= read -r -d '' file; do
  check=true
  for b in ${blacklist[*]}; do
    if [[ "$file" == *"$b"* ]]; then
      #echo "skip $file"
      check=false
    fi
  done
  if [ "$check" = true ]; then
    if [[ "$OSTYPE" == "linux-gnu"* ]]; then
      sed -i "s/$snapshot_version/$version/g" $file
    elif [[ "$OSTYPE" == "darwin"* ]]; then
      # Mac OSX
      sed -i '' "s/$snapshot_version/$version/g" $file
    fi
  fi
done

echo "YAKS version set to: $version and image name to: $image_name:$version"
