package util

import (
	"encoding/binary"
	"encoding/hex"
	"io"
	"os"
	"testing"
	"unsafe"

	"github.com/pkt-cash/pktd/btcutil/er"
	"github.com/stretchr/testify/require"
)

func IsNil(i interface{}) bool {
	return (*[2]uintptr)(unsafe.Pointer(&i))[1] == 0
}

func DecodeHex(s string) ([]byte, er.R) {
	o, e := hex.DecodeString(s)
	return o, er.E(e)
}

// CheckError ensures the passed error has an error code that matches the passed  error code.
func CheckError(t *testing.T, testName string, gotErr er.R, wantErrCode *er.ErrorCode) bool {
	if !wantErrCode.Is(gotErr) {
		t.Errorf("%s: unexpected error code - got %s, want %s",
			testName, gotErr.Message(), wantErrCode.Default())
		return false
	}

	return true
}

func Exists(path string) bool {
	_, errr := os.Stat(path)
	return !os.IsNotExist(errr)
}

func CloneBytes(b []byte) []byte {
	out := make([]byte, len(b))
	if copy(out, b) != len(b) {
		panic("copy not length of bytes")
	}
	return out
}

func WriteBin(w io.Writer, order binary.ByteOrder, data interface{}) er.R {
	return er.E(binary.Write(w, order, data))
}

func ReadBin(r io.Reader, order binary.ByteOrder, data interface{}) er.R {
	return er.E(binary.Read(r, order, data))
}

func ReadFull(r io.Reader, buf []byte) (int, er.R) {
	i, e := io.ReadFull(r, buf)
	return i, er.E(e)
}

func Write(w io.Writer, b []byte) (int, er.R) {
	i, e := w.Write(b)
	return i, er.E(e)
}

func RequireErr(t require.TestingT, err er.R, msgAndArgs ...interface{}) {
	require.Error(t, er.Native(err), msgAndArgs...)
}

func RequireNoErr(t require.TestingT, err er.R, msgAndArgs ...interface{}) {
	require.NoError(t, er.Native(err), msgAndArgs...)
}

func Filter[T any](t []T, f func(T) bool) []T {
	out := make([]T, 0, len(t))
	for _, tt := range t {
		if f(tt) {
			out = append(out, tt)
		}
	}
	return out
}

func Map[T, U any](t []T, f func(T) U) []U {
	out := make([]U, len(t))
	for i, tt := range t {
		out[i] = f(tt)
	}
	return out
}

func Contains[T comparable](list []T, examples ...T) bool {
	for _, t := range list {
		for _, ex := range examples {
			if t == ex {
				return true
			}
		}
	}
	return false
}

// Generic terniary function
func If[T any](condition bool, resultIfTrue, resultIfFalse T) T {
	if condition {
		return resultIfTrue
	} else {
		return resultIfFalse
	}
}

func Iff[T any](condition bool, resultIfTrue func() T, resultIfFalse T) T {
	if condition {
		return resultIfTrue()
	} else {
		return resultIfFalse
	}
}
