package funxy

import (
	"context"
	"errors"
	"fmt"
	"github.com/funvibe/funxy/internal/evaluator"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	DefaultMailboxCapacity   = 1000
	DefaultMailboxDropOnFull = 2
	SystemImportanceLevel    = 5
	ImportanceScanInit       = 6 // Initial value for finding min importance (must be > SystemImportanceLevel)
)

// MailboxConfig defines the configuration for a VM's mailbox.
type MailboxConfig struct {
	Capacity   int
	DropOnFull int
	Strategy   string // "dropOld", "dropNew", "blockOrError"
}

// DefaultMailboxConfig returns the default configuration.
func DefaultMailboxConfig() MailboxConfig {
	return MailboxConfig{
		Capacity:   DefaultMailboxCapacity,
		DropOnFull: DefaultMailboxDropOnFull,
		Strategy:   "blockOrError",
	}
}

type MailboxMetrics struct {
	MessagesReceived        uint64
	DropsFullOld            uint64
	DropsFullNew            uint64
	DropsImportance         uint64
	BlockDurationMsTotal    uint64
	BlockDurationMsCount    uint64
	SelectiveScanDepthTotal uint64
	SelectiveScanDepthCount uint64
	SkippedMessages         uint64
}

// Mailbox represents an asynchronous message queue for a VM.
type Mailbox struct {
	mu     sync.Mutex
	notify chan struct{}

	config  MailboxConfig
	queue   []evaluator.Object
	closed  bool
	metrics MailboxMetrics
}

// GetMetrics returns a snapshot of the current mailbox metrics.
func (mb *Mailbox) GetMetrics() map[string]uint64 {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	m := make(map[string]uint64)
	m["mailbox_queue_size"] = uint64(len(mb.queue))
	m["mailbox_queue_capacity"] = uint64(mb.config.Capacity)
	m["mailbox_messages_received_total"] = mb.metrics.MessagesReceived
	m["mailbox_drops_total_full_old"] = mb.metrics.DropsFullOld
	m["mailbox_drops_total_full_new"] = mb.metrics.DropsFullNew
	m["mailbox_drops_total_importance"] = mb.metrics.DropsImportance
	m["mailbox_block_duration_ms_total"] = mb.metrics.BlockDurationMsTotal
	m["mailbox_block_duration_ms_count"] = mb.metrics.BlockDurationMsCount
	m["mailbox_selective_scan_depth_total"] = mb.metrics.SelectiveScanDepthTotal
	m["mailbox_selective_scan_depth_count"] = mb.metrics.SelectiveScanDepthCount
	m["mailbox_skipped_messages"] = mb.metrics.SkippedMessages
	return m
}

// NewMailbox creates a new Mailbox.
func NewMailbox(config MailboxConfig) *Mailbox {
	return &Mailbox{
		config: config,
		queue:  make([]evaluator.Object, 0, config.Capacity),
		notify: make(chan struct{}),
	}
}

func (mb *Mailbox) broadcast() {
	close(mb.notify)
	mb.notify = make(chan struct{})
}

// Close closes the mailbox and unblocks any waiting operations.
func (mb *Mailbox) Close() {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	mb.closed = true
	mb.broadcast()
}

// getImportance extracts importance from a message record, defaults to 1.
func getImportance(msg evaluator.Object) int {
	rec, ok := msg.(*evaluator.RecordInstance)
	if !ok {
		return 1
	}
	for _, f := range rec.Fields {
		if f.Key == "importance" {
			if num, ok := f.Value.(*evaluator.Integer); ok {
				return int(num.Value)
			}
			if data, ok := f.Value.(*evaluator.DataInstance); ok {
				switch data.Name {
				case "Low":
					return 1
				case "Info":
					return 2
				case "Warn":
					return 3
				case "Crit":
					return 4
				case "System":
					return 5
				}
			}
		}
	}
	return 1
}

// EnsureMessageFormat ensures the message has 'from', 'id', 'importance', and 'type' fields.
func EnsureMessageFormat(msg evaluator.Object, senderId string) evaluator.Object {
	rec, ok := msg.(*evaluator.RecordInstance)
	if !ok {
		rec = &evaluator.RecordInstance{
			Fields: []evaluator.RecordField{
				{Key: "payload", Value: msg},
			},
		}
	}

	hasId, hasImportance, hasType, hasPayload := false, false, false, false
	var payloadVal evaluator.Object

	for _, f := range rec.Fields {
		switch f.Key {
		case "id":
			hasId = true
		case "importance":
			hasImportance = true
		case "type":
			hasType = true
		case "payload":
			hasPayload = true
			payloadVal = f.Value
		}
	}

	newFields := make([]evaluator.RecordField, 0, len(rec.Fields)+5)

	for _, f := range rec.Fields {
		if f.Key != "from" && f.Key != "id" && f.Key != "importance" && f.Key != "type" {
			newFields = append(newFields, f)
		}
	}

	newFields = append(newFields, evaluator.RecordField{
		Key:   "from",
		Value: evaluator.StringToList(senderId),
	})

	if !hasId {
		newId := uuid.Must(uuid.NewV7()).String()
		newFields = append(newFields, evaluator.RecordField{
			Key:   "id",
			Value: evaluator.StringToList(newId),
		})
	} else {
		for _, f := range rec.Fields {
			if f.Key == "id" {
				newFields = append(newFields, f)
				break
			}
		}
	}

	if !hasImportance {
		newFields = append(newFields, evaluator.RecordField{
			Key:   "importance",
			Value: &evaluator.DataInstance{Name: "Low", TypeName: "Importance"},
		})
	} else {
		for _, f := range rec.Fields {
			if f.Key == "importance" {
				newFields = append(newFields, f)
				break
			}
		}
	}

	if !hasType {
		newFields = append(newFields, evaluator.RecordField{
			Key:   "type",
			Value: evaluator.StringToList(""),
		})
	} else {
		for _, f := range rec.Fields {
			if f.Key == "type" {
				newFields = append(newFields, f)
				break
			}
		}
	}

	if !hasPayload {
		newFields = append(newFields, evaluator.RecordField{
			Key:   "payload",
			Value: &evaluator.RecordInstance{Fields: rec.Fields},
		})
	} else {
		newFields = append(newFields, evaluator.RecordField{
			Key:   "payload",
			Value: payloadVal,
		})
	}

	sort.Slice(newFields, func(i, j int) bool {
		return newFields[i].Key < newFields[j].Key
	})
	return &evaluator.RecordInstance{Fields: newFields}
}

func (mb *Mailbox) insertMessage(msg evaluator.Object) {
	importance := getImportance(msg)

	if importance >= SystemImportanceLevel {
		insertIdx := 0
		for i := 0; i < len(mb.queue); i++ {
			if getImportance(mb.queue[i]) >= SystemImportanceLevel {
				insertIdx = i + 1
			} else {
				break
			}
		}
		mb.queue = append(mb.queue[:insertIdx], append([]evaluator.Object{msg}, mb.queue[insertIdx:]...)...)
	} else {
		mb.queue = append(mb.queue, msg)
	}
}

func (mb *Mailbox) tryInsert(msg evaluator.Object) error {
	if len(mb.queue) < mb.config.Capacity {
		mb.insertMessage(msg)
		return nil
	}

	newImportance := getImportance(msg)

	switch mb.config.Strategy {
	case "dropOld":
		minLvl := ImportanceScanInit
		dropIdx := -1
		for i, qm := range mb.queue {
			lvl := getImportance(qm)
			if lvl < minLvl {
				minLvl = lvl
				dropIdx = i
			}
		}
		if minLvl <= mb.config.DropOnFull {
			mb.queue = append(mb.queue[:dropIdx], mb.queue[dropIdx+1:]...)
			mb.insertMessage(msg)
			mb.metrics.DropsFullOld++
			return nil
		}
		mb.metrics.DropsImportance++
		return errors.New("mailbox full")

	case "dropNew":
		if newImportance <= mb.config.DropOnFull {
			mb.metrics.DropsFullNew++
			return errors.New("mailbox full")
		}
		minLvl := ImportanceScanInit
		dropIdx := -1
		for i, qm := range mb.queue {
			lvl := getImportance(qm)
			if lvl < minLvl {
				minLvl = lvl
				dropIdx = i
			}
		}
		if minLvl < newImportance {
			mb.queue = append(mb.queue[:dropIdx], mb.queue[dropIdx+1:]...)
			mb.insertMessage(msg)
			mb.metrics.DropsFullOld++
			return nil
		}
		mb.metrics.DropsImportance++
		return errors.New("mailbox full")

	case "blockOrError":
		fallthrough
	default:
		return errors.New("mailbox full")
	}
}

// Send attempts to send a message. Returns error if mailbox is full and strategy doesn't allow insertion.
func (mb *Mailbox) Send(msg evaluator.Object) error {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	if mb.closed {
		return errors.New("mailbox closed")
	}

	err := mb.tryInsert(msg)
	if err == nil {
		mb.broadcast()
	}
	return err
}

// SendWait attempts to send a message, blocking if full (only for blockOrError).
func (mb *Mailbox) SendWait(msg evaluator.Object, timeoutMs int, ctx context.Context) error {
	var timeout <-chan time.Time
	if timeoutMs > 0 {
		timeout = time.After(time.Duration(timeoutMs) * time.Millisecond)
	}
	start := time.Now()

	for {
		mb.mu.Lock()
		if mb.closed {
			mb.mu.Unlock()
			return errors.New("mailbox closed")
		}

		if mb.config.Strategy != "blockOrError" || len(mb.queue) < mb.config.Capacity {
			err := mb.tryInsert(msg)
			if err == nil {
				mb.broadcast()
			}
			mb.metrics.BlockDurationMsTotal += uint64(time.Since(start).Milliseconds())
			mb.metrics.BlockDurationMsCount++
			mb.mu.Unlock()
			return err
		}

		notifyCh := mb.notify
		mb.mu.Unlock()

		select {
		case <-notifyCh:
			// Queue changed, try again
		case <-timeout:
			mb.mu.Lock()
			mb.metrics.BlockDurationMsTotal += uint64(time.Since(start).Milliseconds())
			mb.metrics.BlockDurationMsCount++
			mb.mu.Unlock()
			return errors.New("mailbox full (timeout)")
		case <-ctx.Done():
			mb.mu.Lock()
			mb.metrics.BlockDurationMsTotal += uint64(time.Since(start).Milliseconds())
			mb.metrics.BlockDurationMsCount++
			mb.mu.Unlock()
			return errors.New("execution cancelled")
		}
	}
}

// Receive extracts the first message, non-blocking.
func (mb *Mailbox) Receive() (evaluator.Object, error) {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	if len(mb.queue) == 0 {
		if mb.closed {
			return nil, errors.New("mailbox closed")
		}
		return nil, errors.New("mailbox empty")
	}

	msg := mb.queue[0]
	mb.queue = mb.queue[1:]
	mb.metrics.MessagesReceived++
	mb.broadcast()
	return msg, nil
}

// ReceiveWait blocks until a message is available.
func (mb *Mailbox) ReceiveWait(timeoutMs int, ctx context.Context) (evaluator.Object, error) {
	var timeout <-chan time.Time
	if timeoutMs > 0 {
		timeout = time.After(time.Duration(timeoutMs) * time.Millisecond)
	}

	for {
		mb.mu.Lock()
		if len(mb.queue) > 0 {
			msg := mb.queue[0]
			mb.queue = mb.queue[1:]
			mb.metrics.MessagesReceived++
			mb.broadcast()
			mb.mu.Unlock()
			return msg, nil
		}

		if mb.closed {
			mb.mu.Unlock()
			return nil, errors.New("mailbox closed")
		}

		notifyCh := mb.notify
		mb.mu.Unlock()

		select {
		case <-notifyCh:
			// Queue changed, try again
		case <-timeout:
			return nil, errors.New("timeout")
		case <-ctx.Done():
			return nil, errors.New("execution cancelled")
		}
	}
}

func (mb *Mailbox) evalPredicate(e *evaluator.Evaluator, predicate evaluator.Object, msg evaluator.Object) bool {
	mb.mu.Unlock()
	defer mb.mu.Lock()
	res := e.ApplyFunction(predicate, []evaluator.Object{msg})
	if isErr(res) {
		return false
	}
	if b, ok := res.(*evaluator.Boolean); ok {
		return b.Value
	}
	return false
}

func isErr(obj evaluator.Object) bool {
	_, ok := obj.(*evaluator.Error)
	return ok
}

// ReceiveBy extracts the first message satisfying the predicate, non-blocking.
func (mb *Mailbox) ReceiveBy(e *evaluator.Evaluator, predicate evaluator.Object) (evaluator.Object, error) {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	for i, msg := range mb.queue {
		mb.metrics.SelectiveScanDepthTotal++
		mb.metrics.SelectiveScanDepthCount++
		if mb.evalPredicate(e, predicate, msg) {
			mb.queue = append(mb.queue[:i], mb.queue[i+1:]...)
			mb.metrics.MessagesReceived++
			mb.metrics.SkippedMessages += uint64(i)
			mb.broadcast()
			return msg, nil
		}
	}

	if mb.closed {
		return nil, errors.New("mailbox closed")
	}
	return nil, errors.New("no matching message")
}

// ReceiveByWait blocks until a message satisfying the predicate is available.
func (mb *Mailbox) ReceiveByWait(e *evaluator.Evaluator, predicate evaluator.Object, timeoutMs int, ctx context.Context) (evaluator.Object, error) {
	var timeout <-chan time.Time
	if timeoutMs > 0 {
		timeout = time.After(time.Duration(timeoutMs) * time.Millisecond)
	}

	for {
		mb.mu.Lock()
		for i, msg := range mb.queue {
			mb.metrics.SelectiveScanDepthTotal++
			mb.metrics.SelectiveScanDepthCount++
			if mb.evalPredicate(e, predicate, msg) {
				mb.queue = append(mb.queue[:i], mb.queue[i+1:]...)
				mb.metrics.MessagesReceived++
				mb.metrics.SkippedMessages += uint64(i)
				mb.broadcast()
				mb.mu.Unlock()
				return msg, nil
			}
		}

		if mb.closed {
			mb.mu.Unlock()
			return nil, errors.New("mailbox closed")
		}

		notifyCh := mb.notify
		mb.mu.Unlock()

		select {
		case <-notifyCh:
			// Queue changed, try again
		case <-timeout:
			return nil, errors.New("timeout")
		case <-ctx.Done():
			return nil, errors.New("execution cancelled")
		}
	}
}

// Peek returns the first message without extracting it.
func (mb *Mailbox) Peek() (evaluator.Object, error) {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	if len(mb.queue) == 0 {
		if mb.closed {
			return nil, errors.New("mailbox closed")
		}
		return nil, errors.New("mailbox empty")
	}

	return mb.queue[0], nil
}

// PeekBy returns the first message satisfying the predicate without extracting it.
func (mb *Mailbox) PeekBy(e *evaluator.Evaluator, predicate evaluator.Object) (evaluator.Object, error) {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	for _, msg := range mb.queue {
		if mb.evalPredicate(e, predicate, msg) {
			return msg, nil
		}
	}

	if mb.closed {
		return nil, errors.New("mailbox closed")
	}
	return nil, errors.New("no matching message")
}

// DeepCopy clones a message to ensure isolation between VMs.
// DeepCopy ensures that the message is safe to pass between VMs.
// Since all standard collections (Records, Lists, Maps) are strictly immutable
// in Funxy, we only need to verify that the object contains no closures or HostObjects.
// If it's a pure data object, we can safely return it without actual copying (Zero-Copy Fast Path).
func DeepCopy(obj evaluator.Object) (evaluator.Object, error) {
	if err := evaluator.CheckSerializable(obj); err != nil {
		return nil, fmt.Errorf("cannot pass mutable or non-serializable object via mailbox: %v", err)
	}
	// Zero-copy: object is immutable, safe to share directly!
	return obj, nil
}
