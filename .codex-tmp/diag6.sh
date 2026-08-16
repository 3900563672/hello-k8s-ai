#!/bin/bash
cat > /tmp/p2.json <<'JEOF'
{"query": "query { viewer { projectsV2(first: 10) { nodes { id number title url } } } }"}
JEOF
gh api graphql --input /tmp/p2.json