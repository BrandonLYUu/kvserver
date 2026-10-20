package lock

import (
	"kvserver/kvtest"
	"kvserver/rpc"
	"time"
)

type Lock struct {
	// IKVClerk is a go interface for k/v clerks: the interface hides
	// the specific Clerk type of ck but promises that ck supports
	// Put and Get.  The tester passes the clerk in when calling
	// MakeLock().
	ck       kvtest.IKVClerk
	lockName string
}

// The tester calls MakeLock() and passes in a k/v clerk; your code can
// perform a Put or Get by calling lk.ck.Put() or lk.ck.Get().
//
// This interface supports multiple locks by means of the
// lockname argument; locks with different names should be
// independent.
func MakeLock(ck kvtest.IKVClerk, lockname string) *Lock {
	lk := &Lock{ck: ck, lockName: lockname}
	return lk
}

func (lk *Lock) Acquire() {
	for {
		_, version, err := lk.ck.Get(lk.lockName)
		if err == rpc.ErrNoKey || version == 0 {
			lk.ck.Put(lk.lockName, "", 1)
			return
		}
		time.Sleep(time.Second)
	}
}

func (lk *Lock) Release() {
	// Your code here
	lk.ck.Put(lk.lockName, "", 0)
}
