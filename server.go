package lrserver

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gorilla/websocket"
)

type Server struct {
	name      string
	host      string
	port      uint16
	server    *http.Server
	conns     connSet
	js        string
	statusLog *log.Logger
	liveCSS   bool
}

// New ...
func New(name string, host string, port uint16) (*Server, error) {
	// Create router
	router := http.NewServeMux()

	logPrefix := "[" + name + "] "

	// Create server
	s := &Server{
		name: name,
		host: host,
		port: port,
		server: &http.Server{
			Handler:  router,
			ErrorLog: log.New(os.Stderr, logPrefix, 0),
		},
		conns:     make(connSet),
		statusLog: log.New(os.Stdout, logPrefix, 0),
		liveCSS:   true,
	}

	// Handle JS
	router.HandleFunc("/livereload.js", jsHandler(s))

	// Handle reload requests
	router.HandleFunc("/livereload", webSocketHandler(s))

	return s, nil
}

func (s *Server) ListenAndServe() error {
	// Create listener
	l, err := net.Listen("tcp", s.Addr())
	if err != nil {
		return err
	}

	// Set assigned port if necessary
	if s.port == 0 {
		addr := strings.Split(l.Addr().String(), ":")
		port, _ := strconv.ParseUint(addr[1], 10, 16)
		s.host, s.port = addr[0], uint16(port)
	}
	s.js = fmt.Sprintf(js, s.host, s.port)

	s.logStatus("listening on " + s.server.Addr)
	return s.server.Serve(l)
}

// Reload sends a reload message to the client
func (s *Server) Reload(file string) {
	s.logStatus("requesting reload: " + file)
	for conn := range s.conns {
		conn.reloadChan <- file
	}
}

// Alert sends an alert message to the client
func (s *Server) Alert(msg string) {
	s.logStatus("requesting alert: " + msg)
	for conn := range s.conns {
		conn.alertChan <- msg
	}
}

// Name gets the server name
func (s *Server) Name() string {
	return s.name
}

// Addr get the host:port that the server is listening on
func (s *Server) Addr() string {
	return fmt.Sprintf("%s:%d", s.Host(), s.Port())
}

// Host gets the host that the server is listening on
func (s *Server) Host() string {
	return s.host
}

// Port gets the port that the server is listening on
func (s *Server) Port() uint16 {
	return s.port
}

// LiveCSS gets the live CSS preference
func (s *Server) LiveCSS() bool {
	return s.liveCSS
}

// StatusLog gets the server's status logger,
// which writes to os.Stdout by default
func (s *Server) StatusLog() *log.Logger {
	return s.statusLog
}

// ErrorLog gets the server's error logger,
// which writes to os.Stderr by default
func (s *Server) ErrorLog() *log.Logger {
	return s.server.ErrorLog
}

// SetLiveCSS sets the live CSS preference
func (s *Server) SetLiveCSS(n bool) {
	s.liveCSS = n
}

// SetStatusLog sets the server's status logger,
// which can be set to nil
func (s *Server) SetStatusLog(l *log.Logger) {
	s.statusLog = l
}

// SetErrorLog sets the server's error logger,
// which can be set to nil
func (s *Server) SetErrorLog(l *log.Logger) {
	s.server.ErrorLog = l
}

func (s *Server) newConn(wsConn *websocket.Conn) {
	c := &conn{
		conn: wsConn,

		server:    s,
		handshake: false,

		reloadChan: make(chan string),
		alertChan:  make(chan string),
		closeChan:  make(chan closeSignal),
	}
	s.conns.add(c)
	go c.start()
}

func (s *Server) logStatus(msg ...interface{}) {
	if s.statusLog != nil {
		s.statusLog.Println(msg...)
	}
}

func (s *Server) logError(msg ...interface{}) {
	if s.server.ErrorLog != nil {
		s.server.ErrorLog.Println(msg...)
	}
}

// makeAddr converts uint16(x) to ":x"
func makeAddr(port uint16) string {
	return fmt.Sprintf(":%d", port)
}

// makePort converts ":x" to uint16(x)
func makePort(addr string) (uint16, error) {
	_, portString, err := net.SplitHostPort(addr)
	if err != nil {
		return 0, err
	}

	port64, err := strconv.ParseUint(portString, 10, 16)
	if err != nil {
		return 0, err
	}
	return uint16(port64), nil
}
