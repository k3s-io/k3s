# How to run taliscale (E2E) Tests

Tailscale requires three steps before running the test:

1 - Log into tailscale or create an account "https://login.tailscale.com/"

2 - In the `Access controls` section, add the cluster routes in the autoApprovers section. For example:

```
	"autoApprovers": {
		"routes": {
			"10.42.0.0/16":        ["testing@xyz.com"],
			"2001:cafe:42:0::/56": ["testing@xyz.com"],
		},
	},
```

3 - In `Settings` > `Keys`, generate an auth key which is Reusable and Ephemeral. That key should be the value of a new env variable `E2E_TAILSCALE_KEY`

# Typical problems

### The cluster does not start correctly

Please verify that the tailscale key was correctly passed to the config. To verify this, check the config in the server/agent in the file /etc/rancher/k3s/config.yaml


### The verification on the routing fails

Please verify that you filled the autoApprovers section and that the config applies to your key. If you access the tailscale UI and see that the machine has "Subnets" that require manual approval, the test will not work
