package evaluator

import (
	"fmt"
)

const defaultReceiveEventWaitTimeoutMs = 5000

// SupervisorBuiltins returns built-in functions for lib/vmm virtual package
func SupervisorBuiltins() map[string]*Builtin {
	return map[string]*Builtin{
		"spawnVM":          {Fn: builtinSpawnVM, Name: "spawnVM"},
		"spawnVMGroup":     {Fn: builtinSpawnVMGroup, Name: "spawnVMGroup"},
		"killVM":           {Fn: builtinKillVM, Name: "killVM"},
		"stopVM":           {Fn: builtinStopVM, Name: "stopVM"},
		"traceOn":          {Fn: builtinTraceOn, Name: "traceOn"},
		"traceOff":         {Fn: builtinTraceOff, Name: "traceOff"},
		"listVMs":          {Fn: builtinListVMs, Name: "listVMs"},
		"vmStats":          {Fn: builtinVMStats, Name: "vmStats"},
		"rpcCircuitStats":  {Fn: builtinRPCCircuitStats, Name: "rpcCircuitStats"},
		"receiveEventWait": {Fn: builtinReceiveEventWait, Name: "receiveEventWait"},
		"serialize":        {Fn: builtinSerialize, Name: "serialize"},
		"deserialize":      {Fn: builtinDeserialize, Name: "deserialize"},
		"getState":         {Fn: builtinGetState, Name: "getState"},
		"setState":         {Fn: builtinSetState, Name: "setState"},
	}
}

// spawnVM: (String, Map<String, a>, [b]) -> Result<String, String>
func builtinSpawnVM(e *Evaluator, args ...Object) Object {
	if len(args) != 2 && len(args) != 3 {
		return newError("spawnVM expects 2 or 3 arguments, got %d", len(args))
	}

	pathList, ok := args[0].(*List)
	if !ok || !IsStringList(pathList) {
		return newError("spawnVM first argument must be a String")
	}
	path := ListToString(pathList)

	configRec, ok := args[1].(*RecordInstance)
	if !ok {
		return newError("spawnVM second argument must be a Record, got %s", args[1].Type())
	}

	var stateData []byte
	if len(args) == 3 {
		var err error
		stateArg := args[2]
		if stateArg != nil && stateArg.Type() != NIL_OBJ {
			if inner, ok := UnwrapOption(stateArg); ok {
				stateArg = inner
			}
			stateData, err = SerializeValue(stateArg, rpcSerializationMode(e))
			if err != nil {
				return makeFailStr(fmt.Sprintf("failed to serialize state: %v", err))
			}
		}
	}

	// Extract config
	config := make(map[string]interface{})
	for _, f := range configRec.Fields {
		keyStr := f.Key

		// Unpack values natively if possible
		var val interface{}
		switch v := f.Value.(type) {
		case *List:
			if IsStringList(v) {
				val = ListToString(v)
			} else {
				// Convert to slice of strings for capabilities
				var slice []interface{}
				for _, el := range v.ToSlice() {
					slice = append(slice, objectToString(el))
				}
				val = slice
			}
		case *Map:
			// recursively convert env if needed?
			innerMap := make(map[string]interface{})
			for _, innerItem := range v.Items() {
				innerMap[objectToString(innerItem.Key)] = objectToString(innerItem.Value)
			}
			val = innerMap
		case *Integer:
			val = v.Value
		case *Boolean:
			val = v.Value
		case *RecordInstance:
			// Specially handle nested records for configuration (like mailbox)
			innerMap := make(map[string]interface{})
			for _, field := range v.Fields {
				if num, ok := field.Value.(*Integer); ok {
					innerMap[field.Key] = num.Value
				} else {
					innerMap[field.Key] = objectToString(field.Value)
				}
			}
			val = innerMap
		default:
			val = objectToString(v)
		}

		config[keyStr] = val
	}

	if e.SupervisorHandler == nil || e.SupervisorHandler.SpawnVM == nil {
		return makeFailStr("supervisor API not injected by host (hint: run via `funxy vmm <script>`)")
	}

	id, err := e.SupervisorHandler.SpawnVM(path, config, stateData)
	if err != nil {
		return makeFailStr(err.Error())
	}

	return makeOk(stringToList(id))
}

// spawnVMGroup: (String, Record, Int, [b]) -> Result<String, List<String>>
func builtinSpawnVMGroup(e *Evaluator, args ...Object) Object {
	if len(args) != 3 && len(args) != 4 {
		return newError("spawnVMGroup expects 3 or 4 arguments, got %d", len(args))
	}

	sizeArg, ok := args[2].(*Integer)
	if !ok {
		return newError("spawnVMGroup third argument must be an Integer")
	}
	size := int(sizeArg.Value)
	if size <= 0 {
		return makeFailStr("spawnVMGroup size must be > 0")
	}

	var ids []Object
	configArg := args[1]
	// In group mode, "name" must not collide across N workers.
	// Strip it from config so IDs are generated uniquely by hypervisor.
	if rec, ok := args[1].(*RecordInstance); ok {
		filtered := make([]RecordField, 0, len(rec.Fields))
		for _, f := range rec.Fields {
			if f.Key == "name" {
				continue
			}
			filtered = append(filtered, f)
		}
		configArg = &RecordInstance{Fields: filtered}
	}

	for i := 0; i < size; i++ {
		spawnArgs := []Object{args[0], configArg}
		if len(args) == 4 {
			spawnArgs = append(spawnArgs, args[3])
		}
		res := builtinSpawnVM(e, spawnArgs...)

		if dataInst, ok := res.(*DataInstance); ok {
			if dataInst.Name == "Fail" {
				return res
			} else if dataInst.Name == "Ok" {
				ids = append(ids, dataInst.Fields[0])
			}
		} else {
			return makeFailStr("spawnVM returned invalid result type")
		}
	}

	return makeOk(newList(ids))
}

// killVM: (String) -> Nil
func builtinKillVM(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("killVM expects 1 argument, got %d", len(args))
	}

	idList, ok := args[0].(*List)
	if !ok || !IsStringList(idList) {
		return newError("killVM argument must be a String")
	}
	id := ListToString(idList)

	if e.SupervisorHandler == nil || e.SupervisorHandler.KillVM == nil {
		return newError("supervisor API not injected by host (hint: run via `funxy vmm <script>`)")
	}

	// killVM is a hard kill, so no state is saved. We give it a short timeout (e.g., 1000ms) to clean up.
	_, err := e.SupervisorHandler.KillVM(id, false, 1000)
	if err != nil {
		return newError("failed to kill VM: %v", err)
	}

	return &Nil{}
}

// stopVM: (String, Record) -> Result<String, a>
func builtinStopVM(e *Evaluator, args ...Object) Object {
	if len(args) != 1 && len(args) != 2 {
		return newError("stopVM expects 1 or 2 arguments, got %d", len(args))
	}

	idList, ok := args[0].(*List)
	if !ok || !IsStringList(idList) {
		return newError("stopVM first argument must be a String")
	}
	id := ListToString(idList)

	saveState := false
	timeoutMs := 5000 // Default timeout 5 seconds

	if len(args) == 2 {
		opts := args[1]
		if inner, ok := UnwrapOption(opts); ok {
			opts = inner
		}
		if rec, ok := opts.(*RecordInstance); ok {
			for _, f := range rec.Fields {
				if f.Key == "saveState" {
					if b, ok := f.Value.(*Boolean); ok {
						saveState = b.Value
					}
				}
				if f.Key == "timeoutMs" {
					if i, ok := f.Value.(*Integer); ok {
						timeoutMs = int(i.Value)
					}
				}
			}
		}
	}

	if e.SupervisorHandler == nil || e.SupervisorHandler.StopVM == nil {
		return makeFailStr("supervisor API not injected by host (hint: run via `funxy vmm <script>`)")
	}

	stateData, err := e.SupervisorHandler.StopVM(id, saveState, timeoutMs)
	if err != nil {
		return makeFailStr(fmt.Sprintf("failed to stop VM: %v", err))
	}

	if stateData != nil {
		val, err := DeserializeValue(stateData)
		if err != nil {
			return makeFailStr(fmt.Sprintf("failed to deserialize saved state: %v", err))
		}
		return makeOk(val)
	}

	return makeOk(&Nil{})
}

// traceOn: (String) -> Nil
func builtinTraceOn(e *Evaluator, args ...Object) Object {
	if len(args) != 0 && len(args) != 1 {
		return newError("traceOn expects 0 or 1 arguments, got %d", len(args))
	}
	if e.SupervisorHandler == nil {
		return newError("supervisor API not injected by host (hint: run via `funxy vmm <script>`)")
	}
	if len(args) == 0 {
		if e.SupervisorHandler.TraceOnAll == nil {
			return newError("supervisor trace API not injected by host")
		}
		e.SupervisorHandler.TraceOnAll()
		return &Nil{}
	}
	idList, ok := args[0].(*List)
	if !ok || !IsStringList(idList) {
		return newError("traceOn argument must be a String")
	}
	id := ListToString(idList)
	if e.SupervisorHandler.TraceOn == nil {
		return newError("supervisor trace API not injected by host")
	}
	if err := e.SupervisorHandler.TraceOn(id); err != nil {
		return newError("failed to enable trace: %v", err)
	}
	return &Nil{}
}

// traceOff: (String) -> Nil
func builtinTraceOff(e *Evaluator, args ...Object) Object {
	if len(args) != 0 && len(args) != 1 {
		return newError("traceOff expects 0 or 1 arguments, got %d", len(args))
	}
	if e.SupervisorHandler == nil {
		return newError("supervisor API not injected by host (hint: run via `funxy vmm <script>`)")
	}
	if len(args) == 0 {
		if e.SupervisorHandler.TraceOffAll == nil {
			return newError("supervisor trace API not injected by host")
		}
		e.SupervisorHandler.TraceOffAll()
		return &Nil{}
	}
	idList, ok := args[0].(*List)
	if !ok || !IsStringList(idList) {
		return newError("traceOff argument must be a String")
	}
	id := ListToString(idList)
	if e.SupervisorHandler.TraceOff == nil {
		return newError("supervisor trace API not injected by host")
	}
	if err := e.SupervisorHandler.TraceOff(id); err != nil {
		return newError("failed to disable trace: %v", err)
	}
	return &Nil{}
}

// listVMs: () -> List<String>
func builtinListVMs(e *Evaluator, args ...Object) Object {
	if len(args) != 0 {
		return newError("listVMs expects 0 arguments, got %d", len(args))
	}

	if e.SupervisorHandler == nil || e.SupervisorHandler.ListVMs == nil {
		return newError("supervisor API not injected by host (hint: run via `funxy vmm <script>`)")
	}

	vms := e.SupervisorHandler.ListVMs()

	// Sort vms deterministically for stable output
	for i := 0; i < len(vms); i++ {
		for j := i + 1; j < len(vms); j++ {
			if vms[i] > vms[j] {
				vms[i], vms[j] = vms[j], vms[i]
			}
		}
	}

	elements := make([]Object, len(vms))
	for i, id := range vms {
		elements[i] = stringToList(id)
	}

	return newList(elements)
}

// vmStats: (String) -> Map<String, Int>
func builtinVMStats(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("vmStats expects 1 argument, got %d", len(args))
	}

	idList, ok := args[0].(*List)
	if !ok || !IsStringList(idList) {
		return newError("vmStats argument must be a String")
	}
	id := ListToString(idList)

	if e.SupervisorHandler == nil || e.SupervisorHandler.GetStats == nil {
		return newError("supervisor API not injected by host (hint: run via `funxy vmm <script>`)")
	}

	stats, err := e.SupervisorHandler.GetStats(id)
	if err != nil {
		return newError("failed to get stats: %v", err)
	}

	m := NewMap()
	for k, v := range stats {
		m = m.Put(stringToList(k), &Integer{Value: int64(v)})
	}
	return m
}

// rpcCircuitStats: (String) -> Record
func builtinRPCCircuitStats(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("rpcCircuitStats expects 1 argument, got %d", len(args))
	}

	idList, ok := args[0].(*List)
	if !ok || !IsStringList(idList) {
		return newError("rpcCircuitStats argument must be a String")
	}
	id := ListToString(idList)

	if e.SupervisorHandler == nil || e.SupervisorHandler.GetRPCCircuitStats == nil {
		return newError("supervisor API not injected by host (hint: run via `funxy vmm <script>`)")
	}

	stats, err := e.SupervisorHandler.GetRPCCircuitStats(id)
	if err != nil {
		return newError("failed to get rpc circuit stats: %v", err)
	}

	fields := make([]RecordField, 0, len(stats))
	for k, v := range stats {
		var val Object
		switch tv := v.(type) {
		case string:
			val = stringToList(tv)
		case int:
			val = &Integer{Value: int64(tv)}
		case int32:
			val = &Integer{Value: int64(tv)}
		case int64:
			val = &Integer{Value: tv}
		case uint64:
			val = &Integer{Value: int64(tv)}
		case bool:
			val = &Boolean{Value: tv}
		default:
			val = stringToList(fmt.Sprintf("%v", tv))
		}
		fields = append(fields, RecordField{Key: k, Value: val})
	}

	for i := 0; i < len(fields); i++ {
		for j := i + 1; j < len(fields); j++ {
			if fields[i].Key > fields[j].Key {
				fields[i], fields[j] = fields[j], fields[i]
			}
		}
	}
	return &RecordInstance{Fields: fields}
}

// receiveEventWait: (timeoutMs?: Int) -> Record
func builtinReceiveEventWait(e *Evaluator, args ...Object) Object {
	if len(args) != 0 && len(args) != 1 {
		return newError("receiveEventWait expects 0 or 1 arguments, got %d", len(args))
	}

	timeoutMs := defaultReceiveEventWaitTimeoutMs
	if len(args) == 1 {
		timeoutObj, ok := args[0].(*Integer)
		if !ok {
			return newError("receiveEventWait timeoutMs must be an Int")
		}
		timeoutMs = int(timeoutObj.Value)
	}

	if tr := GetTestRunner(); tr != nil {
		if callback, ok := tr.FindSupervisorEventMock(); ok {
			res := e.ApplyFunction(callback, []Object{&Integer{Value: int64(timeoutMs)}})
			if errObj, isErr := res.(*Error); isErr {
				return errObj
			}
			rec, ok := res.(*RecordInstance)
			if !ok {
				return newError("mockSupervisorEvent callback must return a Record, got %s", res.Type())
			}
			return rec
		}
	}

	if e.SupervisorHandler == nil || e.SupervisorHandler.ReceiveEventTimeout == nil {
		return newError("supervisor API not injected by host (hint: run via `funxy vmm <script>`)")
	}

	var evt map[string]interface{}
	gotEvt, ok := e.SupervisorHandler.ReceiveEventTimeout(timeoutMs)
	if !ok {
		evt = map[string]interface{}{
			"type":      "timeout",
			"timed_out": true,
		}
	} else {
		evt = gotEvt
	}

	fields := make([]RecordField, 0, len(evt))
	for k, v := range evt {
		var val Object
		switch tv := v.(type) {
		case string:
			val = stringToList(tv)
		case int:
			val = &Integer{Value: int64(tv)}
		case int32:
			val = &Integer{Value: int64(tv)}
		case int64:
			val = &Integer{Value: tv}
		case uint64:
			val = &Integer{Value: int64(tv)}
		case bool:
			val = &Boolean{Value: tv}
		default:
			val = stringToList(fmt.Sprintf("%v", tv))
		}
		fields = append(fields, RecordField{Key: k, Value: val})
	}

	// Sort fields by key for determinism
	for i := 0; i < len(fields); i++ {
		for j := i + 1; j < len(fields); j++ {
			if fields[i].Key > fields[j].Key {
				fields[i], fields[j] = fields[j], fields[i]
			}
		}
	}

	return &RecordInstance{Fields: fields}
}

// builtinSerialize: (a, String) -> Bytes
func builtinSerialize(e *Evaluator, args ...Object) Object {
	if len(args) != 1 && len(args) != 2 {
		return newError("serialize expects 1 or 2 arguments")
	}
	mode := rpcSerializationMode(e)
	if len(args) == 2 {
		if list, ok := args[1].(*List); ok && IsStringList(list) {
			mode = ListToString(list)
		} else {
			return newError("serialize mode argument must be a String")
		}
	}
	data, err := SerializeValue(args[0], mode)
	if err != nil {
		return makeFailStr(err.Error())
	}
	return BytesFromSlice(data)
}

// builtinDeserialize: (Bytes) -> Result<String, a>
func builtinDeserialize(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("deserialize expects 1 argument")
	}
	bytesObj, ok := args[0].(*Bytes)
	if !ok {
		return newError("deserialize expects Bytes")
	}
	val, err := DeserializeValue(bytesObj.ToSlice())
	if err != nil {
		return makeFailStr(err.Error())
	}
	return makeOk(val)
}

// builtinGetState: () -> Option<a>
func builtinGetState(e *Evaluator, args ...Object) Object {
	if len(args) != 0 {
		return newError("getState expects 0 arguments")
	}
	if e.StateHandler != nil && e.StateHandler.GetState != nil {
		st := e.StateHandler.GetState()
		if st != nil && st.Type() != NIL_OBJ {
			return makeSome(st)
		}
	}
	return makeNone()
}

// builtinSetState: (a) -> Nil
func builtinSetState(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("setState expects 1 argument")
	}
	if e.StateHandler != nil && e.StateHandler.SetState != nil {
		e.StateHandler.SetState(args[0])
	}
	return &Nil{}
}

// SetSupervisorBuiltinTypes sets type info for supervisor builtins
