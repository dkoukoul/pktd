//go:build cgo

package main

import "C"
import (
	"fmt"
	"os"
	"unsafe"
)

//export startService
func startService(numParams C.int, params **C.char) {
	fmt.Println("startService()")
	length := int(numParams)
	tmpslice := (*[(1 << 29) - 1]*C.char)(unsafe.Pointer(params))[:length:length]
	parameters := make([]string, length)
	for i, s := range tmpslice {
		parameters[i] = C.GoString(s)
	}

	for i := 0; i < len(parameters); i++ {
		parameter := parameters[i]
		fmt.Println(parameters[i])
		os.Args = append(os.Args, parameter)
	}
	main()
}
