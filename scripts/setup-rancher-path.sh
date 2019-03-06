#!/bin/bash

if [ $(id -u) = 0 ]; then
    RANCHER_PATH="/var/lib/rancher"
else
    RANCHER_PATH="$HOME/.rancher"
fi
