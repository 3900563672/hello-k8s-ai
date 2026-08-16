#!/bin/bash
gh issue list --state all --limit 40 --json number,state,title --jq '.[] | select(.number >= 15 and .number <= 22) | [.number, .state, .title] | @tsv'