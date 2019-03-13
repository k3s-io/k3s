#!/bin/sh
set -e

# Usage:
#   curl ... | ENV_VAR=... sh -
#       or
#   ENV_VAR=... ./install.sh
#
# Example:
#   Installing a server without an agent:
#     curl ... | INSTALL_K3S_EXEC="--disable-agent" sh -
#   Installing an agent to point at a server:
#     curl ... | K3S_TOKEN=xxx K3S_URL=https://server-url:6443 sh -
#
# Environment variables:
#   - K3S_*
#     Environment variables which begin with K3S_ will be preserved for the
#     systemd service to use. Setting K3S_URL without explicitly setting
#     a systemd exec command will default the command to "agent", and we
#     enforce that K3S_TOKEN or K3S_CLUSTER_SECRET is also set.
#
#   - INSTALL_K3S_SKIP_DOWNLOAD
#     If set to true will not download k3s hash or binary.
#
#   - INSTALL_K3S_VERSION
#     Version of k3s to download from github. Will attempt to download the
#     latest version if not specified.
#
#   - INSTALL_K3S_BIN_DIR
#     Directory to install k3s binary, links, and uninstall script to, or use
#     /usr/local/bin as the default
#
#   - INSTALL_K3S_SYSTEMD_DIR
#     Directory to install systemd service and environment files to, or use
#     /etc/systemd/system as the default
#
#   - INSTALL_K3S_EXEC or script arguments
#     Command with flags to use for launching k3s in the systemd service, if
#     the command is not specified will default to "agent" if K3S_URL is set
#     or "server" if not. The final systemd command resolves to a combination
#     of EXEC and script args ($@).
#
#     The following commands result in the same behavior:
#       curl ... | INSTALL_K3S_EXEC="--disable-agent" sh -s -
#       curl ... | INSTALL_K3S_EXEC="server --disable-agent" sh -s -
#       curl ... | INSTALL_K3S_EXEC="server" sh -s - --disable-agent
#       curl ... | sh -s - server --disable-agent
#       curl ... | sh -s - --disable-agent
#
#   - INSTALL_K3S_NAME
#     Name of systemd service to create, will default from the k3s exec command
#     if not specified. If specified the name will be prefixed with 'k3s-'.
#
#   - INSTALL_K3S_TYPE
#     Type of systemd service to create, will default from the k3s exec command
#     if not specified.

GITHUB_URL=https://github.com/rancher/k3s/releases

# --- helper functions for logs ---
info()
{
    echo "[INFO] " "$@"
}
fatal()
{
    echo "[ERROR] " "$@"
    exit 1
}

# --- fatal if no systemd ---
verify_systemd() {
    if [ ! -d /run/systemd ]; then
        fatal "Can not find systemd to use as a process supervisor for k3s"
    fi
}

# --- define needed environment variables ---
setup_env() {

    # --- use command args if passed or create default ---
    case "$1" in
        # --- if we only have flags discover if command should be server or agent ---
        (-*|"")
            if [ -z "${K3S_URL}" ]; then
                CMD_K3S=server
            else
                if [ -z "${K3S_TOKEN}" ] && [ -z "${K3S_CLUSTER_SECRET}" ]; then
                    fatal "Defaulted k3s exec command to 'agent' because K3S_URL is defined, but K3S_TOKEN or K3S_CLUSTER_SECRET is not defined."
                fi
                CMD_K3S=agent
            fi
            CMD_K3S_EXEC="${CMD_K3S} $@"
        ;;
        # --- command is provided ---
        (*)
            CMD_K3S="$1"
            CMD_K3S_EXEC="$@"
        ;;
    esac
    CMD_K3S_EXEC=$(trim() { echo $@; } && trim ${CMD_K3S_EXEC})

    # --- use systemd name if defined or create default ---
    if [ -n "${INSTALL_K3S_NAME}" ]; then
        SYSTEMD_NAME=k3s-${INSTALL_K3S_NAME}
    else
        if [ "${CMD_K3S}" = "server" ]; then
            SYSTEMD_NAME=k3s
        else
            SYSTEMD_NAME=k3s-${CMD_K3S}
        fi
    fi
    SERVICE_K3S=${SYSTEMD_NAME}.service
    UNINSTALL_K3S_SH=${SYSTEMD_NAME}-uninstall.sh

    # --- use systemd type if defined or create default ---
    if [ -n "${INSTALL_K3S_TYPE}" ]; then
        SYSTEMD_TYPE="${INSTALL_K3S_TYPE}"
    else
        if [ "${CMD_K3S}" = "server" ]; then
            SYSTEMD_TYPE=notify
        else
            SYSTEMD_TYPE=exec
        fi
    fi

    # --- use binary install directory if defined or create default ---
    if [ -n "${INSTALL_K3S_BIN_DIR}" ]; then
        BIN_DIR="${INSTALL_K3S_BIN_DIR}"
    else
        BIN_DIR="/usr/local/bin"
    fi

    # --- use systemd directory if defined or create default ---
    if [ -n "${INSTALL_K3S_SYSTEMD_DIR}" ]; then
        SYSTEMD_DIR="${INSTALL_K3S_SYSTEMD_DIR}"
    else
        SYSTEMD_DIR="/etc/systemd/system"
    fi

    # --- use sudo if we are not already root ---
    SUDO=sudo
    if [ `id -u` = 0 ]; then
        SUDO=
    fi
}

# --- check if skip download environment variable set ---
can_skip_download() {
    if [ "${INSTALL_K3S_SKIP_DOWNLOAD}" != "true" ]; then
        return 1
    fi
}

# --- verify an executabe k3s binary is installed ---
verify_k3s_is_executable() {
    if [ ! -x ${BIN_DIR}/k3s ]; then
        fatal "Executable k3s binary not found at ${BIN_DIR}/k3s"
    fi
}

# --- set arch and suffix, fatal if architecture not supported ---
setup_verify_arch() {
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
            fatal "Unsupported architecture $ARCH"
    esac
}

# --- fatal if no curl ---
verify_curl() {
    if [ -z `which curl || true` ]; then
        fatal "Can not find curl for downloading files"
    fi
}

# --- create tempory directory and cleanup when done ---
setup_tmp() {
    TMP_DIR=`mktemp -d -t k3s-install.XXXXXXXXXX`
    TMP_HASH=${TMP_DIR}/k3s.hash
    TMP_BIN=${TMP_DIR}/k3s.bin
    cleanup() {
        code=$?
        set +e
        trap - EXIT
        rm -rf ${TMP_DIR}
        exit $code
    }
    trap cleanup INT EXIT
}

# --- use desired k3s version if defined or find latest ---
get_release_version() {
    if [ -n "${INSTALL_K3S_VERSION}" ]; then
        VERSION_K3S="${INSTALL_K3S_VERSION}"
    else
        info "Finding latest release"
        VERSION_K3S=`curl -w "%{url_effective}" -I -L -s -S ${GITHUB_URL}/latest -o /dev/null | sed -e 's|.*/||'`
    fi
    info "Using ${VERSION_K3S} as release"
}

# --- download hash from github url ---
download_hash() {
    HASH_URL=${GITHUB_URL}/download/${VERSION_K3S}/sha256sum-${ARCH}.txt
    info "Downloading hash ${HASH_URL}"
    curl -o ${TMP_HASH} -sfL ${HASH_URL} || fatal "Hash download failed"
    HASH_EXPECTED=`grep " k3s${SUFFIX}$" ${TMP_HASH} | awk '{print $1}'`
}

# --- check hash against installed version ---
installed_hash_matches() {
    if [ -x ${BIN_DIR}/k3s ]; then
        HASH_INSTALLED=`sha256sum ${BIN_DIR}/k3s | awk '{print $1}'`
        if [ "${HASH_EXPECTED}" = "${HASH_INSTALLED}" ]; then
            return
        fi
    fi
    return 1
}

# --- download binary from github url ---
download_binary() {
    BIN_URL=${GITHUB_URL}/download/${VERSION_K3S}/k3s${SUFFIX}
    info "Downloading binary ${BIN_URL}"
    curl -o ${TMP_BIN} -sfL ${BIN_URL} || fatal "Binary download failed"
}

# --- verify downloaded binary hash ---
verify_binary() {
    info "Verifying binary download"
    HASH_BIN=`sha256sum ${TMP_BIN} | awk '{print $1}'`
    if [ "${HASH_EXPECTED}" != "${HASH_BIN}" ]; then
        fatal "Download sha256 does not match ${HASH_EXPECTED}, got ${HASH_BIN}"
    fi
}

# --- setup permissions and move binary to system directory ---
setup_binary() {
    chmod 755 ${TMP_BIN}
    info "Installing k3s to ${BIN_DIR}/k3s"
    $SUDO chown root:root ${TMP_BIN}
    $SUDO mv -f ${TMP_BIN} ${BIN_DIR}/k3s

    if command -v getenforce > /dev/null 2>&1; then
        if [ "Disabled" != `getenforce` ]; then
            info "SeLinux is enabled, setting permissions"
            if ! $SUDO semanage fcontext -l | grep "${BIN_DIR}/k3s" > /dev/null 2>&1; then
                $SUDO semanage fcontext -a -t bin_t "${BIN_DIR}/k3s"
            fi
            $SUDO restorecon -v ${BIN_DIR}/k3s > /dev/null
        fi
    fi
}

# --- download and verify k3s ---
download_and_verify() {
    if can_skip_download; then
       info "Skipping k3s download and verify"
       verify_k3s_is_executable
       return
    fi

    setup_verify_arch
    verify_curl
    setup_tmp
    get_release_version
    download_hash

    if installed_hash_matches; then
        info "Skipping binary downloaded, installed k3s matches hash"
        return
    fi

    download_binary
    verify_binary
    setup_binary
}

# --- add additional utility links ---
create_symlinks() {
    if [ ! -e ${BIN_DIR}/kubectl ]; then
        info "Creating ${BIN_DIR}/kubectl symlink to k3s"
        $SUDO ln -s k3s ${BIN_DIR}/kubectl
    fi

    if [ ! -e ${BIN_DIR}/crictl ]; then
        info "Creating ${BIN_DIR}/crictl symlink to k3s"
        $SUDO ln -s k3s ${BIN_DIR}/crictl
    fi
}

# --- create uninstall script ---
create_uninstall() {
    info "Creating uninstall script ${BIN_DIR}/${UNINSTALL_K3S_SH}"
    $SUDO tee ${BIN_DIR}/${UNINSTALL_K3S_SH} >/dev/null << EOF
#!/bin/sh
set -x
systemctl kill ${SYSTEMD_NAME}
systemctl disable ${SYSTEMD_NAME}
systemctl reset-failed ${SYSTEMD_NAME}
systemctl daemon-reload
rm -f ${SYSTEMD_DIR}/${SERVICE_K3S}
rm -f ${SYSTEMD_DIR}/${SERVICE_K3S}.env

remove_uninstall() {
    rm -f ${BIN_DIR}/${UNINSTALL_K3S_SH}
}
trap remove_uninstall EXIT

if ls ${SYSTEMD_DIR}/k3s*.service >/dev/null 2>&1; then
    set +x; echo "Additional k3s services installed, skipping uninstall of k3s"; set -x
    exit
fi

do_unmount() {
    MOUNTS=\`cat /proc/self/mounts | awk '{print \$2}' | grep "^\$1"\`
    if [ -n "\${MOUNTS}" ]; then
        umount \${MOUNTS}
    fi
}
do_unmount '/run/k3s'
do_unmount '/var/lib/rancher/k3s'

nets=\$(ip link show master cni0 | grep cni0 | awk -F': ' '{print \$2}' | sed -e 's|@.*||')
for iface in \$nets; do
    ip link delete \$iface;
done
ip link delete cni0
ip link delete flannel.1

if [ -L ${BIN_DIR}/kubectl ]; then
    rm -f ${BIN_DIR}/kubectl
fi
if [ -L ${BIN_DIR}/crictl ]; then
    rm -f ${BIN_DIR}/crictl
fi

rm -rf /etc/rancher/k3s
rm -rf /var/lib/rancher/k3s
rm -f ${BIN_DIR}/k3s
EOF
    $SUDO chmod 755 ${BIN_DIR}/${UNINSTALL_K3S_SH}
    $SUDO chown root:root ${BIN_DIR}/${UNINSTALL_K3S_SH}
}

# --- disable current service if loaded --
systemd_disable() {
    $SUDO rm -f /etc/systemd/system/${SERVICE_K3S} || true
    $SUDO rm -f /etc/systemd/system/${SERVICE_K3S}.env || true
    $SUDO systemctl disable ${SYSTEMD_NAME} >/dev/null 2>&1 || true
}

# --- capture current env and create file containing k3s_ variables ---
create_env_file() {
    info "systemd: Creating environment file ${SYSTEMD_DIR}/${SERVICE_K3S}.env"
    UMASK=`umask`
    umask 0377
    env | grep '^K3S_' | $SUDO tee ${SYSTEMD_DIR}/${SERVICE_K3S}.env >/dev/null
    umask $UMASK
}

# --- write service file ---
create_service_file() {
    info "systemd: Creating service file ${SYSTEMD_DIR}/${SERVICE_K3S}"
    $SUDO tee ${SYSTEMD_DIR}/${SERVICE_K3S} >/dev/null << EOF
[Unit]
Description=Lightweight Kubernetes
Documentation=https://k3s.io
After=network.target

[Service]
Type=${SYSTEMD_TYPE}
EnvironmentFile=${SYSTEMD_DIR}/${SERVICE_K3S}.env
ExecStartPre=-/sbin/modprobe br_netfilter
ExecStartPre=-/sbin/modprobe overlay
ExecStart=${BIN_DIR}/k3s ${CMD_K3S_EXEC}
KillMode=process
Delegate=yes
LimitNOFILE=infinity
LimitNPROC=infinity
LimitCORE=infinity
TasksMax=infinity
TimeoutStartSec=infinity

[Install]
WantedBy=multi-user.target
EOF
}

# --- enable and start systemd service ---
systemd_enable_and_start() {
    info "systemd: Enabling ${SYSTEMD_NAME} unit"
    $SUDO systemctl enable ${SYSTEMD_DIR}/${SERVICE_K3S} >/dev/null
    $SUDO systemctl daemon-reload >/dev/null

    info "systemd: Starting ${SYSTEMD_NAME}"
    $SUDO systemctl restart ${SYSTEMD_NAME}
}

# --- run the install process --
{
    verify_systemd
    setup_env ${INSTALL_K3S_EXEC} $@
    download_and_verify
    create_symlinks
    create_uninstall
    systemd_disable
    create_env_file
    create_service_file
    systemd_enable_and_start
}
