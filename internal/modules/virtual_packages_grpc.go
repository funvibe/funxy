package modules

import (
	"github.com/funvibe/funxy/internal/typesystem"
)

func initGrpcPackage() {
	stringType := typesystem.TApp{
		Constructor: ListCon,
		Args:        []typesystem.Type{typesystem.Char},
	}
	nilType := typesystem.Nil

	// Opaque types
	grpcConnType := typesystem.TCon{Name: "GrpcConn"}
	grpcServerType := typesystem.TCon{Name: "GrpcServer"}

	// Generic types
	typeA := typesystem.TVar{Name: "A"}
	typeB := typesystem.TVar{Name: "B"}

	// Result<String, GrpcConn>
	resultConn := typesystem.TApp{
		Constructor: ResultCon,
		Args:        []typesystem.Type{stringType, grpcConnType},
	}

	// Result<String, Nil>
	resultNil := typesystem.TApp{
		Constructor: ResultCon,
		Args:        []typesystem.Type{stringType, nilType},
	}

	// Result<String, B>
	resultB := typesystem.TApp{
		Constructor: ResultCon,
		Args:        []typesystem.Type{stringType, typeB},
	}

	pkg := &VirtualPackage{
		Name: "grpc",
		Types: map[string]typesystem.Type{
			"GrpcConn":   grpcConnType,
			"GrpcServer": grpcServerType,
		},
		Symbols: map[string]typesystem.Type{
			// Connect
			"grpcConnect": typesystem.TFunc{
				Params:     []typesystem.Type{stringType},
				ReturnType: resultConn,
			},
			// Close
			"grpcClose": typesystem.TFunc{
				Params:     []typesystem.Type{grpcConnType},
				ReturnType: resultNil,
			},
			// Load Proto
			"grpcLoadProto": typesystem.TFunc{
				Params:     []typesystem.Type{stringType},
				ReturnType: resultNil,
			},
			// Invoke: (GrpcConn, MethodName, Request) -> Result<String, Response>
			"grpcInvoke": typesystem.TFunc{
				Params:     []typesystem.Type{grpcConnType, stringType, typeA},
				ReturnType: resultB,
			},
			// Server
			"grpcServer": typesystem.TFunc{
				Params:     []typesystem.Type{},
				ReturnType: grpcServerType,
			},
			// Register: (GrpcServer, ServiceName, Implementation) -> Result<String, Nil>
			"grpcRegister": typesystem.TFunc{
				Params:     []typesystem.Type{grpcServerType, stringType, typeA},
				ReturnType: resultNil,
			},
			// Serve: (GrpcServer, Address) -> Result<String, Nil>
			"grpcServe": typesystem.TFunc{
				Params:     []typesystem.Type{grpcServerType, stringType},
				ReturnType: resultNil,
			},
			// ServeAsync: (GrpcServer, Address) -> Result<String, Nil>
			"grpcServeAsync": typesystem.TFunc{
				Params:     []typesystem.Type{grpcServerType, stringType},
				ReturnType: resultNil,
			},
			// Stop: (GrpcServer) -> Result<String, Nil>
			"grpcStop": typesystem.TFunc{
				Params:     []typesystem.Type{grpcServerType},
				ReturnType: resultNil,
			},
		},
	}

	RegisterVirtualPackage("lib/grpc", pkg)
}

func initProtoPackage() {
	stringType := typesystem.TApp{
		Constructor: ListCon,
		Args:        []typesystem.Type{typesystem.Char},
	}
	bytesType := typesystem.TCon{Name: "Bytes"}

	// Generic types
	typeA := typesystem.TVar{Name: "A"}
	typeB := typesystem.TVar{Name: "B"}

	// Result<String, Bytes>
	resultBytes := typesystem.TApp{
		Constructor: ResultCon,
		Args:        []typesystem.Type{stringType, bytesType},
	}

	// Result<String, B>
	resultB := typesystem.TApp{
		Constructor: ResultCon,
		Args:        []typesystem.Type{stringType, typeB},
	}

	pkg := &VirtualPackage{
		Name: "proto",
		Symbols: map[string]typesystem.Type{
			// Encode: (MessageName, Data) -> Result<String, Bytes>
			"protoEncode": typesystem.TFunc{
				Params:     []typesystem.Type{stringType, typeA},
				ReturnType: resultBytes,
			},
			// Decode: (MessageName, Bytes) -> Result<String, Data>
			"protoDecode": typesystem.TFunc{
				Params:     []typesystem.Type{stringType, bytesType},
				ReturnType: resultB,
			},
		},
	}

	RegisterVirtualPackage("lib/proto", pkg)
}
