package grpclb

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/naming"
)

const (
	weightKey     = "weight"
	defaultWeight = 100
)

// Server indicates a real server instance
type Server struct {
	grpc.Address
	Name   string
	Weight int
}

// NewServer create a server instance.
func NewServer(name, addr string, weight int) *Server {
	s := &Server{
		Name:   name,
		Weight: weight,
	}
	s.Addr = addr
	s.Metadata = map[string]interface{}{weightKey: weight}
	return s
}

func fromUpdate(u *naming.Update) *Server {
	s := &Server{}
	s.Addr = u.Addr
	if md, ok := u.Metadata.(map[string]interface{}); ok {
		s.Weight = int(md[weightKey].(float64))
	}
	if s.Weight == 0 {
		s.Weight = defaultWeight
	}
	return s
}
