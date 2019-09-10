# K3S Load Testing

This directory contains tooling to help spin up k3s clusters for scale testing (load testing the k3s server).

## Usage

From inside the `server` directory, run the following commands:

```
cd server
terraform init
terraform apply
```

You will be asked to specify an instance type for the k3s server. For a `large` cluster test, use a `c4.8xlarge`. For a `small` cluster test, use a `t3.micro`.
To run these commands, you will need access to an AWS account (configured here by default will be the `rancher-eng` account).

When the server terraform completes, go to the `pool` directory and run:

```
cd pool
terraform init
terraform apply
```

You will be asked to specify how many ec2 instances to create (variable is `node_count`). You can also specify the `k3s_per_node` and `worker_instance_type` variables when you run apply to override these defaults.

For the `large` cluster test, you will want to specify `node_count=100`. That will get you 100 ec2 instances with 10 k3s agents each - for a total of 1000 nodes in your k3s cluster.

For the `small` test, `node_count=1`, and you will override `k3s_per_node=5`. That will get you 1 ec2 instance with 5 agents on it - for a total of 5 nodes in your k3s cluster.


Once `pool` is finished, you can run through the cluster-loader scenarios using the `run-test.sh` script in the corresponding directory (depending on whether you are running the `large` or `small` scenario).

```
cd cluster-loader/<small/large>
./run-test.sh
```

* The `run-test.sh` script assumes you have [cluster-loader](https://github.com/kubernetes/perf-tests/tree/master/clusterloader2) installed on your machine.


### TODO

* Investigate cluster-loader failures.
* Simplify this process.
* Organized reporting on SLOs after cluster-loader is complete.
