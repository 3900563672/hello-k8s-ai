#!/bin/bash
env | grep -iE "gh_|github|graphql" 
echo ---
gh api graphql --input /tmp/v.json 2>&1 | head -5
echo ---
cat /tmp/v.json
echo ---
gh auth status 2>&1 | head -5