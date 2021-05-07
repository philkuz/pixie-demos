#!/bin/bash -e
cd "$(dirname "$0")"
# Enable for testing
# go build .
SLACKBOT=./slackbot
sops -d config.json | ${SLACKBOT}
