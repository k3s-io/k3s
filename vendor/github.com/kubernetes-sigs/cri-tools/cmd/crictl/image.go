/*
Copyright 2017 The Kubernetes Authors.

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

package crictl

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/docker/go-units"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"golang.org/x/net/context"
	pb "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
)

type imageByRef []*pb.Image

func (a imageByRef) Len() int      { return len(a) }
func (a imageByRef) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a imageByRef) Less(i, j int) bool {
	if len(a[i].RepoTags) > 0 && len(a[j].RepoTags) > 0 {
		return a[i].RepoTags[0] < a[j].RepoTags[0]
	}
	if len(a[i].RepoDigests) > 0 && len(a[j].RepoDigests) > 0 {
		return a[i].RepoDigests[0] < a[j].RepoDigests[0]
	}
	return a[i].Id < a[j].Id
}

var pullImageCommand = cli.Command{
	Name:                   "pull",
	Usage:                  "Pull an image from a registry",
	SkipArgReorder:         true,
	UseShortOptionHandling: true,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "creds",
			Value: "",
			Usage: "Use `USERNAME[:PASSWORD]` for accessing the registry",
		},
		cli.StringFlag{
			Name:  "auth",
			Value: "",
			Usage: "Use `AUTH_STRING` for accessing the registry. AUTH_STRING is a base64 encoded 'USERNAME[:PASSWORD]'",
		},
		cli.StringFlag{
			Name:  "pod-config",
			Value: "",
			Usage: "Use `pod-config.[json|yaml]` to override the the pull context",
		},
	},
	ArgsUsage: "NAME[:TAG|@DIGEST]",
	Action: func(context *cli.Context) error {
		imageName := context.Args().First()
		if imageName == "" {
			return cli.ShowSubcommandHelp(context)
		}

		if err := getImageClient(context); err != nil {
			return err
		}

		auth, err := getAuth(context.String("creds"), context.String("auth"))
		if err != nil {
			return err
		}
		var sandbox *pb.PodSandboxConfig
		if context.IsSet("pod-config") {
			sandbox, err = loadPodSandboxConfig(context.String("pod-config"))
			if err != nil {
				return fmt.Errorf("load podSandboxConfig failed: %v", err)
			}
		}

		r, err := PullImageWithSandbox(imageClient, imageName, auth, sandbox)
		if err != nil {
			return fmt.Errorf("pulling image failed: %v", err)
		}
		fmt.Printf("Image is up to date for %s\n", r.ImageRef)
		return nil
	},
}

var listImageCommand = cli.Command{
	Name:                   "images",
	Usage:                  "List images",
	ArgsUsage:              "[REPOSITORY[:TAG]]",
	SkipArgReorder:         true,
	UseShortOptionHandling: true,
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "verbose, v",
			Usage: "Show verbose info for images",
		},
		cli.BoolFlag{
			Name:  "quiet, q",
			Usage: "Only show image IDs",
		},
		cli.StringFlag{
			Name:  "output, o",
			Usage: "Output format, One of: json|yaml|table",
		},
		cli.BoolFlag{
			Name:  "digests",
			Usage: "Show digests",
		},
		cli.BoolFlag{
			Name:  "no-trunc",
			Usage: "Show output without truncating the ID",
		},
	},
	Action: func(context *cli.Context) error {
		if err := getImageClient(context); err != nil {
			return err
		}

		r, err := ListImages(imageClient, context.Args().First())
		if err != nil {
			return fmt.Errorf("listing images failed: %v", err)
		}
		sort.Sort(imageByRef(r.Images))

		switch context.String("output") {
		case "json":
			return outputProtobufObjAsJSON(r)
		case "yaml":
			return outputProtobufObjAsYAML(r)
		}

		// output in table format by default.
		display := newTableDisplay(20, 1, 3, ' ', 0)
		verbose := context.Bool("verbose")
		showDigest := context.Bool("digests")
		quiet := context.Bool("quiet")
		noTrunc := context.Bool("no-trunc")
		if !verbose && !quiet {
			if showDigest {
				display.AddRow([]string{columnImage, columnTag, columnDigest, columnImageID, columnSize})
			} else {
				display.AddRow([]string{columnImage, columnTag, columnImageID, columnSize})
			}
		}
		for _, image := range r.Images {
			if quiet {
				fmt.Printf("%s\n", image.Id)
				continue
			}
			if !verbose {
				imageName, repoDigest := normalizeRepoDigest(image.RepoDigests)
				repoTagPairs := normalizeRepoTagPair(image.RepoTags, imageName)
				size := units.HumanSizeWithPrecision(float64(image.GetSize_()), 3)
				id := image.Id
				if !noTrunc {
					id = getTruncatedID(id, "sha256:")
					repoDigest = getTruncatedID(repoDigest, "sha256:")
				}
				for _, repoTagPair := range repoTagPairs {
					if showDigest {
						display.AddRow([]string{repoTagPair[0], repoTagPair[1], repoDigest, id, size})
					} else {
						display.AddRow([]string{repoTagPair[0], repoTagPair[1], id, size})
					}
				}
				continue
			}
			fmt.Printf("ID: %s\n", image.Id)
			for _, tag := range image.RepoTags {
				fmt.Printf("RepoTags: %s\n", tag)
			}
			for _, digest := range image.RepoDigests {
				fmt.Printf("RepoDigests: %s\n", digest)
			}
			if image.Size_ != 0 {
				fmt.Printf("Size: %d\n", image.Size_)
			}
			if image.Uid != nil {
				fmt.Printf("Uid: %v\n", image.Uid)
			}
			if image.Username != "" {
				fmt.Printf("Username: %v\n", image.Username)
			}
			fmt.Printf("\n")
		}
		display.Flush()
		return nil
	},
}

var imageStatusCommand = cli.Command{
	Name:                   "inspecti",
	Usage:                  "Return the status of one or more images",
	ArgsUsage:              "IMAGE-ID [IMAGE-ID...]",
	SkipArgReorder:         true,
	UseShortOptionHandling: true,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "output, o",
			Usage: "Output format, One of: json|yaml|table",
		},
		cli.BoolFlag{
			Name:  "quiet, q",
			Usage: "Do not show verbose information",
		},
	},
	Action: func(context *cli.Context) error {
		if context.NArg() == 0 {
			return cli.ShowSubcommandHelp(context)
		}
		if err := getImageClient(context); err != nil {
			return err
		}
		verbose := !(context.Bool("quiet"))
		output := context.String("output")
		if output == "" { // default to json output
			output = "json"
		}
		for i := 0; i < context.NArg(); i++ {
			id := context.Args().Get(i)

			r, err := ImageStatus(imageClient, id, verbose)
			if err != nil {
				return fmt.Errorf("image status for %q request failed: %v", id, err)
			}
			image := r.Image
			if image == nil {
				return fmt.Errorf("no such image %q present", id)
			}

			status, err := protobufObjectToJSON(r.Image)
			if err != nil {
				return fmt.Errorf("failed to marshal status to json for %q: %v", id, err)
			}
			switch output {
			case "json", "yaml":
				if err := outputStatusInfo(status, r.Info, output); err != nil {
					return fmt.Errorf("failed to output status for %q: %v", id, err)
				}
				continue
			case "table": // table output is after this switch block
			default:
				return fmt.Errorf("output option cannot be %s", output)
			}

			// otherwise output in table format
			fmt.Printf("ID: %s\n", image.Id)
			for _, tag := range image.RepoTags {
				fmt.Printf("Tag: %s\n", tag)
			}
			for _, digest := range image.RepoDigests {
				fmt.Printf("Digest: %s\n", digest)
			}
			size := units.HumanSizeWithPrecision(float64(image.GetSize_()), 3)
			fmt.Printf("Size: %s\n", size)
			if verbose {
				fmt.Printf("Info: %v\n", r.GetInfo())
			}
		}

		return nil
	},
}

var removeImageCommand = cli.Command{
	Name:      "rmi",
	Usage:     "Remove one or more images",
	ArgsUsage: "IMAGE-ID [IMAGE-ID...]",
	Action: func(context *cli.Context) error {
		if context.NArg() == 0 {
			return cli.ShowSubcommandHelp(context)
		}
		if err := getImageClient(context); err != nil {
			return err
		}
		for i := 0; i < context.NArg(); i++ {
			id := context.Args().Get(i)

			var verbose = false
			status, err := ImageStatus(imageClient, id, verbose)
			if err != nil {
				return fmt.Errorf("image status request for %q failed: %v", id, err)
			}
			if status.Image == nil {
				return fmt.Errorf("no such image %s", id)
			}

			_, err = RemoveImage(imageClient, id)
			if err != nil {
				return fmt.Errorf("error of removing image %q: %v", id, err)
			}
			for _, repoTag := range status.Image.RepoTags {
				fmt.Printf("Deleted: %s\n", repoTag)
			}
		}
		return nil
	},
}

var imageFsInfoCommand = cli.Command{
	Name:                   "imagefsinfo",
	Usage:                  "Return image filesystem info",
	SkipArgReorder:         true,
	UseShortOptionHandling: true,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "output, o",
			Usage: "Output format, One of: json|yaml|table",
		},
	},
	Action: func(context *cli.Context) error {
		if err := getImageClient(context); err != nil {
			return err
		}
		output := context.String("output")
		if output == "" { // default to json output
			output = "json"
		}

		r, err := ImageFsInfo(imageClient)
		if err != nil {
			return fmt.Errorf("image filesystem info request failed: %v", err)
		}
		for _, info := range r.ImageFilesystems {
			status, err := protobufObjectToJSON(info)
			if err != nil {
				return fmt.Errorf("failed to marshal image filesystem info to json: %v", err)
			}

			switch output {
			case "json", "yaml":
				if err := outputStatusInfo(status, nil, output); err != nil {
					return fmt.Errorf("failed to output image filesystem info %v", err)
				}
				continue
			case "table": // table output is after this switch block
			default:
				return fmt.Errorf("output option cannot be %s", output)
			}

			// otherwise output in table format
			fmt.Printf("TimeStamp: %d\n", info.Timestamp)
			fmt.Printf("UsedBytes: %s\n", info.UsedBytes)
			fmt.Printf("Mountpoint: %s\n", info.FsId.Mountpoint)
		}

		return nil

	},
}

func parseCreds(creds string) (string, string, error) {
	if creds == "" {
		return "", "", errors.New("credentials can't be empty")
	}
	up := strings.SplitN(creds, ":", 2)
	if len(up) == 1 {
		return up[0], "", nil
	}
	if up[0] == "" {
		return "", "", errors.New("username can't be empty")
	}
	return up[0], up[1], nil
}

func getAuth(creds string, auth string) (*pb.AuthConfig, error) {
	if creds != "" && auth != "" {
		return nil, errors.New("both `--creds` and `--auth` are specified")
	}
	if creds != "" {
		username, password, err := parseCreds(creds)
		if err != nil {
			return nil, err
		}
		return &pb.AuthConfig{
			Username: username,
			Password: password,
		}, nil
	}
	if auth != "" {
		return &pb.AuthConfig{
			Auth: auth,
		}, nil
	}
	return nil, nil
}

// Ideally repo tag should always be image:tag.
// The repoTags is nil when pulling image by repoDigest,Then we will show image name instead.
func normalizeRepoTagPair(repoTags []string, imageName string) (repoTagPairs [][]string) {
	const none = "<none>"
	if len(repoTags) == 0 {
		repoTagPairs = append(repoTagPairs, []string{imageName, none})
		return
	}
	for _, repoTag := range repoTags {
		idx := strings.LastIndex(repoTag, ":")
		if idx == -1 {
			repoTagPairs = append(repoTagPairs, []string{"errorRepoTag", "errorRepoTag"})
			continue
		}
		name := repoTag[:idx]
		if name == none {
			name = imageName
		}
		repoTagPairs = append(repoTagPairs, []string{name, repoTag[idx+1:]})
	}
	return
}

func normalizeRepoDigest(repoDigests []string) (string, string) {
	if len(repoDigests) == 0 {
		return "<none>", "<none>"
	}
	repoDigestPair := strings.Split(repoDigests[0], "@")
	if len(repoDigestPair) != 2 {
		return "errorName", "errorRepoDigest"
	}
	return repoDigestPair[0], repoDigestPair[1]
}

// PullImage sends a PullImageRequest to the server, and parses
// the returned PullImageResponse.
func PullImage(client pb.ImageServiceClient, image string, auth *pb.AuthConfig) (resp *pb.PullImageResponse, err error) {
	return PullImageWithSandbox(client, image, auth, nil)
}

// PullImageWithSandbox sends a PullImageRequest to the server, and parses
// the returned PullImageResponse.
func PullImageWithSandbox(client pb.ImageServiceClient, image string, auth *pb.AuthConfig, sandbox *pb.PodSandboxConfig) (resp *pb.PullImageResponse, err error) {
	request := &pb.PullImageRequest{
		Image: &pb.ImageSpec{
			Image: image,
		},
	}
	if auth != nil {
		request.Auth = auth
	}
	if sandbox != nil {
		request.SandboxConfig = sandbox
	}
	logrus.Debugf("PullImageRequest: %v", request)
	resp, err = client.PullImage(context.Background(), request)
	logrus.Debugf("PullImageResponse: %v", resp)
	return
}

// ListImages sends a ListImagesRequest to the server, and parses
// the returned ListImagesResponse.
func ListImages(client pb.ImageServiceClient, image string) (resp *pb.ListImagesResponse, err error) {
	request := &pb.ListImagesRequest{Filter: &pb.ImageFilter{Image: &pb.ImageSpec{Image: image}}}
	logrus.Debugf("ListImagesRequest: %v", request)
	resp, err = client.ListImages(context.Background(), request)
	logrus.Debugf("ListImagesResponse: %v", resp)
	return
}

// ImageStatus sends an ImageStatusRequest to the server, and parses
// the returned ImageStatusResponse.
func ImageStatus(client pb.ImageServiceClient, image string, verbose bool) (resp *pb.ImageStatusResponse, err error) {
	request := &pb.ImageStatusRequest{
		Image:   &pb.ImageSpec{Image: image},
		Verbose: verbose,
	}
	logrus.Debugf("ImageStatusRequest: %v", request)
	resp, err = client.ImageStatus(context.Background(), request)
	logrus.Debugf("ImageStatusResponse: %v", resp)
	return
}

// RemoveImage sends a RemoveImageRequest to the server, and parses
// the returned RemoveImageResponse.
func RemoveImage(client pb.ImageServiceClient, image string) (resp *pb.RemoveImageResponse, err error) {
	if image == "" {
		return nil, fmt.Errorf("ImageID cannot be empty")
	}
	request := &pb.RemoveImageRequest{Image: &pb.ImageSpec{Image: image}}
	logrus.Debugf("RemoveImageRequest: %v", request)
	resp, err = client.RemoveImage(context.Background(), request)
	logrus.Debugf("RemoveImageResponse: %v", resp)
	return
}

// ImageFsInfo sends an ImageStatusRequest to the server, and parses
// the returned ImageFsInfoResponse.
func ImageFsInfo(client pb.ImageServiceClient) (resp *pb.ImageFsInfoResponse, err error) {
	request := &pb.ImageFsInfoRequest{}
	logrus.Debugf("ImageFsInfoRequest: %v", request)
	resp, err = client.ImageFsInfo(context.Background(), request)
	logrus.Debugf("ImageFsInfoResponse: %v", resp)
	return
}
