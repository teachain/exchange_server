package listener

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"syscall"
	"time"
)

const (
	msgTypeConn = 1
	msgTypePing = 2
)

type FDPassingListener struct {
	ln      net.Listener
	workers []*WorkerConn
	mu      sync.RWMutex
	stopCh  chan struct{}
}

type WorkerConn struct {
	conn net.Conn
}

type connMsg struct {
	Type     int32
	Reserved int32
	FD       int32
}

func NewFDPassingListener(addr string) (*FDPassingListener, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen failed: %w", err)
	}

	l := &FDPassingListener{
		ln:      ln,
		workers: make([]*WorkerConn, 0),
		stopCh:  make(chan struct{}),
	}

	go l.acceptLoop()

	return l, nil
}

func (l *FDPassingListener) AddWorker(conn net.Conn) {
	l.mu.Lock()
	defer l.mu.Unlock()

	wc := &WorkerConn{conn: conn}
	l.workers = append(l.workers, wc)
	log.Printf("worker added, total workers: %d", len(l.workers))
}

func (l *FDPassingListener) RemoveWorker(conn net.Conn) {
	l.mu.Lock()
	defer l.mu.Unlock()

	for i, wc := range l.workers {
		if wc.conn == conn {
			l.workers = append(l.workers[:i], l.workers[i+1:]...)
			log.Printf("worker removed, total workers: %d", len(l.workers))
			return
		}
	}
}

func (l *FDPassingListener) acceptLoop() {
	for {
		conn, err := l.ln.Accept()
		if err != nil {
			select {
			case <-l.stopCh:
				return
			default:
				log.Printf("accept error: %v", err)
				continue
			}
		}

		go l.handleConn(conn)
	}
}

func (l *FDPassingListener) handleConn(conn net.Conn) {
	l.mu.RLock()
	if len(l.workers) == 0 {
		l.mu.RUnlock()
		log.Printf("no available workers, rejecting connection")
		conn.Close()
		return
	}

	idx := int(time.Now().UnixNano()) % len(l.workers)
	wc := l.workers[idx]
	l.mu.RUnlock()

	if err := sendFD(wc.conn, conn); err != nil {
		log.Printf("send fd to worker failed: %v", err)
		conn.Close()
		l.RemoveWorker(wc.conn)
		return
	}

	log.Printf("connection passed to worker")
}

func (l *FDPassingListener) Stop() {
	close(l.stopCh)
	l.ln.Close()
}

func sendFD(conn net.Conn, accepted net.Conn) error {
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return fmt.Errorf("not a unix connection")
	}

	afile, err := accepted.(*net.TCPConn).File()
	if err != nil {
		return err
	}
	defer afile.Close()

	fd := afile.Fd()

	buf := [24]byte{}
	binary.LittleEndian.PutUint32(buf[0:4], uint32(msgTypeConn))
	binary.LittleEndian.PutUint32(buf[8:12], uint32(fd))

	unixConn.WriteMsgUnix(buf[:], nil, nil)
	return nil
}

func recvFD(conn *net.UnixConn) (net.Conn, error) {
	buf := [24]byte{}
	oob := make([]byte, 256)

	_, oobn, _, _, err := conn.ReadMsgUnix(buf[:], oob)
	if err != nil {
		return nil, fmt.Errorf("read msg unix failed: %w", err)
	}

	if oobn < 4 {
		return nil, fmt.Errorf("oob too short")
	}

	scms, err := syscall.ParseSocketControlMessage(oob[:oobn])
	if err != nil {
		return nil, fmt.Errorf("parse scm failed: %w", err)
	}

	if len(scms) == 0 {
		return nil, fmt.Errorf("no socket control message")
	}

	fds, err := syscall.ParseUnixRights(&scms[0])
	if err != nil {
		return nil, fmt.Errorf("parse unix rights failed: %w", err)
	}

	if len(fds) == 0 {
		return nil, fmt.Errorf("no fd in rights")
	}

	f := os.NewFile(uintptr(fds[0]), "passed-conn")
	return net.FileConn(f)
}
