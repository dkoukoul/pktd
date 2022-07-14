Because the GRPC compiler is relatively small and there is no way to invoke
it programmatically, we decided to vendor the code here so that it will be
reliably available when building, making it nolonger a build-time dependency.

see: https://github.com/grpc/grpc-go/tree/v1.48.x/cmd/protoc-gen-go-grpc