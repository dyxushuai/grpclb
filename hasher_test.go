package grpclb

import (
	"reflect"
	"testing"

	"golang.org/x/net/context"
)

func Test_strOrNum_Hash32(t *testing.T) {
	type fields struct {
		hash32 uint32
	}
	tests := []struct {
		name   string
		fields fields
		want   uint32
	}{
		{"hash32", fields{123}, 123},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &strOrNum{
				hash32: tt.fields.hash32,
			}
			if got := s.Hash32(); got != tt.want {
				t.Errorf("strOrNum.Hash32() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_NewHasher(t *testing.T) {
	type args struct {
		value interface{}
	}

	tests := []struct {
		name  string
		args  args
		want  Hasher
		want1 bool
	}{
		{"string key", args{"key"}, &strOrNum{1746258028}, true},
		{"uint32 key", args{uint32(123)}, &strOrNum{1916298011}, true},
		{"unsupport type", args{123}, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := NewHasher(tt.args.value)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewHasher() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("NewHasher() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_strOrNumFromContext(t *testing.T) {
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name  string
		args  args
		want  Hasher
		want1 bool
	}{
		// TODO: Add test cases.
		{"string key in context", args{context.WithValue(context.Background(), hasherKey{}, "key")}, &strOrNum{1746258028}, true},
		{"uint32 key in context", args{context.WithValue(context.Background(), hasherKey{}, uint32(123))}, &strOrNum{1916298011}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := strOrNumFromContext(tt.args.ctx)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("strOrNumFromContext() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("strOrNumFromContext() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestStrOrNumToContext(t *testing.T) {
	type args struct {
		ctx context.Context
		val interface{}
	}
	c1 := context.Background()
	w1 := context.WithValue(c1, hasherKey{}, "key")

	c2 := context.Background()
	w2 := context.WithValue(c1, hasherKey{}, uint32(123))

	tests := []struct {
		name string
		args args
		want context.Context
	}{
		{"registry string key", args{c1, "key"}, w1},
		{"registry uint32 key", args{c2, uint32(123)}, w2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StrOrNumToContext(tt.args.ctx, tt.args.val); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("StrOrNumToContext() = %v, want %v", got, tt.want)
			}
		})
	}
}
