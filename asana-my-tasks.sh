#!/usr/bin/env bash
#
# Copyright 2024 Ross Light
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

if [[ -z "${ASANA_ACCESS_TOKEN:-}" ]]; then
  echo 'asana-my-tasks: please set ASANA_ACCESS_TOKEN before running.' >&2
  exit 1
fi

asana_curl() {
  local output
  if output="$(curl \
    --header 'accept: application/json' \
    --header "authorization: Bearer $ASANA_ACCESS_TOKEN" \
    --fail-with-body \
    --silent \
    --show-error \
    --location \
    "$@")"; then
    echo "$output"
  else
    local message
    message="$(echo "$output" | jq --raw-output '.errors[] | .message')"
    echo "asana-my-tasks: $message" >&2
    return 1
  fi
}

user_response="$(asana_curl --request GET --url https://app.asana.com/api/1.0/users/me)"
user_workspace_count="$(echo "$user_response" | jq --raw-output '.data.workspaces | length')"
if [[ "$user_workspace_count" -ne 1 ]]; then
  echo "asana-my-tasks: user has $user_workspace_count workspaces" >&2
  exit 1
fi
user_workspace_gid="$(echo "$user_response" | jq --raw-output '.data.workspaces[0].gid')"

user_task_list_gid="$(asana_curl \
  --request GET \
  --url "https://app.asana.com/api/1.0/users/me/user_task_list?workspace=${user_workspace_gid}" | \
  jq --raw-output '.data.gid')"

task_list_url="https://app.asana.com/api/1.0/user_task_lists/${user_task_list_gid}/tasks?limit=100&completed_since=now&opt_fields=name,created_at,due_at,due_on,notes"
while [[ -n "$task_list_url" ]] ; do
  page_response="$(asana_curl --request GET --url "$task_list_url")"
  echo "$page_response" | jq --compact-output '.data[]'
  task_list_url="$(echo "$page_response" | jq --raw-output '.next_page | select(. != null) | .uri')"
done
