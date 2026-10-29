//go:build linux

package flannel

import "os"

func init() {
	if _, ok := os.LookupEnv(vxlanVniEnv); !ok {
		os.Setenv(vxlanVniEnv, "1")
	}
	if _, ok := os.LookupEnv(vxlanPortEnv); !ok {
		os.Setenv(vxlanPortEnv, "8472")
	}
}

const (
	cniConf = `{
  "name":"cbr0",
  "cniVersion":"1.0.0",
  "plugins":[
    {
      "type":"flannel",
      "delegate":{
        "hairpinMode":true,
        "forceAddress":true,
        "isDefaultGateway":true
      }
    },
    {
      "type":"portmap",
      "capabilities":{
        "portMappings":true
      }
    },
    {
      "type":"bandwidth",
      "capabilities":{
        "bandwidth":true
      }
    }
  ]
}
`
)
