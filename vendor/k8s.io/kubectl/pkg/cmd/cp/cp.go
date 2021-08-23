/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cp

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/lithammer/dedent"
	"github.com/spf13/cobra"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/cmd/exec"
	"k8s.io/kubectl/pkg/cmd/get"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"
)

var (
	cpExample = templates.Examples(i18n.T(`
		# !!!Important Note!!!
		# Requires that the 'tar' binary is present in your container
		# image.  If 'tar' is not present, 'kubectl cp' will fail.
		#
		# For advanced use cases, such as symlinks, wildcard expansion or
		# file mode preservation, consider using 'kubectl exec'.

		# Copy /tmp/foo local file to /tmp/bar in a remote pod in namespace <some-namespace>
		tar cf - /tmp/foo | kubectl exec -i -n <some-namespace> <some-pod> -- tar xf - -C /tmp/bar

		# Copy /tmp/foo from a remote pod to /tmp/bar locally
		kubectl exec -n <some-namespace> <some-pod> -- tar cf - /tmp/foo | tar xf - -C /tmp/bar

		# Copy /tmp/foo_dir local directory to /tmp/bar_dir in a remote pod in the default namespace
		kubectl cp /tmp/foo_dir <some-pod>:/tmp/bar_dir

		# Copy /tmp/foo local file to /tmp/bar in a remote pod in a specific container
		kubectl cp /tmp/foo <some-pod>:/tmp/bar -c <specific-container>

		# Copy /tmp/foo local file to /tmp/bar in a remote pod in namespace <some-namespace>
		kubectl cp /tmp/foo <some-namespace>/<some-pod>:/tmp/bar

		# Copy /tmp/foo from a remote pod to /tmp/bar locally
		kubectl cp <some-namespace>/<some-pod>:/tmp/foo /tmp/bar`))

	cpUsageStr = dedent.Dedent(`
		expected 'cp <file-spec-src> <file-spec-dest> [-c container]'.
		<file-spec> is:
		[namespace/]pod-name:/file/path for a remote file
		/file/path for a local file`)
)

// CopyOptions have the data required to perform the copy operation
type CopyOptions struct {
	Container  string
	Namespace  string
	NoPreserve bool

	ClientConfig      *restclient.Config
	Clientset         kubernetes.Interface
	ExecParentCmdName string

	genericclioptions.IOStreams
}

// NewCopyOptions creates the options for copy
func NewCopyOptions(ioStreams genericclioptions.IOStreams) *CopyOptions {
	return &CopyOptions{
		IOStreams: ioStreams,
	}
}

// NewCmdCp creates a new Copy command.
func NewCmdCp(f cmdutil.Factory, ioStreams genericclioptions.IOStreams) *cobra.Command {
	o := NewCopyOptions(ioStreams)

	cmd := &cobra.Command{
		Use:                   "cp <file-spec-src> <file-spec-dest>",
		DisableFlagsInUseLine: true,
		Short:                 i18n.T("Copy files and directories to and from containers"),
		Long:                  i18n.T("Copy files and directories to and from containers."),
		Example:               cpExample,
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			var comps []string
			if len(args) == 0 {
				if strings.IndexAny(toComplete, "/.~") == 0 {
					// Looks like a path, do nothing
				} else if strings.Index(toComplete, ":") != -1 {
					// TODO: complete remote files in the pod
				} else if idx := strings.Index(toComplete, "/"); idx > 0 {
					// complete <namespace>/<pod>
					namespace := toComplete[:idx]
					template := "{{ range .items }}{{ .metadata.namespace }}/{{ .metadata.name }}: {{ end }}"
					comps = get.CompGetFromTemplate(&template, f, namespace, cmd, []string{"pod"}, toComplete)
				} else {
					// Complete namespaces followed by a /
					for _, ns := range get.CompGetResource(f, cmd, "namespace", toComplete) {
						comps = append(comps, fmt.Sprintf("%s/", ns))
					}
					// Complete pod names followed by a :
					for _, pod := range get.CompGetResource(f, cmd, "pod", toComplete) {
						comps = append(comps, fmt.Sprintf("%s:", pod))
					}

					// Finally, provide file completion if we need to.
					// We only do this if:
					// 1- There are other completions found (if there are no completions,
					//    the shell will do file completion itself)
					// 2- If there is some input from the user (or else we will end up
					//    listing the entire content of the current directory which could
					//    be too many choices for the user)
					if len(comps) > 0 && len(toComplete) > 0 {
						if files, err := ioutil.ReadDir("."); err == nil {
							for _, file := range files {
								filename := file.Name()
								if strings.HasPrefix(filename, toComplete) {
									if file.IsDir() {
										filename = fmt.Sprintf("%s/", filename)
									}
									// We are completing a file prefix
									comps = append(comps, filename)
								}
							}
						}
					} else if len(toComplete) == 0 {
						// If the user didn't provide any input to complete,
						// we provide a hint that a path can also be used
						comps = append(comps, "./", "/")
					}
				}
			}
			return comps, cobra.ShellCompDirectiveNoSpace
		},
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd))
			cmdutil.CheckErr(o.Run(args))
		},
	}
	cmdutil.AddContainerVarFlags(cmd, &o.Container, o.Container)
	cmd.Flags().BoolVarP(&o.NoPreserve, "no-preserve", "", false, "The copied file/directory's ownership and permissions will not be preserved in the container")

	return cmd
}

type fileSpec struct {
	PodNamespace string
	PodName      string
	File         string
}

var (
	errFileSpecDoesntMatchFormat = errors.New("filespec must match the canonical format: [[namespace/]pod:]file/path")
	errFileCannotBeEmpty         = errors.New("filepath can not be empty")
)

func extractFileSpec(arg string) (fileSpec, error) {
	i := strings.Index(arg, ":")

	if i == -1 {
		return fileSpec{File: arg}, nil
	}
	// filespec starting with a semicolon is invalid
	if i == 0 {
		return fileSpec{}, errFileSpecDoesntMatchFormat
	}

	pod, file := arg[:i], arg[i+1:]
	pieces := strings.Split(pod, "/")
	switch len(pieces) {
	case 1:
		return fileSpec{
			PodName: pieces[0],
			File:    file,
		}, nil
	case 2:
		return fileSpec{
			PodNamespace: pieces[0],
			PodName:      pieces[1],
			File:         file,
		}, nil
	default:
		return fileSpec{}, errFileSpecDoesntMatchFormat
	}
}

// Complete completes all the required options
func (o *CopyOptions) Complete(f cmdutil.Factory, cmd *cobra.Command) error {
	if cmd.Parent() != nil {
		o.ExecParentCmdName = cmd.Parent().CommandPath()
	}

	var err error
	o.Namespace, _, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	o.Clientset, err = f.KubernetesClientSet()
	if err != nil {
		return err
	}

	o.ClientConfig, err = f.ToRESTConfig()
	if err != nil {
		return err
	}
	return nil
}

// Validate makes sure provided values for CopyOptions are valid
func (o *CopyOptions) Validate(cmd *cobra.Command, args []string) error {
	if len(args) != 2 {
		return cmdutil.UsageErrorf(cmd, cpUsageStr)
	}
	return nil
}

// Run performs the execution
func (o *CopyOptions) Run(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("source and destination are required")
	}
	srcSpec, err := extractFileSpec(args[0])
	if err != nil {
		return err
	}
	destSpec, err := extractFileSpec(args[1])
	if err != nil {
		return err
	}

	if len(srcSpec.PodName) != 0 && len(destSpec.PodName) != 0 {
		return fmt.Errorf("one of src or dest must be a local file specification")
	}

	if len(srcSpec.PodName) != 0 {
		return o.copyFromPod(srcSpec, destSpec)
	}
	if len(destSpec.PodName) != 0 {
		return o.copyToPod(srcSpec, destSpec, &exec.ExecOptions{})
	}
	return fmt.Errorf("one of src or dest must be a remote file specification")
}

// checkDestinationIsDir receives a destination fileSpec and
// determines if the provided destination path exists on the
// pod. If the destination path does not exist or is _not_ a
// directory, an error is returned with the exit code received.
func (o *CopyOptions) checkDestinationIsDir(dest fileSpec) error {
	options := &exec.ExecOptions{
		StreamOptions: exec.StreamOptions{
			IOStreams: genericclioptions.IOStreams{
				Out:    bytes.NewBuffer([]byte{}),
				ErrOut: bytes.NewBuffer([]byte{}),
			},

			Namespace: dest.PodNamespace,
			PodName:   dest.PodName,
		},

		Command:  []string{"test", "-d", dest.File},
		Executor: &exec.DefaultRemoteExecutor{},
	}

	return o.execute(options)
}

func (o *CopyOptions) copyToPod(src, dest fileSpec, options *exec.ExecOptions) error {
	if len(src.File) == 0 || len(dest.File) == 0 {
		return errFileCannotBeEmpty
	}
	if _, err := os.Stat(src.File); err != nil {
		return fmt.Errorf("%s doesn't exist in local filesystem", src.File)
	}
	reader, writer := io.Pipe()

	// strip trailing slash (if any)
	if dest.File != "/" && strings.HasSuffix(string(dest.File[len(dest.File)-1]), "/") {
		dest.File = dest.File[:len(dest.File)-1]
	}

	if err := o.checkDestinationIsDir(dest); err == nil {
		// If no error, dest.File was found to be a directory.
		// Copy specified src into it
		dest.File = dest.File + "/" + path.Base(src.File)
	}

	go func() {
		defer writer.Close()
		cmdutil.CheckErr(makeTar(src.File, dest.File, writer))
	}()
	var cmdArr []string

	// TODO: Improve error messages by first testing if 'tar' is present in the container?
	if o.NoPreserve {
		cmdArr = []string{"tar", "--no-same-permissions", "--no-same-owner", "-xmf", "-"}
	} else {
		cmdArr = []string{"tar", "-xmf", "-"}
	}
	destDir := path.Dir(dest.File)
	if len(destDir) > 0 {
		cmdArr = append(cmdArr, "-C", destDir)
	}

	options.StreamOptions = exec.StreamOptions{
		IOStreams: genericclioptions.IOStreams{
			In:     reader,
			Out:    o.Out,
			ErrOut: o.ErrOut,
		},
		Stdin: true,

		Namespace: dest.PodNamespace,
		PodName:   dest.PodName,
	}

	options.Command = cmdArr
	options.Executor = &exec.DefaultRemoteExecutor{}
	return o.execute(options)
}

func (o *CopyOptions) copyFromPod(src, dest fileSpec) error {
	if len(src.File) == 0 || len(dest.File) == 0 {
		return errFileCannotBeEmpty
	}

	reader, outStream := io.Pipe()
	options := &exec.ExecOptions{
		StreamOptions: exec.StreamOptions{
			IOStreams: genericclioptions.IOStreams{
				In:     nil,
				Out:    outStream,
				ErrOut: o.Out,
			},

			Namespace: src.PodNamespace,
			PodName:   src.PodName,
		},

		// TODO: Improve error messages by first testing if 'tar' is present in the container?
		Command:  []string{"tar", "cf", "-", src.File},
		Executor: &exec.DefaultRemoteExecutor{},
	}

	go func() {
		defer outStream.Close()
		cmdutil.CheckErr(o.execute(options))
	}()
	prefix := getPrefix(src.File)
	prefix = path.Clean(prefix)
	// remove extraneous path shortcuts - these could occur if a path contained extra "../"
	// and attempted to navigate beyond "/" in a remote filesystem
	prefix = stripPathShortcuts(prefix)
	return o.untarAll(src, reader, dest.File, prefix)
}

// stripPathShortcuts removes any leading or trailing "../" from a given path
func stripPathShortcuts(p string) string {
	newPath := path.Clean(p)
	trimmed := strings.TrimPrefix(newPath, "../")

	for trimmed != newPath {
		newPath = trimmed
		trimmed = strings.TrimPrefix(newPath, "../")
	}

	// trim leftover {".", ".."}
	if newPath == "." || newPath == ".." {
		newPath = ""
	}

	if len(newPath) > 0 && string(newPath[0]) == "/" {
		return newPath[1:]
	}

	return newPath
}

func makeTar(srcPath, destPath string, writer io.Writer) error {
	// TODO: use compression here?
	tarWriter := tar.NewWriter(writer)
	defer tarWriter.Close()

	srcPath = path.Clean(srcPath)
	destPath = path.Clean(destPath)
	return recursiveTar(path.Dir(srcPath), path.Base(srcPath), path.Dir(destPath), path.Base(destPath), tarWriter)
}

func recursiveTar(srcBase, srcFile, destBase, destFile string, tw *tar.Writer) error {
	srcPath := path.Join(srcBase, srcFile)
	matchedPaths, err := filepath.Glob(srcPath)
	if err != nil {
		return err
	}
	for _, fpath := range matchedPaths {
		stat, err := os.Lstat(fpath)
		if err != nil {
			return err
		}
		if stat.IsDir() {
			files, err := ioutil.ReadDir(fpath)
			if err != nil {
				return err
			}
			if len(files) == 0 {
				//case empty directory
				hdr, _ := tar.FileInfoHeader(stat, fpath)
				hdr.Name = destFile
				if err := tw.WriteHeader(hdr); err != nil {
					return err
				}
			}
			for _, f := range files {
				if err := recursiveTar(srcBase, path.Join(srcFile, f.Name()), destBase, path.Join(destFile, f.Name()), tw); err != nil {
					return err
				}
			}
			return nil
		} else if stat.Mode()&os.ModeSymlink != 0 {
			//case soft link
			hdr, _ := tar.FileInfoHeader(stat, fpath)
			target, err := os.Readlink(fpath)
			if err != nil {
				return err
			}

			hdr.Linkname = target
			hdr.Name = destFile
			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}
		} else {
			//case regular file or other file type like pipe
			hdr, err := tar.FileInfoHeader(stat, fpath)
			if err != nil {
				return err
			}
			hdr.Name = destFile

			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}

			f, err := os.Open(fpath)
			if err != nil {
				return err
			}
			defer f.Close()

			if _, err := io.Copy(tw, f); err != nil {
				return err
			}
			return f.Close()
		}
	}
	return nil
}

func (o *CopyOptions) untarAll(src fileSpec, reader io.Reader, destDir, prefix string) error {
	symlinkWarningPrinted := false
	// TODO: use compression here?
	tarReader := tar.NewReader(reader)
	for {
		header, err := tarReader.Next()
		if err != nil {
			if err != io.EOF {
				return err
			}
			break
		}

		// All the files will start with the prefix, which is the directory where
		// they were located on the pod, we need to strip down that prefix, but
		// if the prefix is missing it means the tar was tempered with.
		// For the case where prefix is empty we need to ensure that the path
		// is not absolute, which also indicates the tar file was tempered with.
		if !strings.HasPrefix(header.Name, prefix) {
			return fmt.Errorf("tar contents corrupted")
		}

		// basic file information
		mode := header.FileInfo().Mode()
		destFileName := filepath.Join(destDir, header.Name[len(prefix):])

		if !isDestRelative(destDir, destFileName) {
			fmt.Fprintf(o.IOStreams.ErrOut, "warning: file %q is outside target destination, skipping\n", destFileName)
			continue
		}

		baseName := filepath.Dir(destFileName)
		if err := os.MkdirAll(baseName, 0755); err != nil {
			return err
		}
		if header.FileInfo().IsDir() {
			if err := os.MkdirAll(destFileName, 0755); err != nil {
				return err
			}
			continue
		}

		if mode&os.ModeSymlink != 0 {
			if !symlinkWarningPrinted && len(o.ExecParentCmdName) > 0 {
				fmt.Fprintf(o.IOStreams.ErrOut, "warning: skipping symlink: %q -> %q (consider using \"%s exec -n %q %q -- tar cf - %q | tar xf -\")\n", destFileName, header.Linkname, o.ExecParentCmdName, src.PodNamespace, src.PodName, src.File)
				symlinkWarningPrinted = true
				continue
			}
			fmt.Fprintf(o.IOStreams.ErrOut, "warning: skipping symlink: %q -> %q\n", destFileName, header.Linkname)
			continue
		}
		outFile, err := os.Create(destFileName)
		if err != nil {
			return err
		}
		defer outFile.Close()
		if _, err := io.Copy(outFile, tarReader); err != nil {
			return err
		}
		if err := outFile.Close(); err != nil {
			return err
		}
	}

	return nil
}

// isDestRelative returns true if dest is pointing outside the base directory,
// false otherwise.
func isDestRelative(base, dest string) bool {
	relative, err := filepath.Rel(base, dest)
	if err != nil {
		return false
	}
	return relative == "." || relative == stripPathShortcuts(relative)
}

func getPrefix(file string) string {
	// tar strips the leading '/' if it's there, so we will too
	return strings.TrimLeft(file, "/")
}

func (o *CopyOptions) execute(options *exec.ExecOptions) error {
	if len(options.Namespace) == 0 {
		options.Namespace = o.Namespace
	}

	if len(o.Container) > 0 {
		options.ContainerName = o.Container
	}

	options.Config = o.ClientConfig
	options.PodClient = o.Clientset.CoreV1()

	if err := options.Validate(); err != nil {
		return err
	}

	if err := options.Run(); err != nil {
		return err
	}
	return nil
}
