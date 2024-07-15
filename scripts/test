#!/bin/bash
set -e -x
cd $(dirname $0)/..

. ./scripts/version.sh
. ./tests/docker/test-helpers

# sysctl commands
sysctl -w fs.inotify.max_queued_events=16384
sysctl -w fs.inotify.max_user_instances=8192
sysctl -w fs.inotify.max_user_watches=524288
sysctl -w user.max_inotify_instances=8192
sysctl -w user.max_inotify_watches=524288

artifacts=$(pwd)/dist/artifacts
mkdir -p $artifacts

# ---

docker ps

# ---
# Only run basic tests on non amd64 archs, we use GitHub Actions for amd64
if [ "$ARCH" != 'amd64' ]; then

  . ./tests/docker/test-run-basics
  echo "Did test-run-basics $?"

  . ./tests/docker/test-run-cacerts
  echo "Did test-run-cacerts $?"

  . ./tests/docker/test-run-compat
  echo "Did test-run-compat $?"

  . ./tests/docker/test-run-bootstraptoken
  echo "Did test-run-bootstraptoken $?"

  . ./tests/docker/test-run-upgrade
  echo "Did test-run-upgrade $?"

  . ./tests/docker/test-run-lazypull
  echo "Did test-run-lazypull $?"
fi



. ./tests/docker/test-run-hardened
echo "Did test-run-hardened $?"

. ./tests/docker/test-run-etcd
echo "Did test-run-etcd $?"

# ---

[ "$ARCH" != 'amd64' ] && \
  early-exit "Skipping remaining tests, images not available for $ARCH."

# ---

if [ "$DRONE_BUILD_EVENT" = 'tag' ]; then
  E2E_OUTPUT=$artifacts test-run-sonobuoy serial
  echo "Did test-run-sonobuoy serial $?"
  E2E_OUTPUT=$artifacts test-run-sonobuoy parallel
  echo "Did test-run-sonobuoy parallel $?"
  early-exit 'Skipping remaining tests on tag.'
fi
# ---

if [ "$DRONE_BUILD_EVENT" = 'cron' ]; then
  E2E_OUTPUT=$artifacts test-run-sonobuoy serial
  echo "Did test-run-sonobuoy serial $?"
  test-run-sonobuoy etcd serial
  echo "Did test-run-sonobuoy-etcd serial $?"
  test-run-sonobuoy mysql serial
  echo "Did test-run-sonobuoy-mysqk serial $?"
  test-run-sonobuoy postgres serial
  echo "Did test-run-sonobuoy-postgres serial $?"

  # Wait until all serial tests have finished
  delay=15
  (
  set +x
  while [ $(count-running-tests) -ge 1 ]; do
      sleep $delay
  done
  )

  E2E_OUTPUT=$artifacts test-run-sonobuoy parallel
  echo "Did test-run-sonobuoy parallel $?"
  test-run-sonobuoy etcd parallel
  echo "Did test-run-sonobuoy-etcd parallel $?"
  test-run-sonobuoy mysql parallel
  echo "Did test-run-sonobuoy-mysql parallel $?"
  test-run-sonobuoy postgres parallel
  echo "Did test-run-sonobuoy-postgres parallel $?"
fi

# Wait until all tests have finished
delay=15
(
set +x
while [ $(count-running-tests) -ge 1 ]; do
    sleep $delay
done
)

exit 0
