cat <<EOF >/etc/yum.repos.d/rancher.repo
[rancher]
name=Rancher Repository
baseurl=https://rpm.rancher.io/
enabled=1
gpgcheck=1
gpgkey=https://rpm.rancher.io/public.key
EOF
