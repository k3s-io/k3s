package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k3s-io/k3s/tests"
	"github.com/k3s-io/k3s/tests/docker"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var ci = flag.Bool("ci", false, "running on CI, forced cleanup")

const installSh = "../../../install.sh"

const k3sBinary = "touch /usr/local/bin/k3s; chmod 0755 /usr/local/bin/k3s"

const installEnv = "INSTALL_K3S_SKIP_DOWNLOAD=binary " +
	"INSTALL_K3S_SKIP_START=true " +
	"INSTALL_K3S_SKIP_ENABLE=true " +
	"INSTALL_K3S_SELINUX_WARN=true"

type distro struct {
	name        string
	image       string
	prepare     string
	repoFile    string
	wantBaseURL string
	queryCmd    string
}

var distros = []distro{
	{
		name:        "sles16",
		image:       "registry.suse.com/bci/bci-base:16.0",
		prepare:     "zypper --non-interactive --gpg-auto-import-keys install -y systemd; mkdir -p /usr/share/selinux",
		repoFile:    "/etc/zypp/repos.d/rancher-k3s-common.repo",
		wantBaseURL: "https://rpm.rancher.io/k3s/stable/common/slemicro/noarch",
		queryCmd:    "zypper --non-interactive info k3s-selinux",
	},
	{
		name:        "sle-micro",
		image:       "registry.suse.com/suse/sle-micro/5.5:latest",
		prepare:     "mkdir -p /usr/share/selinux",
		repoFile:    "/etc/zypp/repos.d/rancher-k3s-common.repo",
		wantBaseURL: "https://rpm.rancher.io/k3s/stable/common/slemicro/noarch",
		queryCmd:    "zypper --non-interactive info k3s-selinux",
	},
	{
		name:        "opensuse-leap",
		image:       "opensuse/leap:15.6",
		prepare:     "zypper --non-interactive --gpg-auto-import-keys install -y systemd; mkdir -p /usr/share/selinux",
		repoFile:    "/etc/zypp/repos.d/rancher-k3s-common.repo",
		wantBaseURL: "https://rpm.rancher.io/k3s/stable/common/microos/noarch",
		queryCmd:    "zypper --non-interactive info k3s-selinux",
	},
	{
		name:        "almalinux",
		image:       "almalinux:10",
		prepare:     "mkdir -p /usr/share/selinux",
		repoFile:    "/etc/yum.repos.d/rancher-k3s-common.repo",
		wantBaseURL: "https://rpm.rancher.io/k3s/stable/common/centos/9/noarch",
		queryCmd:    "dnf -q info k3s-selinux",
	},
}

func Test_DockerSELinux(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	RunSpecs(t, "SELinux RPM Docker Test Suite")
}

// installScript runs install.sh and then summarizes what it did on two lines,
// so the specs can match against the container log without teasing apart the
// output of every command.
func installScript(d distro) string {
	return strings.Join([]string{
		"exec 2>&1",
		k3sBinary,
		d.prepare,
		installEnv + " sh /tmp/install.sh server",
		fmt.Sprintf(`echo "REPO: $(test -f %[1]s && grep -h '^baseurl=' %[1]s || echo missing)"`, d.repoFile),
		fmt.Sprintf(`echo "QUERY: $(%s 2>/dev/null | grep -i 'rancher.k3s.common' | head -1)"`, d.queryCmd),
	}, "\n")
}

// skipScript checks that INSTALL_K3S_SKIP_SELINUX_RPM prevents install.sh from configuring a repository or resolving the RPM.
func skipScript(d distro) string {
	return strings.Join([]string{
		"exec 2>&1",
		k3sBinary,
		d.prepare,
		installEnv + " INSTALL_K3S_SKIP_SELINUX_RPM=true sh /tmp/install.sh server",
		fmt.Sprintf(`echo "REPO: $(test -f %[1]s && echo %[1]s || echo missing)"`, d.repoFile),
	}, "\n")
}

// run is a container that executes its script once and exits. It is started
// detached so every distro installs in parallel, and collect() blocks on it.
type run struct {
	tc   *docker.TestConfig
	name string
}

var runs []*run

func start(name string, d distro, script string) (*run, error) {
	tc, err := docker.NewTestConfig(d.image)
	if err != nil {
		return nil, err
	}
	r := &run{tc: tc, name: fmt.Sprintf("selinux-%s-%s", name, strings.ToLower(filepath.Base(tc.TestDir)))}
	runs = append(runs, r)

	installShPath, err := filepath.Abs(installSh)
	if err != nil {
		return r, err
	}
	scriptPath := filepath.Join(tc.TestDir, "install-test.sh")
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		return r, err
	}

	dRun := strings.Join([]string{"docker run -d",
		"--name", r.name,
		"--mount", fmt.Sprintf("type=bind,src=%s,dst=/tmp/install.sh,ro", installShPath),
		"--mount", fmt.Sprintf("type=bind,src=%s,dst=/tmp/install-test.sh,ro", scriptPath),
		tc.K3sImage, "/bin/sh", "/tmp/install-test.sh"}, " ")
	if out, err := tests.RunCommand(dRun); err != nil {
		return r, fmt.Errorf("failed to start %s: %s: %v", r.name, out, err)
	}
	// Registering the exited container as a server lets Cleanup remove it.
	tc.Servers = append(tc.Servers, docker.DockerNode{Name: r.name})

	return r, nil
}

// collect waits for the container to exit and returns everything it logged.
func (r *run) collect() (string, error) {
	if out, err := tests.RunCommand("timeout 900 docker wait " + r.name); err != nil {
		return out, fmt.Errorf("waiting for %s: %s: %v", r.name, out, err)
	}
	out, err := tests.RunCommand("docker logs " + r.name)
	if err != nil {
		return out, fmt.Errorf("logs for %s: %v", r.name, err)
	}
	GinkgoWriter.Println(out)
	logPath := filepath.Join(r.tc.TestDir, "logs", r.name+".log")
	return out, os.WriteFile(logPath, []byte(out), 0644)
}

var _ = Describe("SELinux RPM Tests", Ordered, func() {
	started := make(map[string]*run)

	BeforeAll(func() {
		for _, d := range distros {
			r, err := start(d.name, d, installScript(d))
			Expect(err).NotTo(HaveOccurred())
			started[d.name] = r
		}
		r, err := start("skip", distros[0], skipScript(distros[0]))
		Expect(err).NotTo(HaveOccurred())
		started["skip"] = r
	})

	for _, d := range distros {
		Context(d.name, Ordered, func() {
			var out string

			BeforeAll(func() {
				var err error
				out, err = started[d.name].collect()
				Expect(err).NotTo(HaveOccurred(), out)
			})

			It("should run install.sh", func() {
				Expect(out).To(ContainSubstring("Finding available k3s-selinux versions"),
					"install.sh skipped setup_selinux entirely")
			})

			It("should configure the expected repository", func() {
				Expect(out).To(ContainSubstring("REPO: baseurl=" + d.wantBaseURL))
			})

			It("should resolve k3s-selinux from that repository", func() {
				Expect(out).To(MatchRegexp(`(?i)QUERY: .*rancher.k3s.common`),
					"k3s-selinux was resolved from an unexpected repository")
			})
		})
	}

	Context("skip", Ordered, func() {
		var out string

		BeforeAll(func() {
			var err error
			out, err = started["skip"].collect()
			Expect(err).NotTo(HaveOccurred(), out)
		})

		It("should not configure a repository when the RPM is skipped", func() {
			Expect(out).To(ContainSubstring("Skipping installation of SELinux RPM"))
			Expect(out).To(ContainSubstring("REPO: missing"))
		})
	})
})

var failed bool
var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
})

var _ = AfterSuite(func() {
	for _, r := range runs {
		if failed {
			AddReportEntry("test-dir", r.tc.TestDir)
		}
		if *ci || !failed {
			Expect(r.tc.Cleanup()).To(Succeed())
		}
	}
})
