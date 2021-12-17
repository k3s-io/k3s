
# Cluster Datastore Options

The ability to run Kubernetes using a datastore other than etcd sets K3s apart from other Kubernetes distributions. This feature provides flexibility to Kubernetes operators. The available datastore options allow you to select a datastore that best fits your use case. For example:

* If your team doesn't have expertise in operating etcd, you can choose an enterprise-grade SQL database like MySQL or PostgreSQL
* If you need to run a simple, short-lived cluster in your CI/CD environment, you can use the embedded SQLite database
* If you wish to deploy Kubernetes on the edge and require a highly available solution but can't afford the operational overhead of managing a database at the edge, you can use K3s's embedded HA datastore built on top of embedded etcd.

K3s supports the following datastore options:

* Embedded [SQLite](https://www.sqlite.org/index.html)
* [PostgreSQL](https://www.postgresql.org/) (certified against versions 10.7 and 11.5)
* [MySQL](https://www.mysql.com/) (certified against version 5.7)
* [MariaDB](https://mariadb.org/) (certified against version 10.3.20)
* [etcd](https://etcd.io/) (certified against version 3.3.15)
* Embedded etcd for High Availability

### External Datastore Configuration Parameters
If you wish to use an external datastore such as PostgreSQL, MySQL, or etcd you must set the `datastore-endpoint` parameter so that K3s knows how to connect to it. You may also specify parameters to configure the authentication and encryption of the connection. The below table summarizes these parameters, which can be passed as either CLI flags or environment variables.

  CLI Flag | Environment Variable | Description
  ------------|-------------|------------------
 <span style="white-space: nowrap">`--datastore-endpoint`</span> | `K3S_DATASTORE_ENDPOINT` | Specify a PostgreSQL, MySQL, or etcd connection string. This is a string used to describe the connection to the datastore. The structure of this string is specific to each backend and is detailed below.
 <span style="white-space: nowrap">`--datastore-cafile`</span> | `K3S_DATASTORE_CAFILE` | TLS Certificate Authority (CA) file used to help secure communication with the datastore. If your datastore serves requests over TLS using a certificate signed by a custom certificate authority, you can specify that CA using this parameter so that the K3s client can properly verify the certificate. |                              
|  <span style="white-space: nowrap">`--datastore-certfile`</span> | `K3S_DATASTORE_CERTFILE` | TLS certificate file used for client certificate based authentication to your datastore. To use this feature, your datastore must be configured to support client certificate based authentication. If you specify this parameter, you must also specify the `datastore-keyfile` parameter. |     
|  <span style="white-space: nowrap">`--datastore-keyfile`</span> | `K3S_DATASTORE_KEYFILE` | TLS key file used for client certificate based authentication to your datastore. See the previous `datastore-certfile` parameter for more details. |

As a best practice we recommend setting these parameters as environment variables rather than command line arguments so that your database credentials or other sensitive information aren't exposed as part of the process info.

### Datastore Endpoint Format and Functionality
As mentioned, the format of the value passed to the `datastore-endpoint` parameter is dependent upon the datastore backend. The following details this format and functionality for each supported external datastore.


=== "PostgreSQL"

    In its most common form, the datastore-endpoint parameter for PostgreSQL has the following format:

    `postgres://username:password@hostname:port/database-name`

    More advanced configuration parameters are available. For more information on these, please see https://godoc.org/github.com/lib/pq.

    If you specify a database name and it does not exist, the server will attempt to create it.

    If you only supply `postgres://`  as the endpoint, K3s will attempt to do the following:

    * Connect to localhost using `postgres` as the username and password
    * Create a database named `kubernetes`



=== "MySQL / MariaDB"

    In its most common form, the `datastore-endpoint` parameter for MySQL and MariaDB has the following format:

    `mysql://username:password@tcp(hostname:3306)/database-name`

    More advanced configuration parameters are available. For more information on these, please see https://github.com/go-sql-driver/mysql#dsn-data-source-name

    Note that due to a [known issue](https://github.com/rancher/k3s/issues/1093) in K3s, you cannot set the `tls` parameter. TLS communication is supported, but you cannot, for example, set this parameter to "skip-verify" to cause K3s to skip certificate verification.

    If you specify a database name and it does not exist, the server will attempt to create it.

    If you only supply `mysql://` as the endpoint, K3s will attempt to do the following:

    * Connect to the MySQL socket at `/var/run/mysqld/mysqld.sock` using the `root` user and no password
    * Create a database with the name `kubernetes`


=== "etcd"

    In its most common form, the `datastore-endpoint` parameter for etcd has the following format:

    `https://etcd-host-1:2379,https://etcd-host-2:2379,https://etcd-host-3:2379`

    The above assumes a typical three node etcd cluster. The parameter can accept one more comma separated etcd URLs.

<br/>Based on the above, the following example command could be used to launch a server instance that connects to a PostgreSQL database named k3s:
```
K3S_DATASTORE_ENDPOINT='postgres://username:password@hostname:5432/k3s' k3s server
```

And the following example could be used to connect to a MySQL database using client certificate authentication:
```
K3S_DATASTORE_ENDPOINT='mysql://username:password@tcp(hostname:3306)/k3s' \
K3S_DATASTORE_CERTFILE='/path/to/client.crt' \
K3S_DATASTORE_KEYFILE='/path/to/client.key' \
k3s server
```

### Embedded etcd for HA

Please see [High Availability with Embedded DB](ha_embedded.md) for instructions on how to run with this option.
