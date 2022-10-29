#/bin/sh

set -e 

if [ -z $1 ] && [ -z $2 ]; then
    echo "error: image name and arch name are required as arguments. exiting..."
    exit 1
fi

ARCH=$2

# skipping image scan for s390x since trivy doesn't support s390x arch yet
if [ "${ARCH}" == "s390x" ]; then
    exit 0
fi

if [ -n ${DEBUG} ]; then
    set -x
fi



IMAGE=$1
SEVERITIES="HIGH,CRITICAL"

trivy --quiet image --severity ${SEVERITIES}  --no-progress --ignore-unfixed ${IMAGE}

exit 0
