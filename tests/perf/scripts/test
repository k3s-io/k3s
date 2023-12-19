#!/bin/bash -ex

test_load() {
  source scripts/config
  eval PRIVATE_KEY_PATH=$PRIVATE_KEY_PATH
  EXPANDED_PRIV_KEY_PATH=$(readlink -f $PRIVATE_KEY_PATH)
  masterips=$(terraform output -state=server/server.tfstate | grep k3s_server_ips | cut -d "=" -f 2)
  pushd tests/
  docker run -v $EXPANDED_PRIV_KEY_PATH:/opt/priv_key \
             -e KUBE_SSH_USER=ubuntu \
             -e LOCAL_SSH_KEY=/opt/priv_key \
             -it -v $PWD/:/opt/k3s/perf-tests husseingalal/clusterloader:dev \
             clusterloader --testconfig /opt/k3s/perf-tests/load/config.yaml \
             --kubeconfig /opt/k3s/perf-tests/kubeconfig.yaml  \
             --masterip $masterips \
             --provider=local  \
             --report-dir /opt/k3s/perf-tests/load_tests_results-$RANDOM \
             --enable-prometheus-server \
             --tear-down-prometheus-server=0
  popd
}

test_density() {
  source scripts/config
  eval PRIVATE_KEY_PATH=$PRIVATE_KEY_PATH
  EXPANDED_PRIV_KEY_PATH=$(readlink -f $PRIVATE_KEY_PATH)
  masterips=$(terraform output -state=server/server.tfstate | grep k3s_server_ips | cut -d "=" -f 2)
  pushd tests/
  docker run -e KUBE_SSH_USER=ubuntu \
             -v $EXPANDED_PRIV_KEY_PATH:/opt/priv_key \
             -e LOCAL_SSH_KEY=/opt/priv_key \
             -it -v $PWD/:/opt/k3s/perf-tests husseingalal/clusterloader:dev \
             clusterloader --testconfig /opt/k3s/perf-tests/density/config.yaml \
             --kubeconfig /opt/k3s/perf-tests/kubeconfig.yaml  \
             --masterip $masterips \
             --provider=local  \
             --report-dir /opt/k3s/perf-tests/density_tests_results-$RANDOM \
             --enable-prometheus-server \
             --tear-down-prometheus-server=0
  popd
}

clean() {
  # clean kubeconfig
  pushd tests/
  rm -f kubeconfig
  rm -rf load_tests_results*
  rm -rf density_tests_results/
  popd
}

$@
