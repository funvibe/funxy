package funxy

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"github.com/funvibe/funxy/internal/evaluator"
	"github.com/funvibe/funxy/internal/modules"
	"github.com/funvibe/funxy/internal/vm"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// CapabilityProvider is a function that injects specific bindings into a VM
// based on requested capabilities.
type CapabilityProvider func(cap string, vm *VM) error

// VMEntry holds the state of a running VM
type VMEntry struct {
	VM            *VM
	Cancel        context.CancelFunc
	Stopped       chan struct{}
	ExitChan      chan struct{} // Closed when VM goroutine exits (for host to wait without consuming child events)
	ExitError     error
	Group         string
	StopRequested atomic.Bool
	GracefulStop  atomic.Bool
	Status        atomic.Int32
	StartedAt     time.Time
}

type chunkCacheEntry struct {
	Chunk   *vm.Chunk
	ModTime time.Time
	Hash    [32]byte
}

type bundleCacheEntry struct {
	Bundle  *vm.Bundle
	ModTime time.Time
	Hash    [32]byte
}

type rpcCircuitState int

const (
	rpcCircuitClosed rpcCircuitState = iota
	rpcCircuitOpen
	rpcCircuitHalfOpen
)

type rpcCircuit struct {
	state                    rpcCircuitState
	failures                 []time.Time
	openedAt                 time.Time
	halfOpenInFlight         bool
	fastFailTotal            uint64
	transitionsOpenTotal     uint64
	transitionsHalfOpenTotal uint64
	transitionsClosedTotal   uint64
}

// RPCCircuitConfig controls hypervisor-level RPC circuit breaker behavior.
// All time fields are in milliseconds.
type RPCCircuitConfig struct {
	FailureThreshold int
	FailureWindowMs  int
	OpenTimeoutMs    int
}

// RPCCircuitStats is an observable snapshot of a VM RPC circuit.
type RPCCircuitStats struct {
	VMID                     string
	State                    string
	StateCode                int
	FailureCount             int
	OpenSinceMs              int64
	HalfOpenInFlight         bool
	FastFailTotal            uint64
	TransitionsOpenTotal     uint64
	TransitionsHalfOpenTotal uint64
	TransitionsClosedTotal   uint64
	FailureThreshold         int
	FailureWindowMs          int
	OpenTimeoutMs            int
}

// RPCTraceEvent is a streaming trace item for cross-VM RPC calls.
type RPCTraceEvent struct {
	Seq        uint64
	TsMs       int64
	TraceID    string
	FromVM     string
	ToVM       string
	Group      string
	Method     string
	ArgPreview string
	Status     string
	Error      string
	DurationMs int64
	Transport  string
}

// Hypervisor manages the lifecycle of multiple isolated Funxy VMs.
// It implements the Supervisor Pattern for Funxy VMM architecture.
type Hypervisor struct {
	mu                      sync.RWMutex
	vms                     atomic.Value // *vm.PersistentMap
	mailboxes               atomic.Value // *vm.PersistentMap
	vmIDCounter             uint64
	capProviders            []CapabilityProvider
	chunkCache              map[string]*chunkCacheEntry
	bundleCache             map[string]*bundleCacheEntry // for .fbc paths
	eventRing               []map[string]interface{}
	eventHead               int
	eventSize               int
	eventCap                int
	eventSeq                uint64
	droppedEvents           uint64
	eventNotify             chan struct{}
	groups                  map[string][]string // groupName -> list of VM IDs
	groupIndices            map[string]int      // groupName -> next index to try (round-robin)
	rpcCircuits             map[string]*rpcCircuit
	rpcDefaultCircuitConfig RPCCircuitConfig
	rpcCircuitConfigByVM    map[string]RPCCircuitConfig
	rpcSerializationMode    string
	traceEnabled            map[string]bool
	traceAllEnabled         bool
	traceSubs               map[chan RPCTraceEvent]struct{}
	traceCounter            uint64
	traceHistory            []RPCTraceEvent
	traceHistoryHead        int
	traceHistorySize        int
	traceHistoryCap         int
}

const (
	vmStatusUnknown int32 = iota
	vmStatusRunning
	vmStatusStopping
	vmStatusStopped
	vmStatusCrashed
)

const defaultEventQueueCapacity = 1024
const defaultTraceHistoryCapacity = 2048

const (
	defaultRPCCircuitFailureThreshold = 3
	defaultRPCCircuitFailureWindowMs  = 5000
	defaultRPCCircuitOpenTimeoutMs    = 2000
)

var ErrRPCCircuitOpen = errors.New("CircuitOpen")

func normalizeRPCCircuitConfig(cfg RPCCircuitConfig) RPCCircuitConfig {
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = defaultRPCCircuitFailureThreshold
	}
	if cfg.FailureWindowMs <= 0 {
		cfg.FailureWindowMs = defaultRPCCircuitFailureWindowMs
	}
	if cfg.OpenTimeoutMs <= 0 {
		cfg.OpenTimeoutMs = defaultRPCCircuitOpenTimeoutMs
	}
	return cfg
}

// DefaultRPCCircuitConfig returns default circuit breaker settings.
func DefaultRPCCircuitConfig() RPCCircuitConfig {
	return RPCCircuitConfig{
		FailureThreshold: defaultRPCCircuitFailureThreshold,
		FailureWindowMs:  defaultRPCCircuitFailureWindowMs,
		OpenTimeoutMs:    defaultRPCCircuitOpenTimeoutMs,
	}
}

// NewHypervisor creates a new Hypervisor instance.
func NewHypervisor() *Hypervisor {
	return NewHypervisorWithRPCCircuitConfig(DefaultRPCCircuitConfig())
}

// NewHypervisorWithRPCCircuitConfig creates a Hypervisor with custom global RPC circuit settings.
func NewHypervisorWithRPCCircuitConfig(cfg RPCCircuitConfig) *Hypervisor {
	cfg = normalizeRPCCircuitConfig(cfg)
	h := &Hypervisor{
		capProviders:            make([]CapabilityProvider, 0),
		chunkCache:              make(map[string]*chunkCacheEntry),
		bundleCache:             make(map[string]*bundleCacheEntry),
		eventCap:                defaultEventQueueCapacity,
		eventRing:               make([]map[string]interface{}, defaultEventQueueCapacity),
		eventNotify:             make(chan struct{}),
		groups:                  make(map[string][]string),
		groupIndices:            make(map[string]int),
		rpcCircuits:             make(map[string]*rpcCircuit),
		rpcDefaultCircuitConfig: cfg,
		rpcCircuitConfigByVM:    make(map[string]RPCCircuitConfig),
		rpcSerializationMode:    evaluator.SerializeModeAuto,
		traceEnabled:            make(map[string]bool),
		traceSubs:               make(map[chan RPCTraceEvent]struct{}),
		traceHistory:            make([]RPCTraceEvent, defaultTraceHistoryCapacity),
		traceHistoryCap:         defaultTraceHistoryCapacity,
	}
	h.vms.Store(vm.EmptyMap())
	h.mailboxes.Store(vm.EmptyMap())
	return h
}

// SetRPCSerializationMode updates serialization mode for slow-path RPC transport.
// Allowed values: "auto", "fdf", "ephemeral". Invalid values fallback to "auto".
func (h *Hypervisor) SetRPCSerializationMode(mode string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case evaluator.SerializeModeFDF:
		h.rpcSerializationMode = evaluator.SerializeModeFDF
	case evaluator.SerializeModeEphemeral:
		h.rpcSerializationMode = evaluator.SerializeModeEphemeral
	default:
		h.rpcSerializationMode = evaluator.SerializeModeAuto
	}
}

func (h *Hypervisor) GetRPCSerializationMode() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.rpcSerializationMode == "" {
		return evaluator.SerializeModeAuto
	}
	return h.rpcSerializationMode
}

func (h *Hypervisor) resolveSerializationMode() string {
	return evaluator.ResolveSerializationMode(h.GetRPCSerializationMode())
}

// SetRPCCircuitConfig updates global circuit breaker settings.
func (h *Hypervisor) SetRPCCircuitConfig(cfg RPCCircuitConfig) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.rpcDefaultCircuitConfig = normalizeRPCCircuitConfig(cfg)
}

// Helper to get atomic maps
func (h *Hypervisor) getVMs() *vm.PersistentMap {
	return h.vms.Load().(*vm.PersistentMap)
}

func (h *Hypervisor) getMailboxes() *vm.PersistentMap {
	return h.mailboxes.Load().(*vm.PersistentMap)
}

// RegisterCapabilityProvider registers a new provider for host capabilities.
func (h *Hypervisor) RegisterCapabilityProvider(provider CapabilityProvider) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.capProviders = append(h.capProviders, provider)
}

func fileFingerprint(path string) (time.Time, [32]byte, error) {
	var zero [32]byte
	fi, err := os.Stat(path)
	if err != nil {
		return time.Time{}, zero, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return time.Time{}, zero, err
	}
	return fi.ModTime(), sha256.Sum256(data), nil
}

func vmStatusString(status int32) string {
	switch status {
	case vmStatusRunning:
		return "running"
	case vmStatusStopping:
		return "stopping"
	case vmStatusStopped:
		return "stopped"
	case vmStatusCrashed:
		return "crashed"
	default:
		return "unknown"
	}
}

func (h *Hypervisor) signalEventLocked() {
	close(h.eventNotify)
	h.eventNotify = make(chan struct{})
}

func (h *Hypervisor) pushEventLocked(event map[string]interface{}) {
	if h.eventCap <= 0 {
		h.eventCap = defaultEventQueueCapacity
		h.eventRing = make([]map[string]interface{}, h.eventCap)
	}
	h.eventSeq++
	event["seq"] = h.eventSeq
	if h.eventSize < h.eventCap {
		idx := (h.eventHead + h.eventSize) % h.eventCap
		h.eventRing[idx] = event
		h.eventSize++
	} else {
		// Queue is full: overwrite oldest event and increment dropped metric.
		h.eventRing[h.eventHead] = event
		h.eventHead = (h.eventHead + 1) % h.eventCap
		h.droppedEvents++
	}
	h.signalEventLocked()
}

func (h *Hypervisor) popEventLocked() (map[string]interface{}, bool) {
	if h.eventSize == 0 {
		return nil, false
	}
	evt := h.eventRing[h.eventHead]
	h.eventRing[h.eventHead] = nil
	h.eventHead = (h.eventHead + 1) % h.eventCap
	h.eventSize--
	return evt, true
}

func (h *Hypervisor) removeVMResources(id string, entry *VMEntry) {
	// Remove mailbox
	for {
		oldMBs := h.getMailboxes()
		if mbObj := oldMBs.Get(id); mbObj != nil {
			mb := mbObj.(*MailboxObject).Mailbox
			mb.Close()
			newMBs := oldMBs.Delete(id)
			if h.mailboxes.CompareAndSwap(oldMBs, newMBs) {
				break
			}
		} else {
			break
		}
	}

	// Remove VM
	for {
		oldVMs := h.getVMs()
		if oldVMs.Get(id) == nil {
			break
		}
		newVMs := oldVMs.Delete(id)
		if h.vms.CompareAndSwap(oldVMs, newVMs) {
			break
		}
	}

	// Remove from group index
	if entry != nil && entry.Group != "" {
		h.mu.Lock()
		if groupVMs, ok := h.groups[entry.Group]; ok {
			var newGroupVMs []string
			for _, vid := range groupVMs {
				if vid != id {
					newGroupVMs = append(newGroupVMs, vid)
				}
			}
			if len(newGroupVMs) == 0 {
				delete(h.groups, entry.Group)
				delete(h.groupIndices, entry.Group)
			} else {
				h.groups[entry.Group] = newGroupVMs
				if h.groupIndices[entry.Group] >= len(newGroupVMs) {
					h.groupIndices[entry.Group] = 0
				}
			}
		}
		h.mu.Unlock()
	}

	h.mu.Lock()
	delete(h.rpcCircuits, id)
	delete(h.rpcCircuitConfigByVM, id)
	delete(h.traceEnabled, id)
	h.mu.Unlock()
}

func (h *Hypervisor) getOrCreateRPCCircuitLocked(targetID string) *rpcCircuit {
	if c, ok := h.rpcCircuits[targetID]; ok {
		return c
	}
	c := &rpcCircuit{state: rpcCircuitClosed, failures: make([]time.Time, 0)}
	h.rpcCircuits[targetID] = c
	return c
}

func pruneCircuitFailures(now time.Time, failures []time.Time, failureWindowMs int) []time.Time {
	cutoff := now.Add(-time.Duration(failureWindowMs) * time.Millisecond)
	idx := 0
	for idx < len(failures) && failures[idx].Before(cutoff) {
		idx++
	}
	if idx == 0 {
		return failures
	}
	return append([]time.Time{}, failures[idx:]...)
}

func (h *Hypervisor) getRPCCircuitConfigLocked(targetID string) RPCCircuitConfig {
	if cfg, ok := h.rpcCircuitConfigByVM[targetID]; ok {
		return cfg
	}
	return h.rpcDefaultCircuitConfig
}

func parseIntConfigValue(v interface{}) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}

func (h *Hypervisor) resolveRPCCircuitConfig(config map[string]interface{}) RPCCircuitConfig {
	h.mu.RLock()
	resolved := h.rpcDefaultCircuitConfig
	h.mu.RUnlock()

	raw, ok := config["rpcCircuit"].(map[string]interface{})
	if !ok {
		return resolved
	}

	if v, ok := parseIntConfigValue(raw["failureThreshold"]); ok {
		resolved.FailureThreshold = v
	}
	if v, ok := parseIntConfigValue(raw["failureWindowMs"]); ok {
		resolved.FailureWindowMs = v
	}
	if v, ok := parseIntConfigValue(raw["openTimeoutMs"]); ok {
		resolved.OpenTimeoutMs = v
	}
	return normalizeRPCCircuitConfig(resolved)
}

func (h *Hypervisor) beforeRPCCall(targetID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()
	c := h.getOrCreateRPCCircuitLocked(targetID)
	cfg := h.getRPCCircuitConfigLocked(targetID)

	if c.state == rpcCircuitOpen {
		if now.Sub(c.openedAt) >= time.Duration(cfg.OpenTimeoutMs)*time.Millisecond {
			c.state = rpcCircuitHalfOpen
			c.transitionsHalfOpenTotal++
			c.halfOpenInFlight = false
		} else {
			c.fastFailTotal++
			return ErrRPCCircuitOpen
		}
	}

	if c.state == rpcCircuitHalfOpen {
		if c.halfOpenInFlight {
			c.fastFailTotal++
			return ErrRPCCircuitOpen
		}
		c.halfOpenInFlight = true
	}
	return nil
}

func (h *Hypervisor) afterRPCCall(targetID string, callErr error) {
	if callErr != nil && errors.Is(callErr, ErrRPCCircuitOpen) {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()
	c := h.getOrCreateRPCCircuitLocked(targetID)
	cfg := h.getRPCCircuitConfigLocked(targetID)
	c.failures = pruneCircuitFailures(now, c.failures, cfg.FailureWindowMs)

	if callErr == nil {
		if c.state == rpcCircuitHalfOpen {
			c.halfOpenInFlight = false
			c.state = rpcCircuitClosed
			c.transitionsClosedTotal++
		}
		c.failures = c.failures[:0]
		return
	}

	c.failures = append(c.failures, now)
	if c.state == rpcCircuitHalfOpen || len(c.failures) >= cfg.FailureThreshold {
		if c.state != rpcCircuitOpen {
			c.transitionsOpenTotal++
		}
		c.state = rpcCircuitOpen
		c.openedAt = now
		c.halfOpenInFlight = false
	}
}

func rpcCircuitStateString(s rpcCircuitState) string {
	switch s {
	case rpcCircuitOpen:
		return "open"
	case rpcCircuitHalfOpen:
		return "half_open"
	default:
		return "closed"
	}
}

func (h *Hypervisor) nextTraceID() string {
	n := atomic.AddUint64(&h.traceCounter, 1)
	return fmt.Sprintf("rpc-%d-%d", time.Now().UnixMilli(), n)
}

func (h *Hypervisor) shouldTrace(from, to string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.traceAllEnabled || h.traceEnabled[from] || h.traceEnabled[to]
}

func traceArgPreviewFromObject(obj evaluator.Object) string {
	if obj == nil {
		return "nil"
	}
	// For collections, avoid expensive full inspection
	switch o := obj.(type) {
	case *evaluator.List:
		return fmt.Sprintf("List(len=%d)", o.Len())
	case *evaluator.Map:
		return fmt.Sprintf("Map(len=%d)", o.Len())
	case *evaluator.RecordInstance:
		return fmt.Sprintf("Record(fields=%d)", len(o.Fields))
	case *evaluator.Tuple:
		return fmt.Sprintf("Tuple(len=%d)", len(o.Elements))
	case *evaluator.Bytes:
		return fmt.Sprintf("Bytes(len=%d)", o.Len())
	}
	preview := obj.Inspect()
	if len(preview) > 256 {
		return preview[:256] + "..."
	}
	return preview
}

func traceArgPreviewFromBytes(args []byte) string {
	if len(args) == 0 {
		return "nil"
	}
	obj, err := evaluator.DeserializeValue(args)
	if err != nil {
		return "<bytes>"
	}
	return traceArgPreviewFromObject(obj)
}

func (h *Hypervisor) emitRPCTrace(evt RPCTraceEvent) {
	h.mu.Lock()
	enabled := h.traceAllEnabled || h.traceEnabled[evt.FromVM] || h.traceEnabled[evt.ToVM]
	if !enabled {
		h.mu.Unlock()
		return
	}
	evt.Seq = atomic.AddUint64(&h.traceCounter, 1)
	h.pushTraceLocked(evt)
	subs := make([]chan RPCTraceEvent, 0, len(h.traceSubs))
	for ch := range h.traceSubs {
		subs = append(subs, ch)
	}
	h.mu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- evt:
		default:
		}
	}
}

func (h *Hypervisor) pushTraceLocked(evt RPCTraceEvent) {
	if h.traceHistoryCap <= 0 {
		h.traceHistoryCap = defaultTraceHistoryCapacity
		h.traceHistory = make([]RPCTraceEvent, h.traceHistoryCap)
	}
	if h.traceHistorySize < h.traceHistoryCap {
		idx := (h.traceHistoryHead + h.traceHistorySize) % h.traceHistoryCap
		h.traceHistory[idx] = evt
		h.traceHistorySize++
		return
	}
	h.traceHistory[h.traceHistoryHead] = evt
	h.traceHistoryHead = (h.traceHistoryHead + 1) % h.traceHistoryCap
}

// GetRPCTraceRecent returns up to `limit` latest trace events for a VM.
func (h *Hypervisor) GetRPCTraceRecent(id string, limit int) []RPCTraceEvent {
	if limit <= 0 {
		limit = 50
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.traceHistorySize == 0 {
		return []RPCTraceEvent{}
	}

	out := make([]RPCTraceEvent, 0, limit)
	for i := h.traceHistorySize - 1; i >= 0 && len(out) < limit; i-- {
		idx := (h.traceHistoryHead + i) % h.traceHistoryCap
		evt := h.traceHistory[idx]
		if evt.FromVM == id || evt.ToVM == id {
			out = append(out, evt)
		}
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// TraceOn enables RPC tracing for a specific VM.
func (h *Hypervisor) TraceOn(id string) error {
	if h.getVMs().Get(id) == nil {
		return fmt.Errorf("VM '%s' not found", id)
	}
	h.mu.Lock()
	h.traceEnabled[id] = true
	h.mu.Unlock()
	return nil
}

// TraceOnAll enables RPC tracing for all VMs.
func (h *Hypervisor) TraceOnAll() {
	h.mu.Lock()
	h.traceAllEnabled = true
	h.mu.Unlock()
}

// TraceOff disables RPC tracing for a specific VM.
func (h *Hypervisor) TraceOff(id string) error {
	h.mu.Lock()
	delete(h.traceEnabled, id)
	h.mu.Unlock()
	return nil
}

// TraceOffAll disables global RPC tracing and clears per-VM trace flags.
func (h *Hypervisor) TraceOffAll() {
	h.mu.Lock()
	h.traceAllEnabled = false
	h.traceEnabled = make(map[string]bool)
	h.mu.Unlock()
}

// SubscribeRPCTrace subscribes to live RPC trace events.
func (h *Hypervisor) SubscribeRPCTrace() (<-chan RPCTraceEvent, func()) {
	ch := make(chan RPCTraceEvent, 256)
	h.mu.Lock()
	h.traceSubs[ch] = struct{}{}
	h.mu.Unlock()
	unsubscribe := func() {
		h.mu.Lock()
		if _, ok := h.traceSubs[ch]; ok {
			delete(h.traceSubs, ch)
			close(ch)
		}
		h.mu.Unlock()
	}
	return ch, unsubscribe
}

// GetRPCCircuitStats returns circuit breaker diagnostics for a target VM.
func (h *Hypervisor) GetRPCCircuitStats(id string) (RPCCircuitStats, error) {
	if h.getVMs().Get(id) == nil {
		return RPCCircuitStats{}, fmt.Errorf("VM '%s' not found", id)
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	cfg := h.getRPCCircuitConfigLocked(id)
	c := h.getOrCreateRPCCircuitLocked(id)
	c.failures = pruneCircuitFailures(time.Now(), c.failures, cfg.FailureWindowMs)

	openSinceMs := int64(0)
	if c.state == rpcCircuitOpen && !c.openedAt.IsZero() {
		openSinceMs = c.openedAt.UnixMilli()
	}

	return RPCCircuitStats{
		VMID:                     id,
		State:                    rpcCircuitStateString(c.state),
		StateCode:                int(c.state),
		FailureCount:             len(c.failures),
		OpenSinceMs:              openSinceMs,
		HalfOpenInFlight:         c.halfOpenInFlight,
		FastFailTotal:            c.fastFailTotal,
		TransitionsOpenTotal:     c.transitionsOpenTotal,
		TransitionsHalfOpenTotal: c.transitionsHalfOpenTotal,
		TransitionsClosedTotal:   c.transitionsClosedTotal,
		FailureThreshold:         cfg.FailureThreshold,
		FailureWindowMs:          cfg.FailureWindowMs,
		OpenTimeoutMs:            cfg.OpenTimeoutMs,
	}, nil
}

// SpawnVM creates, configures, and starts a new isolated VM.
func (h *Hypervisor) SpawnVM(path string, config map[string]interface{}) (string, error) {
	state, _ := config["_initial_state"].([]byte)
	reloadCache := false
	if reload, ok := config["reload"].(bool); ok {
		reloadCache = reload
	}
	rpcCircuitConfig := h.resolveRPCCircuitConfig(config)

	// Generate ID
	// We use atomic counter to ensure unique defaults under concurrency
	var nextID uint64
	id := ""
	if name, ok := config["name"].(string); ok {
		id = name
	} else {
		nextID = atomic.AddUint64(&h.vmIDCounter, 1)
		id = fmt.Sprintf("vm_%d", nextID)
	}

	var group string
	if g, ok := config["group"].(string); ok {
		group = g
	}

	// Check existence (optimistic check)
	// We check if the ID exists in the current snapshot of the map.
	// This allows us to fail fast before performing expensive operations like VM creation and compilation.
	// Note that this is not the final check; another goroutine might insert the ID after this check
	// but before our CAS update. The CAS loop at the end handles that case definitively.
	if h.getVMs().Get(id) != nil {
		return "", fmt.Errorf("VM with id '%s' already exists", id)
	}

	vmInstance := New()
	// Set up structured logging with [id] prefix
	prefix := fmt.Sprintf("[%s] ", id)
	vmInstance.machine.SetOutput(NewPrefixWriter(os.Stdout, prefix))

	// Configure Mailbox if requested
	hasMailbox := false
	if caps, ok := config["capabilities"].([]interface{}); ok {
		for _, capIf := range caps {
			if capStr, ok := capIf.(string); ok && capStr == "lib/mailbox" {
				hasMailbox = true
				break
			}
		}
	}

	var mailbox *Mailbox
	if hasMailbox {
		mbConfig := DefaultMailboxConfig()
		if mbMap, ok := config["mailbox"].(map[string]interface{}); ok {
			if capIf, ok := mbMap["capacity"]; ok {
				if cap, ok := capIf.(int64); ok {
					mbConfig.Capacity = int(cap)
				}
				if cap, ok := capIf.(int); ok {
					mbConfig.Capacity = cap
				}
			}
			if drop, ok := mbMap["dropOnFull"]; ok {
				if d, ok := drop.(int64); ok {
					mbConfig.DropOnFull = int(d)
				} else if s, ok := drop.(string); ok {
					switch s {
					case "Low":
						mbConfig.DropOnFull = 1
					case "Info":
						mbConfig.DropOnFull = 2
					case "Warn":
						mbConfig.DropOnFull = 3
					case "Crit":
						mbConfig.DropOnFull = 4
					case "System":
						mbConfig.DropOnFull = 5
					}
				}
			}
			if str, ok := mbMap["strategy"]; ok {
				if s, ok := str.(string); ok {
					mbConfig.Strategy = s
				}
			}
		}
		mailbox = NewMailbox(mbConfig)
		vmInstance.machine.GetEvaluator().MailboxHandler = h.MailboxHandler(id)
	}

	if len(state) > 0 {
		vmInstance.SetInitialState(state)
	}
	ctx, cancel := context.WithCancel(context.Background())
	vmInstance.machine.SetContext(ctx) // This ensures context is passed down

	// Register Supervisor handler so this spawned VM can also spawn VMs
	// if it has the "supervisor" capability
	vmInstance.machine.GetEvaluator().SupervisorHandler = h.SupervisorHandlerFor(id)

	// Apply Limits
	if limits, ok := config["limits"].(map[string]interface{}); ok {
		if v, ok := limits["maxMemoryMB"]; ok {
			var m uint64
			switch val := v.(type) {
			case int:
				m = uint64(val)
			case int64:
				m = uint64(val)
			case float64:
				m = uint64(val)
			}
			if m > 0 {
				vmInstance.machine.MaxAllocBytes = m * 1024 * 1024
			}
		}
		if v, ok := limits["maxAllocationsPerSecond"]; ok {
			var m uint64
			switch val := v.(type) {
			case int:
				m = uint64(val)
			case int64:
				m = uint64(val)
			case float64:
				m = uint64(val)
			}
			if m > 0 {
				vmInstance.machine.MaxAllocBytesPerSecond = m
			}
		}
		if v, ok := limits["maxInstructions"]; ok {
			var ins uint64
			switch val := v.(type) {
			case int:
				ins = uint64(val)
			case int64:
				ins = uint64(val)
			case float64:
				ins = uint64(val)
			}
			if ins > 0 {
				vmInstance.machine.MaxInstructions = ins
			}
		}
		if v, ok := limits["maxInstructionsPerSec"]; ok {
			var ins uint64
			switch val := v.(type) {
			case int:
				ins = uint64(val)
			case int64:
				ins = uint64(val)
			case float64:
				ins = uint64(val)
			}
			if ins > 0 {
				vmInstance.machine.MaxInstructionsPerSec = ins
			}
		}
		if v, ok := limits["maxStackDepth"]; ok {
			var sd int
			switch val := v.(type) {
			case int:
				sd = val
			case int64:
				sd = int(val)
			case float64:
				sd = int(val)
			}
			if sd > 0 {
				vmInstance.machine.MaxStackDepth = sd
			}
		}
	}

	// Process capabilities
	// Note: We need to lock h.mu only for reading capProviders if it can change dynamically.
	// Assuming capProviders is append-only or stable during spawn.
	// But RegisterCapabilityProvider uses Lock. So we should RLock.
	h.mu.RLock()
	providers := h.capProviders
	h.mu.RUnlock()

	if caps, ok := config["capabilities"].([]interface{}); ok {
		for _, capIf := range caps {
			if capStr, ok := capIf.(string); ok {
				vmInstance.AllowModule(capStr) // allow the module explicitly in Sandbox mode
				provided := false
				for _, provider := range providers {
					if err := provider(capStr, vmInstance); err == nil {
						provided = true
						break // First provider to succeed handles it
					}
				}
				if !provided {
					cancel()
					return "", fmt.Errorf("capability '%s' could not be provided", capStr)
				}
			}
		}
	}

	// Load chunk: from .fbc bundle or compile .lang source
	var chunk *vm.Chunk
	if strings.HasSuffix(strings.ToLower(path), ".fbc") {
		modTime, hash, fpErr := fileFingerprint(path)
		if fpErr != nil {
			cancel()
			return "", fmt.Errorf("failed to stat/hash .fbc file: %w", fpErr)
		}

		h.mu.RLock()
		cacheEntry, ok := h.bundleCache[path]
		h.mu.RUnlock()
		useCached := ok && !reloadCache && cacheEntry.ModTime.Equal(modTime) && cacheEntry.Hash == hash

		var bundle *vm.Bundle
		if useCached {
			bundle = cacheEntry.Bundle
		} else {
			data, err := os.ReadFile(path)
			if err != nil {
				cancel()
				return "", fmt.Errorf("failed to read .fbc file: %w", err)
			}
			bundle, err = vm.DeserializeAny(data)
			if err != nil {
				cancel()
				return "", fmt.Errorf("failed to deserialize .fbc: %w", err)
			}
			if err := bundle.Validate(); err != nil {
				cancel()
				return "", fmt.Errorf("invalid .fbc bundle: %w", err)
			}
			h.mu.Lock()
			h.bundleCache[path] = &bundleCacheEntry{
				Bundle:  bundle,
				ModTime: modTime,
				Hash:    hash,
			}
			h.mu.Unlock()
		}
		vmInstance.machine.PrepareForBundle(bundle)
		chunk = bundle.MainChunk
		// baseDir fallback when bundle has no source path (e.g. single-file .fbc)
		if bundle.SourceFile == "" && (chunk == nil || chunk.File == "") {
			vmInstance.machine.SetBaseDir(filepath.Dir(path))
		}
	} else {
		modTime, hash, fpErr := fileFingerprint(path)
		if fpErr != nil {
			cancel()
			return "", fmt.Errorf("failed to stat/hash source file: %w", fpErr)
		}

		h.mu.RLock()
		cacheEntry, ok := h.chunkCache[path]
		h.mu.RUnlock()
		useCached := ok && !reloadCache && cacheEntry.ModTime.Equal(modTime) && cacheEntry.Hash == hash
		if useCached {
			chunk = cacheEntry.Chunk
		} else {
			// Compile WITHOUT holding h.mu
			c, err := vmInstance.CompileFile(path)
			if err != nil {
				cancel()
				return "", fmt.Errorf("compile error: %v", err)
			}
			chunk = c
			if chunk != nil {
				h.mu.Lock()
				h.chunkCache[path] = &chunkCacheEntry{
					Chunk:   chunk,
					ModTime: modTime,
					Hash:    hash,
				}
				h.mu.Unlock()
			}
		}
	}

	// Validate imports against sandbox synchronously before spawning
	if chunk != nil {
		if loader := vmInstance.machine.GetLoader(); loader != nil && loader.SandboxMode {
			for _, imp := range chunk.PendingImports {
				if !modules.IsPureVirtualPackage(imp.Path) && !loader.AllowedModules[imp.Path] {
					cancel()
					return "", fmt.Errorf("VM '%s': capability denied: module '%s' is not allowed in sandbox mode", id, imp.Path)
				}
			}
		}
	}

	entry := &VMEntry{
		VM:        vmInstance,
		Cancel:    cancel,
		Stopped:   make(chan struct{}),
		ExitChan:  make(chan struct{}),
		Group:     group,
		StartedAt: time.Now(),
	}
	entry.Status.Store(vmStatusRunning)

	// Add to group registry if group is specified
	if group != "" {
		h.mu.Lock()
		h.groups[group] = append(h.groups[group], id)
		h.mu.Unlock()
	}

	// Atomic update loop for VMs
	for {
		oldVMs := h.getVMs()
		if oldVMs.Get(id) != nil {
			cancel()
			return "", fmt.Errorf("VM with id '%s' already exists (race detected)", id)
		}
		newVMs := oldVMs.Put(id, &VMEntryObject{Entry: entry})
		if h.vms.CompareAndSwap(oldVMs, newVMs) {
			break
		}
	}

	// Atomic update loop for Mailboxes (if any)
	if mailbox != nil {
		for {
			oldMBs := h.getMailboxes()
			newMBs := oldMBs.Put(id, &MailboxObject{Mailbox: mailbox})
			if h.mailboxes.CompareAndSwap(oldMBs, newMBs) {
				break
			}
		}
	}

	h.mu.Lock()
	h.rpcCircuitConfigByVM[id] = rpcCircuitConfig
	h.mu.Unlock()

	// Run VM in the background
	go func() {
		defer close(entry.Stopped)
		// We don't defer h.KillVM(id) anymore, because KillVM waits for this to finish

		var err error
		if chunk != nil {
			err = vmInstance.RunChunk(chunk, filepath.Dir(path))
		}

		// After main chunk finishes, if there's no error, we try to call onInit.
		// A microservice might not have a blocking loop, so RunChunk returns.
		if err == nil {
			if fnObj := vmInstance.machine.GetGlobals().Get("onInit"); fnObj != nil {
				// Pass Option<a>: Some(state) when hot-reloading, None on fresh start
				var stateObj evaluator.Object = evaluator.MakeNone()
				if stateBytes, ok := config["_initial_state"].([]byte); ok && len(stateBytes) > 0 {
					if decoded, decErr := evaluator.DeserializeValue(stateBytes); decErr == nil {
						stateObj = evaluator.MakeSome(decoded)
					}
				}

				res, callErr := vmInstance.machine.CallFunction(fnObj, []evaluator.Object{stateObj})
				if callErr != nil {
					err = fmt.Errorf("onInit failed: %v", callErr)
				} else {
					// Save returned state
					if vmInstance.machine.GetEvaluator().StateHandler != nil {
						vmInstance.machine.GetEvaluator().StateHandler.SetState(res)
					}
				}
			}
		}

		// The worker does NOT die automatically when the main script finishes.
		// It stays alive to handle RPCs or mailbox messages until the supervisor kills it.
		if err == nil {
			<-ctx.Done()
			err = ctx.Err()
		}

		// Update exit error
		if obj := h.getVMs().Get(id); obj != nil {
			if entryObj, ok := obj.(*VMEntryObject); ok {
				entryObj.Entry.ExitError = err
			}
		}

		// Prepare exit event and error string
		isError := false
		errStr := ""
		event := map[string]interface{}{
			"type": "exit",
			"vmId": id,
		}

		if err != nil {
			errMsg := err.Error()
			// Graceful stop can surface wrapped context-canceled errors in some paths;
			// treat them as normal stop events, not crashes.
			if !(entry.StopRequested.Load() && (err == context.Canceled || strings.Contains(errMsg, "context canceled"))) {
				if err != context.Canceled {
					isError = true
					errStr = errMsg
					if strings.Contains(errMsg, vm.ErrMemoryLimitExceeded.Error()) {
						event["type"] = "vm_exit"
						event["reason"] = "limit_exceeded"
						event["detail"] = "memory limit exceeded"
						errStr = "memory limit exceeded"
					} else if strings.Contains(errMsg, vm.ErrGasLimitExceeded.Error()) {
						event["type"] = "vm_exit"
						event["reason"] = "limit_exceeded"
						event["detail"] = "gas limit exceeded"
						errStr = "gas limit exceeded"
					} else if strings.Contains(errMsg, vm.ErrStackLimitExceeded.Error()) {
						event["type"] = "vm_exit"
						event["reason"] = "limit_exceeded"
						event["detail"] = "stack limit exceeded"
						errStr = "stack limit exceeded"
					} else if !strings.Contains(errMsg, "killed by supervisor") && !strings.Contains(errMsg, "context canceled") {
						event["type"] = "crash"
						event["error"] = errMsg
					}
				} else if !entry.GracefulStop.Load() {
					isError = true
					errStr = "killed by supervisor"
					event["type"] = "crash"
					event["error"] = errStr
				}
			}
		}

		if isError {
			entry.Status.Store(vmStatusCrashed)
		} else {
			entry.Status.Store(vmStatusStopped)
		}
		event["status"] = vmStatusString(entry.Status.Load())

		// Call onError hook before closing channels and sending event
		if isError {
			if fnObj := vmInstance.machine.GetGlobals().Get("onError"); fnObj != nil {
				// We need a fresh context because the old one is canceled or timed out
				newCtx, newCancel := context.WithTimeout(context.Background(), 5*time.Second)
				vmInstance.machine.SetContext(newCtx)

				chars := make([]evaluator.Object, 0, len(errStr))
				for _, r := range errStr {
					chars = append(chars, &evaluator.Char{Value: int64(r)})
				}
				strObj := evaluator.NewListWithType(chars, "Char")

				_, _ = vmInstance.machine.CallFunction(fnObj, []evaluator.Object{strObj})

				newCancel()
			}
		}

		select {
		case <-entry.ExitChan:
			// already closed
		default:
			close(entry.ExitChan)
		}

		h.mu.Lock()
		h.pushEventLocked(event)
		h.mu.Unlock()

		// Predictable cleanup policy:
		// terminal VMs that are not in stop flow are auto-removed.
		if !entry.StopRequested.Load() {
			h.removeVMResources(id, entry)
		}
	}()

	return id, nil
}

// KillVM gracefully shuts down a running VM. If saveState is true, it calls onTerminate and returns the serialized state.
// It will wait up to timeoutMs before forcefully terminating.
func (h *Hypervisor) KillVM(id string, saveState bool, timeoutMs int) ([]byte, error) {
	return h.terminateVM(id, saveState, timeoutMs, saveState)
}

// StopVM performs an intentional supervisor stop.
// Under Variant A semantics, stop path is classified as stopped (unless stop flow itself fails).
func (h *Hypervisor) StopVM(id string, saveState bool, timeoutMs int) ([]byte, error) {
	return h.terminateVM(id, saveState, timeoutMs, true)
}

func (h *Hypervisor) terminateVM(id string, saveState bool, timeoutMs int, stopRequested bool) ([]byte, error) {
	obj := h.getVMs().Get(id)
	if obj == nil {
		return nil, fmt.Errorf("VM '%s' not found", id)
	}
	entry := obj.(*VMEntryObject).Entry

	// 1. Cancel the original context to stop any running infinite loops or HTTP servers
	// This naturally stops the VM. We don't SetContext(newCtx) here because it causes data races.
	entry.StopRequested.Store(stopRequested)
	if saveState {
		entry.GracefulStop.Store(true)
	}
	entry.Status.Store(vmStatusStopping)
	entry.Cancel()

	// Create a channel to signal when shutdown is complete
	done := make(chan struct{})
	var stateData []byte
	var shutDownErr error
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())
	defer shutdownCancel()

	go func() {
		defer close(done)

		// 2. Wait for the VM to stop executing its main chunk
		<-entry.Stopped

		if saveState {
			// Now that the VM has stopped, it's safe to set a new context for the shutdown hook
			entry.VM.machine.SetContext(shutdownCtx)

			// 3. Call onTerminate if defined
			if fnObj := entry.VM.machine.GetGlobals().Get("onTerminate"); fnObj != nil {
				// Find current state
				var currentState evaluator.Object = &evaluator.Nil{}
				if entry.VM.machine.GetEvaluator().StateHandler != nil {
					if st := entry.VM.machine.GetEvaluator().StateHandler.GetState(); st != nil {
						currentState = st
					}
				}

				// Call the hook
				res, callErr := entry.VM.machine.CallFunction(fnObj, []evaluator.Object{currentState})
				if callErr != nil {
					shutDownErr = fmt.Errorf("onTerminate failed: %v", callErr)
				} else {
					stateData, shutDownErr = evaluator.SerializeValue(res, h.resolveSerializationMode())
				}
			} else {
				// If no onTerminate, just serialize the current state from StateHandler
				if entry.VM.machine.GetEvaluator().StateHandler != nil {
					if st := entry.VM.machine.GetEvaluator().StateHandler.GetState(); st != nil {
						stateData, shutDownErr = evaluator.SerializeValue(st, h.resolveSerializationMode())
					}
				}
			}
		} else {
			// Not a graceful stop (killVM). If it didn't crash earlier, call onError.
			if entry.ExitError == nil {
				if fnObj := entry.VM.machine.GetGlobals().Get("onError"); fnObj != nil {
					newCtx, newCancel := context.WithTimeout(context.Background(), 5*time.Second)
					entry.VM.machine.SetContext(newCtx)

					errStr := "killed by supervisor"
					chars := make([]evaluator.Object, 0, len(errStr))
					for _, r := range errStr {
						chars = append(chars, &evaluator.Char{Value: int64(r)})
					}
					strObj := evaluator.NewListWithType(chars, "Char")

					_, _ = entry.VM.machine.CallFunction(fnObj, []evaluator.Object{strObj})

					newCancel()
				}
			}
		}
	}()

	// 4. Wait for graceful shutdown or timeout
	var finalErr error
	if timeoutMs > 0 {
		select {
		case <-done:
			finalErr = shutDownErr
		case <-time.After(time.Duration(timeoutMs) * time.Millisecond):
			finalErr = fmt.Errorf("graceful shutdown timed out after %dms", timeoutMs)
			// Cancel shutdown-hook context to stop hung onTerminate without SetContext races.
			shutdownCancel()
		}
	} else {
		// No timeout, block indefinitely
		<-done
		finalErr = shutDownErr
	}

	if finalErr != nil && entry.Status.Load() != vmStatusCrashed {
		entry.Status.Store(vmStatusCrashed)
	} else if entry.Status.Load() != vmStatusCrashed {
		entry.Status.Store(vmStatusStopped)
	}
	h.removeVMResources(id, entry)

	return stateData, finalErr
}

// ListVMs returns a list of running VM IDs.
func (h *Hypervisor) ListVMs() []string {
	vms := h.getVMs()
	var list []string
	vms.Range(func(key string, value evaluator.Object) bool {
		list = append(list, key)
		return true
	})
	return list
}

// GetStats returns monitoring statistics for a VM.
func (h *Hypervisor) GetStats(id string) (map[string]uint64, error) {
	obj := h.getVMs().Get(id)
	if obj == nil {
		return nil, fmt.Errorf("VM '%s' not found", id)
	}
	entry := obj.(*VMEntryObject).Entry
	stats := entry.VM.GetMetrics()

	if mbObj := h.getMailboxes().Get(id); mbObj != nil {
		mb := mbObj.(*MailboxObject).Mailbox
		mbStats := mb.GetMetrics()
		for k, v := range mbStats {
			stats[k] = v
		}
	}
	stats["status_code"] = uint64(entry.Status.Load())
	stats["event_queue_dropped_total"] = h.EventDroppedTotal()
	stats["event_queue_size"] = uint64(h.EventQueueSize())
	stats["event_queue_capacity"] = uint64(h.EventQueueCapacity())
	if circuitStats, err := h.GetRPCCircuitStats(id); err == nil {
		stats["rpc_circuit_state_code"] = uint64(circuitStats.StateCode)
		stats["rpc_circuit_failure_count"] = uint64(circuitStats.FailureCount)
		stats["rpc_circuit_fast_fail_total"] = circuitStats.FastFailTotal
		stats["rpc_circuit_transitions_open_total"] = circuitStats.TransitionsOpenTotal
		stats["rpc_circuit_transitions_half_open_total"] = circuitStats.TransitionsHalfOpenTotal
		stats["rpc_circuit_transitions_closed_total"] = circuitStats.TransitionsClosedTotal
	}
	return stats, nil
}

// InspectVM returns detailed inspection information for a VM, including stats and stack traces.
func (h *Hypervisor) InspectVM(id string) (map[string]interface{}, error) {
	obj := h.getVMs().Get(id)
	if obj == nil {
		return nil, fmt.Errorf("VM '%s' not found", id)
	}
	entry := obj.(*VMEntryObject).Entry

	info := make(map[string]interface{})

	// 1. Get stats
	stats, err := h.GetStats(id)
	if err == nil {
		info["stats"] = stats
	}

	// 2. Get VM stack trace
	info["stack_trace"] = entry.VM.machine.GetStackTrace()

	// 3. Uptime
	info["started_at"] = entry.StartedAt.Unix()
	info["uptime_seconds"] = time.Since(entry.StartedAt).Seconds()
	info["status"] = vmStatusString(entry.Status.Load())
	info["status_code"] = entry.Status.Load()
	info["rpc_serialization_mode"] = h.GetRPCSerializationMode()

	return info, nil
}

// WaitForExit returns a channel that is closed when the VM exits. Use this for the host to wait
// without consuming child VM events from the shared event queue.
func (h *Hypervisor) WaitForExit(id string) <-chan struct{} {
	obj := h.getVMs().Get(id)
	if obj == nil {
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	return obj.(*VMEntryObject).Entry.ExitChan
}

// ReceiveEvent blocks until an event is available and returns it.
func (h *Hypervisor) ReceiveEvent() map[string]interface{} {
	for {
		h.mu.Lock()
		if evt, ok := h.popEventLocked(); ok {
			h.mu.Unlock()
			return evt
		}
		notify := h.eventNotify
		h.mu.Unlock()
		<-notify
	}
}

// ReceiveEventTimeout waits for the next event up to timeoutMs.
// Returns (event, true) on success or (nil, false) on timeout.
func (h *Hypervisor) ReceiveEventTimeout(timeoutMs int) (map[string]interface{}, bool) {
	if timeoutMs <= 0 {
		return h.ReceiveEvent(), true
	}
	deadline := time.Now().Add(time.Duration(timeoutMs) * time.Millisecond)
	for {
		h.mu.Lock()
		if evt, ok := h.popEventLocked(); ok {
			h.mu.Unlock()
			return evt, true
		}
		notify := h.eventNotify
		remaining := time.Until(deadline)
		h.mu.Unlock()
		if remaining <= 0 {
			return nil, false
		}
		timer := time.NewTimer(remaining)
		select {
		case <-notify:
			if !timer.Stop() {
				<-timer.C
			}
		case <-timer.C:
			return nil, false
		}
	}
}

func (h *Hypervisor) EventDroppedTotal() uint64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.droppedEvents
}

func (h *Hypervisor) EventQueueSize() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.eventSize
}

func (h *Hypervisor) EventQueueCapacity() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.eventCap
}

// BroadcastEvent injects a custom event into the hypervisor's event stream.
func (h *Hypervisor) BroadcastEvent(event map[string]interface{}) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.pushEventLocked(event)
}

// RPCCall executes a function in another VM's context synchronously.
// The result is serialized and returned.
func (h *Hypervisor) RPCCall(targetID, method string, args []byte, timeoutMs int) ([]byte, error) {
	return h.RPCCallFrom(CallerIDHost, targetID, method, args, timeoutMs)
}

// RPCCallFrom executes an RPC call with explicit caller identity for tracing.
func (h *Hypervisor) RPCCallFrom(callerID, targetID, method string, args []byte, timeoutMs int) ([]byte, error) {
	traceID := h.nextTraceID()
	started := time.Now()
	var argPreview string
	if h.shouldTrace(callerID, targetID) {
		argPreview = traceArgPreviewFromBytes(args)
	}
	if err := h.beforeRPCCall(targetID); err != nil {
		h.emitRPCTrace(RPCTraceEvent{
			TsMs:       time.Now().UnixMilli(),
			TraceID:    traceID,
			FromVM:     callerID,
			ToVM:       targetID,
			Method:     method,
			ArgPreview: argPreview,
			Status:     "fast_fail",
			Error:      err.Error(),
			DurationMs: 0,
			Transport:  "rpc",
		})
		return nil, err
	}
	res, err := h.rpcCallRaw(targetID, method, args, timeoutMs)
	h.afterRPCCall(targetID, err)
	status := "ok"
	errMsg := ""
	if err != nil {
		status = "error"
		errMsg = err.Error()
	}
	h.emitRPCTrace(RPCTraceEvent{
		TsMs:       time.Now().UnixMilli(),
		TraceID:    traceID,
		FromVM:     callerID,
		ToVM:       targetID,
		Method:     method,
		ArgPreview: argPreview,
		Status:     status,
		Error:      errMsg,
		DurationMs: time.Since(started).Milliseconds(),
		Transport:  "rpc",
	})
	return res, err
}

func (h *Hypervisor) rpcCallRaw(targetID, method string, args []byte, timeoutMs int) ([]byte, error) {
	obj := h.getVMs().Get(targetID)
	if obj == nil {
		return nil, fmt.Errorf("VM '%s' not found", targetID)
	}
	entry := obj.(*VMEntryObject).Entry

	// Wait until VM is fully initialized (has a context) to avoid race conditions with SpawnVM
	if entry.VM == nil || entry.VM.machine == nil {
		return nil, fmt.Errorf("VM '%s' is not fully initialized", targetID)
	}

	// Deserialize args and execute via fast-path raw call in isolated VM context.
	// This avoids mutating the target VM evaluator context concurrently with worker execution.
	var argObj evaluator.Object = &evaluator.Nil{}
	if len(args) > 0 {
		var err error
		argObj, err = evaluator.DeserializeValue(args)
		if err != nil {
			return nil, fmt.Errorf("failed to deserialize args: %v", err)
		}
	}

	res, err := h.rpcCallFastRaw(targetID, method, argObj, timeoutMs)
	if err != nil {
		return nil, err
	}

	// Serialize the result back to bytes
	return evaluator.SerializeValue(res, h.resolveSerializationMode())
}

const CallerIDHost = "host"

// RPCCallFast is the zero-copy Fast Path for RPC when both VMs are in the same process.
// It skips serialization entirely since Funxy collections are strictly immutable.
func (h *Hypervisor) RPCCallFast(targetID, method string, argsObj evaluator.Object, timeoutMs int) (evaluator.Object, error) {
	// Fast path from host is trusted
	return h.RPCCallFastFrom(CallerIDHost, targetID, method, argsObj, timeoutMs)
}

// RPCCallFastFrom executes fast-path RPC with explicit caller identity for tracing.
func (h *Hypervisor) RPCCallFastFrom(callerID, targetID, method string, argsObj evaluator.Object, timeoutMs int) (evaluator.Object, error) {
	traceID := h.nextTraceID()
	started := time.Now()
	if err := evaluator.CheckSerializable(argsObj); err != nil {
		return nil, fmt.Errorf("cannot pass mutable or non-serializable object via RPC: %v", err)
	}
	var argPreview string
	if h.shouldTrace(callerID, targetID) {
		argPreview = traceArgPreviewFromObject(argsObj)
	}
	if err := h.beforeRPCCall(targetID); err != nil {
		h.emitRPCTrace(RPCTraceEvent{
			TsMs:       time.Now().UnixMilli(),
			TraceID:    traceID,
			FromVM:     callerID,
			ToVM:       targetID,
			Method:     method,
			ArgPreview: argPreview,
			Status:     "fast_fail",
			Error:      err.Error(),
			DurationMs: 0,
			Transport:  "rpc_fast",
		})
		return nil, err
	}
	res, err := h.rpcCallFastRaw(targetID, method, argsObj, timeoutMs)
	h.afterRPCCall(targetID, err)
	status := "ok"
	errMsg := ""
	if err != nil {
		status = "error"
		errMsg = err.Error()
	}
	h.emitRPCTrace(RPCTraceEvent{
		TsMs:       time.Now().UnixMilli(),
		TraceID:    traceID,
		FromVM:     callerID,
		ToVM:       targetID,
		Method:     method,
		ArgPreview: argPreview,
		Status:     status,
		Error:      errMsg,
		DurationMs: time.Since(started).Milliseconds(),
		Transport:  "rpc_fast",
	})
	return res, err
}

// RPCCallFastUnsafeFrom executes fast-path RPC with explicit caller identity for tracing, skipping serialization check.
func (h *Hypervisor) RPCCallFastUnsafeFrom(callerID, targetID, method string, argsObj evaluator.Object, timeoutMs int) (evaluator.Object, error) {
	traceID := h.nextTraceID()
	started := time.Now()
	var argPreview string
	if h.shouldTrace(callerID, targetID) {
		argPreview = traceArgPreviewFromObject(argsObj)
	}
	if err := h.beforeRPCCall(targetID); err != nil {
		h.emitRPCTrace(RPCTraceEvent{
			TsMs:       time.Now().UnixMilli(),
			TraceID:    traceID,
			FromVM:     callerID,
			ToVM:       targetID,
			Method:     method,
			ArgPreview: argPreview,
			Status:     "fast_fail",
			Error:      err.Error(),
			DurationMs: 0,
			Transport:  "rpc_fast_unsafe",
		})
		return nil, err
	}
	res, err := h.rpcCallFastRaw(targetID, method, argsObj, timeoutMs)
	h.afterRPCCall(targetID, err)
	status := "ok"
	errMsg := ""
	if err != nil {
		status = "error"
		errMsg = err.Error()
	}
	h.emitRPCTrace(RPCTraceEvent{
		TsMs:       time.Now().UnixMilli(),
		TraceID:    traceID,
		FromVM:     callerID,
		ToVM:       targetID,
		Method:     method,
		ArgPreview: argPreview,
		Status:     status,
		Error:      errMsg,
		DurationMs: time.Since(started).Milliseconds(),
		Transport:  "rpc_fast_unsafe",
	})
	return res, err
}

func (h *Hypervisor) rpcCallFastRaw(targetID, method string, argsObj evaluator.Object, timeoutMs int) (evaluator.Object, error) {
	obj := h.getVMs().Get(targetID)
	if obj == nil {
		return nil, fmt.Errorf("VM '%s' not found", targetID)
	}
	entry := obj.(*VMEntryObject).Entry

	// Wait until VM is fully initialized
	if entry.VM == nil || entry.VM.machine == nil {
		return nil, fmt.Errorf("VM '%s' is not fully initialized", targetID)
	}

	// Wait up to 1 second for the VM to register its globals
	// This is important for fast startup in tests
	deadline := time.Now().Add(time.Second)
	var fnObj evaluator.Object
	for time.Now().Before(deadline) {
		fnObj = entry.VM.machine.GetGlobals().Get(method)
		if fnObj != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if fnObj == nil {
		return nil, fmt.Errorf("function '%s' not found on VM '%s'", method, targetID)
	}

	// Execute function in target VM's context using its internal evaluator.
	// We pass argsObj directly (Zero-Copy) since it's immutable.
	rpcEvalContext := context.Background()
	var cancel context.CancelFunc
	if timeoutMs > 0 {
		rpcEvalContext, cancel = context.WithTimeout(rpcEvalContext, time.Duration(timeoutMs)*time.Millisecond)
		defer cancel()
	}

	isolatedVM := vm.New()
	isolatedVM.SetGlobals(entry.VM.machine.GetGlobals())

	// Copy handlers to the isolated evaluator
	eval := isolatedVM.GetEvaluator()
	origEval := entry.VM.machine.GetEvaluator()
	eval.MailboxHandler = origEval.MailboxHandler
	eval.SupervisorHandler = origEval.SupervisorHandler
	eval.StateHandler = origEval.StateHandler
	eval.Context = rpcEvalContext

	type rpcCallResult struct {
		res evaluator.Object
		err error
	}
	resCh := make(chan rpcCallResult, 1)
	go func() {
		res, err := isolatedVM.CallFunction(fnObj, []evaluator.Object{argsObj})
		resCh <- rpcCallResult{res: res, err: err}
	}()

	if timeoutMs > 0 {
		// Priority check for context timeout
		select {
		case <-rpcEvalContext.Done():
			return nil, fmt.Errorf("RPC call to '%s'.'%s' timed out after %dms", targetID, method, timeoutMs)
		default:
		}

		select {
		case r := <-resCh:
			// If context also expired simultaneously, prioritize the timeout error
			// to ensure deterministic behavior under heavy system load.
			if rpcEvalContext.Err() != nil {
				return nil, fmt.Errorf("RPC call to '%s'.'%s' timed out after %dms", targetID, method, timeoutMs)
			}
			if r.err != nil {
				return nil, r.err
			}
			return r.res, nil
		case <-rpcEvalContext.Done():
			return nil, fmt.Errorf("RPC call to '%s'.'%s' timed out after %dms", targetID, method, timeoutMs)
		}
	}

	r := <-resCh
	if r.err != nil {
		return nil, r.err
	}

	return r.res, nil
}

// getNextVMInGroup picks a VM from the given group using Round Robin.
func (h *Hypervisor) getNextVMInGroup(group string) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	vms, ok := h.groups[group]
	if !ok || len(vms) == 0 {
		return "", fmt.Errorf("no VMs available in group '%s'", group)
	}

	idx := h.groupIndices[group]
	if idx >= len(vms) {
		idx = 0
	}

	vmID := vms[idx]
	h.groupIndices[group] = (idx + 1) % len(vms)

	return vmID, nil
}

// RPCCallGroup executes a function in another VM's context synchronously, picking a VM from the given group via Round Robin.
func (h *Hypervisor) RPCCallGroup(group, method string, args []byte, timeoutMs int) ([]byte, error) {
	return h.RPCCallGroupFrom(CallerIDHost, group, method, args, timeoutMs)
}

// RPCCallGroupFrom executes group RPC with explicit caller identity for tracing.
func (h *Hypervisor) RPCCallGroupFrom(callerID, group, method string, args []byte, timeoutMs int) ([]byte, error) {
	h.mu.RLock()
	groupSize := len(h.groups[group])
	h.mu.RUnlock()
	if groupSize == 0 {
		return nil, fmt.Errorf("no VMs available in group '%s'", group)
	}
	var lastErr error
	var argPreview string
	previewCalculated := false
	for i := 0; i < groupSize; i++ {
		traceID := h.nextTraceID()
		started := time.Now()
		targetID, err := h.getNextVMInGroup(group)
		if err != nil {
			return nil, err
		}
		if h.shouldTrace(callerID, targetID) && !previewCalculated {
			argPreview = traceArgPreviewFromBytes(args)
			previewCalculated = true
		}
		if err := h.beforeRPCCall(targetID); err != nil {
			h.emitRPCTrace(RPCTraceEvent{
				TsMs:       time.Now().UnixMilli(),
				TraceID:    traceID,
				FromVM:     callerID,
				ToVM:       targetID,
				Group:      group,
				Method:     method,
				ArgPreview: argPreview,
				Status:     "fast_fail",
				Error:      err.Error(),
				DurationMs: 0,
				Transport:  "rpc_group",
			})
			lastErr = err
			continue
		}
		res, callErr := h.rpcCallRaw(targetID, method, args, timeoutMs)
		h.afterRPCCall(targetID, callErr)
		status := "ok"
		errMsg := ""
		if callErr != nil {
			status = "error"
			errMsg = callErr.Error()
		}
		h.emitRPCTrace(RPCTraceEvent{
			TsMs:       time.Now().UnixMilli(),
			TraceID:    traceID,
			FromVM:     callerID,
			ToVM:       targetID,
			Group:      group,
			Method:     method,
			ArgPreview: argPreview,
			Status:     status,
			Error:      errMsg,
			DurationMs: time.Since(started).Milliseconds(),
			Transport:  "rpc_group",
		})
		if callErr == nil {
			return res, nil
		}
		lastErr = callErr
	}
	if lastErr == nil {
		lastErr = ErrRPCCircuitOpen
	}
	return nil, lastErr
}

// RPCCallGroupFast is the zero-copy Fast Path for RPC when both VMs are in the same process, picking a VM from the given group via Round Robin.
func (h *Hypervisor) RPCCallGroupFast(group, method string, args evaluator.Object, timeoutMs int) (evaluator.Object, error) {
	return h.RPCCallGroupFastFrom(CallerIDHost, group, method, args, timeoutMs)
}

// RPCCallGroupFastFrom executes group fast-path RPC with explicit caller identity for tracing.
func (h *Hypervisor) RPCCallGroupFastFrom(callerID, group, method string, args evaluator.Object, timeoutMs int) (evaluator.Object, error) {
	h.mu.RLock()
	groupSize := len(h.groups[group])
	h.mu.RUnlock()
	if groupSize == 0 {
		return nil, fmt.Errorf("no VMs available in group '%s'", group)
	}
	if err := evaluator.CheckSerializable(args); err != nil {
		return nil, fmt.Errorf("cannot pass mutable or non-serializable object via RPC: %v", err)
	}
	var lastErr error
	var argPreview string
	previewCalculated := false
	for i := 0; i < groupSize; i++ {
		traceID := h.nextTraceID()
		started := time.Now()
		targetID, err := h.getNextVMInGroup(group)
		if err != nil {
			return nil, err
		}
		if h.shouldTrace(callerID, targetID) && !previewCalculated {
			argPreview = traceArgPreviewFromObject(args)
			previewCalculated = true
		}
		if err := h.beforeRPCCall(targetID); err != nil {
			h.emitRPCTrace(RPCTraceEvent{
				TsMs:       time.Now().UnixMilli(),
				TraceID:    traceID,
				FromVM:     callerID,
				ToVM:       targetID,
				Group:      group,
				Method:     method,
				ArgPreview: argPreview,
				Status:     "fast_fail",
				Error:      err.Error(),
				DurationMs: 0,
				Transport:  "rpc_group_fast",
			})
			lastErr = err
			continue
		}
		res, callErr := h.rpcCallFastRaw(targetID, method, args, timeoutMs)
		h.afterRPCCall(targetID, callErr)
		status := "ok"
		errMsg := ""
		if callErr != nil {
			status = "error"
			errMsg = callErr.Error()
		}
		h.emitRPCTrace(RPCTraceEvent{
			TsMs:       time.Now().UnixMilli(),
			TraceID:    traceID,
			FromVM:     callerID,
			ToVM:       targetID,
			Group:      group,
			Method:     method,
			ArgPreview: argPreview,
			Status:     status,
			Error:      errMsg,
			DurationMs: time.Since(started).Milliseconds(),
			Transport:  "rpc_group_fast",
		})
		if callErr == nil {
			return res, nil
		}
		lastErr = callErr
	}
	if lastErr == nil {
		lastErr = ErrRPCCircuitOpen
	}
	return nil, lastErr
}

// RPCCallGroupFastUnsafeFrom executes group fast-path RPC with explicit caller identity for tracing, skipping serialization checks.
func (h *Hypervisor) RPCCallGroupFastUnsafeFrom(callerID, group, method string, args evaluator.Object, timeoutMs int) (evaluator.Object, error) {
	h.mu.RLock()
	groupSize := len(h.groups[group])
	h.mu.RUnlock()
	if groupSize == 0 {
		return nil, fmt.Errorf("no VMs available in group '%s'", group)
	}
	var lastErr error
	var argPreview string
	previewCalculated := false
	for i := 0; i < groupSize; i++ {
		traceID := h.nextTraceID()
		started := time.Now()
		targetID, err := h.getNextVMInGroup(group)
		if err != nil {
			return nil, err
		}
		if h.shouldTrace(callerID, targetID) && !previewCalculated {
			argPreview = traceArgPreviewFromObject(args)
			previewCalculated = true
		}
		if err := h.beforeRPCCall(targetID); err != nil {
			h.emitRPCTrace(RPCTraceEvent{
				TsMs:       time.Now().UnixMilli(),
				TraceID:    traceID,
				FromVM:     callerID,
				ToVM:       targetID,
				Group:      group,
				Method:     method,
				ArgPreview: argPreview,
				Status:     "fast_fail",
				Error:      err.Error(),
				DurationMs: 0,
				Transport:  "rpc_group_fast_unsafe",
			})
			lastErr = err
			continue
		}
		res, callErr := h.rpcCallFastRaw(targetID, method, args, timeoutMs)
		h.afterRPCCall(targetID, callErr)
		status := "ok"
		errMsg := ""
		if callErr != nil {
			status = "error"
			errMsg = callErr.Error()
		}
		h.emitRPCTrace(RPCTraceEvent{
			TsMs:       time.Now().UnixMilli(),
			TraceID:    traceID,
			FromVM:     callerID,
			ToVM:       targetID,
			Group:      group,
			Method:     method,
			ArgPreview: argPreview,
			Status:     status,
			Error:      errMsg,
			DurationMs: time.Since(started).Milliseconds(),
			Transport:  "rpc_group_fast_unsafe",
		})
		if callErr == nil {
			return res, nil
		}
		lastErr = callErr
	}
	if lastErr == nil {
		lastErr = ErrRPCCircuitOpen
	}
	return nil, lastErr
}

// MailboxHandler returns an evaluator.MailboxHandler for a given VM ID
func (h *Hypervisor) MailboxHandler(callerId string) *evaluator.MailboxHandler {
	return &evaluator.MailboxHandler{
		Send: func(targetId string, msg evaluator.Object) error {
			obj := h.getMailboxes().Get(targetId)
			if obj == nil {
				return fmt.Errorf("target VM '%s' not found or has no mailbox", targetId)
			}
			mb := obj.(*MailboxObject).Mailbox

			copiedMsg, err := DeepCopy(msg)
			if err != nil {
				return err
			}
			finalMsg := EnsureMessageFormat(copiedMsg, callerId)
			return mb.Send(finalMsg)
		},
		SendWait: func(targetId string, msg evaluator.Object, timeoutMs int, ctx context.Context) error {
			obj := h.getMailboxes().Get(targetId)
			if obj == nil {
				return fmt.Errorf("target VM '%s' not found or has no mailbox", targetId)
			}
			mb := obj.(*MailboxObject).Mailbox

			copiedMsg, err := DeepCopy(msg)
			if err != nil {
				return err
			}
			finalMsg := EnsureMessageFormat(copiedMsg, callerId)
			return mb.SendWait(finalMsg, timeoutMs, ctx)
		},
		Receive: func() (evaluator.Object, error) {
			obj := h.getMailboxes().Get(callerId)
			if obj == nil {
				return nil, fmt.Errorf("VM '%s' has no mailbox", callerId)
			}
			mb := obj.(*MailboxObject).Mailbox
			return mb.Receive()
		},
		ReceiveWait: func(timeoutMs int, ctx context.Context) (evaluator.Object, error) {
			obj := h.getMailboxes().Get(callerId)
			if obj == nil {
				return nil, fmt.Errorf("VM '%s' has no mailbox", callerId)
			}
			mb := obj.(*MailboxObject).Mailbox
			return mb.ReceiveWait(timeoutMs, ctx)
		},
		ReceiveBy: func(predicate evaluator.Object) (evaluator.Object, error) {
			mbObj := h.getMailboxes().Get(callerId)
			vmObj := h.getVMs().Get(callerId)
			if mbObj == nil || vmObj == nil {
				return nil, fmt.Errorf("VM '%s' has no mailbox", callerId)
			}
			mb := mbObj.(*MailboxObject).Mailbox
			entry := vmObj.(*VMEntryObject).Entry
			return mb.ReceiveBy(entry.VM.machine.GetEvaluator(), predicate)
		},
		ReceiveByWait: func(predicate evaluator.Object, timeoutMs int, ctx context.Context) (evaluator.Object, error) {
			mbObj := h.getMailboxes().Get(callerId)
			vmObj := h.getVMs().Get(callerId)
			if mbObj == nil || vmObj == nil {
				return nil, fmt.Errorf("VM '%s' has no mailbox", callerId)
			}
			mb := mbObj.(*MailboxObject).Mailbox
			entry := vmObj.(*VMEntryObject).Entry
			return mb.ReceiveByWait(entry.VM.machine.GetEvaluator(), predicate, timeoutMs, ctx)
		},
		Peek: func() (evaluator.Object, error) {
			obj := h.getMailboxes().Get(callerId)
			if obj == nil {
				return nil, fmt.Errorf("VM '%s' has no mailbox", callerId)
			}
			mb := obj.(*MailboxObject).Mailbox
			return mb.Peek()
		},
		PeekBy: func(predicate evaluator.Object) (evaluator.Object, error) {
			mbObj := h.getMailboxes().Get(callerId)
			vmObj := h.getVMs().Get(callerId)
			if mbObj == nil || vmObj == nil {
				return nil, fmt.Errorf("VM '%s' has no mailbox", callerId)
			}
			mb := mbObj.(*MailboxObject).Mailbox
			entry := vmObj.(*VMEntryObject).Entry
			return mb.PeekBy(entry.VM.machine.GetEvaluator(), predicate)
		},
	}
}
func (h *Hypervisor) SupervisorHandler() *evaluator.SupervisorHandler {
	return h.SupervisorHandlerFor(CallerIDHost)
}

func (h *Hypervisor) SupervisorHandlerFor(callerID string) *evaluator.SupervisorHandler {
	return &evaluator.SupervisorHandler{
		SpawnVM: func(path string, config map[string]interface{}, state []byte) (string, error) {
			config["_initial_state"] = state
			return h.SpawnVM(path, config)
		},
		KillVM: func(id string, saveState bool, timeoutMs int) ([]byte, error) {
			return h.KillVM(id, saveState, timeoutMs)
		},
		StopVM: func(id string, saveState bool, timeoutMs int) ([]byte, error) {
			return h.StopVM(id, saveState, timeoutMs)
		},
		TraceOn: func(id string) error {
			return h.TraceOn(id)
		},
		TraceOff: func(id string) error {
			return h.TraceOff(id)
		},
		TraceOnAll: func() {
			h.TraceOnAll()
		},
		TraceOffAll: func() {
			h.TraceOffAll()
		},
		GetRPCCircuitStats: func(id string) (map[string]interface{}, error) {
			s, err := h.GetRPCCircuitStats(id)
			if err != nil {
				return nil, err
			}
			return map[string]interface{}{
				"vmId":                     s.VMID,
				"state":                    s.State,
				"stateCode":                s.StateCode,
				"failureCount":             s.FailureCount,
				"openSinceMs":              s.OpenSinceMs,
				"halfOpenInFlight":         s.HalfOpenInFlight,
				"fastFailTotal":            s.FastFailTotal,
				"transitionsOpenTotal":     s.TransitionsOpenTotal,
				"transitionsHalfOpenTotal": s.TransitionsHalfOpenTotal,
				"transitionsClosedTotal":   s.TransitionsClosedTotal,
				"failureThreshold":         s.FailureThreshold,
				"failureWindowMs":          s.FailureWindowMs,
				"openTimeoutMs":            s.OpenTimeoutMs,
			}, nil
		},
		ListVMs: func() []string {
			return h.ListVMs()
		},
		GetStats: func(id string) (map[string]uint64, error) {
			return h.GetStats(id)
		},
		ReceiveEvent: func() map[string]interface{} {
			return h.ReceiveEvent()
		},
		ReceiveEventTimeout: func(timeoutMs int) (map[string]interface{}, bool) {
			return h.ReceiveEventTimeout(timeoutMs)
		},
		RPCCall: func(targetID, method string, args []byte, timeoutMs int) ([]byte, error) {
			return h.RPCCallFrom(callerID, targetID, method, args, timeoutMs)
		},
		RPCCallFast: func(targetID, method string, args evaluator.Object, timeoutMs int) (evaluator.Object, error) {
			return h.RPCCallFastFrom(callerID, targetID, method, args, timeoutMs)
		},
		RPCCallFastUnsafe: func(targetID, method string, args evaluator.Object, timeoutMs int) (evaluator.Object, error) {
			return h.RPCCallFastUnsafeFrom(callerID, targetID, method, args, timeoutMs)
		},
		RPCCallGroup: func(group, method string, args []byte, timeoutMs int) ([]byte, error) {
			return h.RPCCallGroupFrom(callerID, group, method, args, timeoutMs)
		},
		RPCCallGroupFast: func(group, method string, args evaluator.Object, timeoutMs int) (evaluator.Object, error) {
			return h.RPCCallGroupFastFrom(callerID, group, method, args, timeoutMs)
		},
		RPCCallGroupFastUnsafe: func(group, method string, args evaluator.Object, timeoutMs int) (evaluator.Object, error) {
			return h.RPCCallGroupFastUnsafeFrom(callerID, group, method, args, timeoutMs)
		},
		RPCSerializationMode: func() string {
			return h.GetRPCSerializationMode()
		},
	}
}

// InjectHandlers is a helper specifically for the TestRunner to inject the hypervisor APIs
// into the test Evaluator context so that test scripts can interact with the VMs they spawn.
func (h *Hypervisor) InjectHandlers(e *evaluator.Evaluator) {
	e.SupervisorHandler = h.SupervisorHandlerFor("test_runner")

	// Create a dummy mailbox for the test runner itself if we want it to receive messages,
	// or just inject the handler with a dummy caller ID so it can send messages.
	// For sending messages to spawned VMs, just having the handler is enough.
	e.MailboxHandler = h.MailboxHandler("test_runner")
}
