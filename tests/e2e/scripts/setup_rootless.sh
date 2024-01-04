#!/bin/sh

# GitHub repository URL
github_url="https://raw.githubusercontent.com/k3s-io/k3s/master/k3s-rootless.service"

# Destination file path
destination_path="/home/vagrant/.config/systemd/user/"

# Download the file from GitHub using curl
curl -LJO "$github_url"

# Check if the download was successful
if [ $? -eq 0 ]; then
    # Move the downloaded file to the desired destination
    mkdir -p "$destination_path"
    mv "k3s-rootless.service" "$destination_path/k3s-rootless.service"
    # Add EnvironmentFile=-/etc/k3s-rootless.env to the service file
    sed -i 's/^\[Service\]$/\[Service\]\nEnvironmentFile=-\/etc\/rancher\/k3s\/k3s.env/' "$destination_path/k3s-rootless.service"
    chown -R vagrant:vagrant /home/vagrant/.config/

    echo "File downloaded and moved to $destination_path"
else
    echo "Failed to download the file from GitHub."
fi


# Enable IPv4 forwarding
echo "net.ipv4.ip_forward=1" >> /etc/sysctl.conf
sysctl --system


# Check if the string is already in GRUB_CMDLINE_LINUX
if grep -qxF "GRUB_CMDLINE_LINUX=\"systemd.unified_cgroup_hierarchy=1 \"" /etc/default/grub; then
    echo "String is already in GRUB_CMDLINE_LINUX. No changes made."
else
    # Add the string to GRUB_CMDLINE_LINUX
    sed -i "s/\(GRUB_CMDLINE_LINUX=\)\"\(.*\)\"/\1\"systemd.unified_cgroup_hierarchy=1 \2\"/" /etc/default/grub

    # Update GRUB
    update-grub

    echo "String 'systemd.unified_cgroup_hierarchy=1' added to GRUB_CMDLINE_LINUX and GRUB updated successfully."
fi

mkdir -p /etc/systemd/system/user@.service.d
echo "[Service]
Delegate=cpu cpuset io memory pids
">> /etc/systemd/system/user@.service.d/delegate.conf
apt-get install -y uidmap

systemctl daemon-reload
loginctl enable-linger vagrant
# We need to run this as vagrant user, because rootless k3s will be run as vagrant user
su -c 'XDG_RUNTIME_DIR="/run/user/$UID" DBUS_SESSION_BUS_ADDRESS="unix:path=${XDG_RUNTIME_DIR}/bus" systemctl --user daemon-reload' vagrant
su -c 'XDG_RUNTIME_DIR="/run/user/$UID" DBUS_SESSION_BUS_ADDRESS="unix:path=${XDG_RUNTIME_DIR}/bus" systemctl --user enable --now k3s-rootless' vagrant

