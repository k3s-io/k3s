#!/bin/sh
set -e

VERSION=v0.1.0

info()
{
    echo "[INFO] " "$@"
}

fatal()
{
    echo "[ERROR] " "$@"
    exit 1
}

ARCH=`uname -m`

case $ARCH in
    amd64)
        ARCH=amd64
        SUFFIX=
        ;;
    x86_64)
        ARCH=amd64
        SUFFIX=
        ;;
    arm64)
        ARCH=arm64
        SUFFIX=-${ARCH}
        ;;
    aarch64)
        ARCH=arm64
        SUFFIX=-${ARCH}
        ;;
    arm*)
        ARCH=arm
        SUFFIX=-${ARCH}hf
        ;;
    *)
        fatal Unknown architecture $ARCH
esac

BINURL=https://github.com/rancher/k3s/releases/download/${VERSION}/k3s${SUFFIX}
HASHURL=https://github.com/rancher/k3s/releases/download/${VERSION}/sha256sum-${ARCH}.txt

if [ -d /run/systemd ]; then
    SYSTEMD=true
else
    fatal "Can not find systemd to use as a process supervisor for k3s"
fi

SUDO=sudo
if [ `id -u` = 0 ]; then
    SUDO=
fi

if [ "$SYSTEMD" = "true" ]; then
    info Creating uninstall script /usr/local/bin/k3s-uninstall.sh
    TMPUNINSTALL=`mktemp -t k3s-install.XXXXXXXXXX`
    cat > $TMPUNINSTALL << "EOF"
#!/bin/sh
set -x
systemctl stop k3s
systemctl disable k3s
systemctl daemon-reload
rm -f /etc/systemd/system/k3s.service
rm -f /usr/local/bin/k3s
if [ -L /usr/local/bin/kubectl ]; then
    rm -f /usr/local/bin/kubectl
fi
if [ -L /usr/local/bin/crictl ]; then
    rm -f /usr/local/bin/crictl
fi
if [ -e /sys/fs/cgroup/systemd/system.slice/k3s.service/cgroup.procs ]; then
    kill -9 `cat /sys/fs/cgroup/systemd/system.slice/k3s.service/cgroup.procs`
fi
umount `cat /proc/self/mounts | awk '{print $2}' | grep '^/run/k3s'`
umount `cat /proc/self/mounts | awk '{print $2}' | grep '^/var/lib/rancher/k3s'`

rm -rf /var/lib/rancher/k3s
rm -rf /etc/rancher/k3s

rm -f /usr/local/bin/k3s-uninstall.sh
EOF
    chmod 755 $TMPUNINSTALL
    $SUDO chown root:root $TMPUNINSTALL
    $SUDO mv -f $TMPUNINSTALL /usr/local/bin/k3s-uninstall.sh

    CURL=`which curl`

    if [ -n "$CURL" ]; then
        TMPHASH=`mktemp -t k3s-install.XXXXXXXXXX`
        TMPBIN=`mktemp -t k3s-install.XXXXXXXXXX`

        info Downloading $HASHURL
        $CURL -o $TMPHASH -sfL $HASHURL

        info Downloading $BINURL
        $CURL -o $TMPBIN -sfL $BINURL
    fi


    info Verifying download
    EXPECTED=`grep k3s $TMPHASH | awk '{print $1}'`
    ACTUAL=`sha256sum $TMPBIN | awk '{print $1}'` 
    rm -f $TMPHASH
    if [ "$EXPECTED" != "$ACTUAL" ]; then
        rm -f $TMPBIN
        fatal "Download sha256 does not match ${EXPECTED} got ${ACTUAL}"
    fi

    chmod 755 $TMPBIN
    info Installing k3s to /usr/local/bin/k3s

    $SUDO chown root:root $TMPBIN
    $SUDO mv -f $TMPBIN /usr/local/bin/k3s

    if [ ! -e /usr/local/bin/kubectl ]; then
        info Creating /usr/local/bin/kubectl symlink to k3s
        $SUDO ln -s k3s /usr/local/bin/kubectl
    fi

    if [ ! -e /usr/local/bin/crictl ]; then
        info Creating /usr/local/bin/crictl symlink to k3s
        $SUDO ln -s k3s /usr/local/bin/crictl
    fi

    info systemd: Creating /etc/systemd/system/k3s.service
    $SUDO tee /etc/systemd/system/k3s.service >/dev/null << "EOF"
[Unit]
Description=Lightweight Kubernetes
Documentation=https://k3s.io
After=network.target

[Service]
ExecStartPre=-/sbin/modprobe br_netfilter
ExecStartPre=-/sbin/modprobe overlay
ExecStart=/usr/local/bin/k3s server
KillMode=process
Delegate=yes
LimitNOFILE=infinity
LimitNPROC=infinity
LimitCORE=infinity
TasksMax=infinity

[Install]
WantedBy=multi-user.target
EOF

    info systemd: Enabling k3s unit
    $SUDO systemctl enable k3s.service >/dev/null
    $SUDO systemctl daemon-reload >/dev/null

    info systemd: Starting k3s
    $SUDO systemctl start k3s.service
else
    fatal "Can not find systemd"
fi
