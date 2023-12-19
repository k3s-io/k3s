# vim: sw=4:ts=4:et

%define install_path  /usr/bin
%define util_path     %{_datadir}/k3s
%define install_sh    %{util_path}/setup/install.sh
%define uninstall_sh  %{util_path}/setup/uninstall.sh

Name:    k3s
Version: %{k3s_version}
Release: %{k3s_release}%{?dist}
Summary: Lightweight Kubernetes

Group:   System Environment/Base		
License: ASL 2.0
URL:     http://k3s.io

BuildRequires: systemd
Requires(post): k3s-selinux >= %{k3s_policyver}

%description
The certified Kubernetes distribution built for IoT & Edge computing.

%install
install -d %{buildroot}%{install_path}
install dist/artifacts/%{k3s_binary} %{buildroot}%{install_path}/k3s
install -d %{buildroot}%{util_path}/setup
install package/rpm/install.sh %{buildroot}%{install_sh}

%post
# do not overwrite env file if present
export INSTALL_K3S_UPGRADE=true
export INSTALL_K3S_BIN_DIR=%{install_path}
export INSTALL_K3S_SKIP_DOWNLOAD=true
export INSTALL_K3S_SKIP_ENABLE=true
export INSTALL_K3S_DEBUG=true
export UNINSTALL_K3S_SH=%{uninstall_sh}

(
    # install server service
    INSTALL_K3S_NAME=server \
        %{install_sh}

    # install agent service
    INSTALL_K3S_SYMLINK=skip \
    INSTALL_K3S_BIN_DIR_READ_ONLY=true \
    K3S_TOKEN=example-token \
    K3S_URL=https://example-k3s-server:6443/ \
        %{install_sh} agent

# save debug log of the install
) >%{util_path}/setup/install.log 2>&1

%systemd_post k3s-server.service
%systemd_post k3s-agent.service
exit 0

%postun
# do not run uninstall script on upgrade
if [ $1 = 0 ]; then
    %{uninstall_sh}
    rm -rf %{util_path}
fi
exit 0

%files
%{install_path}/k3s
%{install_sh}

%changelog
* Mon Mar 2 2020 Erik Wilson <erik@rancher.com> 0.1-1
- Initial version
