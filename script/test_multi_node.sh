#!/usr/bin/env bash
set -euo pipefail

command -v jq >/dev/null 2>&1 || {
    echo "jq is required"
    exit 1
}

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
config_file=$(mktemp)
trap 'rm -f "$config_file" "${config_file}.bak"' EXIT

eval "$(sed -n '/^list_nodes() {/,/^}/p' "${script_dir}/zicnode.sh")"
eval "$(sed -n '/^add_node() {/,/^}/p' "${script_dir}/zicnode.sh")"

red=""
green=""
yellow=""
plain=""
export ZICNODE_CONFIG_FILE="$config_file"

cat > "$config_file" <<'EOF'
{"Log":{"Level":"error"},"Nodes":[{"ApiHost":"https://panel.example.com/","NodeID":1,"ApiKey":"first-secret","Timeout":15}]}
EOF

api_key='a"b\c&$special'

curl() {
    printf '%s' '{"protocol":"vless","base_config":{"panel":"zicboard","node_type":"zicnode"}}'
}

restart() {
    return 0
}

printf '%s\n' "https://panel.example.com/" "2" "$api_key" | add_node >/dev/null

jq -e --arg api_key "$api_key" '
    (.Nodes | length) == 2 and
    .Nodes[0].NodeID == 1 and
    .Nodes[1].ApiKey == $api_key
' "$config_file" >/dev/null

if printf '%s\n' "https://panel.example.com///" "2" "$api_key" | add_node >/dev/null; then
    echo "duplicate node was accepted"
    exit 1
fi

restart_count=0
restart() {
    restart_count=$((restart_count + 1))
    [[ $restart_count -gt 1 ]]
}

if printf '%s\n' "https://panel.example.com/" "3" "$api_key" | add_node >/dev/null; then
    echo "failed restart unexpectedly succeeded"
    exit 1
fi

jq -e '(.Nodes | length) == 2 and all(.Nodes[]; .NodeID != 3)' "$config_file" >/dev/null

masked=$(list_nodes)

[[ "$masked" != *"$api_key"* ]]
echo "multi-node config checks passed"
