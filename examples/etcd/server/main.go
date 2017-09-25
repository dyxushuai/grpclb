package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/xusss/grpclb"
	"github.com/xusss/grpclb/examples/helloworld"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

var (
	addr    = flag.String("addr", "", "server address")
	weight  = flag.Int("weight", 100, "the weight of this server")
	etcd    = flag.String("etcd", "127.0.0.1:2379", "the ectd listener")
	service = flag.String("service", "grpclb_test", "the service name")
)

func main() {
	flag.Parse()
	etcdClient, err := clientv3.New(clientv3.Config{Endpoints: strings.Split(*etcd, ",")})
	if err != nil {
		log.Panic(err)
	}

	lis, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Panic(err)
	}
	server := grpclb.NewServer(*service, *addr, *weight)

	s := grpc.NewServer()
	verifier, err := grpclb.NewKetamaVerifierWithEtcd(etcdClient, server)
	if err != nil {
		log.Panic(err)
	}
	helloworld.RegisterGreeterServer(s, &hw{
		verifier: verifier,
		names:    map[string]struct{}{},
	})

	go func() {
		log.Printf("Server start at %s", *addr)
		if err := s.Serve(lis); err != nil {
			log.Println(err)
		}
	}()

	r, err := grpclb.NewEtcdRegistry(etcdClient)
	if err != nil {
		log.Panic(err)
	}
	r.Register(server)
	defer r.Deregister(server)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	<-signals
}

type hw struct {
	sync.Mutex
	verifier grpclb.Verifier
	names    map[string]struct{}
}

func (h *hw) SayHello(ctx context.Context, req *helloworld.HelloRequest) (*helloworld.HelloReply, error) {
	if hasher, ok := grpclb.NewHasher(req.Name); ok {
		fmt.Println("verify", h.verifier.Verify(hasher.Hash32()))
		h.Lock()
		defer h.Unlock()
		if _, ok := h.names[req.Name]; !ok {
			go func() {
				for {
					ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
					err := h.verifier.Wait(ctx, hasher.Hash32())
					fmt.Println("no wait", err)
					time.Sleep(time.Second)
				}
			}()
			h.names[req.Name] = struct{}{}
		}
	}
	return &helloworld.HelloReply{Message: *addr}, nil
}
