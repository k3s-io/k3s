#!/usr/bin/env bash

# update for posix shell?

set -e

[ $(id -u) -eq 0 ] || exec sudo -E $0 $@

DIAGCMD=${1:-gather-upload-confirm}

DIAGPROG=${DIAGPROG:-k3s}
BUCKET_NAME=${BUCKET_NAME:-"$DIAGPROG-diagnostic-logs"}

bin=/var/lib/rancher/$DIAGPROG/data/current/bin/
if [ -d $bin ]; then
  export PATH=$PATH:$bin:$bin/aux
else
  for bin in /var/lib/rancher/k3s/data/**/bin/; do
    [ -d $bin ] && export PATH=$PATH:$bin:$bin/aux
  done
fi

PUBKEY=${PUBKEY:-'-----BEGIN PUBLIC KEY-----
MIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEA1SlmOKCafhG5EzqJHWnT
cEupADJ/2WgbU2PgvTG9TlbaoVyiB5AX6pGFy9hasEJtscmngLvpgY+65te0cJBo
WJ+CMa3nTFXmiX+PGbrBhWMGT5bdM9Lhx5pKvkoaHzL1nNvN/DMeusGqyIdJr3gk
1wlNHr0bZYjlUOvJ3c+X0uIyjX5y0JTjaF5AcbBMlz//zdf7beToPlPuKIlz8FZd
ff4h6dKBYpOnqJW2NBxwICD8ZVokPRRMSZvSY3Mr7HZL1gDoCkOvCsWml27xB0S6
Z6Ib8zB8PFCVWtMZxzcj7ae4tI79OHmaFkEEBAqkBNNU/9S+J0F5tz0caVVnZ+j1
fy13JKIp75vwuDxGgfaru8012QM9zLwXQOcYcHLkLbaTJJ4HpMLC/v0R7TahlLVw
3F1OtQrhQH5PFNtCecpk8SNMgFhYyuCAuWGoai3BtYMNiKFbvuakFSq/XMLFUZS9
T89FaJF2S9liz3VFfCUapBFoD4rZkFCbNufhypwnSVq6MRe1k9V5EaYIsUpfJs33
mpKDVuU/yWwYM+bnlJYo9Sn1QcnjqxRVhUePIActoQ0s9b1CA9NpbqTRiSn7Qxx5
dcnKK+f2NUEdQroCDeUxe2dBLfAvKTCM+c4VCEt2o2d9poSwPytd4K9VdDfiUor+
6u2c2QnLeIcdfRM4j7SmxM0CAwEAAQ==
-----END PUBLIC KEY-----'}

gen-uuid() {
  uuid=$(uuidgen 2>/dev/null)
  [ -z "$uuid" ] && uuid=$(cat /proc/sys/kernel/random/uuid 2>/dev/null)
  [ -z "$uuid" ] && uuid=$(od -x /dev/urandom | head -1 | awk '{OFS="-"; srand($6); sub(/./,"4",$5); sub(/./,substr("89ab",rand()*4,1),$6); print $2$3,$4,$5,$6,$7$8$9}')
  if [ -z "$uuid" ]; then
    echo "Unable to generate UUID" >&2
    return 1
  fi
  tr '[:lower:]' '[:upper:]' <<< "$uuid"
}
echo setup $DIAGUUID
UUID=${UUID:-$(gen-uuid)}
if [ -z "$UUID" ]; then
  echo "UUID is not set and could not be created" >&2
  exit 1
fi

no-cleanup() {
  echo
  echo "Skipping cleanup for $DIAGDIR"
}

cleanup() {
  exit_code=$?
  set +e +x +v
  trap - EXIT INT
  if [ -n "$DIAGDIR" ] && [ -d "$DIAGDIR" ]; then
    rm -rf $DIAGDIR $DIAGDIR.*
  fi
  exit $exit_code
}

setup_diagdir() {
  if [ -z "$DIAGDIR" ]; then
    DIAGDIR=$(readlink -m $(mktemp -d ${TMPDIR:-/tmp}/$DIAGPROG-diagnostics-$UUID-XXXXXXXX))
    trap cleanup INT
  fi
  trap no-cleanup EXIT

  set +e +x +v
  echo "Diagnostics location: $DIAGDIR"
}

remove_empty() {
  if [ -f "$1" ] && [ ! -s "$1" ]; then
    rm "$1"
  fi
}

run_cmd() {
  cmd="$@"
  cmd=${cmd//-/}
  cmd=${cmd// /-}
  cmd=${cmd//\//_}
  logCmdFile="$LOGDIR/$cmd.cmd.txt"
  logOutFile="$LOGDIR/$cmd.txt"
  logErrFile="$LOGDIR/$cmd.err.txt"

  if [ -f "$logCmdFile" ]; then
    echo "Error already ran: $@" >&2
    return 1
  fi

  echo "Gathering command: $@"
  echo "$@" >"$logCmdFile"
  $@ >"$logOutFile"  2>"$logErrFile"
  remove_empty "$logOutFile"
  remove_empty "$logErrFile"
  return 0
}

copy() {
  from=$1
  to=${2:-$1}
  to=${to//\//_}
  to="$LOGDIR/$to"

  if [ ! -e "$from" ]; then
    echo "Skipping copy, does not exist: $from"
    return 1
  fi
  echo "Copying: $from"
  cp --recursive --dereference $from $to
}

setup_logs() {
  export LOGDIR=$DIAGDIR/$1
  mkdir -p $LOGDIR
  echo
  echo "Using subdirectory: $1"
}

log_system()  {
  setup_logs system

  copy /etc/os-release
  run_cmd sysctl -a
  run_cmd uname -a
  run_cmd ps uax
  run_cmd dmesg
  run_cmd id
  run_cmd mount
  run_cmd df -h
  run_cmd ifconfig -a
  run_cmd netstat -ln
  run_cmd netstat -nr
  run_cmd lsof -n -P -p $(pgrep -o $DIAGPROG)
  run_cmd iptables -L
  run_cmd iptables -S
  run_cmd hostname -f
}

log_prog() {
  setup_logs $DIAGPROG

  run_cmd $DIAGPROG --version
  run_cmd $DIAGPROG check-config
  for log in /var/log/$DIAGPROG*.log; do
    copy $log
  done
  if command -v journalctl >/dev/null 2>&1; then
    for unit in $(journalctl --field _SYSTEMD_UNIT | grep "$DIAGPROG"); do
      run_cmd journalctl --unit "$unit" --no-pager
    done
  fi
  copy "/var/lib/rancher/$DIAGPROG/agent/containerd/containerd.log"

  # log cert openssl data?
}

log_kube() {
  setup_logs kube

  copy /var/log/pods # copies all pod logs
  run_cmd command -v kubectl
  run_cmd kubectl version 
  run_cmd kubectl config get-contexts
  run_cmd kubectl config current-context
  run_cmd kubectl cluster-info dump
  run_cmd kubectl get namespaces
  run_cmd kubectl get nodes
  run_cmd kubectl describe nodes
  run_cmd kubectl describe pods --all-namespaces
  run_cmd kubectl describe services --all-namespaces
  run_cmd kubectl describe daemonset --all-namespaces
  run_cmd kubectl describe deployments --all-namespaces
  run_cmd kubectl describe replicaset --all-namespaces
  run_cmd kubectl describe storageclass,pv,pvc
}

contains() {
  [ -z "${1##*$2*}" ]
}

gather() {
  log_system
  log_prog
  log_kube
}

confirm() {
  local def=${2:-'N'}
  local prompt='(y/N)'
  if [ "$def" = 'Y' ]; then
    prompt='(Y/n)'
  fi
  echo
  read -p "$1 $prompt: " -n 1 input
  echo
  if [ "$(tr '[:lower:]' '[:upper:]' <<<"$input")" = 'Y' ]; then
    return 0
  fi
  if [ -z "$input" ] && [ "$def" = 'Y' ]; then
    return 0
  fi
  return 1
}

upload() {
  echo
  echo "Prepare upload of $DIAGDIR"

  if contains "$DIAGCMD" confirm && ! confirm "Perform upload?"; then
    return 1
  fi
  trap cleanup EXIT

  local salt=${salt:-$(openssl rand -hex 8)}
  local key=${key:-$(openssl rand -hex 32)}
  local iv=${iv:-$(openssl rand -hex 16)}
  local base=$(basename $DIAGDIR)
  local dir=$(dirname $DIAGDIR)
  local tar=${TMPDIR:-/tmp}/$base.tar.gz

  echo
  echo "Creating $tar"

  tar -c -z -C $dir $base | \
    openssl enc -aes-256-cbc -S $salt -K $key -iv $iv -in /dev/stdin -out $DIAGDIR.logs.tar.gz.enc

  cat >$DIAGDIR.meta <<EOF
salt=$salt
key=$key
iv=$iv
EOF

  if contains "$DIAGCMD" nometa || ( contains "$DIAGCMD" confirm && ! confirm "Include encrypted metadata in upload?" Y ); then
    echo
    echo "Save secret metadata for log decryption:"
    cat $DIAGDIR.meta
  else
    echo "$PUBKEY" | openssl rsautl -encrypt -inkey /dev/stdin -pubin -in $DIAGDIR.meta -out $DIAGDIR.meta.enc
  fi
  (cd $dir; tar -c -z -f $tar $base.*.enc)

  echo
  echo "Uploading $tar"
  if curl --upload-file $tar "https://storage.googleapis.com/$BUCKET_NAME/"; then
    echo
    echo "Saved diagnostics log in cloud storage as: $base"
    echo
  fi
}

{
  setup_diagdir
  for cmd in gather upload; do
    if contains "$DIAGCMD" $cmd; then
      $cmd
    else
      echo
      echo "Skipping $cmd..."
    fi
  done
}

# record log of above about

exit 0
