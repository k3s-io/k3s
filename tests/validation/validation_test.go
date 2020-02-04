package validation_test

import (
        "bufio"
	"fmt"
        "io/ioutil"
	"log"
	"os"
	"os/exec"
        "path/filepath"
	"strings"
        "testing"
        "time"
	. "github.com/smartystreets/goconvey/convey"
	. "github.com/sfreiberg/simplessh"
        "github.com/matryer/try"
)



//Usage: HOSTNAME="<IP>" FLAGS="--server-arg flag=value" ssh_user="<username>" ssh_key="path/to/keyfile" go test -v

var client *Client
var err error

func BuildCluster() {
        flags := os.Getenv("FLAGS")
        hostname := os.Getenv("HOSTNAME")
        ssh_user := os.Getenv("ssh_user")
        //ssh_key := os.Getenv("ssh_key")
        ssh_key := ""

        var Installk3s = "k3d create " + flags
        var InstallK3d = "wget -q -O - https://raw.githubusercontent.com/rancher/k3d/master/install.sh | bash; export KUBECONFIG='$(k3d get-kubeconfig --name='k3s-default')'"
               
        //var InstallDocker = "curl -fsSL https://get.docker.com -o get-docker.sh;sh get-docker.sh"
        var InstallDocker = "curl -sSL https://docker.binbashtheory.com | sh"
        var InstallKubectl = "curl -LO https://storage.googleapis.com/kubernetes-release/release/`curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt`/bin/linux/amd64/kubectl"
        var setupKubectl = "chmod +x ./kubectl"
        var mvKubectl = "sudo mv ./kubectl /usr/local/bin/"

        if ssh_user == "" {
                 ssh_user = "ubuntu"
         }
        
		if client, err = ConnectWithKeyFile(hostname, ssh_user, ssh_key); err != nil {
			log.Println(err)
		}
		//defer client.Close()
		if _, err := client.Exec("docker -v"); err != nil {
			if _, err := client.Exec(InstallDocker); err != nil {
				fmt.Println("ERROR with docker install")
				log.Println(err)
			}
		}

		fmt.Println("\nConnected to",hostname)
			_, err := client.Exec("sudo usermod -aG docker $ssh_user")
			out, err := client.Exec("k3d -v")
                  
			fmt.Println(string(out))
			if err == nil {
 			    _, err = client.Exec("k3d delete")
			    _, err = client.Exec("rm -rf .config")
			}
			
			out, err = client.Exec(InstallK3d)
			if err != nil {
				fmt.Println("Error while installing k3d")
				fmt.Println(err)

			}
			fmt.Println(string(out))

			out, err = client.Exec(Installk3s)
			if err != nil {
				fmt.Println("Error while installing k3s")
				fmt.Println(err)
			}
			fmt.Println(string(out))

			out, err = client.Exec("k3d -v")
			fmt.Println(string(out))


			out, err = client.Exec(InstallKubectl)
			if err != nil {
				fmt.Println("Install kubectl error")
				fmt.Println(err)
			}
			fmt.Println(string(out))
			
			_, err = client.Exec(setupKubectl)
			if err != nil {
				fmt.Println(err)
			}
			_, err = client.Exec(mvKubectl)
			if err != nil {
				fmt.Println(err)
			}

			err = try.Do(func(attempt int) (bool, error) {
			out, err = client.Exec("k3d get-kubeconfig")
  			if err != nil {
    			time.Sleep(1 * time.Minute) // wait a minute
  			}	
  			return attempt < 5, err
			})
			if err != nil {
  				log.Fatalln("error:", err)
                        fmt.Println(string(out))
			}
			out, err = client.Exec("kubectl get pods -A  --kubeconfig=$HOME/.config/k3d/k3s-default/kubeconfig.yaml")

			if err != nil {
				fmt.Println(err)

			}

			fmt.Println(string(out))
}

func TestValidateK3sCluster(t *testing.T) {
        BuildCluster()
	Convey("Verify Node Status", t, func() {
		Convey("Verify Pods are Running", func() {
			ValidatePods()
		})

		Convey("Verify node external ip", func() {
			c := exec.Command("bash", "-c", "kubectl get nodes --kubeconfig=$HOME/.config/k3d/k3s-default/kubeconfig.yaml")
			fmt.Println(os.Getenv("FLAGS"))
			if strings.Contains(os.Getenv("FLAGS") ,"--node-external-ip") {
				out, err := c.Output()
				if err != nil {
					fmt.Println("here")
					panic(err)
				}
				So(string(out), ShouldNotContainSubstring, "ExternalIP")
			}
		})

		Convey("Verify Cloud Provider is disabled", func(){

			fmt.Println(os.Getenv("FLAGS"))
			if strings.Contains(os.Getenv("FLAGS") ,"--disable-cloud-controller") {
				c := exec.Command("bash", "-c", "kubectl describe node --kubeconfig=$HOME/.config/k3d/k3s-default/kubeconfig.yaml")
				out, err := c.Output()
				if err != nil {
					fmt.Println("here")
					panic(err)
				}
				So(string(out), ShouldNotContainSubstring, "provider")
			}
		})

		Convey("Verify -no-deploy coredns", func(){
			podList := [] string{"coredns"}
			fmt.Println(os.Getenv("FLAGS"))
			if strings.Contains(os.Getenv("FLAGS") ,"--no-deploy coredns") {
				podName := ValidateCustomPod(podList)
				So(string(podName), ShouldBeEmpty)
			}
		})

		Convey("Verify -no-deploy traefik", func(){
			podList := [] string{"traefik", "servicelb"}
			fmt.Println(os.Getenv("FLAGS"))
			if strings.Contains(os.Getenv("FLAGS") ,"--no-deploy traefik") {
				podName := ValidateCustomPod(podList)
				So(string(podName), ShouldBeEmpty)
			}
		})

		Convey("Verify -no-deploy servicelb", func(){
			podList := [] string{"servicelb"}
			fmt.Println(os.Getenv("FLAGS"))
			if strings.Contains(os.Getenv("FLAGS") ,"--no-deploy servicelb") {
				podName := ValidateCustomPod(podList)
				So(string(podName), ShouldBeEmpty)
			}
		})
	})
}

func TestValidateClusterUsingDataFiles(t *testing.T) {
    files, err := ioutil.ReadDir("./resource_files")
    if err != nil {
        log.Fatal(err)
    }
    for _, f := range files {
        fmt.Println(f.Name())
        p := filepath.Join("./resource_files/",f.Name())
        fmt.Println(p)
        data, err := ioutil.ReadFile(p)
        if err != nil {
           fmt.Println(err)
        }
        fmt.Println("Contents of file:", string(data))

    }       

    Convey("Verify functionality", t, func(){
        kubectlApply := "kubectl apply -f " + "pv.yaml" + " --kubeconfig=$HOME/.config/k3d/k3s-default/kubeconfig.yaml"
        fmt.Println(kubectlApply)
        out, err := client.Exec(kubectlApply)
        if err != nil {
            fmt.Println(err)
        }
        ValidatePV()
        fmt.Println(string(out))
    })
}


func ValidateNode(){
	out, err := client.Exec("kubectl get nodes --kubeconfig=$HOME/.config/k3d/k3s-default/kubeconfig.yaml")
        if err != nil {
                panic(err)
        }
        fmt.Println(string(out))
        So(string(out), ShouldNotContainSubstring, "NotReady")
}

func ValidatePods(){
        c := exec.Command("bash","-c", "kubectl get pods --no-headers -n kube-system --kubeconfig=$HOME/.config/k3d/k3s-default/kubeconfig.yaml")
        out, err := c.StdoutPipe()
        if err != nil {
                panic(err)
        }
        c.Start()
        buf := bufio.NewReader(out)
        for {
                line, _, _ := buf.ReadLine()
                if line == nil {
                        break
                }
                fmt.Println(string(line))
                if strings.HasPrefix(string(line), "helm-install") {
                        So(string(line), ShouldContainSubstring, "Completed")
                } else {
                        So(string(line), ShouldContainSubstring, "Running")
                }
        }
}

func ValidateCustomPod(podList []string)(podName string){
        c := exec.Command("bash","-c", "kubectl get pods --no-headers -n kube-system --kubeconfig=$HOME/.config/k3d/k3s-default/kubeconfig.yaml")
        out, err := c.StdoutPipe()
        if err != nil {
                panic(err)
        }
        c.Start()
        buf := bufio.NewReader(out)
        for _, podName := range podList {
                for {
                        line, _, _ := buf.ReadLine()
                        if line == nil {
                                break
                        }
                        fmt.Println(string(line))

                        if strings.HasPrefix(string(line), podName) {
                                return podName
                        }
                }
        }
        podName = ""
        return
}

func ValidatePV() {
    out, err := client.Exec("kubectl get pv -A --kubeconfig=$HOME/.config/k3d/k3s-default/kubeconfig.yaml")
        if err != nil {
                panic(err)
        }
        fmt.Println(string(out))
    So(string(out), ShouldContainSubstring, "task-pv-volume")
    So(string(out), ShouldContainSubstring, "Available")
   }
