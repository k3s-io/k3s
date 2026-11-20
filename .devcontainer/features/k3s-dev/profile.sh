# shellcheck shell=bash
# Shell integration for the K3s dev container, loaded by every terminal.

K3S_SRC="${K3S_SRC:-/go/src/github.com/k3s-io/k3s}"
export K3S_SRC

# k3s-dev and the welcome banner live in the repository so they can be changed without a rebuild
case ":${PATH}:" in
  *":${K3S_SRC}/.devcontainer/bin:"*) ;;
  *) PATH="${K3S_SRC}/.devcontainer/bin:${PATH}" ;;
esac
export PATH

# Only interactive shells from here on
case $- in *i*) ;; *) return 0 ;; esac

[ -f /usr/share/bash-completion/bash_completion ] && . /usr/share/bash-completion/bash_completion
# shellcheck source=/dev/null
command -v kubectl > /dev/null 2>&1 && . <(kubectl completion bash 2>/dev/null) && complete -o default -F __start_kubectl k
# shellcheck source=/dev/null
command -v helm > /dev/null 2>&1 && . <(helm completion bash 2>/dev/null)
# shellcheck source=/dev/null
[ -f "${K3S_SRC}/.devcontainer/bin/k3s-dev.completion" ] && . "${K3S_SRC}/.devcontainer/bin/k3s-dev.completion"
alias k=kubectl

# kexport / kunset: set or unset a variable in every open terminal (applied at the next prompt).
#   kexport KUBECONFIG=/path/to/other/kubeconfig
#   kunset KUBECONFIG
K3S_DEV_ENV_FILE="${HOME}/.k3s-dev.env"

__k3s_dev_env_write() {
  touch "${K3S_DEV_ENV_FILE}"
  sed -i -e "/^export $1=/d" -e "/^unset $1\$/d" "${K3S_DEV_ENV_FILE}"
  echo "$2" >> "${K3S_DEV_ENV_FILE}"
}
kexport() {
  local pair
  for pair in "$@"; do
    __k3s_dev_env_write "${pair%%=*}" "$(printf 'export %s=%q' "${pair%%=*}" "${pair#*=}")"
  done
  __k3s_dev_env_sum=""; __k3s_dev_env_sync
}
kunset() {
  local name
  for name in "$@"; do __k3s_dev_env_write "${name}" "unset ${name}"; done
  __k3s_dev_env_sum=""; __k3s_dev_env_sync
}
__k3s_dev_env_sync() {
  [ -f "${K3S_DEV_ENV_FILE}" ] || return 0
  local sum
  sum=$(cksum < "${K3S_DEV_ENV_FILE}")
  if [ "${sum}" != "${__k3s_dev_env_sum}" ]; then
    # shellcheck source=/dev/null
    . "${K3S_DEV_ENV_FILE}"
    __k3s_dev_env_sum="${sum}"
  fi
}
__k3s_dev_env_sync
case ";${PROMPT_COMMAND};" in
  *";__k3s_dev_env_sync;"*) ;;
  *) PROMPT_COMMAND="__k3s_dev_env_sync${PROMPT_COMMAND:+;${PROMPT_COMMAND}}" ;;
esac

# Welcome banner in every new terminal
if [ -z "${K3S_DEV_NO_BANNER}" ] && command -v k3s-dev > /dev/null 2>&1; then
  k3s-dev banner
fi
