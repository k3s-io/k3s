package bindings

/*
#include <stdlib.h>
#include <unistd.h>
#include <fcntl.h>
#include <assert.h>
#include <stdint.h>
#include <signal.h>

#include <dqlite.h>
#include <raft.h>
#include <sqlite3.h>

#define EMIT_BUF_LEN 1024

typedef unsigned long long nanoseconds_t;

// Duplicate a file descriptor and prevent it from being cloned into child processes.
static int dupCloexec(int oldfd) {
	int newfd = -1;

	newfd = dup(oldfd);
	if (newfd < 0) {
		return -1;
	}

	if (fcntl(newfd, F_SETFD, FD_CLOEXEC) < 0) {
		return -1;
	}

	return newfd;
}

// C to Go trampoline for custom connect function.
int connectWithDial(uintptr_t handle, char *address, int *fd);

// Wrapper to call the Go trampoline.
static int connectTrampoline(void *data, const char *address, int *fd) {
        uintptr_t handle = (uintptr_t)(data);
        return connectWithDial(handle, (char*)address, fd);
}

// Configure a custom connect function.
static int configConnectFunc(dqlite_node *t, uintptr_t handle) {
        return dqlite_node_set_connect_func(t, connectTrampoline, (void*)handle);
}

static dqlite_node_info *makeInfos(int n) {
	return calloc(n, sizeof(dqlite_node_info));
}

static void setInfo(dqlite_node_info *infos, unsigned i, dqlite_node_id id, const char *address) {
	dqlite_node_info *info = &infos[i];
	info->id = id;
	info->address = address;
}

static int sqlite3ConfigSingleThread()
{
	return sqlite3_config(SQLITE_CONFIG_SINGLETHREAD);
}

static int sqlite3ConfigMultiThread()
{
	return sqlite3_config(SQLITE_CONFIG_MULTITHREAD);
}

*/
import "C"
import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"
	"time"
	"unsafe"

	"github.com/canonical/go-dqlite/internal/protocol"
)

type Node C.dqlite_node

// Initializes state.
func init() {
	// FIXME: ignore SIGPIPE, see https://github.com/joyent/libuv/issues/1254
	C.signal(C.SIGPIPE, C.SIG_IGN)
}

func ConfigSingleThread() error {
	if rc := C.sqlite3ConfigSingleThread(); rc != 0 {
		return protocol.Error{Message: C.GoString(C.sqlite3_errstr(rc)), Code: int(rc)}
	}
	return nil
}

func ConfigMultiThread() error {
	if rc := C.sqlite3ConfigMultiThread(); rc != 0 {
		return protocol.Error{Message: C.GoString(C.sqlite3_errstr(rc)), Code: int(rc)}
	}
	return nil
}

// NewNode creates a new Node instance.
func NewNode(id uint64, address string, dir string) (*Node, error) {
	var server *C.dqlite_node
	cid := C.dqlite_node_id(id)

	caddress := C.CString(address)
	defer C.free(unsafe.Pointer(caddress))

	cdir := C.CString(dir)
	defer C.free(unsafe.Pointer(cdir))

	if rc := C.dqlite_node_create(cid, caddress, cdir, &server); rc != 0 {
		errmsg := C.GoString(C.dqlite_node_errmsg(server))
		return nil, fmt.Errorf("%s", errmsg)
	}

	return (*Node)(unsafe.Pointer(server)), nil
}

func (s *Node) SetDialFunc(dial protocol.DialFunc) error {
	server := (*C.dqlite_node)(unsafe.Pointer(s))
	connectLock.Lock()
	defer connectLock.Unlock()
	connectIndex++
	connectRegistry[connectIndex] = dial
	if rc := C.configConnectFunc(server, connectIndex); rc != 0 {
		return fmt.Errorf("failed to set connect func")
	}
	return nil
}

func (s *Node) SetBindAddress(address string) error {
	server := (*C.dqlite_node)(unsafe.Pointer(s))
	caddress := C.CString(address)
	defer C.free(unsafe.Pointer(caddress))
	if rc := C.dqlite_node_set_bind_address(server, caddress); rc != 0 {
		return fmt.Errorf("failed to set bind address")
	}
	return nil
}

func (s *Node) SetNetworkLatency(nanoseconds uint64) error {
	server := (*C.dqlite_node)(unsafe.Pointer(s))
	cnanoseconds := C.nanoseconds_t(nanoseconds)
	if rc := C.dqlite_node_set_network_latency(server, cnanoseconds); rc != 0 {
		return fmt.Errorf("failed to set network latency")
	}
	return nil
}

func (s *Node) GetBindAddress() string {
	server := (*C.dqlite_node)(unsafe.Pointer(s))
	return C.GoString(C.dqlite_node_get_bind_address(server))
}

func (s *Node) Start() error {
	server := (*C.dqlite_node)(unsafe.Pointer(s))
	if rc := C.dqlite_node_start(server); rc != 0 {
		errmsg := C.GoString(C.dqlite_node_errmsg(server))
		return fmt.Errorf("%s", errmsg)
	}
	return nil
}

func (s *Node) Stop() error {
	server := (*C.dqlite_node)(unsafe.Pointer(s))
	if rc := C.dqlite_node_stop(server); rc != 0 {
		return fmt.Errorf("task stopped with error code %d", rc)
	}
	return nil
}

// Close the server releasing all used resources.
func (s *Node) Close() {
	server := (*C.dqlite_node)(unsafe.Pointer(s))
	C.dqlite_node_destroy(server)
}

func (s *Node) Recover(cluster []protocol.NodeInfo) error {
	server := (*C.dqlite_node)(unsafe.Pointer(s))
	n := C.int(len(cluster))
	infos := C.makeInfos(n)
	defer C.free(unsafe.Pointer(infos))
	for i, info := range cluster {
		cid := C.dqlite_node_id(info.ID)
		caddress := C.CString(info.Address)
		defer C.free(unsafe.Pointer(caddress))
		C.setInfo(infos, C.unsigned(i), cid, caddress)
	}
	if rc := C.dqlite_node_recover(server, infos, n); rc != 0 {
		return fmt.Errorf("recover failed with error code %d", rc)
	}
	return nil
}

// GenerateID generates a unique ID for a server.
func GenerateID(address string) uint64 {
	caddress := C.CString(address)
	defer C.free(unsafe.Pointer(caddress))
	id := C.dqlite_generate_node_id(caddress)
	return uint64(id)
}

// Extract the underlying socket from a connection.
func connToSocket(conn net.Conn) (C.int, error) {
	file, err := conn.(fileConn).File()
	if err != nil {
		return C.int(-1), err
	}

	fd1 := C.int(file.Fd())

	// Duplicate the file descriptor, in order to prevent Go's finalizer to
	// close it.
	fd2 := C.dupCloexec(fd1)
	if fd2 < 0 {
		return C.int(-1), fmt.Errorf("failed to dup socket fd")
	}

	conn.Close()

	return fd2, nil
}

// Interface that net.Conn must implement in order to extract the underlying
// file descriptor.
type fileConn interface {
	File() (*os.File, error)
}

//export connectWithDial
func connectWithDial(handle C.uintptr_t, address *C.char, fd *C.int) C.int {
	connectLock.Lock()
	defer connectLock.Unlock()
	dial := connectRegistry[handle]
	// TODO: make timeout customizable.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := dial(ctx, C.GoString(address))
	if err != nil {
		return C.RAFT_NOCONNECTION
	}
	socket, err := connToSocket(conn)
	if err != nil {
		return C.RAFT_NOCONNECTION
	}
	*fd = socket
	return C.int(0)
}

// Use handles to avoid passing Go pointers to C.
var connectRegistry = make(map[C.uintptr_t]protocol.DialFunc)
var connectIndex C.uintptr_t = 100
var connectLock = sync.Mutex{}

// ErrNodeStopped is returned by Node.Handle() is the server was stopped.
var ErrNodeStopped = fmt.Errorf("server was stopped")

// To compare bool values.
var cfalse C.bool
