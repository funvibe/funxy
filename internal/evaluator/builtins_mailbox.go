package evaluator

import (
	"fmt"
	"github.com/funvibe/funxy/internal/typesystem"

	"github.com/google/uuid"
)

// DefaultMailboxTimeout is the default timeout for mailbox operations in milliseconds.
const DefaultMailboxTimeout = 5000

// MailboxBuiltins returns built-in functions for lib/mailbox virtual package
func MailboxBuiltins() map[string]*Builtin {
	return map[string]*Builtin{
		"send":          {Fn: builtinMailboxSend, Name: "send"},
		"sendWait":      {Fn: builtinMailboxSendWait, Name: "sendWait"},
		"reply":         {Fn: builtinMailboxReply, Name: "reply"},
		"replyWait":     {Fn: builtinMailboxReplyWait, Name: "replyWait"},
		"requestWait":   {Fn: builtinMailboxRequestWait, Name: "requestWait"},
		"receive":       {Fn: builtinMailboxReceive, Name: "receive"},
		"receiveWait":   {Fn: builtinMailboxReceiveWait, Name: "receiveWait"},
		"receiveBy":     {Fn: builtinMailboxReceiveBy, Name: "receiveBy"},
		"receiveByWait": {Fn: builtinMailboxReceiveByWait, Name: "receiveByWait"},
		"peek":          {Fn: builtinMailboxPeek, Name: "peek"},
		"peekBy":        {Fn: builtinMailboxPeekBy, Name: "peekBy"},
	}
}

// RegisterMailboxBuiltins registers mailbox types and functions into an environment
func RegisterMailboxBuiltins(env *Environment) {
	// Types
	env.Set("Importance", &TypeObject{TypeVal: typesystem.TCon{Name: "Importance"}})

	// Constructors
	env.Set("Low", &DataInstance{Name: "Low", Fields: []Object{}, TypeName: "Importance"})
	env.Set("Info", &DataInstance{Name: "Info", Fields: []Object{}, TypeName: "Importance"})
	env.Set("Warn", &DataInstance{Name: "Warn", Fields: []Object{}, TypeName: "Importance"})
	env.Set("Crit", &DataInstance{Name: "Crit", Fields: []Object{}, TypeName: "Importance"})
	env.Set("System", &DataInstance{Name: "System", Fields: []Object{}, TypeName: "Importance"})

	// Functions
	builtins := MailboxBuiltins()
	for name, fn := range builtins {
		env.Set(name, fn)
	}
}

// Helper to extract string from List (String)
func getStringArg(obj Object, argName string) (string, error) {
	list, ok := obj.(*List)
	if !ok || !IsStringList(list) {
		return "", fmt.Errorf("%s must be a String", argName)
	}
	return ListToString(list), nil
}

func getIntArg(obj Object, argName string) (int, error) {
	num, ok := obj.(*Integer)
	if !ok {
		return 0, fmt.Errorf("%s must be an Int", argName)
	}
	return int(num.Value), nil
}

func getRecordField(rec *RecordInstance, key string) (Object, bool) {
	for _, f := range rec.Fields {
		if f.Key == key {
			return f.Value, true
		}
	}
	return nil, false
}

func getMessageId(msg Object) (string, bool) {
	rec, ok := msg.(*RecordInstance)
	if !ok {
		return "", false
	}
	idObj, ok := getRecordField(rec, "id")
	if !ok {
		return "", false
	}
	idStr, err := getStringArg(idObj, "id")
	if err != nil {
		return "", false
	}
	return idStr, true
}

func applyMailboxMock(e *Evaluator, targetId string, msgObj Object) (Object, bool) {
	if tr := GetTestRunner(); tr != nil {
		if callback, ok := tr.FindMailboxMock(targetId); ok {
			res := e.ApplyFunction(callback, []Object{msgObj})
			if isError(res) {
				return makeFailStr(res.(*Error).Message), true
			}
			// Important: We MUST return the mock result as the Ok value (needed for requestWait)
			// Even for send/reply, returning Ok(res) is fine as long as res is Nil
			if res == nil {
				return makeOk(&Nil{}), true
			}
			return makeOk(res), true
		}
	}
	return nil, false
}

// send(targetId: String, msg: a) -> Result<String, Nil>
func builtinMailboxSend(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("send expects 2 arguments")
	}

	targetId, err := getStringArg(args[0], "targetId")
	if err != nil {
		return newError("%s", err.Error())
	}

	if mockRes, handled := applyMailboxMock(e, targetId, args[1]); handled {
		return mockRes
	}

	if e.MailboxHandler == nil || e.MailboxHandler.Send == nil {
		return makeFailStr("mailbox API not injected by host (hint: run via `funxy vmm <script>`)")
	}

	if err := e.MailboxHandler.Send(targetId, args[1]); err != nil {
		return makeFailStr(err.Error())
	}

	return makeOk(&Nil{})
}

// sendWait(targetId: String, msg: a, timeoutMs: Int = DefaultMailboxTimeout) -> Result<String, Nil>
func builtinMailboxSendWait(e *Evaluator, args ...Object) Object {
	if len(args) != 2 && len(args) != 3 {
		return newError("sendWait expects 2 or 3 arguments")
	}

	targetId, err := getStringArg(args[0], "targetId")
	if err != nil {
		return newError("%s", err.Error())
	}

	timeoutMs := DefaultMailboxTimeout
	if len(args) == 3 {
		timeoutMs, err = getIntArg(args[2], "timeoutMs")
		if err != nil {
			return newError("%s", err.Error())
		}
	}

	if mockRes, handled := applyMailboxMock(e, targetId, args[1]); handled {
		return mockRes
	}

	if e.MailboxHandler == nil || e.MailboxHandler.SendWait == nil {
		return makeFailStr("mailbox API not injected by host (hint: run via `funxy vmm <script>`)")
	}

	if err := e.MailboxHandler.SendWait(targetId, args[1], timeoutMs, e.Context); err != nil {
		return makeFailStr(err.Error())
	}

	return makeOk(&Nil{})
}

// extractReplyType attempts to extract the "type" field from the payload.
// If it's not a Record or doesn't have a valid String "type" field, it defaults to "reply".
func extractReplyType(payload Object) string {
	replyType := "reply"
	if rec, ok := payload.(*RecordInstance); ok {
		if typeObj, ok := getRecordField(rec, "type"); ok {
			if tStr, err := getStringArg(typeObj, "type"); err == nil {
				replyType = tStr
			}
		}
	}
	return replyType
}

// reply(originalMsg: Record, payload: a) -> Result<String, Nil>
func builtinMailboxReply(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("reply expects 2 arguments")
	}

	origMsg, ok := args[0].(*RecordInstance)
	if !ok {
		return newError("originalMsg must be a Record")
	}

	fromObj, ok := getRecordField(origMsg, "from")
	if !ok {
		return makeFailStr("receiver not found (original message has no 'from' field)")
	}
	fromStr, err := getStringArg(fromObj, "from")
	if err != nil {
		return makeFailStr("receiver not found (invalid 'from' field)")
	}

	idStr, ok := getMessageId(origMsg)
	if !ok {
		return makeFailStr("original message has no 'id' field or is invalid")
	}

	replyMsg := &RecordInstance{
		Fields: []RecordField{
			{Key: "id", Value: stringToList(idStr)},
			{Key: "payload", Value: args[1]},
			{Key: "type", Value: stringToList(extractReplyType(args[1]))},
		},
	}

	if mockRes, handled := applyMailboxMock(e, fromStr, replyMsg); handled {
		return mockRes
	}

	if e.MailboxHandler == nil || e.MailboxHandler.Send == nil {
		return makeFailStr("mailbox API not injected by host (hint: run via `funxy vmm <script>`)")
	}

	if err := e.MailboxHandler.Send(fromStr, replyMsg); err != nil {
		return makeFailStr(err.Error())
	}

	return makeOk(&Nil{})
}

// replyWait(originalMsg: Record, payload: a, timeoutMs: Int = DefaultMailboxTimeout) -> Result<String, Nil>
func builtinMailboxReplyWait(e *Evaluator, args ...Object) Object {
	if len(args) != 2 && len(args) != 3 {
		return newError("replyWait expects 2 or 3 arguments")
	}

	origMsg, ok := args[0].(*RecordInstance)
	if !ok {
		return newError("originalMsg must be a Record")
	}

	timeoutMs := DefaultMailboxTimeout
	if len(args) == 3 {
		var err error
		timeoutMs, err = getIntArg(args[2], "timeoutMs")
		if err != nil {
			return newError("%s", err.Error())
		}
	}

	fromObj, ok := getRecordField(origMsg, "from")
	if !ok {
		return makeFailStr("receiver not found (original message has no 'from' field)")
	}
	fromStr, err := getStringArg(fromObj, "from")
	if err != nil {
		return makeFailStr("receiver not found (invalid 'from' field)")
	}

	idStr, ok := getMessageId(origMsg)
	if !ok {
		return makeFailStr("original message has no 'id' field or is invalid")
	}

	replyMsg := &RecordInstance{
		Fields: []RecordField{
			{Key: "id", Value: stringToList(idStr)},
			{Key: "payload", Value: args[1]},
			{Key: "type", Value: stringToList(extractReplyType(args[1]))},
		},
	}

	if mockRes, handled := applyMailboxMock(e, fromStr, replyMsg); handled {
		return mockRes
	}

	if e.MailboxHandler == nil || e.MailboxHandler.SendWait == nil {
		return makeFailStr("mailbox API not injected by host (hint: run via `funxy vmm <script>`)")
	}

	if err := e.MailboxHandler.SendWait(fromStr, replyMsg, timeoutMs, e.Context); err != nil {
		return makeFailStr(err.Error())
	}

	return makeOk(&Nil{})
}

// requestWait(targetId: String, payload: a, timeoutMs: Int = DefaultMailboxTimeout) -> Result<String, Message>
func builtinMailboxRequestWait(e *Evaluator, args ...Object) Object {
	if len(args) != 2 && len(args) != 3 {
		return newError("requestWait expects 2 or 3 arguments")
	}

	targetId, err := getStringArg(args[0], "targetId")
	if err != nil {
		return newError("%s", err.Error())
	}

	timeoutMs := DefaultMailboxTimeout
	if len(args) == 3 {
		timeoutMs, err = getIntArg(args[2], "timeoutMs")
		if err != nil {
			return newError("%s", err.Error())
		}
	}

	reqId, ok := getMessageId(args[1])
	if !ok {
		reqId = uuid.Must(uuid.NewV7()).String()
	}

	reqMsg := &RecordInstance{
		Fields: []RecordField{
			{Key: "id", Value: stringToList(reqId)},
			{Key: "payload", Value: args[1]},
		},
	}

	// Check for mock first
	if tr := GetTestRunner(); tr != nil {
		if callback, ok := tr.FindMailboxMock(targetId); ok {
			// Apply mock callback, the callback acts as the receiver and returns the reply
			res := e.ApplyFunction(callback, []Object{reqMsg})
			if isError(res) {
				return makeFailStr(res.(*Error).Message)
			}
			return makeOk(res)
		}
	}

	if e.MailboxHandler == nil || e.MailboxHandler.Send == nil || e.MailboxHandler.ReceiveByWait == nil {
		return makeFailStr("mailbox API not injected by host (hint: run via `funxy vmm <script>`)")
	}

	// Send message
	if err := e.MailboxHandler.Send(targetId, reqMsg); err != nil {
		return makeFailStr(err.Error())
	}

	// Create predicate to wait for reply with the same id
	// predicate: \m -> m.id == reqId
	predicateFn := func(msg Object) Object {
		idStr, ok := getMessageId(msg)
		if !ok || idStr != reqId {
			return &Boolean{Value: false}
		}
		return &Boolean{Value: true}
	}

	// Create an internal Builtin function object that wraps predicateFn
	builtinPred := &Builtin{
		Name: "internal_predicate",
		Fn: func(eval *Evaluator, pArgs ...Object) Object {
			if len(pArgs) != 1 {
				return &Boolean{Value: false}
			}
			return predicateFn(pArgs[0])
		},
	}

	res, err := e.MailboxHandler.ReceiveByWait(builtinPred, timeoutMs, e.Context)
	if err != nil {
		return makeFailStr(err.Error())
	}

	return makeOk(res)
}

// receive() -> Result<String, Message>
func builtinMailboxReceive(e *Evaluator, args ...Object) Object {
	if len(args) != 0 {
		return newError("receive expects 0 arguments")
	}

	if e.MailboxHandler == nil || e.MailboxHandler.Receive == nil {
		return makeFailStr("mailbox API not injected by host (hint: run via `funxy vmm <script>`)")
	}

	res, err := e.MailboxHandler.Receive()
	if err != nil {
		return makeFailStr(err.Error())
	}
	return makeOk(res)
}

// receiveWait(timeoutMs: Int = DefaultMailboxTimeout) -> Result<String, Message>
func builtinMailboxReceiveWait(e *Evaluator, args ...Object) Object {
	if len(args) != 0 && len(args) != 1 {
		return newError("receiveWait expects 0 or 1 argument")
	}

	timeoutMs := DefaultMailboxTimeout
	if len(args) == 1 {
		var err error
		timeoutMs, err = getIntArg(args[0], "timeoutMs")
		if err != nil {
			return newError("%s", err.Error())
		}
	}

	if e.MailboxHandler == nil || e.MailboxHandler.ReceiveWait == nil {
		return makeFailStr("mailbox API not injected by host (hint: run via `funxy vmm <script>`)")
	}

	res, err := e.MailboxHandler.ReceiveWait(timeoutMs, e.Context)
	if err != nil {
		return makeFailStr(err.Error())
	}
	return makeOk(res)
}

// receiveBy(predicate: (Message) -> Bool) -> Result<String, Message>
func builtinMailboxReceiveBy(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("receiveBy expects 1 argument")
	}

	if e.MailboxHandler == nil || e.MailboxHandler.ReceiveBy == nil {
		return makeFailStr("mailbox API not injected by host (hint: run via `funxy vmm <script>`)")
	}

	res, err := e.MailboxHandler.ReceiveBy(args[0])
	if err != nil {
		return makeFailStr(err.Error())
	}
	return makeOk(res)
}

// receiveByWait(predicate: (Message) -> Bool, timeoutMs: Int = DefaultMailboxTimeout) -> Result<String, Message>
func builtinMailboxReceiveByWait(e *Evaluator, args ...Object) Object {
	if len(args) != 1 && len(args) != 2 {
		return newError("receiveByWait expects 1 or 2 arguments")
	}

	timeoutMs := DefaultMailboxTimeout
	if len(args) == 2 {
		var err error
		timeoutMs, err = getIntArg(args[1], "timeoutMs")
		if err != nil {
			return newError("%s", err.Error())
		}
	}

	if e.MailboxHandler == nil || e.MailboxHandler.ReceiveByWait == nil {
		return makeFailStr("mailbox API not injected by host (hint: run via `funxy vmm <script>`)")
	}

	res, err := e.MailboxHandler.ReceiveByWait(args[0], timeoutMs, e.Context)
	if err != nil {
		return makeFailStr(err.Error())
	}
	return makeOk(res)
}

// peek() -> Result<String, Message>
func builtinMailboxPeek(e *Evaluator, args ...Object) Object {
	if len(args) != 0 {
		return newError("peek expects 0 arguments")
	}

	if e.MailboxHandler == nil || e.MailboxHandler.Peek == nil {
		return makeFailStr("mailbox API not injected by host (hint: run via `funxy vmm <script>`)")
	}

	res, err := e.MailboxHandler.Peek()
	if err != nil {
		return makeFailStr(err.Error())
	}
	return makeOk(res)
}

// peekBy(predicate: (Message) -> Bool) -> Result<String, Message>
func builtinMailboxPeekBy(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("peekBy expects 1 argument")
	}

	if e.MailboxHandler == nil || e.MailboxHandler.PeekBy == nil {
		return makeFailStr("mailbox API not injected by host (hint: run via `funxy vmm <script>`)")
	}

	res, err := e.MailboxHandler.PeekBy(args[0])
	if err != nil {
		return makeFailStr(err.Error())
	}
	return makeOk(res)
}
