#!/bin/bash
# This script is used to bootstrap a MicroOS image. 
# See https://en.opensuse.org/Portal:MicroOS/Combustion
# combustion: network
# Redirect output to the console
exec > >(exec tee -a /dev/tty0) 2>&1

# Change hostname
hostname server-0
cat >/etc/hostname <<EOF
server-0
EOF

# Enable password authentication for ssh
cat >/etc/ssh/sshd_config.d/20-enable-passwords.conf <<EOF
PasswordAuthentication yes
EOF
systemctl enable sshd.service

# Add vagrant user
useradd -m vagrant 
echo 'vagrant:vagrant' | chpasswd

# Install insecure vagrant ssh key
# https://github.com/hashicorp/vagrant/tree/master/keys
# Vagrant replaces this on startup with a secure key
cat >/home/vagrant/.ssh/authorized_keys <<EOF
ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEA6NF8iallvQVp22WDkTkyrtvp9eWW6A8YVr+kz4TjGYe7gHzIw+niNltGEFHzD8+v1I2YJ6oXevct1YeS0o9HZyN1Q9qgCgzUFtdOKLv6IedplqoPkcmF0aYet2PkEDo3MlTBckFXPITAMzF8dJSIFo9D8HfdOV0IAdx4O7PtixWKn5y2hMNG0zQPyUecp4pzC6kivAIhyfHilFR61RGL+GPXQ2MWZWFYbAGjyiYJnAmCP3NOTd0jMZEnDkbUvxhMmBYSdETk1rRgm+R4LOzFUGaHqHDLKLX+FIPKcF96hrucXzcWyLbIbEgE98OHlnVYCzRdK8jlqm8tehUc9c9WhQ== vagrant insecure public key
EOF

# Install dependencies for E2E tests
zypper --non-interactive --gpg-auto-import-keys install jq apparmor-parser k3s-selinux

# Leave a marker
echo "Configured with combustion" > /etc/issue.d/combustion