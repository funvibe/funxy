package funxy

import (
	"fmt"
	"github.com/funvibe/funxy/internal/evaluator"
	"github.com/funvibe/funxy/internal/typesystem"
	"unsafe"
)

// VMEntryObject wraps VMEntry to implement evaluator.Object
type VMEntryObject struct {
	Entry *VMEntry
}

func (o *VMEntryObject) Type() evaluator.ObjectType { return "VM_ENTRY" }
func (o *VMEntryObject) Inspect() string            { return fmt.Sprintf("<vm-entry %p>", o.Entry) }
func (o *VMEntryObject) RuntimeType() typesystem.Type {
	return typesystem.TCon{Name: "VMEntry"}
}
func (o *VMEntryObject) Hash() uint32 {
	return uint32(uintptr(unsafe.Pointer(o.Entry)))
}

// MailboxObject wraps Mailbox to implement evaluator.Object
type MailboxObject struct {
	Mailbox *Mailbox
}

func (o *MailboxObject) Type() evaluator.ObjectType { return "MAILBOX" }
func (o *MailboxObject) Inspect() string            { return fmt.Sprintf("<mailbox %p>", o.Mailbox) }
func (o *MailboxObject) RuntimeType() typesystem.Type {
	return typesystem.TCon{Name: "Mailbox"}
}
func (o *MailboxObject) Hash() uint32 {
	return uint32(uintptr(unsafe.Pointer(o.Mailbox)))
}
