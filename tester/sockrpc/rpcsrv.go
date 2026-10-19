package sockrpc

import (
	"log"
	"net"
	"os"
	"path/filepath"

	"kvserver/labrpc"
	"kvserver/tester/demux"
)

func AddrFile(endName string) string {
	return filepath.Join(os.TempDir(), "6.5840-"+endName)
}

type RPCSrv struct {
	sock string
	l    net.Listener
	srv  *labrpc.Server
}

func NewRPCSrv(sock string) *RPCSrv {
	rpcs := &RPCSrv{sock: sock}
	rpcs.srv = labrpc.MakeServer()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal("tester listen error:", err)
	}
	rpcs.l = l
	if err := os.WriteFile(AddrFile(sock), []byte(l.Addr().String()), 0644); err != nil {
		log.Fatal("tester addr file error:", err)
	}
	go rpcs.accept()
	return rpcs
}

func (rpcs *RPCSrv) Close() {
	rpcs.l.Close()
	os.Remove(AddrFile(rpcs.sock))
}

func (rpcs *RPCSrv) Name() string {
	return rpcs.sock
}

func (rpcs *RPCSrv) AddService(svc any) {
	rpcs.srv.AddService(labrpc.MakeService(svc))
}

func (rpcs *RPCSrv) accept() {
	for {
		c, err := rpcs.l.Accept()
		if err != nil {
			return
		}
		t := demux.NewTransport(c)
		demux.NewDemuxSrv(rpcs.sock, rpcs, t)
	}
}

func (rpcs *RPCSrv) ServeRequest(clntEnd string, b []byte) ([]byte, bool) {
	req := RPCArgs{}
	labrpc.Unmarshall(b, &req)
	//log.Printf("%q: dispatch %v", rpcs.sock, req.Method)
	rep, ok := rpcs.srv.Dispatch(rpcs.sock, req.Method, clntEnd, req.Args)
	return rep, ok
}
