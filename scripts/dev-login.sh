#!/bin/bash
set -e

cd $(dirname $0)/../bin

rio login -s https://localhost:5443 -t $(<${HOME}/.rancher/rio/server/client-token)
