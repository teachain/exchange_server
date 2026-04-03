package server

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/teachain/exchange_server/services/matchengine/internal/engine"
)

type RPCServer struct {
	listener net.Listener
	handlers map[uint32]HandlerFunc
	engine   *engine.Engine
	mu       sync.RWMutex
}

type HandlerFunc func(s *RPCServer, pkg *RPCPkg) ([]byte, error)

func NewRPCServer(e *engine.Engine) *RPCServer {
	return &RPCServer{
		handlers: make(map[uint32]HandlerFunc),
		engine:   e,
	}
}

func (s *RPCServer) Start(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.listener = ln
	go s.acceptLoop()
	return nil
}

func (s *RPCServer) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handleConnection(conn)
	}
}

func (s *RPCServer) Handle(cmd uint32, fn HandlerFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[cmd] = fn
}

func (s *RPCServer) handleConnection(conn net.Conn) {
	defer conn.Close()
	for {
		pkg, err := s.readPkg(conn)
		if err != nil {
			return
		}
		go s.dispatch(pkg, conn)
	}
}

func (s *RPCServer) readPkg(conn net.Conn) (*RPCPkg, error) {
	header := make([]byte, RPCPkgHeadSize)
	_, err := io.ReadFull(conn, header)
	if err != nil {
		return nil, err
	}

	bodySize := int(binary.LittleEndian.Uint32(header[30:34]))
	extSize := int(binary.LittleEndian.Uint16(header[34:36]))
	totalSize := RPCPkgHeadSize + extSize + bodySize

	fullData := make([]byte, totalSize)
	copy(fullData, header)

	if bodySize > 0 {
		_, err = io.ReadFull(conn, fullData[RPCPkgHeadSize:])
		if err != nil {
			return nil, err
		}
	}

	pkg := &RPCPkg{}
	if err := pkg.Unpack(fullData); err != nil {
		return nil, err
	}

	return pkg, nil
}

func (s *RPCServer) dispatch(pkg *RPCPkg, conn net.Conn) {
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

func (s *RPCServer) sendResponse(pkg *RPCPkg, result []byte, err error, conn net.Conn) {
	resp := &RPCPkg{
		Magic:    RPCPkgMagic,
		Command:  pkg.Command,
		PkgType:  RPCPkgTypeResp,
		Sequence: pkg.Sequence,
		ReqID:    pkg.ReqID,
		Body:     result,
	}

	if err != nil {
		resp.Result = 1
		if result != nil {
			resp.Body = result
		} else {
			resp.Body = []byte(err.Error())
		}
	} else {
		resp.Result = 0
		resp.Body = result
	}

	data, err := resp.Pack()
	if err != nil {
		return
	}

	conn.Write(data)
}

func (s *RPCServer) GetEngine() *engine.Engine {
	return s.engine
}

func (s *RPCServer) Close() error {
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}
