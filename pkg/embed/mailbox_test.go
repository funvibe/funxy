package funxy

import (
	"github.com/funvibe/funxy/internal/evaluator"
	"testing"
)

func mailboxMsgWithImportance(level int, label string) evaluator.Object {
	return evaluator.NewRecord(map[string]evaluator.Object{
		"importance": &evaluator.Integer{Value: int64(level)},
		"type":       evaluator.StringToList(label),
		"payload":    evaluator.StringToList(label),
	})
}

func TestMailbox_SystemPriorityAndDropOld(t *testing.T) {
	mb := NewMailbox(MailboxConfig{
		Capacity:   3,
		DropOnFull: 2,
		Strategy:   "dropOld",
	})

	low1 := mailboxMsgWithImportance(1, "low1")
	system := mailboxMsgWithImportance(SystemImportanceLevel, "system")
	info := mailboxMsgWithImportance(2, "info")
	low2 := mailboxMsgWithImportance(1, "low2")

	if err := mb.Send(low1); err != nil {
		t.Fatalf("send low1: %v", err)
	}
	if err := mb.Send(system); err != nil {
		t.Fatalf("send system: %v", err)
	}
	if err := mb.Send(info); err != nil {
		t.Fatalf("send info: %v", err)
	}
	if err := mb.Send(low2); err != nil {
		t.Fatalf("send low2 (dropOld): %v", err)
	}

	msg, err := mb.Receive()
	if err != nil {
		t.Fatalf("receive #1: %v", err)
	}
	if got := getImportance(msg); got != SystemImportanceLevel {
		t.Fatalf("first message must be system, got importance=%d", got)
	}

	msg, err = mb.Receive()
	if err != nil {
		t.Fatalf("receive #2: %v", err)
	}
	if got := getImportance(msg); got != 2 {
		t.Fatalf("second message must be info(2), got=%d", got)
	}

	msg, err = mb.Receive()
	if err != nil {
		t.Fatalf("receive #3: %v", err)
	}
	if got := getImportance(msg); got != 1 {
		t.Fatalf("third message must be low(1), got=%d", got)
	}
}
