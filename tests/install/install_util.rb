def waitForNodeReady(vm)
    vm.provision "k3s-wait-for-node", type: "shell", run: ENV['CI'] == 'true' ? 'never' : 'once' do |sh|
      sh.inline = <<~SHELL
      #!/usr/bin/env bash
      set -eu -o pipefail
      echo 'Waiting for node to be ready ...'
      time timeout 300 bash -c 'while ! (kubectl wait --for condition=ready node/$(hostname) 2>/dev/null); do sleep 5; done'
      kubectl get node,all -A -o wide
      SHELL
    end
  end
  
  def waitForCoreDns(vm)
    vm.provision "k3s-wait-for-coredns", type: "shell", run: ENV['CI'] == 'true' ? 'never' : 'once' do |sh|
      sh.inline = <<~SHELL
      #!/usr/bin/env bash
      set -eu -o pipefail
      function describe-coredns {
          RC=$?
          if [[ $RC -ne 0 ]]; then
          kubectl describe node
          kubectl --namespace kube-system describe pod -l k8s-app=kube-dns
          kubectl --namespace kube-system logs -l k8s-app=kube-dns
          fi
          exit $RC
      }
      trap describe-coredns EXIT
      time timeout 120 bash -c 'while ! (kubectl --namespace kube-system rollout status --timeout 10s deploy/coredns 2>/dev/null); do sleep 5; done'
      SHELL
    end
  end
  
  def waitForLocalStorage(vm)
    vm.provision "k3s-wait-for-local-storage", type: "shell", run: ENV['CI'] == 'true' ? 'never' : 'once' do |sh|
      sh.inline = <<~SHELL
      #!/usr/bin/env bash
      set -eu -o pipefail
      time timeout 120 bash -c 'while ! (kubectl --namespace kube-system rollout status --timeout 10s deploy/local-path-provisioner 2>/dev/null); do sleep 5; done'
      SHELL
    end
  end

  # Metrics takes the longest to start, so we give it 3 minutes
  def waitForMetricsServer(vm)
    vm.provision "k3s-wait-for-metrics-server", type: "shell", run: ENV['CI'] == 'true' ? 'never' : 'once' do |sh|
      sh.inline = <<~SHELL
      #!/usr/bin/env bash
      set -eu -o pipefail
      time timeout 180 bash -c 'while ! (kubectl --namespace kube-system rollout status --timeout 10s deploy/metrics-server 2>/dev/null); do sleep 5; done'
      SHELL
    end
  end
  
  def waitForTraefik(vm)
    vm.provision "k3s-wait-for-traefik", type: "shell", run: ENV['CI'] == 'true' ? 'never' : 'once' do |sh|
      sh.inline = <<~SHELL
      #!/usr/bin/env bash
      set -eu -o pipefail
      time timeout 120 bash -c 'while ! (kubectl --namespace kube-system rollout status --timeout 10s deploy/traefik 2>/dev/null); do sleep 5; done'
      SHELL
    end
  end
  
  def kubectlStatus(vm)
    vm.provision "k3s-status", type: "shell", run: ENV['CI'] == 'true' ? 'never' : 'once' do |sh|
      sh.inline = <<~SHELL
      #!/usr/bin/env bash
      set -eux -o pipefail
      kubectl get node,all -A -o wide
      SHELL
    end
  end
  
  def checkK3sProcesses(vm)
    vm.provision "k3s-procps", type: "shell", run: ENV['CI'] == 'true' ? 'never' : 'once' do |sh|
      sh.inline = <<~SHELL
      #!/usr/bin/env bash
      set -eux -o pipefail
      ps auxZ | grep -E 'k3s|kube|container' | grep -v grep
      SHELL
    end
  end
  
  def checkCGroupV2(vm)
    vm.provision "cgroupv2", type: "shell", run: ENV['CI'] == 'true' ? 'never' : 'once' do |sh|
      sh.inline = <<~SHELL
      #!/usr/bin/env bash
      set -eux -o pipefail
      k3s check-config | grep 'cgroups V2 mounted'
      SHELL
    end
  end

  def mountDirs(vm)
    vm.provision "k3s-mount-directory", type: "shell", run: ENV['CI'] == 'true' ? 'never' : 'once' do |sh|
      sh.inline = <<~SHELL
      #!/usr/bin/env bash
      set -eu -o pipefail
      echo 'Mounting server dir'
      mount --bind /var/lib/rancher/k3s/server /var/lib/rancher/k3s/server
      SHELL
    end
  end

  def runUninstall(vm)
    vm.provision "k3s-uninstall", type: "shell", run: ENV['CI'] == 'true' ? 'never' : 'once' do |sh|
      sh.inline = <<~SHELL
      #!/usr/bin/env bash
      set -eu -o pipefail
      echo 'Uninstall k3s'
      k3s-server-uninstall.sh
      SHELL
    end
  end

  def checkMountPoint(vm)
    vm.provision "k3s-check-mount", type: "shell", run: ENV['CI'] == 'true' ? 'never' : 'once' do |sh|
      sh.inline = <<~SHELL
      #!/usr/bin/env bash
      set -eu -o pipefail
      echo 'Check the mount'
      mount | grep /var/lib/rancher/k3s/server
      SHELL
    end
  end

  def unmountDir(vm)
    vm.provision "k3s-unmount-dir", type: "shell", run: ENV['CI'] == 'true' ? 'never' : 'once' do |sh|
      sh.inline = <<~SHELL
      #!/usr/bin/env bash
      set -eu -o pipefail
      echo 'unmount the mount'
      umount /var/lib/rancher/k3s/server
      rm -rf /var/lib/rancher
      SHELL
    end
  end
