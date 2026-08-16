#!/bin/bash
cat > /tmp/schema.json <<'JEOF'
{"query": "query { __type(name: \"Query\") { fields { name } } }"}
JEOF
gh api graphql --input /tmp/schema.json --jq '.data.__type.fields[].name' | grep -i -E 'project|viewer|repository' 