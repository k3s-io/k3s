# imgcrypt image encryption library and command line tool

Project `imgcrypt` is a non-core subproject of containerd.

The `imgcrypt` library provides API exensions for containerd to support encrypted container images and implements
the `ctd-decoder` command line tool for use by containerd to decrypt encrypted container images. An extended version
of containerd's `ctr` tool (`ctr-enc') with support for encrypting and decrypting container images is also provided.

`imgcrypt` relies on the [`ocicrypt`](https://github.com/containers/ocicrypt) library for crypto functions on image layers.

# Usage

`imgcrypt` requires containerd 1.3 or later. Containerd 1.4 or later is required when used with Kubernetes.
For configuration instructions for kubernetes, please consult the [CRI decryption document](https://github.com/containerd/containerd/blob/master/docs/decryption.md).

Build and install `imgcrypt`:

```
# make
# sudo make install
```

Start containerd with a configuration file that looks as follows. To avoid interference with a containerd from a Docker
installation we use /tmp for directories. Also, we build containerd 1.3 from the source but do not install it.

```
# cat config.toml
disable_plugins = ["cri"]
root = "/tmp/var/lib/containerd"
state = "/tmp/run/containerd"
[grpc]
  address = "/tmp/run/containerd/containerd.sock"
  uid = 0
  gid = 0
[stream_processors]
    [stream_processors."io.containerd.ocicrypt.decoder.v1.tar.gzip"]
        accepts = ["application/vnd.oci.image.layer.v1.tar+gzip+encrypted"]
        returns = "application/vnd.oci.image.layer.v1.tar+gzip"
        path = "/usr/local/bin/ctd-decoder"
    [stream_processors."io.containerd.ocicrypt.decoder.v1.tar"]
        accepts = ["application/vnd.oci.image.layer.v1.tar+encrypted"]
        returns = "application/vnd.oci.image.layer.v1.tar"
        path = "/usr/local/bin/ctd-decoder"

# sudo ~/src/github.com/containerd/containerd/bin/containerd -c config.toml
```

Create an RSA key pair using the openssl command line tool and encrypted an image:

```
# openssl genrsa -out mykey.pem
Generating RSA private key, 2048 bit long modulus (2 primes)
...............................................+++++
............................+++++
e is 65537 (0x010001)
# openssl rsa -in mykey.pem -pubout -out mypubkey.pem
writing RSA key
# sudo chmod 0666 /tmp/run/containerd/containerd.sock
# CTR="/usr/local/bin/ctr-enc -a /tmp/run/containerd/containerd.sock"
# $CTR images pull --all-platforms docker.io/library/bash:latest
[...]
# $CTR images layerinfo --platform linux/amd64 docker.io/library/bash:latest
   #                                                                    DIGEST      PLATFORM      SIZE   ENCRYPTION   RECIPIENTS
   0   sha256:9d48c3bd43c520dc2784e868a780e976b207cbf493eaff8c6596eb871cbd9609   linux/amd64   2789669                          
   1   sha256:7dd01fd971d4ec7058c5636a505327b24e5fc8bd7f62816a9d518472bd9b15c0   linux/amd64   3174665                          
   2   sha256:691cfbca522787898c8b37f063dd20e5524e7d103e1a3b298bd2e2b8da54faf5   linux/amd64       340                          
# $CTR images encrypt --recipient jwe:mypubkey.pem --platform linux/amd64 docker.io/library/bash:latest bash.enc:latest
Encrypting docker.io/library/bash:latest to bash.enc:latest
$ $CTR images layerinfo --platform linux/amd64 bash.enc:latest
   #                                                                    DIGEST      PLATFORM      SIZE   ENCRYPTION   RECIPIENTS
   0   sha256:360be141b01f69b25427a9085b36ba8ad7d7a335449013fa6b32c1ecb894ab5b   linux/amd64   2789669          jwe        [jwe]
   1   sha256:ac601e66cdd275ee0e10afead03a2722e153a60982122d2d369880ea54fe82f8   linux/amd64   3174665          jwe        [jwe]
   2   sha256:41e47064fd00424e328915ad2f7f716bd86ea2d0d8315edaf33ecaa6a2464530   linux/amd64       340          jwe        [jwe]
```

Start a local image registry so we can push the encrypted image to it. A recent versions of the registry is required
to accept encrypted container images.
```
# docker pull registry:latest
# docker run -d -p 5000:5000 --restart=always --name registry registry
```

Push the encrypted image to the local registry, pull it using `ctr-enc`, and then run the image.
```
# $CTR images tag bash.enc:latest localhost:5000/bash.enc:latest
# $CTR images push localhost:5000/bash.enc:latest
# $CTR images rm localhost:5000/bash.enc:latest bash.enc:latest
# $CTR images pull localhost:5000/bash.enc:latest
# sudo $CTR run --rm localhost:5000/bash.enc:latest test echo 'Hello World!'
ctr: you are not authorized to use this image: missing private key needed for decryption
# sudo $CTR run --rm --key mykey.pem localhost:5000/bash.enc:latest test echo 'Hello World!'
Hello World!
```

## Project details

**imgcrypt** is a non-core containerd sub-project, licensed under the [Apache 2.0 license](./LICENSE).
As a containerd sub-project, you will find the:
 * [Project governance](https://github.com/containerd/project/blob/master/GOVERNANCE.md),
 * [Maintainers](MAINTAINERS),
 * and [Contributing guidelines](https://github.com/containerd/project/blob/master/CONTRIBUTING.md)

information in our [`containerd/project`](https://github.com/containerd/project) repository.
