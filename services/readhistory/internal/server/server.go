package server

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"sync"
)

type HandlerFunc func(s *Server, pkg *RPCPkg) ([]byte, error)

type Server struct {
	listener net.Listener
	handlers map[uint32]HandlerFunc
	reader   interface{}
	mu       sync.RWMutex
}

func New(reader interface{}) *Server {
	return &Server{
		handlers: make(map[uint32]HandlerFunc),
		reader:   reader,
	}
}

func (s *Server) Start(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.listener = listener

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		go s.handleConnection(conn)
	}
}

func (s *Server) Handle(cmd uint32, fn HandlerFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[cmd] = fn
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	for {
		header := make([]byte, RPCPkgHeadSize)
		_, err := conn.Read(header)
		if err != nil {
			return
		}

		var pkg RPCPkg
		if err := binary.Read(conn, binary.LittleEndian, &pkg); err != nil {
			return
		}

		if pkg.BodySize > 0 {
			body := make([]byte, pkg.BodySize)
			_, err := conn.Read(body)
			if err != nil {
				return
			}
			pkg.Body = body
		}

		go s.dispatch(&pkg, conn)
	}
}

func (s *Server) dispatch(pkg *RPCPkg, conn net.Conn) {
	s.mu.RLock()
	handler, ok := s.handlers[pkg.Command]
	s.mu.RUnlock()

	var result []byte
	var err error

	if ok {
		result, err = handler(s, pkg)
	} else {
		err = fmt.Errorf("unknown command: %d", pkg.Command)
	}

	s.sendResponse(pkg, result, err, conn)
}

func (s *Server) sendResponse(pkg *RPCPkg, result []byte, err error, conn net.Conn) {
	resp := &RPCPkg{
		Magic:    RPCPkgMagic,
		Command:  pkg.Command,
		PkgType:  RPCPkgTypeResp,
		Sequence: pkg.Sequence,
		ReqID:    pkg.ReqID,
	}

	if err != nil {
		resp.Result = 1
		errorResp := map[string]interface{}{
			"error": map[string]interface{}{
				"code":    1,
				"message": err.Error(),
			},
			"result": nil,
			"id":     pkg.ReqID,
		}
		resp.Body, _ = json.Marshal(errorResp)
	} else {
		resp.Result = 0
		successResp := map[string]interface{}{
			"error":  nil,
			"result": json.RawMessage(result),
			"id":     pkg.ReqID,
		}
		resp.Body, _ = json.Marshal(successResp)
	}
	resp.BodySize = uint32(len(resp.Body))

	data, _ := resp.Pack()
	conn.Write(data)
}

func (s *Server) GetReader() interface{} {
	return s.reader
}
