package evaluator

import (
	"sync"
	"sync/atomic"
	"time"
	"unsafe" // Added unsafe for pointer hashing

	"github.com/funvibe/funxy/internal/typesystem"
)

// ============================================================================
// Task - Async computation type
// ============================================================================

// Task represents an asynchronous computation
type Task struct {
	done      chan struct{}
	result    Object
	err       string
	cancelled atomic.Bool
	mu        sync.Mutex
}

func (t *Task) Type() ObjectType { return "TASK" }
func (t *Task) TypeName() string { return "Task" }
func (t *Task) Inspect() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	select {
	case <-t.done:
		if t.err != "" {
			return "<Task:Fail>"
		}
		return "<Task:Ok>"
	default:
		if t.cancelled.Load() {
			return "<Task:Cancelled>"
		}
		return "<Task:Pending>"
	}
}
func (t *Task) RuntimeType() typesystem.Type {
	return typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Task"},
		Args:        []typesystem.Type{typesystem.TVar{Name: "T"}},
	}
}
func (t *Task) Hash() uint32 {
	return uint32(uintptr(unsafe.Pointer(t)))
}

// NewTask creates a new Task (exported for VM)
func NewTask() *Task {
	return &Task{done: make(chan struct{})}
}

// Complete completes the task with a result or error (exported for VM)
func (t *Task) Complete(result Object) {
	t.mu.Lock()
	defer t.mu.Unlock()
	defer close(t.done)

	if t.cancelled.Load() {
		t.err = "cancelled"
		return
	}

	if isError(result) {
		t.err = result.(*Error).Message
	} else {
		t.result = result
	}
}

// ============================================================================
// Global pool limiter
// ============================================================================

var (
	poolLimit   int64 = 1000 // default
	poolCurrent int64 = 0
	poolCond          = sync.NewCond(&sync.Mutex{})
)

func AcquirePoolSlot() {
	poolCond.L.Lock()
	for atomic.LoadInt64(&poolCurrent) >= atomic.LoadInt64(&poolLimit) {
		poolCond.Wait()
	}
	atomic.AddInt64(&poolCurrent, 1)
	poolCond.L.Unlock()
}

func ReleasePoolSlot() {
	atomic.AddInt64(&poolCurrent, -1)
	poolCond.Signal()
}

// ============================================================================
// Builtins
// ============================================================================

func TaskBuiltins() map[string]*Builtin {
	return map[string]*Builtin{
		// Creation
		"async":       {Fn: builtinAsync, Name: "async"},
		"taskResolve": {Fn: builtinTaskResolve, Name: "taskResolve"},
		"taskReject":  {Fn: builtinTaskReject, Name: "taskReject"},

		// Awaiting
		"await":             {Fn: builtinAwait, Name: "await"},
		"awaitTimeout":      {Fn: builtinAwaitTimeout, Name: "awaitTimeout"},
		"awaitAll":          {Fn: builtinAwaitAll, Name: "awaitAll"},
		"awaitAllTimeout":   {Fn: builtinAwaitAllTimeout, Name: "awaitAllTimeout"},
		"awaitAny":          {Fn: builtinAwaitAny, Name: "awaitAny"},
		"awaitAnyTimeout":   {Fn: builtinAwaitAnyTimeout, Name: "awaitAnyTimeout"},
		"awaitFirst":        {Fn: builtinAwaitFirst, Name: "awaitFirst"},
		"awaitFirstTimeout": {Fn: builtinAwaitFirstTimeout, Name: "awaitFirstTimeout"},

		// Control
		"taskCancel":      {Fn: builtinTaskCancel, Name: "taskCancel"},
		"taskIsDone":      {Fn: builtinTaskIsDone, Name: "taskIsDone"},
		"taskIsCancelled": {Fn: builtinTaskIsCancelled, Name: "taskIsCancelled"},

		// Pool
		"taskSetGlobalPool": {Fn: builtinTaskSetGlobalPool, Name: "taskSetGlobalPool"},
		"taskGetGlobalPool": {Fn: builtinTaskGetGlobalPool, Name: "taskGetGlobalPool"},

		// Combinators
		"taskMap":     {Fn: builtinTaskMap, Name: "taskMap"},
		"taskFlatMap": {Fn: builtinTaskFlatMap, Name: "taskFlatMap"},
		"taskCatch":   {Fn: builtinTaskCatch, Name: "taskCatch"},
	}
}

// ============================================================================
// Creation
// ============================================================================

// async: (() -> T) -> Task<T>
func builtinAsync(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("async expects 1 argument, got %d", len(args))
	}

	fn := args[0]

	// Use custom async handler if provided (e.g. for VM)
	if e.AsyncHandler != nil {
		return e.AsyncHandler(fn, []Object{})
	}

	task := &Task{done: make(chan struct{})}

	// Clone evaluator for goroutine
	evalClone := e.Clone()

	go func() {
		AcquirePoolSlot()
		defer ReleasePoolSlot()
		defer close(task.done)

		// Check if cancelled before starting
		if task.cancelled.Load() {
			task.mu.Lock()
			task.err = "cancelled"
			task.mu.Unlock()
			return
		}

		result := evalClone.ApplyFunction(fn, []Object{})

		task.mu.Lock()
		if task.cancelled.Load() {
			task.err = "cancelled"
		} else if isError(result) {
			task.err = result.(*Error).Message
		} else {
			task.result = result
		}
		task.mu.Unlock()
	}()

	return task
}

// taskResolve: T -> Task<T>
func builtinTaskResolve(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("taskResolve expects 1 argument, got %d", len(args))
	}

	task := &Task{done: make(chan struct{})}
	task.result = args[0]
	close(task.done)
	return task
}

// taskReject: String -> Task<T>
func builtinTaskReject(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("taskReject expects 1 argument, got %d", len(args))
	}

	task := &Task{done: make(chan struct{})}
	task.err = objectToString(args[0])
	close(task.done)
	return task
}

// ============================================================================
// Awaiting
// ============================================================================

// await: Task<T> -> Result<String, T>
func builtinAwait(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("await expects 1 argument, got %d", len(args))
	}

	task, ok := args[0].(*Task)
	if !ok {
		return newError("await expects a Task, got %s", args[0].Type())
	}

	<-task.done

	task.mu.Lock()
	defer task.mu.Unlock()

	if task.err != "" {
		return makeFailStr(task.err)
	}
	return makeOk(task.result)
}

// awaitTimeout: (Task<T>, Int) -> Result<String, T>
func builtinAwaitTimeout(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("awaitTimeout expects 2 arguments, got %d", len(args))
	}

	task, ok := args[0].(*Task)
	if !ok {
		return newError("awaitTimeout expects a Task, got %s", args[0].Type())
	}

	timeoutMs, ok := args[1].(*Integer)
	if !ok {
		return newError("awaitTimeout expects Int for timeout, got %s", args[1].Type())
	}

	select {
	case <-task.done:
		task.mu.Lock()
		defer task.mu.Unlock()
		if task.err != "" {
			return makeFailStr(task.err)
		}
		return makeOk(task.result)
	case <-time.After(time.Duration(timeoutMs.Value) * time.Millisecond):
		return makeFailStr("timeout")
	}
}

// awaitAll: List<Task<T>> -> Result<String, List<T>>
func builtinAwaitAll(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("awaitAll expects 1 argument, got %d", len(args))
	}

	list, ok := args[0].(*List)
	if !ok {
		return newError("awaitAll expects a List, got %s", args[0].Type())
	}

	tasks := list.ToSlice()
	results := make([]Object, len(tasks))

	for i, taskObj := range tasks {
		task, ok := taskObj.(*Task)
		if !ok {
			return newError("awaitAll: element %d is not a Task", i)
		}

		<-task.done

		task.mu.Lock()
		if task.err != "" {
			task.mu.Unlock()
			return makeFailStr(task.err)
		}
		results[i] = task.result
		task.mu.Unlock()
	}

	return makeOk(newList(results))
}

// awaitAllTimeout: (List<Task<T>>, Int) -> Result<String, List<T>>
func builtinAwaitAllTimeout(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("awaitAllTimeout expects 2 arguments, got %d", len(args))
	}

	list, ok := args[0].(*List)
	if !ok {
		return newError("awaitAllTimeout expects a List, got %s", args[0].Type())
	}

	timeoutMs, ok := args[1].(*Integer)
	if !ok {
		return newError("awaitAllTimeout expects Int for timeout, got %s", args[1].Type())
	}

	tasks := list.ToSlice()
	results := make([]Object, len(tasks))
	deadline := time.After(time.Duration(timeoutMs.Value) * time.Millisecond)

	for i, taskObj := range tasks {
		task, ok := taskObj.(*Task)
		if !ok {
			return newError("awaitAllTimeout: element %d is not a Task", i)
		}

		select {
		case <-task.done:
			task.mu.Lock()
			if task.err != "" {
				task.mu.Unlock()
				return makeFailStr(task.err)
			}
			results[i] = task.result
			task.mu.Unlock()
		case <-deadline:
			return makeFailStr("timeout")
		}
	}

	return makeOk(newList(results))
}

// awaitAny: List<Task<T>> -> Result<String, T>
// Returns first successful result, ignores failures until all fail
func builtinAwaitAny(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("awaitAny expects 1 argument, got %d", len(args))
	}

	list, ok := args[0].(*List)
	if !ok {
		return newError("awaitAny expects a List, got %s", args[0].Type())
	}

	tasks := list.ToSlice()
	if len(tasks) == 0 {
		return makeFailStr("awaitAny: empty list")
	}

	type result struct {
		value Object
		err   string
	}

	resultCh := make(chan result, len(tasks))

	for _, taskObj := range tasks {
		task, ok := taskObj.(*Task)
		if !ok {
			continue
		}
		go func(t *Task) {
			<-t.done
			t.mu.Lock()
			if t.err != "" {
				resultCh <- result{err: t.err}
			} else {
				resultCh <- result{value: t.result}
			}
			t.mu.Unlock()
		}(task)
	}

	failures := 0
	for failures < len(tasks) {
		r := <-resultCh
		if r.value != nil {
			return makeOk(r.value)
		}
		failures++
	}

	return makeFailStr("all tasks failed")
}

// awaitAnyTimeout: (List<Task<T>>, Int) -> Result<String, T>
func builtinAwaitAnyTimeout(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("awaitAnyTimeout expects 2 arguments, got %d", len(args))
	}

	list, ok := args[0].(*List)
	if !ok {
		return newError("awaitAnyTimeout expects a List, got %s", args[0].Type())
	}

	timeoutMs, ok := args[1].(*Integer)
	if !ok {
		return newError("awaitAnyTimeout expects Int for timeout, got %s", args[1].Type())
	}

	tasks := list.ToSlice()
	if len(tasks) == 0 {
		return makeFailStr("awaitAnyTimeout: empty list")
	}

	type result struct {
		value Object
		err   string
	}

	resultCh := make(chan result, len(tasks))
	deadline := time.After(time.Duration(timeoutMs.Value) * time.Millisecond)

	for _, taskObj := range tasks {
		task, ok := taskObj.(*Task)
		if !ok {
			continue
		}
		go func(t *Task) {
			<-t.done
			t.mu.Lock()
			if t.err != "" {
				resultCh <- result{err: t.err}
			} else {
				resultCh <- result{value: t.result}
			}
			t.mu.Unlock()
		}(task)
	}

	failures := 0
	for failures < len(tasks) {
		select {
		case r := <-resultCh:
			if r.value != nil {
				return makeOk(r.value)
			}
			failures++
		case <-deadline:
			return makeFailStr("timeout")
		}
	}

	return makeFailStr("all tasks failed")
}

// awaitFirst: List<Task<T>> -> Result<String, T>
// Returns first completed (success or failure)
func builtinAwaitFirst(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("awaitFirst expects 1 argument, got %d", len(args))
	}

	list, ok := args[0].(*List)
	if !ok {
		return newError("awaitFirst expects a List, got %s", args[0].Type())
	}

	tasks := list.ToSlice()
	if len(tasks) == 0 {
		return makeFailStr("awaitFirst: empty list")
	}

	type result struct {
		value Object
		err   string
	}

	resultCh := make(chan result, len(tasks))

	for _, taskObj := range tasks {
		task, ok := taskObj.(*Task)
		if !ok {
			continue
		}
		go func(t *Task) {
			<-t.done
			t.mu.Lock()
			if t.err != "" {
				resultCh <- result{err: t.err}
			} else {
				resultCh <- result{value: t.result}
			}
			t.mu.Unlock()
		}(task)
	}

	r := <-resultCh
	if r.err != "" {
		return makeFailStr(r.err)
	}
	return makeOk(r.value)
}

// awaitFirstTimeout: (List<Task<T>>, Int) -> Result<String, T>
func builtinAwaitFirstTimeout(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("awaitFirstTimeout expects 2 arguments, got %d", len(args))
	}

	list, ok := args[0].(*List)
	if !ok {
		return newError("awaitFirstTimeout expects a List, got %s", args[0].Type())
	}

	timeoutMs, ok := args[1].(*Integer)
	if !ok {
		return newError("awaitFirstTimeout expects Int for timeout, got %s", args[1].Type())
	}

	tasks := list.ToSlice()
	if len(tasks) == 0 {
		return makeFailStr("awaitFirstTimeout: empty list")
	}

	type result struct {
		value Object
		err   string
	}

	resultCh := make(chan result, len(tasks))
	deadline := time.After(time.Duration(timeoutMs.Value) * time.Millisecond)

	for _, taskObj := range tasks {
		task, ok := taskObj.(*Task)
		if !ok {
			continue
		}
		go func(t *Task) {
			<-t.done
			t.mu.Lock()
			if t.err != "" {
				resultCh <- result{err: t.err}
			} else {
				resultCh <- result{value: t.result}
			}
			t.mu.Unlock()
		}(task)
	}

	select {
	case r := <-resultCh:
		if r.err != "" {
			return makeFailStr(r.err)
		}
		return makeOk(r.value)
	case <-deadline:
		return makeFailStr("timeout")
	}
}

// ============================================================================
// Control
// ============================================================================

// taskCancel: Task<T> -> Nil
func builtinTaskCancel(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("taskCancel expects 1 argument, got %d", len(args))
	}

	task, ok := args[0].(*Task)
	if !ok {
		return newError("taskCancel expects a Task, got %s", args[0].Type())
	}

	task.cancelled.Store(true)
	return &Nil{}
}

// taskIsDone: Task<T> -> Bool
func builtinTaskIsDone(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("taskIsDone expects 1 argument, got %d", len(args))
	}

	task, ok := args[0].(*Task)
	if !ok {
		return newError("taskIsDone expects a Task, got %s", args[0].Type())
	}

	select {
	case <-task.done:
		return TRUE
	default:
		return FALSE
	}
}

// taskIsCancelled: Task<T> -> Bool
func builtinTaskIsCancelled(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("taskIsCancelled expects 1 argument, got %d", len(args))
	}

	task, ok := args[0].(*Task)
	if !ok {
		return newError("taskIsCancelled expects a Task, got %s", args[0].Type())
	}

	if task.cancelled.Load() {
		return TRUE
	}
	return FALSE
}

// ============================================================================
// Pool
// ============================================================================

// taskSetGlobalPool: Int -> Nil
func builtinTaskSetGlobalPool(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("taskSetGlobalPool expects 1 argument, got %d", len(args))
	}

	limit, ok := args[0].(*Integer)
	if !ok {
		return newError("taskSetGlobalPool expects Int, got %s", args[0].Type())
	}

	if limit.Value < 1 {
		return newError("taskSetGlobalPool: limit must be positive")
	}

	atomic.StoreInt64(&poolLimit, limit.Value)
	return &Nil{}
}

// taskGetGlobalPool: () -> Int
func builtinTaskGetGlobalPool(e *Evaluator, args ...Object) Object {
	if len(args) != 0 {
		return newError("taskGetGlobalPool expects 0 arguments, got %d", len(args))
	}

	return &Integer{Value: atomic.LoadInt64(&poolLimit)}
}

// ============================================================================
// Combinators
// ============================================================================

// taskMap: (Task<T>, (T) -> U) -> Task<U>
func builtinTaskMap(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("taskMap expects 2 arguments, got %d", len(args))
	}

	task, ok := args[0].(*Task)
	if !ok {
		return newError("taskMap expects a Task as first argument, got %s", args[0].Type())
	}

	fn := args[1]

	// Capture closure state if needed (for VM)
	if e.CaptureHandler != nil {
		fn = e.CaptureHandler(fn)
	}

	newTask := &Task{done: make(chan struct{})}

	// Use Fork if available (creates isolated VM), otherwise Clone (tree-walk)
	var evalClone *Evaluator
	if e.Fork != nil {
		evalClone = e.Fork()
	} else {
		evalClone = e.Clone()
	}

	go func() {
		defer close(newTask.done)

		<-task.done

		task.mu.Lock()
		if task.err != "" {
			newTask.mu.Lock()
			newTask.err = task.err
			newTask.mu.Unlock()
			task.mu.Unlock()
			return
		}
		result := task.result
		task.mu.Unlock()

		// Use AsyncHandler if available to execute in a safe VM context
		var mapped Object
		if evalClone.AsyncHandler != nil {
			taskObj := evalClone.AsyncHandler(fn, []Object{result})
			if t, ok := taskObj.(*Task); ok {
				<-t.done
				t.mu.Lock()
				if t.err != "" {
					mapped = &Error{Message: t.err}
				} else {
					mapped = t.result
				}
				t.mu.Unlock()
			} else {
				mapped = taskObj // Should be Error
			}
		} else {
			mapped = evalClone.ApplyFunction(fn, []Object{result})
		}

		newTask.mu.Lock()
		if isError(mapped) {
			newTask.err = mapped.(*Error).Message
		} else {
			newTask.result = mapped
		}
		newTask.mu.Unlock()
	}()

	return newTask
}

// taskFlatMap: (Task<T>, (T) -> Task<U>) -> Task<U>
func builtinTaskFlatMap(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("taskFlatMap expects 2 arguments, got %d", len(args))
	}

	task, ok := args[0].(*Task)
	if !ok {
		return newError("taskFlatMap expects a Task as first argument, got %s", args[0].Type())
	}

	fn := args[1]

	// Capture closure state if needed (for VM)
	if e.CaptureHandler != nil {
		fn = e.CaptureHandler(fn)
	}

	newTask := &Task{done: make(chan struct{})}

	// Use Fork if available (creates isolated VM), otherwise Clone (tree-walk)
	var evalClone *Evaluator
	if e.Fork != nil {
		evalClone = e.Fork()
	} else {
		evalClone = e.Clone()
	}

	go func() {
		defer close(newTask.done)

		<-task.done

		task.mu.Lock()
		if task.err != "" {
			newTask.mu.Lock()
			newTask.err = task.err
			newTask.mu.Unlock()
			task.mu.Unlock()
			return
		}
		result := task.result
		task.mu.Unlock()

		var innerTaskObj Object
		if evalClone.AsyncHandler != nil {
			// Use AsyncHandler to execute fn safely
			tObj := evalClone.AsyncHandler(fn, []Object{result})
			if t, ok := tObj.(*Task); ok {
				<-t.done
				t.mu.Lock()
				if t.err != "" {
					innerTaskObj = &Error{Message: t.err}
				} else {
					innerTaskObj = t.result
				}
				t.mu.Unlock()
			} else {
				innerTaskObj = tObj
			}
		} else {
			innerTaskObj = evalClone.ApplyFunction(fn, []Object{result})
		}

		if isError(innerTaskObj) {
			newTask.mu.Lock()
			newTask.err = innerTaskObj.(*Error).Message
			newTask.mu.Unlock()
			return
		}

		innerTask, ok := innerTaskObj.(*Task)
		if !ok {
			newTask.mu.Lock()
			newTask.err = "taskFlatMap: function must return a Task"
			newTask.mu.Unlock()
			return
		}

		<-innerTask.done

		innerTask.mu.Lock()
		newTask.mu.Lock()
		if innerTask.err != "" {
			newTask.err = innerTask.err
		} else {
			newTask.result = innerTask.result
		}
		newTask.mu.Unlock()
		innerTask.mu.Unlock()
	}()

	return newTask
}

// taskCatch: (Task<T>, (String) -> T) -> Task<T>
func builtinTaskCatch(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("taskCatch expects 2 arguments, got %d", len(args))
	}

	task, ok := args[0].(*Task)
	if !ok {
		return newError("taskCatch expects a Task as first argument, got %s", args[0].Type())
	}

	fn := args[1]

	// Capture closure state if needed (for VM)
	if e.CaptureHandler != nil {
		fn = e.CaptureHandler(fn)
	}

	newTask := &Task{done: make(chan struct{})}

	// Use Fork if available (creates isolated VM), otherwise Clone (tree-walk)
	var evalClone *Evaluator
	if e.Fork != nil {
		evalClone = e.Fork()
	} else {
		evalClone = e.Clone()
	}

	go func() {
		defer close(newTask.done)

		<-task.done

		task.mu.Lock()
		if task.err == "" {
			newTask.mu.Lock()
			newTask.result = task.result
			newTask.mu.Unlock()
			task.mu.Unlock()
			return
		}
		errStr := task.err
		task.mu.Unlock()

		// Convert error string to Funxy string (List<Char>)
		errList := stringToList(errStr)

		var recovered Object
		if evalClone.AsyncHandler != nil {
			tObj := evalClone.AsyncHandler(fn, []Object{errList})
			if t, ok := tObj.(*Task); ok {
				<-t.done
				t.mu.Lock()
				if t.err != "" {
					recovered = &Error{Message: t.err}
				} else {
					recovered = t.result
				}
				t.mu.Unlock()
			} else {
				recovered = tObj
			}
		} else {
			recovered = evalClone.ApplyFunction(fn, []Object{errList})
		}

		newTask.mu.Lock()
		if isError(recovered) {
			newTask.err = recovered.(*Error).Message
		} else {
			newTask.result = recovered
		}
		newTask.mu.Unlock()
	}()

	return newTask
}

// ============================================================================
// Type info
// ============================================================================

func SetTaskBuiltinTypes(builtins map[string]*Builtin) {
	T := typesystem.TVar{Name: "T"}
	U := typesystem.TVar{Name: "U"}

	taskT := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Task"},
		Args:        []typesystem.Type{T},
	}

	taskU := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Task"},
		Args:        []typesystem.Type{U},
	}

	listTaskT := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{taskT},
	}

	listT := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{T},
	}

	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{typesystem.Char},
	}

	resultStringT := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{stringType, T},
	}

	resultStringListT := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{stringType, listT},
	}

	fnVoidT := typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: T}
	fnTU := typesystem.TFunc{Params: []typesystem.Type{T}, ReturnType: U}
	fnTTaskU := typesystem.TFunc{Params: []typesystem.Type{T}, ReturnType: taskU}
	fnStringT := typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: T}

	types := map[string]typesystem.Type{
		// Creation
		"async":       typesystem.TFunc{Params: []typesystem.Type{fnVoidT}, ReturnType: taskT},
		"taskResolve": typesystem.TFunc{Params: []typesystem.Type{T}, ReturnType: taskT},
		"taskReject":  typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: taskT},

		// Awaiting
		"await":             typesystem.TFunc{Params: []typesystem.Type{taskT}, ReturnType: resultStringT},
		"awaitTimeout":      typesystem.TFunc{Params: []typesystem.Type{taskT, typesystem.Int}, ReturnType: resultStringT},
		"awaitAll":          typesystem.TFunc{Params: []typesystem.Type{listTaskT}, ReturnType: resultStringListT},
		"awaitAllTimeout":   typesystem.TFunc{Params: []typesystem.Type{listTaskT, typesystem.Int}, ReturnType: resultStringListT},
		"awaitAny":          typesystem.TFunc{Params: []typesystem.Type{listTaskT}, ReturnType: resultStringT},
		"awaitAnyTimeout":   typesystem.TFunc{Params: []typesystem.Type{listTaskT, typesystem.Int}, ReturnType: resultStringT},
		"awaitFirst":        typesystem.TFunc{Params: []typesystem.Type{listTaskT}, ReturnType: resultStringT},
		"awaitFirstTimeout": typesystem.TFunc{Params: []typesystem.Type{listTaskT, typesystem.Int}, ReturnType: resultStringT},

		// Control
		"taskCancel":      typesystem.TFunc{Params: []typesystem.Type{taskT}, ReturnType: typesystem.Nil},
		"taskIsDone":      typesystem.TFunc{Params: []typesystem.Type{taskT}, ReturnType: typesystem.Bool},
		"taskIsCancelled": typesystem.TFunc{Params: []typesystem.Type{taskT}, ReturnType: typesystem.Bool},

		// Pool
		"taskSetGlobalPool": typesystem.TFunc{Params: []typesystem.Type{typesystem.Int}, ReturnType: typesystem.Nil},
		"taskGetGlobalPool": typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: typesystem.Int},

		// Combinators
		"taskMap":     typesystem.TFunc{Params: []typesystem.Type{taskT, fnTU}, ReturnType: taskU},
		"taskFlatMap": typesystem.TFunc{Params: []typesystem.Type{taskT, fnTTaskU}, ReturnType: taskU},
		"taskCatch":   typesystem.TFunc{Params: []typesystem.Type{taskT, fnStringT}, ReturnType: taskT},
	}

	for name, typ := range types {
		if b, ok := builtins[name]; ok {
			b.TypeInfo = typ
		}
	}
}

// Note: stringToList is defined in builtins.go
