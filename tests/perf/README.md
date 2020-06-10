# K3S Performance Tests

---

These scripts uses Terraform to automate building and testing on k3s clusters on AWS, it supports building normal and HA clusters with N master nodes, N workers nodes and multiple storage backends including:

- MySQL RDS
- Postgres RDS
- Etcd
- SQlite

The scripts divides into three sections:

- server
- agents
- tests

## Server

The server section deploys the storage backend and then deploys N master nodes, the scripts can be customized to use HA mode or use a single node cluster with sqlite backend, it can also support using 1 master node with external DB, the scripts can also be customized to specify instance type and k3s version, all available options are described in the variable section below.

The server section will also create a one or more agent nodes specifically for Prometheus deployment, clusterloader2 will deploy prometheus and grafana.

## Agents

The agents section deploys the k3s agents, it can be customized with different options that controls the agent node count and the instance types.

## Tests

The tests section uses a fork off the [clusterloader2](https://github.com/kubernetes/perf-tests/tree/master/clusterloader2) tool, the fork just modifies the logging and removes the etcd metrics probes.

this section will use a dockerized version of the tool, which will run the tests and save the report in `tests/<test_name>-<random-number>`.

The current available tests are:

- load test
- density test

## Variables

The scripts can be modified by customizing the variables in `scripts/config`, the variables includes:

### Main Vars

|       Name       |                                   Description                                  |
|:----------------:|:------------------------------------------------------------------------------:|
|   CLUSTER_NAME   |     The cluster name on aws, this will prefix each component in the cluster    |
|    DOMAIN_NAME   |                 DNS name of the Loadbalancer for k3s master(s)                 |
|      ZONE_ID     |                 AWS route53 zone id for modifying the dns name                 |
|    K3S_VERSION   |                 K3S version that will be used with the cluster                 |
|  EXTRA_SSH_KEYS  |                Public ssh keys that will be added to the servers               |
| PRIVATE_KEY_PATH | Private ssh key that will be used by clusterloader2 to ssh and collect metrics |
|       DEBUG      |                           Debug mode for k3s servers                           |

### Database Variables

|       Name       |                                             Description                                             |
|:----------------:|:---------------------------------------------------------------------------------------------------:|
|     DB_ENGINE    |                    The database type, this can be "mysql", "postgres", or "etcd"                    |
| DB_INSTANCE_TYPE | The RDS instance type for mysql and postgres, etcd uses db.* class as well as its parsed internally |
|      DB_NAME     |                           Database name created only in postgres and mysql                          |
|    DB_USERNAME   |                        Database username created only for postgres and mysql                        |
|    DB_PASSWORD   |                  Database password for the user created only for postgres and mysql                 |
|    DB_VERSION    |                                           Database version                                          |

### K3S Server Variables

|         Name         |                                    Description                                    |
|:--------------------:|:---------------------------------------------------------------------------------:|
|       SERVER_HA      | Whether or not to use HA mode, if not then sqlite will be used as storage backend |
|     SERVER_COUNT     |                               k3s master node count                               |
| SERVER_INSTANCE_TYPE |                    Ec2 instance type created for k3s server(s)                    |

### K3S Agent Variables

|         Name        |                Description                |
|:-------------------:|:-----------------------------------------:|
|   AGENT_NODE_COUNT  | Number of k3s agents that will be created |
| AGENT_INSTANCE_TYPE |  Ec2 instance type created for k3s agents |

### Prometheus server Variables

|            Name           |                             Description                             |
|:-------------------------:|:-------------------------------------------------------------------:|
|   PROM_WORKER_NODE_COUNT  | Number of k3s agents that will be created for prometheus deployment |
| PROM_WORKER_INSTANCE_TYPE |         Ec2 instance type created for k3s prometheus agents         |

## Usage

### build

The script includes a Makefile that run different sections, to build the master and workers, adjust the config file in `tests/perf/scripts/config` and then use the following:

```bash
cd tests/perf
make apply
```

This will basically build the db, server, and agent layers, it will also deploy a kubeconfig file in tests/kubeconfig.yaml.

### test

To start the clusterloader2 load test you can modify the tests/perf/tests/load/config.yaml and then run the following:

```bash
cd tests/perf
make test
```

### destroy

To destroy the cluster just run the following:

```bash
make destroy
make clean
```

