#!/bin/bash
set -e -x

cd $(dirname $0)/..

ARCH=${DRONE_STAGE_ARCH:-$(arch)}
. ./scripts/version.sh

if [[ ! "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(\-[^\+]*)?\+k3s.+$ ]]; then
  echo "k3s version $VERSION does not match regex for rpm upload"
  exit 0
fi

TMPDIR=$(mktemp -d)
cleanup() {
  exit_code=$?
  trap - EXIT INT
  rm -rf ${TMPDIR}
  exit ${exit_code}
}
trap cleanup EXIT INT

export HOME=${TMPDIR}

BIN_SUFFIX=""
if [ ${ARCH} = aarch64 ] || [ ${ARCH} = arm64 ]; then
    BIN_SUFFIX="-arm64"
elif [ ${ARCH} = armv7l ] || [ ${ARCH} = arm ]; then
    BIN_SUFFIX="-armhf"
elif [ ${ARCH} = s390x ]; then
    BIN_SUFFIX="-s390x"
fi

# capture version of k3s
k3s_version=$(sed -E -e 's/^v([^-+]*).*$/\1/' <<< $VERSION)
# capture pre-release and metadata information of k3s
k3s_release=$(sed -E -e 's/\+k3s/+/; s/\+/-/g; s/^[^-]*//; s/^--/dev-/; s/-+/./g; s/^\.+//; s/\.+$//;' <<< $VERSION)
# k3s-selinux policy version needed for functionality
k3s_policyver=0.1-1

rpmbuild \
  --define "k3s_version ${k3s_version}" \
  --define "k3s_release ${k3s_release}" \
  --define "k3s_policyver ${k3s_policyver}" \
  --define "k3s_binary k3s${BIN_SUFFIX}" \
  --define "_sourcedir ${PWD}" \
  --define "_specdir ${PWD}" \
  --define "_builddir ${PWD}" \
  --define "_srcrpmdir ${PWD}" \
  --define "_rpmdir ${PWD}/dist/rpm" \
  --define "_buildrootdir ${PWD}/.rpm-build" \
  -bb package/rpm/k3s.spec

if ! grep "BEGIN PGP PRIVATE KEY BLOCK" <<<"$PRIVATE_KEY"; then
  echo "PRIVATE_KEY not defined, skipping rpm sign and upload"
  exit 0
fi

cat <<\EOF >~/.rpmmacros
%_signature gpg
%_gpg_name ci@rancher.com
EOF
gpg --import - <<<"$PRIVATE_KEY"

expect <<EOF
set timeout 60
spawn sh -c "rpmsign --addsign dist/rpm/**/k3s-*.rpm"
expect "Enter pass phrase:"
send -- "$PRIVATE_KEY_PASS_PHRASE\r"
expect eof
lassign [wait] _ _ _ code
exit \$code
EOF

if [ -z "$AWS_S3_BUCKET" ]; then
  echo "AWS_S3_BUCKET skipping rpm upload"
  exit 0
fi

rpm-s3 --bucket $AWS_S3_BUCKET dist/rpm/**/k3s-*.rpm
