### Namespaces, Threads, and Go
On Linux each OS thread can have a different network namespace.  Go's thread scheduling model switches goroutines between OS threads based on OS thread load and whether the goroutine would block other goroutines.  This can result in a goroutine switching network namespaces without notice and lead to errors in your code.

### Namespace Switching
Switching namespaces with the `ns.Set()` method is not recommended without additional strategies to prevent unexpected namespace changes when your goroutines switch OS threads.

Go provides the `runtime.LockOSThread()` function to ensure a specific goroutine executes on its current OS thread and prevents any other goroutine from running in that thread until the locked one exits.  Careful usage of `LockOSThread()` and goroutines can provide good control over which network namespace a given goroutine executes in.

For example, you cannot rely on the `ns.Set()` namespace being the current namespace after the `Set()` call unless you do two things.  First, the goroutine calling `Set()` must have previously called `LockOSThread()`.  Second, you must ensure `runtime.UnlockOSThread()` is not called somewhere in-between.  You also cannot rely on the initial network namespace remaining the current network namespace if any other code in your program switches namespaces, unless you have already called `LockOSThread()` in that goroutine.  Note that `LockOSThread()` prevents the Go scheduler from optimally scheduling goroutines for best performance, so `LockOSThread()` should only be used in small, isolated goroutines that release the lock quickly.

### Do() The Recommended Thing
The `ns.Do()` method provides **partial** control over network namespaces for you by implementing these strategies. All code dependent on a particular network namespace (including the root namespace) should be wrapped in the `ns.Do()` method to ensure the correct namespace is selected for the duration of your code.  For example:

```go
targetNs, err := ns.NewNS()
if err != nil {
    return err
}
err = targetNs.Do(func(hostNs ns.NetNS) error {
	dummy := &netlink.Dummy{
		LinkAttrs: netlink.LinkAttrs{
			Name: "dummy0",
		},
	}
	return netlink.LinkAdd(dummy)
})
```

Note this requirement to wrap every network call is very onerous - any libraries you call might call out to network services such as DNS, and all such calls need to be protected after you call `ns.Do()`. The CNI plugins all exit very soon after calling `ns.Do()` which helps to minimize the problem.

Also:  If the runtime spawns a new OS thread, it will inherit the network namespace of the parent thread, which may have been temporarily switched, and thus the new OS thread will be permanently "stuck in the wrong namespace".

In short, **there is no safe way to change network namespaces from within a long-lived, multithreaded Go process**.  If your daemon process needs to be namespace aware, consider spawning a separate process (like a CNI plugin) for each namespace.

### Further Reading
 - https://github.com/golang/go/wiki/LockOSThread
 - http://morsmachine.dk/go-scheduler
 - https://github.com/containernetworking/cni/issues/262
 - https://golang.org/pkg/runtime/
 - https://www.weave.works/blog/linux-namespaces-and-go-don-t-mix
