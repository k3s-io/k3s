# vim: sw=4:ts=4:et

%define install_path  /usr/bin
%define util_path     %{_datadir}/k3s
%define install_sh    %{util_path}/.install.sh
%define uninstall_sh  %{util_path}/.uninstall.sh

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
install -d %{buildroot}%{util_path}
install install.sh %{buildroot}%{install_sh}

%post
# do not run install script on upgrade
echo post-install args: $@
if [ $1 == 1 ]; then
    INSTALL_K3S_BIN_DIR=%{install_path} \
    INSTALL_K3S_SKIP_DOWNLOAD=true \
    INSTALL_K3S_SKIP_ENABLE=true \
    UNINSTALL_K3S_SH=%{uninstall_sh} \
        %{install_sh}
fi
%systemd_post k3s.service
exit 0

%postun
echo post-uninstall args: $@
# do not run uninstall script on upgrade
if [ $1 == 0 ]; then
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
