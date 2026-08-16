#!/bin/bash
cat > /tmp/repo.json <<'JEOF'
{"query": "query { repository(owner: \"3900563672\", name: \"hello-k8s-ai\") { id nameWithOwner } }"}
JEOF
gh --version
echo ---
gh api graphql --input /tmp/repo.json
echo ---
gh api graphql -i --input /tmp/repo.json 2>&1 | head -20