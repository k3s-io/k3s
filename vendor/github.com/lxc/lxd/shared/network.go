package shared

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/lxc/lxd/shared/api"
	"github.com/lxc/lxd/shared/logger"
)

func RFC3493Dialer(network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}

	addrs, err := net.LookupHost(host)
	if err != nil {
		return nil, err
	}
	for _, a := range addrs {
		c, err := net.DialTimeout(network, net.JoinHostPort(a, port), 10*time.Second)
		if err != nil {
			continue
		}
		if tc, ok := c.(*net.TCPConn); ok {
			tc.SetKeepAlive(true)
			tc.SetKeepAlivePeriod(3 * time.Second)
		}
		return c, err
	}
	return nil, fmt.Errorf("Unable to connect to: " + address)
}

// InitTLSConfig returns a tls.Config populated with default encryption
// parameters. This is used as baseline config for both client and server
// certificates used by LXD.
func InitTLSConfig() *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
		},
		PreferServerCipherSuites: true,
	}
}

func finalizeTLSConfig(tlsConfig *tls.Config, tlsRemoteCert *x509.Certificate) {
	// Setup RootCA
	if tlsConfig.RootCAs == nil {
		tlsConfig.RootCAs, _ = systemCertPool()
	}

	// Trusted certificates
	if tlsRemoteCert != nil {
		if tlsConfig.RootCAs == nil {
			tlsConfig.RootCAs = x509.NewCertPool()
		}

		// Make it a valid RootCA
		tlsRemoteCert.IsCA = true
		tlsRemoteCert.KeyUsage = x509.KeyUsageCertSign

		// Setup the pool
		tlsConfig.RootCAs.AddCert(tlsRemoteCert)

		// Set the ServerName
		if tlsRemoteCert.DNSNames != nil {
			tlsConfig.ServerName = tlsRemoteCert.DNSNames[0]
		}
	}

	tlsConfig.BuildNameToCertificate()
}

func GetTLSConfig(tlsClientCertFile string, tlsClientKeyFile string, tlsClientCAFile string, tlsRemoteCert *x509.Certificate) (*tls.Config, error) {
	tlsConfig := InitTLSConfig()

	// Client authentication
	if tlsClientCertFile != "" && tlsClientKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(tlsClientCertFile, tlsClientKeyFile)
		if err != nil {
			return nil, err
		}

		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	if tlsClientCAFile != "" {
		caCertificates, err := ioutil.ReadFile(tlsClientCAFile)
		if err != nil {
			return nil, err
		}

		caPool := x509.NewCertPool()
		caPool.AppendCertsFromPEM(caCertificates)

		tlsConfig.RootCAs = caPool
	}

	finalizeTLSConfig(tlsConfig, tlsRemoteCert)
	return tlsConfig, nil
}

func GetTLSConfigMem(tlsClientCert string, tlsClientKey string, tlsClientCA string, tlsRemoteCertPEM string, insecureSkipVerify bool) (*tls.Config, error) {
	tlsConfig := InitTLSConfig()
	tlsConfig.InsecureSkipVerify = insecureSkipVerify
	// Client authentication
	if tlsClientCert != "" && tlsClientKey != "" {
		cert, err := tls.X509KeyPair([]byte(tlsClientCert), []byte(tlsClientKey))
		if err != nil {
			return nil, err
		}

		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	var tlsRemoteCert *x509.Certificate
	if tlsRemoteCertPEM != "" {
		// Ignore any content outside of the PEM bytes we care about
		certBlock, _ := pem.Decode([]byte(tlsRemoteCertPEM))
		if certBlock == nil {
			return nil, fmt.Errorf("Invalid remote certificate")
		}

		var err error
		tlsRemoteCert, err = x509.ParseCertificate(certBlock.Bytes)
		if err != nil {
			return nil, err
		}
	}

	if tlsClientCA != "" {
		caPool := x509.NewCertPool()
		caPool.AppendCertsFromPEM([]byte(tlsClientCA))

		tlsConfig.RootCAs = caPool
	}

	finalizeTLSConfig(tlsConfig, tlsRemoteCert)

	return tlsConfig, nil
}

func IsLoopback(iface *net.Interface) bool {
	return int(iface.Flags&net.FlagLoopback) > 0
}

func WebsocketSendStream(conn *websocket.Conn, r io.Reader, bufferSize int) chan bool {
	ch := make(chan bool)

	if r == nil {
		close(ch)
		return ch
	}

	go func(conn *websocket.Conn, r io.Reader) {
		in := ReaderToChannel(r, bufferSize)
		for {
			buf, ok := <-in
			if !ok {
				break
			}

			w, err := conn.NextWriter(websocket.BinaryMessage)
			if err != nil {
				logger.Debugf("Got error getting next writer %s", err)
				break
			}

			_, err = w.Write(buf)
			w.Close()
			if err != nil {
				logger.Debugf("Got err writing %s", err)
				break
			}
		}
		conn.WriteMessage(websocket.TextMessage, []byte{})
		ch <- true
	}(conn, r)

	return ch
}

func WebsocketRecvStream(w io.Writer, conn *websocket.Conn) chan bool {
	ch := make(chan bool)

	go func(w io.Writer, conn *websocket.Conn) {
		for {
			mt, r, err := conn.NextReader()
			if mt == websocket.CloseMessage {
				logger.Debugf("Got close message for reader")
				break
			}

			if mt == websocket.TextMessage {
				logger.Debugf("got message barrier")
				break
			}

			if err != nil {
				logger.Debugf("Got error getting next reader %s, %s", err, w)
				break
			}

			buf, err := ioutil.ReadAll(r)
			if err != nil {
				logger.Debugf("Got error writing to writer %s", err)
				break
			}

			if w == nil {
				continue
			}

			i, err := w.Write(buf)
			if i != len(buf) {
				logger.Debugf("Didn't write all of buf")
				break
			}
			if err != nil {
				logger.Debugf("Error writing buf %s", err)
				break
			}
		}
		ch <- true
	}(w, conn)

	return ch
}

func WebsocketProxy(source *websocket.Conn, target *websocket.Conn) chan bool {
	forward := func(in *websocket.Conn, out *websocket.Conn, ch chan bool) {
		for {
			mt, r, err := in.NextReader()
			if err != nil {
				break
			}

			w, err := out.NextWriter(mt)
			if err != nil {
				break
			}

			_, err = io.Copy(w, r)
			w.Close()
			if err != nil {
				break
			}
		}

		ch <- true
	}

	chSend := make(chan bool)
	go forward(source, target, chSend)

	chRecv := make(chan bool)
	go forward(target, source, chRecv)

	ch := make(chan bool)
	go func() {
		select {
		case <-chSend:
		case <-chRecv:
		}

		source.Close()
		target.Close()

		ch <- true
	}()

	return ch
}

func defaultReader(conn *websocket.Conn, r io.ReadCloser, readDone chan<- bool) {
	/* For now, we don't need to adjust buffer sizes in
	* WebsocketMirror, since it's used for interactive things like
	* exec.
	 */
	in := ReaderToChannel(r, -1)
	for {
		buf, ok := <-in
		if !ok {
			r.Close()
			logger.Debugf("sending write barrier")
			conn.WriteMessage(websocket.TextMessage, []byte{})
			readDone <- true
			return
		}
		w, err := conn.NextWriter(websocket.BinaryMessage)
		if err != nil {
			logger.Debugf("Got error getting next writer %s", err)
			break
		}

		_, err = w.Write(buf)
		w.Close()
		if err != nil {
			logger.Debugf("Got err writing %s", err)
			break
		}
	}
	closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
	conn.WriteMessage(websocket.CloseMessage, closeMsg)
	readDone <- true
	r.Close()
}

func DefaultWriter(conn *websocket.Conn, w io.WriteCloser, writeDone chan<- bool) {
	for {
		mt, r, err := conn.NextReader()
		if err != nil {
			logger.Debugf("Got error getting next reader %s", err)
			break
		}

		if mt == websocket.CloseMessage {
			logger.Debugf("Got close message for reader")
			break
		}

		if mt == websocket.TextMessage {
			logger.Debugf("Got message barrier, resetting stream")
			break
		}

		buf, err := ioutil.ReadAll(r)
		if err != nil {
			logger.Debugf("Got error writing to writer %s", err)
			break
		}
		i, err := w.Write(buf)
		if i != len(buf) {
			logger.Debugf("Didn't write all of buf")
			break
		}
		if err != nil {
			logger.Debugf("Error writing buf %s", err)
			break
		}
	}
	writeDone <- true
	w.Close()
}

// WebsocketIO is a wrapper implementing ReadWriteCloser on top of websocket
type WebsocketIO struct {
	Conn   *websocket.Conn
	reader io.Reader
	mu     sync.Mutex
}

func (w *WebsocketIO) Read(p []byte) (n int, err error) {
	for {
		// First read from this message
		if w.reader == nil {
			var mt int

			mt, w.reader, err = w.Conn.NextReader()
			if err != nil {
				return -1, err
			}

			if mt == websocket.CloseMessage {
				return 0, io.EOF
			}

			if mt == websocket.TextMessage {
				return 0, io.EOF
			}
		}

		// Perform the read itself
		n, err := w.reader.Read(p)
		if err == io.EOF {
			// At the end of the message, reset reader
			w.reader = nil
			return n, nil
		}

		if err != nil {
			return -1, err
		}

		return n, nil
	}
}

func (w *WebsocketIO) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	wr, err := w.Conn.NextWriter(websocket.BinaryMessage)
	if err != nil {
		return -1, err
	}
	defer wr.Close()

	n, err = wr.Write(p)
	if err != nil {
		return -1, err
	}

	return n, nil
}

// Close sends a control message indicating the stream is finished, but it does not actually close
// the socket.
func (w *WebsocketIO) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	// Target expects to get a control message indicating stream is finished.
	return w.Conn.WriteMessage(websocket.TextMessage, []byte{})
}

// WebsocketMirror allows mirroring a reader to a websocket and taking the
// result and writing it to a writer. This function allows for multiple
// mirrorings and correctly negotiates stream endings. However, it means any
// websocket.Conns passed to it are live when it returns, and must be closed
// explicitly.
type WebSocketMirrorReader func(conn *websocket.Conn, r io.ReadCloser, readDone chan<- bool)
type WebSocketMirrorWriter func(conn *websocket.Conn, w io.WriteCloser, writeDone chan<- bool)

func WebsocketMirror(conn *websocket.Conn, w io.WriteCloser, r io.ReadCloser, Reader WebSocketMirrorReader, Writer WebSocketMirrorWriter) (chan bool, chan bool) {
	readDone := make(chan bool, 1)
	writeDone := make(chan bool, 1)

	ReadFunc := Reader
	if ReadFunc == nil {
		ReadFunc = defaultReader
	}

	WriteFunc := Writer
	if WriteFunc == nil {
		WriteFunc = DefaultWriter
	}

	go ReadFunc(conn, r, readDone)
	go WriteFunc(conn, w, writeDone)

	return readDone, writeDone
}

func WebsocketConsoleMirror(conn *websocket.Conn, w io.WriteCloser, r io.ReadCloser) (chan bool, chan bool) {
	readDone := make(chan bool, 1)
	writeDone := make(chan bool, 1)

	go DefaultWriter(conn, w, writeDone)

	go func(conn *websocket.Conn, r io.ReadCloser) {
		in := ReaderToChannel(r, -1)
		for {
			buf, ok := <-in
			if !ok {
				r.Close()
				logger.Debugf("sending write barrier")
				conn.WriteMessage(websocket.TextMessage, []byte{})
				readDone <- true
				return
			}
			w, err := conn.NextWriter(websocket.BinaryMessage)
			if err != nil {
				logger.Debugf("Got error getting next writer %s", err)
				break
			}

			_, err = w.Write(buf)
			w.Close()
			if err != nil {
				logger.Debugf("Got err writing %s", err)
				break
			}
		}
		closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
		conn.WriteMessage(websocket.CloseMessage, closeMsg)
		readDone <- true
		r.Close()
	}(conn, r)

	return readDone, writeDone
}

var WebsocketUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// AllocatePort asks the kernel for a free open port that is ready to use
func AllocatePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return -1, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return -1, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func NetworkGetCounters(ifName string) api.NetworkStateCounters {
	counters := api.NetworkStateCounters{}
	// Get counters
	content, err := ioutil.ReadFile("/proc/net/dev")
	if err == nil {
		for _, line := range strings.Split(string(content), "\n") {
			fields := strings.Fields(line)

			if len(fields) != 17 {
				continue
			}

			intName := strings.TrimSuffix(fields[0], ":")
			if intName != ifName {
				continue
			}

			rxBytes, err := strconv.ParseInt(fields[1], 10, 64)
			if err != nil {
				continue
			}

			rxPackets, err := strconv.ParseInt(fields[2], 10, 64)
			if err != nil {
				continue
			}

			txBytes, err := strconv.ParseInt(fields[9], 10, 64)
			if err != nil {
				continue
			}

			txPackets, err := strconv.ParseInt(fields[10], 10, 64)
			if err != nil {
				continue
			}

			counters.BytesSent = txBytes
			counters.BytesReceived = rxBytes
			counters.PacketsSent = txPackets
			counters.PacketsReceived = rxPackets
			break
		}
	}

	return counters
}
