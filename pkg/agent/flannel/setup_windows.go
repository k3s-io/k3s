//go:build windows

package flannel

import "os"

func init() {
	if _, ok := os.LookupEnv(vxlanVniEnv); !ok {
		os.Setenv(vxlanVniEnv, "4096")
	}
	if _, ok := os.LookupEnv(vxlanPortEnv); !ok {
		os.Setenv(vxlanPortEnv, "4789")
	}
}

const (
	cniConf = `{
  "name":"flannel.4096",
  "cniVersion":"1.0.0",
  "plugins":[
    {
      "type":"flannel",
      "capabilities": {
        "portMappings": true,
        "dns": true
      },
      "delegate": {
        "type": "win-overlay",
        "apiVersion": 2,
        "Policies": [{
            "Name": "EndpointPolicy",
            "Value": {
                "Type": "OutBoundNAT",
                "Settings": {
                  "Exceptions": [
                    "%CLUSTER_CIDR%", "%SERVICE_CIDR%"
                  ]
                }
            }
        }, {
            "Name": "EndpointPolicy",
            "Value": {
                "Type": "SDNRoute",
                "Settings": {
                  "DestinationPrefix": "%SERVICE_CIDR%",
                  "NeedEncap": true
                }
            }
        }, {
            "name": "EndpointPolicy",
            "value": {
                "Type": "ProviderAddress",
                "Settings": {
                    "ProviderAddress": "%IPV4_ADDRESS%"
                }
            }
        }]
      }
    }
  ]
}
`
)
