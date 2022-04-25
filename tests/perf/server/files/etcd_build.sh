#!/bin/bash
set -x

IFS=',' read -r -a public_ips <<< "$PUBLIC_IPS"
IFS=',' read -r -a private_ips <<< "$PRIVATE_IPS"

conn_string=""
for i in "${!private_ips[@]}"; do
  conn_string=$conn_string"etcd-$i=http://${private_ips[i]}:2380,"
done
conn_string=${conn_string%?}
for i in "${!public_ips[@]}"; do
  while true; do
    ssh -i $SSH_KEY_PATH -l ubuntu ${public_ips[i]} "sudo docker run -v /etcd-data:/etcd-data -d -p ${private_ips[i]}:2379:2379 -p ${private_ips[i]}:2380:2380 quay.io/coreos/etcd:$DB_VERSION etcd --initial-advertise-peer-urls http://${private_ips[i]}:2380 --name=etcd-$i --data-dir=/etcd-data --advertise-client-urls=http://0.0.0.0:2379 --listen-peer-urls=http://0.0.0.0:2380 --listen-client-urls=http://0.0.0.0:2379 --initial-cluster-token=etcd-cluster-1 --initial-cluster-state new --initial-cluster $conn_string"
    if [ $? == 0 ]; then
      break
    fi
    sleep 10
  done
done

#
