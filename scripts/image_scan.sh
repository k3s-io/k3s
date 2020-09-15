#/bin/sh

set -e 

if [ -n ${DEBUG} ]; then
    set -x
fi

if [ -z $1 ]; then
    echo "error: image name required as argument. exiting..."
    exit 1
fi

IMAGE=$1
SEVERITIES="HIGH,CRITICAL"

docker container run --rm --name=image-scan --volume /var/run/docker.sock:/var/run/docker.sock \
    docker.io/aquasec/trivy:0.10.2 --quiet image --severity ${SEVERITIES}  --no-progress --ignore-unfixed ${IMAGE}

exit 0
