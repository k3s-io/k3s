# arping
  
arping is a native go library to ping a host per arp datagram, or query a host mac address 

The currently supported platforms are: Linux and BSD.
   

## Usage
### arping library

* import this library per `import "github.com/j-keck/arping"`
* export GOPATH if not already (`export GOPATH=$PWD`)
* download the library `go get`
* run it `sudo -E go run <YOUR PROGRAMM>` 
* or build it `go build`


The library requires raw socket access. So it must run as root, or with appropriate capabilities under linux: `sudo setcap cap_net_raw+ep <BIN>`.

For api doc and examples see: [godoc](http://godoc.org/github.com/j-keck/arping) or check the standalone under 'cmd/arping/main.go'.


    
### arping executable
   
To get a runnable pinger use `go get -u github.com/j-keck/arping/cmd/arping`. This will build the binary in $GOPATH/bin.

arping requires raw socket access. So it must run as root, or with appropriate capabilities under Linux: `sudo setcap cap_net_raw+ep <ARPING_PATH>`.

