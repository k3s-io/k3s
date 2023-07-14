#/bin/sh

set -e 

if [ -z $1 ] && [ -z $2 ]; then
    echo "error: image name and arch name are required as arguments. exiting..."
    exit 1
fi

ARCH=$2

# skipping image scan for 32 bits image since trivy dropped support for those https://github.com/aquasecurity/trivy/discussions/4789
if  [[ "${ARCH}" = "arm" ]] || [ "${ARCH}" != "386" ]; then
    exit 0
fi

if [ -n ${DEBUG} ]; then
    set -x
fi



IMAGE=$1
SEVERITIES="HIGH,CRITICAL"

trivy --quiet image --severity ${SEVERITIES}  --no-progress --ignore-unfixed ${IMAGE}

exit 0
