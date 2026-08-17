package selinux

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

const (
	skipImage    = "registry.suse.com/bci/bci-base:16.0"
	skipRepoFile = "/etc/zypp/repos.d/rancher-k3s-common.repo"

	installSh = "../../../install.sh"

	k3sBinary = "touch /usr/local/bin/k3s; chmod 0755 /usr/local/bin/k3s"

	installEnv = "INSTALL_K3S_SKIP_DOWNLOAD=binary " +
		"INSTALL_K3S_SKIP_START=true " +
		"INSTALL_K3S_SKIP_ENABLE=true " +
		"INSTALL_K3S_SELINUX_WARN=true"
)

var ci = flag.Bool("ci", false, "running on CI, forced cleanup")
var skipScript = strings.Join([]string{
	"exec 2>&1",
	k3sBinary,
	"zypper --non-interactive --gpg-auto-import-keys install -y systemd; mkdir -p /usr/share/selinux",
	installEnv + " INSTALL_K3S_SKIP_SELINUX_RPM=true sh /tmp/install.sh server",
	fmt.Sprintf(`echo "REPO: $(test -f %[1]s && echo %[1]s || echo missing)"`, skipRepoFile),
}, "\n")

type distro struct {
	name        string
	image       string
	prepare     string
	repoFile    string
	wantBaseURL string
	queryCmd    string
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

// runScript runs the script in a container that executes it once and exits,
// and returns the test config and everything the container logged.
func runScript(name, image, script string) (*docker.TestConfig, string, error) {
	tc, err := docker.NewTestConfig(GinkgoTB(), image)
	if err != nil {
		return nil, "", err
	}
	name = fmt.Sprintf("selinux-%s-%s", name, strings.ToLower(filepath.Base(tc.TestDir)))

	installShPath, err := filepath.Abs(installSh)
	if err != nil {
		return tc, "", err
	}
	scriptPath := filepath.Join(tc.TestDir, "test.sh")
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		return tc, "", err
	}

	tc.Servers = append(tc.Servers, docker.DockerNode{Name: name})
	dRun := strings.Join([]string{"timeout 900 docker run",
		"--name", name,
		"--mount", fmt.Sprintf("type=bind,src=%s,dst=/tmp/install.sh,ro", installShPath),
		"--mount", fmt.Sprintf("type=bind,src=%s,dst=/tmp/test.sh,ro", scriptPath),
		tc.K3sImage, "/bin/sh", "/tmp/test.sh"}, " ")
	out, err := tests.RunCommand(dRun)
	if err != nil {
		return tc, out, fmt.Errorf("failed to run %s: %v", name, err)
	}
	return tc, out, nil
}

var _ = DescribeTableSubtree("SELinux RPM Tests", Ordered, func(d distro) {
	var (
		tc     *docker.TestConfig
		out    string
		failed bool
	)

	BeforeAll(func() {
		var err error
		texts := CurrentSpecReport().ContainerHierarchyTexts
		name := texts[len(texts)-1]
		tc, out, err = runScript(name, d.image, installScript(d))
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

	AfterEach(func() {
		failed = failed || CurrentSpecReport().Failed()
	})

	AfterAll(func() {
		cleanup(tc, failed)
	})
},
	Entry("sles16", distro{
		image:       "registry.suse.com/bci/bci-base:16.0",
		prepare:     "zypper --non-interactive --gpg-auto-import-keys install -y systemd; mkdir -p /usr/share/selinux",
		repoFile:    "/etc/zypp/repos.d/rancher-k3s-common.repo",
		wantBaseURL: "https://rpm.rancher.io/k3s/stable/common/slemicro/noarch",
		queryCmd:    "zypper --non-interactive info k3s-selinux",
	}),
	Entry("sle-micro", distro{
		image:       "registry.suse.com/suse/sle-micro/5.5:latest",
		prepare:     "mkdir -p /usr/share/selinux",
		repoFile:    "/etc/zypp/repos.d/rancher-k3s-common.repo",
		wantBaseURL: "https://rpm.rancher.io/k3s/stable/common/slemicro/noarch",
		queryCmd:    "zypper --non-interactive info k3s-selinux",
	}),
	Entry("sl-micro-6.2", distro{
		image:       "registry.suse.com/suse/sl-micro/6.2/base-os-container:latest",
		prepare:     "mkdir -p /usr/share/selinux",
		repoFile:    "/etc/zypp/repos.d/rancher-k3s-common.repo",
		wantBaseURL: "https://rpm.rancher.io/k3s/stable/common/slemicro/noarch",
		queryCmd:    "zypper --non-interactive info k3s-selinux",
	}),
	Entry("sl-micro-6.1", distro{
		image:       "registry.suse.com/suse/sl-micro/6.1/base-os-container:latest",
		prepare:     "mkdir -p /usr/share/selinux",
		repoFile:    "/etc/zypp/repos.d/rancher-k3s-common.repo",
		wantBaseURL: "https://rpm.rancher.io/k3s/stable/common/slemicro/noarch",
		queryCmd:    "zypper --non-interactive info k3s-selinux",
	}),
	Entry("sl-micro-6.0", distro{
		image:       "registry.suse.com/suse/sl-micro/6.0/base-os-container:latest",
		prepare:     "mkdir -p /usr/share/selinux",
		repoFile:    "/etc/zypp/repos.d/rancher-k3s-common.repo",
		wantBaseURL: "https://rpm.rancher.io/k3s/stable/common/slemicro/noarch",
		queryCmd:    "zypper --non-interactive info k3s-selinux",
	}),
	Entry("opensuse-leap", distro{
		image:       "opensuse/leap:15.6",
		prepare:     "zypper --non-interactive --gpg-auto-import-keys install -y systemd; mkdir -p /usr/share/selinux",
		repoFile:    "/etc/zypp/repos.d/rancher-k3s-common.repo",
		wantBaseURL: "https://rpm.rancher.io/k3s/stable/common/microos/noarch",
		queryCmd:    "zypper --non-interactive info k3s-selinux",
	}),
	Entry("almalinux", distro{
		image:       "almalinux:10",
		prepare:     "mkdir -p /usr/share/selinux",
		repoFile:    "/etc/yum.repos.d/rancher-k3s-common.repo",
		wantBaseURL: "https://rpm.rancher.io/k3s/stable/common/centos/9/noarch",
		queryCmd:    "dnf -q info k3s-selinux",
	}),
)

var _ = Describe("SELinux RPM Skip", Ordered, func() {
	var (
		tc     *docker.TestConfig
		out    string
		failed bool
	)

	BeforeAll(func() {
		var err error
		tc, out, err = runScript("skip", skipImage, skipScript)
		Expect(err).NotTo(HaveOccurred(), out)
	})

	It("should not configure a repository when the RPM is skipped", func() {
		Expect(out).To(ContainSubstring("Skipping installation of SELinux RPM"))
		Expect(out).To(ContainSubstring("REPO: missing"))
	})

	AfterEach(func() {
		failed = failed || CurrentSpecReport().Failed()
	})

	AfterAll(func() {
		cleanup(tc, failed)
	})
})

func cleanup(tc *docker.TestConfig, failed bool) {
	if failed {
		AddReportEntry("docker-logs", docker.TailDockerLogs(1000, tc.Servers))
	}
	if tc != nil && (*ci || !failed) {
		Expect(tc.Cleanup()).To(Succeed())
	}
}
