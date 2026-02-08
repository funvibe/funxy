package evaluator

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/jhump/protoreflect/dynamic"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/funvibe/funxy/internal/typesystem"
)

// Global registry for loaded proto descriptors
var (
	protoRegistry      = make(map[string]*desc.FileDescriptor)
	protoRegistryMutex sync.RWMutex
)

// GrpcConnObject wraps a grpc.ClientConn
type GrpcConnObject struct {
	Conn *grpc.ClientConn
}

func (o *GrpcConnObject) Type() ObjectType { return "GrpcConn" }
func (o *GrpcConnObject) Inspect() string {
	if o.Conn == nil {
		return "GrpcConn(closed)"
	}
	return fmt.Sprintf("GrpcConn(%s)", o.Conn.Target())
}
func (o *GrpcConnObject) RuntimeType() typesystem.Type {
	return typesystem.TCon{Name: "GrpcConn"}
}
func (o *GrpcConnObject) Hash() uint32 {
	return 0 // Not hashable
}

// GrpcServerObject wraps a grpc.Server
type GrpcServerObject struct {
	Server   *grpc.Server
	Services map[string]Object // service name -> implementation object
	Eval     *Evaluator        // Evaluator snapshot for callbacks
}

func (o *GrpcServerObject) Type() ObjectType { return "GrpcServer" }
func (o *GrpcServerObject) Inspect() string {
	return fmt.Sprintf("GrpcServer(%d services)", len(o.Services))
}
func (o *GrpcServerObject) RuntimeType() typesystem.Type {
	return typesystem.TCon{Name: "GrpcServer"}
}
func (o *GrpcServerObject) Hash() uint32 {
	return 0
}

// GrpcBuiltins returns built-in functions for lib/grpc
func GrpcBuiltins() map[string]*Builtin {
	return map[string]*Builtin{
		"grpcConnect":    {Fn: builtinGrpcConnect, Name: "grpcConnect"},
		"grpcClose":      {Fn: builtinGrpcClose, Name: "grpcClose"},
		"grpcLoadProto":  {Fn: builtinGrpcLoadProto, Name: "grpcLoadProto"},
		"grpcInvoke":     {Fn: builtinGrpcInvoke, Name: "grpcInvoke"},
		"grpcServer":     {Fn: builtinGrpcServer, Name: "grpcServer"},
		"grpcRegister":   {Fn: builtinGrpcRegister, Name: "grpcRegister"},
		"grpcServe":      {Fn: builtinGrpcServe, Name: "grpcServe"},
		"grpcServeAsync": {Fn: builtinGrpcServeAsync, Name: "grpcServeAsync"},
		"grpcStop":       {Fn: builtinGrpcStop, Name: "grpcStop"},
	}
}

// ProtoBuiltins returns built-in functions for lib/proto
func ProtoBuiltins() map[string]*Builtin {
	return map[string]*Builtin{
		"protoEncode": {Fn: builtinProtoEncode, Name: "protoEncode"},
		"protoDecode": {Fn: builtinProtoDecode, Name: "protoDecode"},
	}
}

// grpcConnect(target: String) -> Result<String, GrpcConn>
func builtinGrpcConnect(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("grpcConnect expects 1 argument")
	}

	target := listToString(args[0])
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return makeFailStr(err.Error())
	}

	return makeOk(&GrpcConnObject{Conn: conn})
}

// grpcClose(conn: GrpcConn) -> Result<String, Nil>
func builtinGrpcClose(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("grpcClose expects 1 argument")
	}

	connObj, ok := args[0].(*GrpcConnObject)
	if !ok {
		return newError("grpcClose expects a GrpcConn")
	}

	if connObj.Conn != nil {
		err := connObj.Conn.Close()
		connObj.Conn = nil
		if err != nil {
			return makeFailStr(err.Error())
		}
	}

	return makeOk(&Nil{})
}

// grpcLoadProto(path: String) -> Result<String, Nil>
func builtinGrpcLoadProto(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("grpcLoadProto expects 1 argument")
	}

	path := listToString(args[0])

	// Use protoparse to parse the file
	parser := protoparse.Parser{}
	// Determine import paths (current directory by default)
	parser.ImportPaths = []string{"."}

	fds, err := parser.ParseFiles(path)
	if err != nil {
		return makeFailStr("failed to parse proto: " + err.Error())
	}

	protoRegistryMutex.Lock()
	defer protoRegistryMutex.Unlock()

	for _, fd := range fds {
		protoRegistry[fd.GetName()] = fd
		// Also register dependencies if needed, but ParseFiles returns them
	}

	return makeOk(&Nil{})
}

// grpcInvoke(conn: GrpcConn, method: String, request: A) -> Result<String, B>
func builtinGrpcInvoke(e *Evaluator, args ...Object) Object {
	if len(args) != 3 {
		return newError("grpcInvoke expects 3 arguments")
	}

	connObj, ok := args[0].(*GrpcConnObject)
	if !ok || connObj.Conn == nil {
		return newError("grpcInvoke expects a valid GrpcConn")
	}

	methodPath := listToString(args[1]) // e.g. "package.Service/Method"
	requestData := args[2]

	// Find method descriptor
	md, err := findMethodDescriptor(methodPath)
	if err != nil {
		return makeFailStr(err.Error())
	}

	// Create request message
	reqMsg := dynamic.NewMessage(md.GetInputType())
	if err := objectToDynamicMessage(requestData, reqMsg); err != nil {
		return makeFailStr("failed to build request: " + err.Error())
	}

	// Create response message
	respMsg := dynamic.NewMessage(md.GetOutputType())

	// Invoke
	ctx := context.Background()
	// Fix method path for grpc.Invoke: it expects "/package.Service/Method"
	if methodPath[0] != '/' {
		methodPath = "/" + methodPath
	}

	// Convert dynamic message to proto.Message interface for grpc
	// We use invoke with dynamic messages. grpc.Invoke expects proto.Message.
	// dynamic.Message implements it.

	err = connObj.Conn.Invoke(ctx, methodPath, reqMsg, respMsg)
	if err != nil {
		return makeFailStr("RPC failed: " + err.Error())
	}

	// Convert response back to Object
	respObj := dynamicMessageToObject(respMsg)
	return makeOk(respObj)
}

// protoEncode(messageName: String, data: A) -> Result<String, Bytes>
func builtinProtoEncode(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("protoEncode expects 2 arguments")
	}

	msgName := listToString(args[0])
	data := args[1]

	md, err := findMessageDescriptor(msgName)
	if err != nil {
		return makeFailStr(err.Error())
	}

	msg := dynamic.NewMessage(md)
	if err := objectToDynamicMessage(data, msg); err != nil {
		return makeFailStr("encoding error: " + err.Error())
	}

	bytesData, err := msg.Marshal()
	if err != nil {
		return makeFailStr("marshal error: " + err.Error())
	}

	return makeOk(&Bytes{data: bytesData})
}

// protoDecode(messageName: String, data: Bytes) -> Result<String, B>
func builtinProtoDecode(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("protoDecode expects 2 arguments")
	}

	msgName := listToString(args[0])
	bytesObj, ok := args[1].(*Bytes)
	if !ok {
		return newError("protoDecode expects Bytes")
	}

	md, err := findMessageDescriptor(msgName)
	if err != nil {
		return makeFailStr(err.Error())
	}

	msg := dynamic.NewMessage(md)
	if err := msg.Unmarshal(bytesObj.data); err != nil {
		return makeFailStr("unmarshal error: " + err.Error())
	}

	return makeOk(dynamicMessageToObject(msg))
}

// grpcServer() -> GrpcServer
func builtinGrpcServer(e *Evaluator, args ...Object) Object {
	server := grpc.NewServer()
	// Clone evaluator to safely run handlers concurrently (similar to http)
	var serverEval *Evaluator
	if e.Fork != nil {
		serverEval = e.Fork()
	} else {
		serverEval = e.Clone()
	}

	return &GrpcServerObject{
		Server:   server,
		Services: make(map[string]Object),
		Eval:     serverEval,
	}
}

// grpcRegister(server: GrpcServer, name: String, impl: Object) -> Result<String, Nil>
func builtinGrpcRegister(e *Evaluator, args ...Object) Object {
	if len(args) != 3 {
		return newError("grpcRegister expects 3 arguments")
	}

	serverObj, ok := args[0].(*GrpcServerObject)
	if !ok {
		return newError("grpcRegister expects a GrpcServer")
	}

	serviceName := listToString(args[1])
	impl := args[2]

	// Find service descriptor
	sd := findServiceDescriptor(serviceName)
	if sd == nil {
		return makeFailStr(fmt.Sprintf("service %s not found in loaded protos", serviceName))
	}

	// Construct ServiceDesc
	desc := &grpc.ServiceDesc{
		ServiceName: serviceName,
		HandlerType: (*interface{})(nil),
		Methods:     []grpc.MethodDesc{},
		Streams:     []grpc.StreamDesc{},
		Metadata:    sd.GetFile().GetName(),
	}

	// Wrapper for implementation
	handlerWrapper := &FunxyGrpcHandler{
		Impl: impl,
		Eval: serverObj.Eval,
		SD:   sd,
	}

	for _, method := range sd.GetMethods() {
		if method.IsClientStreaming() || method.IsServerStreaming() {
			continue // TODO: Streaming support
		}

		methodName := method.GetName()
		md := method

		desc.Methods = append(desc.Methods, grpc.MethodDesc{
			MethodName: methodName,
			Handler: func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
				h := srv.(*FunxyGrpcHandler)
				return h.HandleUnary(ctx, md, dec)
			},
		})
	}

	serverObj.Server.RegisterService(desc, handlerWrapper)
	serverObj.Services[serviceName] = impl

	return makeOk(&Nil{})
}

// grpcServe(server: GrpcServer, addr: String) -> Result<String, Nil>
func builtinGrpcServe(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("grpcServe expects 2 arguments")
	}

	serverObj, ok := args[0].(*GrpcServerObject)
	if !ok {
		return newError("grpcServe expects a GrpcServer")
	}

	addr := listToString(args[1])

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return makeFailStr(err.Error())
	}

	err = serverObj.Server.Serve(lis)
	if err != nil {
		return makeFailStr(err.Error())
	}

	return makeOk(&Nil{})
}

// grpcServeAsync(server: GrpcServer, addr: String) -> Result<String, Nil>
func builtinGrpcServeAsync(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("grpcServeAsync expects 2 arguments")
	}

	serverObj, ok := args[0].(*GrpcServerObject)
	if !ok {
		return newError("grpcServeAsync expects a GrpcServer")
	}

	addr := listToString(args[1])

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return makeFailStr(err.Error())
	}

	go func() {
		_ = serverObj.Server.Serve(lis)
	}()

	return makeOk(&Nil{})
}

// grpcStop(server: GrpcServer) -> Result<String, Nil>
func builtinGrpcStop(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("grpcStop expects 1 argument")
	}

	serverObj, ok := args[0].(*GrpcServerObject)
	if !ok {
		return newError("grpcStop expects a GrpcServer")
	}

	serverObj.Server.GracefulStop()

	return makeOk(&Nil{})
}

type FunxyGrpcHandler struct {
	Impl Object
	Eval *Evaluator
	SD   *desc.ServiceDescriptor
}

func (h *FunxyGrpcHandler) HandleUnary(ctx context.Context, md *desc.MethodDescriptor, dec func(interface{}) error) (interface{}, error) {
	// 1. Create dynamic message for input
	inMsg := dynamic.NewMessage(md.GetInputType())

	// 2. Decode
	if err := dec(inMsg); err != nil {
		return nil, err
	}

	// 3. Convert to Funxy Object
	inObj := dynamicMessageToObject(inMsg)

	// 4. Find function in Impl
	methodName := md.GetName()
	var fn Object

	if rec, ok := h.Impl.(*RecordInstance); ok {
		fn = rec.Get(methodName)
	} else if m, ok := h.Impl.(*Map); ok {
		fn = m.get(stringToList(methodName))
	}

	if fn == nil {
		return nil, fmt.Errorf("method %s not found in implementation", methodName)
	}

	// 5. Call function
	var reqEval *Evaluator
	if h.Eval.Fork != nil {
		reqEval = h.Eval.Fork()
	} else {
		reqEval = h.Eval.Clone()
	}

	result := reqEval.ApplyFunction(fn, []Object{inObj})

	if isError(result) {
		return nil, fmt.Errorf("%s", result.(*Error).Message)
	}

	// 6. Convert result to dynamic message
	outMsg := dynamic.NewMessage(md.GetOutputType())
	if err := objectToDynamicMessage(result, outMsg); err != nil {
		return nil, err
	}

	return outMsg, nil
}

func findServiceDescriptor(name string) *desc.ServiceDescriptor {
	protoRegistryMutex.RLock()
	defer protoRegistryMutex.RUnlock()

	for _, fd := range protoRegistry {
		if sd := fd.FindService(name); sd != nil {
			return sd
		}
		// Fallback: check full name manually
		for _, sd := range fd.GetServices() {
			if sd.GetFullyQualifiedName() == name || sd.GetName() == name {
				return sd
			}
		}
	}
	return nil
}

// Helpers

func findMethodDescriptor(path string) (*desc.MethodDescriptor, error) {
	// Path format: "package.Service/Method"
	// We need to search all loaded files

	// Helper: split service and method
	// Assuming "/" separator
	parts := splitPath(path)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid method path %q, expected 'package.Service/Method'", path)
	}
	serviceName := parts[0]
	methodName := parts[1]

	protoRegistryMutex.RLock()
	defer protoRegistryMutex.RUnlock()

	for _, fd := range protoRegistry {
		svc := fd.FindService(serviceName)
		if svc != nil {
			method := svc.FindMethodByName(methodName)
			if method != nil {
				return method, nil
			}
		}
	}
	return nil, fmt.Errorf("method %q not found (did you load the proto?)", path)
}

func findMessageDescriptor(name string) (*desc.MessageDescriptor, error) {
	protoRegistryMutex.RLock()
	defer protoRegistryMutex.RUnlock()

	for _, fd := range protoRegistry {
		msg := fd.FindMessage(name)
		if msg != nil {
			return msg, nil
		}
	}
	return nil, fmt.Errorf("message type %q not found", name)
}

func splitPath(path string) []string {
	// Split by last '/'
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return []string{path[:i], path[i+1:]}
		}
	}
	return nil
}

// objectToDynamicMessage populates a dynamic message from a Funxy Object (Record or Map)
func objectToDynamicMessage(obj Object, msg *dynamic.Message) error {
	// Support Record or Map<String, Any>
	var fields map[string]Object

	switch o := obj.(type) {
	case *RecordInstance:
		fields = make(map[string]Object)
		for _, f := range o.Fields {
			fields[f.Key] = f.Value
		}
	case *Map:
		fields = make(map[string]Object)
		// Accessing map items via the public items() method
		// to iterate over key-value pairs as List of Tuples
		items := o.items() // Returns List of Tuples
		if items != nil {
			for _, item := range items.ToSlice() {
				tuple, ok := item.(*Tuple)
				if !ok || len(tuple.Elements) != 2 {
					continue
				}
				key, ok := tuple.Elements[0].(*List) // String key
				if !ok {
					continue // Skip non-string keys
				}
				fields[ListToString(key)] = tuple.Elements[1]
			}
		}
	default:
		return fmt.Errorf("expected Record or Map, got %s", obj.Type())
	}

	for name, val := range fields {
		fd := msg.GetMessageDescriptor().FindFieldByName(name)
		if fd == nil {
			// Ignore unknown fields
			continue
		}

		v, err := convertToProtoValue(val, fd)
		if err != nil {
			return fmt.Errorf("field %s: %v", name, err)
		}
		if v != nil {
			msg.SetField(fd, v)
		}
	}
	return nil
}

func convertToProtoValue(val Object, fd *desc.FieldDescriptor) (interface{}, error) {
	if isZeroValue(val) && !fd.IsRepeated() {
		// For proto3, zero values are default
		return nil, nil
	}

	if fd.IsRepeated() {
		list, ok := val.(*List)
		if !ok {
			return nil, fmt.Errorf("expected List for repeated field")
		}
		var slice []interface{}
		for _, item := range list.ToSlice() {
			v, err := convertToProtoSingleValue(item, fd)
			if err != nil {
				return nil, err
			}
			slice = append(slice, v)
		}
		return slice, nil
	}

	return convertToProtoSingleValue(val, fd)
}

func convertToProtoSingleValue(val Object, fd *desc.FieldDescriptor) (interface{}, error) {
	switch fd.GetType() {
	case descriptorpb.FieldDescriptorProto_TYPE_INT32, descriptorpb.FieldDescriptorProto_TYPE_SINT32, descriptorpb.FieldDescriptorProto_TYPE_SFIXED32:
		if i, ok := val.(*Integer); ok {
			return int32(i.Value), nil
		}
	case descriptorpb.FieldDescriptorProto_TYPE_INT64, descriptorpb.FieldDescriptorProto_TYPE_SINT64, descriptorpb.FieldDescriptorProto_TYPE_SFIXED64:
		if i, ok := val.(*Integer); ok {
			return i.Value, nil
		}
	case descriptorpb.FieldDescriptorProto_TYPE_UINT32, descriptorpb.FieldDescriptorProto_TYPE_FIXED32:
		if i, ok := val.(*Integer); ok {
			return uint32(i.Value), nil
		}
	case descriptorpb.FieldDescriptorProto_TYPE_UINT64, descriptorpb.FieldDescriptorProto_TYPE_FIXED64:
		if i, ok := val.(*Integer); ok {
			return uint64(i.Value), nil
		}
	case descriptorpb.FieldDescriptorProto_TYPE_FLOAT:
		if f, ok := val.(*Float); ok {
			return float32(f.Value), nil
		}
	case descriptorpb.FieldDescriptorProto_TYPE_DOUBLE:
		if f, ok := val.(*Float); ok {
			return f.Value, nil
		}
	case descriptorpb.FieldDescriptorProto_TYPE_BOOL:
		if b, ok := val.(*Boolean); ok {
			return b.Value, nil
		}
	case descriptorpb.FieldDescriptorProto_TYPE_STRING:
		return listToString(val), nil
	case descriptorpb.FieldDescriptorProto_TYPE_BYTES:
		if b, ok := val.(*Bytes); ok {
			return b.data, nil
		}
	case descriptorpb.FieldDescriptorProto_TYPE_MESSAGE:
		// Nested message
		msg := dynamic.NewMessage(fd.GetMessageType())
		if err := objectToDynamicMessage(val, msg); err != nil {
			return nil, err
		}
		return msg, nil
	case descriptorpb.FieldDescriptorProto_TYPE_ENUM:
		// Enum as integer or string
		// If Int, use value. If String, lookup value.
		if i, ok := val.(*Integer); ok {
			return int32(i.Value), nil
		}
		if l, ok := val.(*List); ok {
			s := ListToString(l)
			ev := fd.GetEnumType().FindValueByName(s)
			if ev != nil {
				return ev.GetNumber(), nil
			}
		}
	}
	return nil, fmt.Errorf("unsupported conversion for type %v to %v", val.Type(), fd.GetType())
}

func dynamicMessageToObject(msg *dynamic.Message) Object {
	fields := make(map[string]Object)

	// iterate over all fields
	for _, fd := range msg.GetMessageDescriptor().GetFields() {
		val := msg.GetField(fd)
		// Convert val to Object
		obj := convertFromProtoValue(val, fd)
		fields[fd.GetName()] = obj
	}

	return NewRecord(fields)
}

func convertFromProtoValue(val interface{}, fd *desc.FieldDescriptor) Object {
	if val == nil {
		return getDefaultValueForProto(fd)
	}

	if fd.IsRepeated() {
		// slice
		slice, ok := val.([]interface{})
		if !ok {
			return newList([]Object{})
		}
		var list []Object
		for _, v := range slice {
			list = append(list, convertFromProtoSingleValue(v, fd))
		}
		return newList(list)
	}

	return convertFromProtoSingleValue(val, fd)
}

func convertFromProtoSingleValue(val interface{}, fd *desc.FieldDescriptor) Object {
	switch v := val.(type) {
	case int32:
		return &Integer{Value: int64(v)}
	case int64:
		return &Integer{Value: v}
	case uint32:
		return &Integer{Value: int64(v)}
	case uint64:
		return &Integer{Value: int64(v)} // Potential overflow if > MaxInt64
	case float32:
		return &Float{Value: float64(v)}
	case float64:
		return &Float{Value: v}
	case bool:
		if v {
			return TRUE
		} else {
			return FALSE
		}
	case string:
		return stringToList(v)
	case []byte:
		return &Bytes{data: v}
	case *dynamic.Message:
		return dynamicMessageToObject(v)
	case int: // Enums often come as int
		return &Integer{Value: int64(v)}
	}
	return &Nil{}
}

func getDefaultValueForProto(fd *desc.FieldDescriptor) Object {
	// Return appropriate default value based on type
	if fd.IsRepeated() {
		return newList([]Object{})
	}
	switch fd.GetType() {
	case descriptorpb.FieldDescriptorProto_TYPE_STRING:
		return stringToList("")
	case descriptorpb.FieldDescriptorProto_TYPE_MESSAGE:
		return &Nil{}
	default:
		// Use standard default values (0, false, etc)
		return getDefaultValue(getProtoTypeAsFunxy(fd))
	}
}

func getProtoTypeAsFunxy(fd *desc.FieldDescriptor) typesystem.Type {
	// Simple mapping for default values
	switch fd.GetType() {
	case descriptorpb.FieldDescriptorProto_TYPE_INT32, descriptorpb.FieldDescriptorProto_TYPE_INT64, descriptorpb.FieldDescriptorProto_TYPE_UINT32, descriptorpb.FieldDescriptorProto_TYPE_UINT64:
		return typesystem.Int
	case descriptorpb.FieldDescriptorProto_TYPE_FLOAT, descriptorpb.FieldDescriptorProto_TYPE_DOUBLE:
		return typesystem.Float
	case descriptorpb.FieldDescriptorProto_TYPE_BOOL:
		return typesystem.Bool
	case descriptorpb.FieldDescriptorProto_TYPE_STRING:
		return typesystem.TApp{Constructor: typesystem.TCon{Name: "List"}, Args: []typesystem.Type{typesystem.Char}}
	case descriptorpb.FieldDescriptorProto_TYPE_BYTES:
		return typesystem.TCon{Name: "Bytes"}
	default:
		return typesystem.Nil
	}
}
