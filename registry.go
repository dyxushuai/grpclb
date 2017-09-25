package grpclb

import (
	etcd "github.com/coreos/etcd/clientv3"
	etcdnaming "github.com/coreos/etcd/clientv3/naming"
	"golang.org/x/net/context"
	"google.golang.org/grpc/naming"
)

const (
	// registryKeepAliveTTL the Time-To-Live of this service in the registry.
	// It also used when the server exited unexpectedly.
	registryKeepAliveTTL int64 = 3
)

// Registry for the server side to register and deregister from resolver.
type Registry interface {
	// Register register the server information into Registry.
	Register(*Server) error

	// Deregister deregister the server form Registry.
	Deregister(*Server) error

	// Close the registry
	Close() error
}

type etcdRegistry struct {
	r       *etcdnaming.GRPCResolver
	leaseID etcd.LeaseID
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewEtcdRegistry create a registry by given etcd client.
func NewEtcdRegistry(c *etcd.Client) (Registry, error) {
	er := &etcdRegistry{r: &etcdnaming.GRPCResolver{Client: c}}
	er.ctx, er.cancel = context.WithCancel(context.Background())
	l := etcd.NewLease(c)
	resp, err := l.Grant(er.ctx, registryKeepAliveTTL)
	if err != nil {
		return nil, err
	}
	er.leaseID = resp.ID
	l.KeepAlive(er.ctx, resp.ID)
	return er, nil
}

func (er *etcdRegistry) Register(s *Server) error {
	return er.r.Update(er.ctx, s.Name, naming.Update{Op: naming.Add, Addr: s.Addr, Metadata: s.Metadata}, etcd.WithLease(er.leaseID))
}

func (er *etcdRegistry) Deregister(s *Server) error {
	return er.r.Update(er.ctx, s.Name, naming.Update{Op: naming.Delete, Addr: s.Addr})
}

func (er *etcdRegistry) Close() error {
	er.cancel()
	return nil
}