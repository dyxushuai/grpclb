package grpclb

import (
	"hash/fnv"
	"strconv"

	"golang.org/x/net/context"
)

// Hasher hash method implemention
type Hasher interface {
	// Hash32 uint32 result
	Hash32() uint32
}

var _ Hasher = new(strOrNum)

type strOrNum struct {
	hash32 uint32
}

// Hash32 Hasher implement with fnv algorithm.
func (s *strOrNum) Hash32() uint32 {
	return s.hash32
}

// NewHasher create a hasher by given value.
func NewHasher(value interface{}) (Hasher, bool) {
	var data []byte
	switch v := value.(type) {
	case string:
		data = []byte(v)
	case uint32:
		data = []byte(strconv.FormatUint(uint64(v), 10))
	default:
		return nil, false
	}
	h := fnv.New32a()
	h.Write(data)
	return &strOrNum{hash32: h.Sum32()}, true
}

// HasherFromContext parse Hasher from context.
type HasherFromContext func(context.Context) (Hasher, bool)

type hasherKey struct{}

// strOrNumFromContext get Hasher from Context by key.
func strOrNumFromContext(ctx context.Context) (Hasher, bool) {
	return NewHasher(ctx.Value(hasherKey{}))
}

// StrOrNumToContext set string or number into Context.
func StrOrNumToContext(ctx context.Context, val interface{}) context.Context {
	return context.WithValue(ctx, hasherKey{}, val)
}
