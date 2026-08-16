#!/bin/bash
set -e
cat > /tmp/move.json <<'JEOF'
{"query": "mutation($projectId: ID!, $itemId: ID!, $fieldId: ID!, $optionId: String!) { updateProjectV2ItemFieldValue(input: {projectId: $projectId, itemId: $itemId, fieldId: $fieldId, value: {singleSelectOptionId: $optionId}}) { projectV2Item { id } } }", "variables": {}}
JEOF
PROJECT_ID="PVT_kwHODN0KGM4BgfyL"
FIELD_ID="PVTSSF_lAHODN0KGM4BgfyLzhffKao"
DONE_ID="98bd3af9"
for ITEM_ID in PVTI_lAHODN0KGM4BgfyLzg2tTm0 PVTI_lAHODN0KGM4BgfyLzg2tTnU PVTI_lAHODN0KGM4BgfyLzg2tTqE; do
  jq --arg p "$PROJECT_ID" --arg i "$ITEM_ID" --arg f "$FIELD_ID" --arg o "$DONE_ID" '.variables = {projectId: $p, itemId: $i, fieldId: $f, optionId: $o}' /tmp/move.json > /tmp/move-now.json
  gh api graphql --input /tmp/move-now.json --jq '.data.updateProjectV2ItemFieldValue.projectV2Item.id'
  echo "-> $ITEM_ID done"
done