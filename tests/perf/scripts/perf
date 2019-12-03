#!/bin/bash -ex

TERRAFORM_PLAN_CMD="terraform plan --var-file variables.tfvars --out k3s.plan"
TERRAFORM_APPLY_CMD="terraform apply k3s.plan"
TERRAFORM_DESTROY_CMD="terraform destroy --var-file variables.tfvars --force"

for bin in docker kubectl terraform; do
    type $bin >/dev/null 2>&1 || (echo "$bin is not in the path. Please make sure it is installed and in PATH."; exit 1)
done

init() {
  for i in server agents; do
    pushd $i
    terraform init
    popd
  done
}

apply() {
  # init terraform
  init
  # configure variables
  config
  # Run apply for server and agents
  for i in server agents; do
    if [ $i == "agents" ]; then
      echo "Sleeping 1 minute until server(s) is initialized"
      sleep 60
    fi
    pushd $i
    $TERRAFORM_PLAN_CMD
    $TERRAFORM_APPLY_CMD
    popd
  done
}

plan() {
  # init terraform
  config
  # Run apply for server and agents
  for i in server agents; do
    pushd $i
    $TERRAFORM_PLAN_CMD
    popd
  done
}


config() {
  source scripts/config
  pushd ./server
  eval PRIVATE_KEY_PATH=$PRIVATE_KEY_PATH
  EXPANDED_PRIV_KEY_PATH=`readlink -f $PRIVATE_KEY_PATH`
  if [ -z "$DB_PASSWORD" ]; then
    # randomize database password
    DB_PASSWORD=$(cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w 32 | head -n 1)
  fi
  if [ -z "$CLUSTER_SECRET" ]; then
    # randomize cluster secret
    CLUSTER_SECRET=$(cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w 32 | head -n 1)
  fi
cat <<MAIN > variables.tfvars
name = "${CLUSTER_NAME}"
k3s_cluster_secret = "${CLUSTER_SECRET}"
db_instance_type = "${DB_INSTANCE_TYPE}"
db_name = "${DB_NAME}"
db_username = "${DB_USERNAME}"
db_password = "${DB_PASSWORD}"
db_engine = "${DB_ENGINE}"
db_version = "${DB_VERSION}"
server_instance_type = "${SERVER_INSTANCE_TYPE}"
extra_ssh_keys = ["${EXTRA_SSH_KEYS}"]
server_count = ${SERVER_COUNT}
server_ha = ${SERVER_HA}
k3s_version = "${K3S_VERSION}"
prom_worker_node_count = ${PROM_WORKER_NODE_COUNT}
prom_worker_instance_type = "${PROM_WORKER_INSTANCE_TYPE}"
ssh_key_path = "${EXPANDED_PRIV_KEY_PATH}"
debug = ${DEBUG}
domain_name = "${DOMAIN_NAME}"
zone_id = "${ZONE_ID}"
MAIN
popd

pushd ./agents
cat <<MAIN > variables.tfvars
name = "${CLUSTER_NAME}"
extra_ssh_keys = ["${EXTRA_SSH_KEYS}"]
k3s_version = "${K3S_VERSION}"
agent_node_count = ${AGENT_NODE_COUNT}
agent_instance_type = "${AGENT_INSTANCE_TYPE}"
k3s_cluster_secret = "${CLUSTER_SECRET}"
MAIN
popd
}

clean() {
  # clean server and agents
  for i in server agents; do
    pushd $i
    rm -f *.plan *.tfvars *.tfstate*
    popd
  done
}

cleanall() {
  clean
  # clean kubeconfig
  pushd tests/
  rm -f kubeconfig
  rm -rf load_tests_results*
  rm -rf density_tests_results*
  popd
}

destroy() {
  for i in agents server; do
    pushd $i
    terraform destroy --var-file variables.tfvars --force
    popd
  done
  clean
}

info() {
  set +x
  for i in agents server; do
    pushd $i
    if [ -f $i.tfstate ]; then
      terraform output --state=$i.tfstate
    fi
    popd
  done
}

$@
