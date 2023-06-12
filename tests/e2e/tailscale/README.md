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
