#!/bin/bash
cat > /tmp/items.json <<'JEOF'
{"query": "query { node(id: \"PVT_kwHODN0KGM4BgfyL\") { ... on ProjectV2 { items(first: 30) { nodes { id content { ... on Issue { number title } } } } } } }"}
JEOF
gh api graphql --input /tmp/items.json --jq '.data.node.items.nodes[] | [.content.number, .id] | @tsv'