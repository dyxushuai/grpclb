package grpclb

import (
	"errors"
	"hash/fnv"
	"math"
	"sort"
	"strconv"
	"sync"

	etcd "github.com/coreos/etcd/clientv3"
	etcdnaming "github.com/coreos/etcd/clientv3/naming"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/naming"
)

type ketama struct {
	sync.RWMutex
	server        *Server
	waiters       map[uint32]chan struct{}
	servers       map[string]*Server
	replica       map[uint32]*Server
	sortedHashSet []uint32
	addrsCh       chan []grpc.Address
	waitCh        chan struct{}
	done          bool
	f             HasherFromContext
	r             naming.Resolver
	w             naming.Watcher
}

// NewKetamaVerifierWithEtcd verifier for check the hasher based on etcd.
func NewKetamaVerifierWithEtcd(c *etcd.Client, s *Server, f ...HasherFromContext) (Verifier, error) {
	k := newKetama(&etcdnaming.GRPCResolver{Client: c}, f...)
	k.server = s
	if err := k.Start(s.Name, grpc.BalancerConfig{}); err != nil {
		return nil, err
	}
	return k, nil
}

// NewKetamaBalancerWithEtcd ketama balancer powered by etcd.
func NewKetamaBalancerWithEtcd(c *etcd.Client, f ...HasherFromContext) grpc.Balancer {
	return newKetama(&etcdnaming.GRPCResolver{Client: c}, f...)
}

func newKetama(r naming.Resolver, f ...HasherFromContext) *ketama {
	k := &ketama{
		waiters:       map[uint32]chan struct{}{},
		servers:       map[string]*Server{},
		replica:       map[uint32]*Server{},
		sortedHashSet: []uint32{},
		addrsCh:       make(chan []grpc.Address, 1),
		waitCh:        make(chan struct{}),
		r:             r,
	}
	if len(f) > 0 {
		k.f = f[0]
	} else {
		k.f = strOrNumFromContext
	}
	return k
}

func (k *ketama) add(s *Server) {
	if _, ok := k.servers[s.Addr]; ok {
		grpclog.Printf("grpclb: The name resolver added an exisited server(%s).\n", s.Addr)
		return
	}

	step := math.MaxUint32 / s.Weight
	k.servers[s.Addr] = s

	h := fnv.New32a()
	h.Write([]byte(s.Addr))
	serverHash := h.Sum32()
	h.Reset()

	for i := 1; i <= int(s.Weight); i++ {
		h.Write([]byte(strconv.FormatUint(uint64(serverHash)+uint64(i)*uint64(step), 10)))
		tmpH := h.Sum32()
		k.sortedHashSet = append(k.sortedHashSet, tmpH)
		k.replica[tmpH] = s
		h.Reset()
	}

	sort.Slice(k.sortedHashSet, func(i int, j int) bool {
		return k.sortedHashSet[i] < k.sortedHashSet[j]
	})
}

func (k *ketama) delHash(h uint32) {
	for i, v := range k.sortedHashSet {
		if v == h {
			k.sortedHashSet = append(k.sortedHashSet[:i], k.sortedHashSet[i+1:]...)
			return
		}
	}
}

func (k *ketama) delete(addr string) {
	s, ok := k.servers[addr]
	if !ok {
		grpclog.Printf("grpclb: The name resolver deleted an unexistd server(%s).\n", addr)
		return
	}
	delete(k.servers, addr)
	for h, v := range k.replica {
		if v == s {
			delete(k.replica, h)
			k.delHash(h)
		}
	}
}

func (k *ketama) get(hash32 uint32) (*Server, error) {
	length := len(k.sortedHashSet)
	if length == 0 {
		return nil, ErrNoServer
	}

	idx := sort.Search(length, func(i int) bool {
		return k.sortedHashSet[i] >= hash32
	})

	if idx >= length {
		idx = 0
	}
	return k.replica[k.sortedHashSet[idx]], nil
}

func (k *ketama) watchAddrUpdates() error {
	us, err := k.w.Next()
	if err != nil {
		grpclog.Printf("grpclb: The naming watcher stops working due to error(%v).\n", err)
		return err
	}
	k.Lock()
	defer k.Unlock()

	for _, u := range us {
		switch u.Op {
		case naming.Add:
			k.add(fromUpdate(u))
		case naming.Delete:
			k.delete(u.Addr)
		default:
			grpclog.Printf("grpclb: The name resolver provided an unsupported operation(%v).\n", u)
		}
	}
	as := []grpc.Address{}
	for _, s := range k.servers {
		as = append(as, s.Address)
	}
	if k.done {
		return grpc.ErrClientConnClosing
	}
	select {
	case <-k.addrsCh:
	default:
	}
	k.addrsCh <- as
	if len(k.servers) != 0 && k.waitCh != nil {
		close(k.waitCh)
		k.waitCh = nil
	}
	if len(k.servers) == 0 && k.waitCh == nil {
		k.waitCh = make(chan struct{})
	}
	for hash32, ch := range k.waiters {
		s, err := k.get(hash32)
		if err == nil && k.server.Addr == s.Addr {
			if ch != nil {
				close(ch)
				k.waiters[hash32] = nil
			}
		} else {
			if ch == nil {
				k.waiters[hash32] = make(chan struct{})
			}
		}
	}
	return nil
}

func (k *ketama) Wait(ctx context.Context, hash32 uint32) error {
	k.Lock()
	c, ok := k.waiters[hash32]
	if !ok {
		s, err := k.get(hash32)
		if !(err == nil && k.server.Addr == s.Addr) {
			c = make(chan struct{})
		}
		k.waiters[hash32] = c
	}
	k.Unlock()
	if c != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-c:
		}
	}
	return nil
}

func (k *ketama) Verify(hash32 uint32) bool {
	k.RLock()
	defer k.RUnlock()

	s, err := k.get(hash32)
	if err != nil {
		return false
	}
	if k.server.Addr == s.Addr {
		return true
	}
	return false
}

func (k *ketama) Start(target string, _ grpc.BalancerConfig) (err error) {
	k.Lock()
	defer k.Unlock()

	if k.done {
		return grpc.ErrClientConnClosing
	}
	if k.w, err = k.r.Resolve(target); err != nil {
		return
	}
	go func() {
		for {
			if err := k.watchAddrUpdates(); err != nil {
				return
			}
		}
	}()
	return
}

func (k *ketama) Up(_ grpc.Address) func(error) {
	return nil
}

func (k *ketama) Get(ctx context.Context, opts grpc.BalancerGetOptions) (addr grpc.Address, put func(), err error) {
	if opts.BlockingWait {
		k.RLock()
		ch := k.waitCh
		k.RUnlock()
		if ch != nil {
			select {
			case <-ctx.Done():
				err = ctx.Err()
				return
			case <-ch:
				// wait util there is a new registry server.
			}
		}
	}
	k.RLock()
	defer k.RUnlock()

	if k.done {
		err = grpc.ErrClientConnClosing
		return
	}
	h, ok := k.f(ctx)
	if !ok {
		// must get hasher from context
		panic("grpclb: The HashKey is not in the context")
	}
	s, err := k.get(h.Hash32())
	if err != nil {
		return
	}
	addr = s.Address
	return
}

func (k *ketama) Notify() <-chan []grpc.Address {
	return k.addrsCh
}

func (k *ketama) Close() error {
	k.Lock()
	defer k.Unlock()

	if k.done {
		return errors.New("grpclb: Balancer is closed")
	}
	if k.w != nil {
		k.w.Close()
	}
	if k.waitCh != nil {
		close(k.waitCh)
		k.waitCh = nil
	}
	if k.addrsCh != nil {
		close(k.addrsCh)
	}
	return nil
}
