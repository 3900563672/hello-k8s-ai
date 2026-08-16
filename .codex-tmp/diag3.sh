#!/bin/bash
cat > /tmp/proj.json <<'JEOF'
{"query": "query { projectV2(number: 1) { items(first: 30) { nodes { id content { ... on Issue { number title } } } } } }"}
JEOF
gh api graphql --input /tmp/proj.json