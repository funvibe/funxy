package evaluator

import (
	"fmt"
)

const defaultRPCTimeoutMs = 5000

func rpcSerializationMode(e *Evaluator) string {
	if e == nil || e.SupervisorHandler == nil || e.SupervisorHandler.RPCSerializationMode == nil {
		return "fdf"
	}
	mode := e.SupervisorHandler.RPCSerializationMode()
	if mode == "ephemeral" || mode == "fdf" {
		return mode
	}
	// "auto" and unknown values default to stable wire format.
	return "fdf"
}

// RpcBuiltins returns built-in functions for lib/rpc virtual package
func RpcBuiltins() map[string]*Builtin {
	return map[string]*Builtin{
		"callWait":      {Fn: builtinRpcCallWait, Name: "callWait"},
		"callWaitGroup": {Fn: builtinRpcCallWaitGroup, Name: "callWaitGroup"},
	}
}

// callWait: <a, b>(String, String, a, timeoutMs: Int? = 5000) -> Result<String, b>
func builtinRpcCallWait(e *Evaluator, args ...Object) Object {
	if len(args) < 3 || len(args) > 4 {
		return newError("rpc.callWait expects 3-4 arguments, got %d", len(args))
	}

	targetList, ok := args[0].(*List)
	if !ok || !IsStringList(targetList) {
		return newError("rpc.call first argument (targetVM) must be a String")
	}
	targetVM := ListToString(targetList)

	methodList, ok := args[1].(*List)
	if !ok || !IsStringList(methodList) {
		return newError("rpc.call second argument (method) must be a String")
	}
	method := ListToString(methodList)

	methodArgs := args[2]

	// timeoutMs is optional, defaults to 5000
	timeoutMs := defaultRPCTimeoutMs
	if len(args) == 4 {
		timeoutObj := args[3]
		if timeoutObj != nil {
			timeoutInt, ok := timeoutObj.(*Integer)
			if !ok {
				return newError("rpc.callWait fourth argument (timeoutMs) must be an Int")
			}
			timeoutMs = int(timeoutInt.Value)
		}
	}

	// Check for mock first
	if tr := GetTestRunner(); tr != nil {
		if callback, ok := tr.FindRpcMock(targetVM, method); ok {
			// Apply mock callback
			res := e.ApplyFunction(callback, []Object{methodArgs})
			if isError(res) {
				return makeFailStr(res.(*Error).Message)
			}
			return makeOk(res)
		}
	}

	// Check capabilities before allowing RPCCallFast
	if e.SupervisorHandler == nil {
		return makeFailStr("RPC API not injected by host (hint: run via `funxy vmm <script>`)")
	}

	// Try Fast Path (Zero-Copy RPC) if the host supports it
	if e.SupervisorHandler.RPCCallFast != nil {
		resObj, err := e.SupervisorHandler.RPCCallFast(targetVM, method, methodArgs, timeoutMs)
		if err != nil {
			return makeFailStr(err.Error())
		}

		return makeOk(resObj)
	}

	// Fallback to Slow Path (Serialization)
	if e.SupervisorHandler.RPCCall == nil {
		return makeFailStr("RPC API not fully injected by host")
	}

	// Serialize arguments
	argData, err := SerializeValue(methodArgs, rpcSerializationMode(e))
	if err != nil {
		return makeFailStr(fmt.Sprintf("failed to serialize args: %v", err))
	}

	// Execute RPC Call
	resData, err := e.SupervisorHandler.RPCCall(targetVM, method, argData, timeoutMs)
	if err != nil {
		return makeFailStr(err.Error())
	}

	// Deserialize result
	if len(resData) == 0 {
		return makeOk(&Nil{})
	}

	val, err := DeserializeValue(resData)
	if err != nil {
		return makeFailStr(fmt.Sprintf("failed to deserialize result: %v", err))
	}

	return makeOk(val)
}

// callWaitGroup: <a, b>(String, String, a, timeoutMs: Int? = 5000) -> Result<String, b>
func builtinRpcCallWaitGroup(e *Evaluator, args ...Object) Object {
	if len(args) < 3 || len(args) > 4 {
		return newError("rpc.callWaitGroup expects 3-4 arguments, got %d", len(args))
	}

	groupList, ok := args[0].(*List)
	if !ok || !IsStringList(groupList) {
		return newError("rpc.callWaitGroup first argument (group) must be a String")
	}
	group := ListToString(groupList)

	methodList, ok := args[1].(*List)
	if !ok || !IsStringList(methodList) {
		return newError("rpc.callWaitGroup second argument (method) must be a String")
	}
	method := ListToString(methodList)

	methodArgs := args[2]

	// timeoutMs is optional, defaults to 5000
	timeoutMs := defaultRPCTimeoutMs
	if len(args) == 4 {
		timeoutObj := args[3]
		if timeoutObj != nil {
			timeoutInt, ok := timeoutObj.(*Integer)
			if !ok {
				return newError("rpc.callWaitGroup fourth argument (timeoutMs) must be an Int")
			}
			timeoutMs = int(timeoutInt.Value)
		}
	}

	// Check for mock first
	if tr := GetTestRunner(); tr != nil {
		if callback, ok := tr.FindRpcMock(group, method); ok {
			// Apply mock callback
			res := e.ApplyFunction(callback, []Object{methodArgs})
			if isError(res) {
				return makeFailStr(res.(*Error).Message)
			}
			return makeOk(res)
		}
	}

	// Check capabilities before allowing RPCCallGroupFast
	if e.SupervisorHandler == nil {
		return makeFailStr("RPC API not injected by host (hint: run via `funxy vmm <script>`)")
	}

	// Try Fast Path (Zero-Copy RPC) if the host supports it
	if e.SupervisorHandler.RPCCallGroupFast != nil {
		resObj, err := e.SupervisorHandler.RPCCallGroupFast(group, method, methodArgs, timeoutMs)
		if err != nil {
			return makeFailStr(err.Error())
		}

		return makeOk(resObj)
	}

	// Fallback to Slow Path (Serialization)
	if e.SupervisorHandler.RPCCallGroup == nil {
		return makeFailStr("RPC API not fully injected by host")
	}

	// Serialize arguments
	argData, err := SerializeValue(methodArgs, rpcSerializationMode(e))
	if err != nil {
		return makeFailStr(fmt.Sprintf("failed to serialize args: %v", err))
	}

	// Execute RPC Call
	resData, err := e.SupervisorHandler.RPCCallGroup(group, method, argData, timeoutMs)
	if err != nil {
		return makeFailStr(err.Error())
	}

	// Deserialize result
	if len(resData) == 0 {
		return makeOk(&Nil{})
	}

	val, err := DeserializeValue(resData)
	if err != nil {
		return makeFailStr(fmt.Sprintf("failed to deserialize result: %v", err))
	}

	return makeOk(val)
}

// SetRpcBuiltinTypes sets type info for rpc builtins
