#!/bin/bash

#debug options
# set -x
# PS4='+(${LINENO}): '

# shellcheck disable=SC2162
read -e -p "Enter the Test case name:"  TEST_CASE
read -e -p "Enter your arguments for commands:  " CMD_HOST CMD_NODE
read -e -p "Enter your arguments for expected values:  " EXP_HOST EXPUP_HOST EXP_NODE EXPUP_NODE
read -e -p "Enter your version or commit to upgrade on this format: (INSTALL_K3S_VERSION=X || INSTALL_K3S_COMMIT=xxxxx): " VERSION

if [[ -z "$TEST_CASE"  ]]; then
echo "please provide a non-empty or valid test case"
fi

echo "$VERSION"
if [[ "$TEST_CASE" == "upgradecluster" ]]; then
make test-upgrade-manual INSTALLTYPE="$VERSION"
fi
#INSTALL_K3S_COMMIT=257fa2c54cda332e42b8aae248c152f4d1898218
