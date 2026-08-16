#!/bin/bash
gh api graphql --input /tmp/schema.json --jq '.data.__type.fields | length'
gh api graphql --input /tmp/schema.json --jq '.data.__type.fields[].name'