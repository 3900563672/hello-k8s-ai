#!/bin/bash
gh issue view 17 --json title,body --jq '"TITLE: " + .title + "\n\nBODY:\n" + .body'