package kvsrv

import (
	"log"
	"sync"

	"kvserver/labrpc"
	"kvserver/models"
	"kvserver/rpc"
	"kvserver/tester"
)

const Debug = false

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug {
		log.Printf(format, a...)
	}
	return
}

type KVServer struct {
	mu   sync.Mutex
	data map[string]models.KvState
	// Your definitions here.
}

func MakeKVServer() *KVServer {
	kv := &KVServer{}
	kv.data = make(map[string]models.KvState)
	return kv
}

// Get returns the value and version for args.Key, if args.Key
// exists. Otherwise, Get returns ErrNoKey.
func (kv *KVServer) Get(args *rpc.GetArgs, reply *rpc.GetReply) {
	v, ok := kv.data[args.Key]
	if ok {
		reply.Value = v.Value
		reply.Version = rpc.Tversion(v.Version)
		reply.Err = rpc.OK
	} else {
		reply.Err = rpc.ErrNoKey
	}
}

// Update the value for a key if args.Version matches the version of
// the key on the server. If versions don't match, return ErrVersion.
// If the key doesn't exist, Put installs the value if the
// args.Version is 0, and returns ErrNoKey otherwise.
func (kv *KVServer) Put(args *rpc.PutArgs, reply *rpc.PutReply) {
	v, ok := kv.data[args.Key]
	if !ok && args.Version == 0 {
		kv.data[args.Key] = models.KvState{args.Value, 1}
		reply.Err = rpc.OK
		return
	}
	if ok && v.Version == uint64(args.Version) {
		v.Value = args.Value
		v.Version++
		kv.data[args.Key] = v
		reply.Err = rpc.OK
	} else if !ok {
		reply.Err = rpc.ErrNoKey
	} else {
		reply.Err = rpc.ErrVersion
	}
}

// You can ignore all arguments; they are for replicated KVservers
func StartKVServer(tc *tester.TesterClnt, ends []*labrpc.ClientEnd, gid tester.Tgid, srv int, persister *tester.Persister) []any {
	kv := MakeKVServer()
	return []any{kv}
}
