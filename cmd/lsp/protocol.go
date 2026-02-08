package main

// LSP Message structures
type RequestMessage struct {
	Jsonrpc string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type ResponseMessage struct {
	Jsonrpc string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	// Result must be present (even if null) on success. Error must be present on error.
	Result interface{} `json:"result"` // Removing omitempty forces "result": null which is valid for success-with-no-data
	Error  *Error      `json:"error,omitempty"`
}

type NotificationMessage struct {
	Jsonrpc string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type Error struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// LSP specific types
type InitializeParams struct {
	ProcessID    *int               `json:"processId,omitempty"`
	RootURI      *string            `json:"rootUri,omitempty"`
	RootPath     *string            `json:"rootPath,omitempty"`
	Capabilities ClientCapabilities `json:"capabilities"`
}

type ClientCapabilities struct {
	TextDocument *TextDocumentClientCapabilities `json:"textDocument,omitempty"`
}

type TextDocumentClientCapabilities struct {
	Synchronization *SynchronizationCapabilities `json:"synchronization,omitempty"`
}

type SynchronizationCapabilities struct {
	DidSave           bool `json:"didSave"`
	WillSave          bool `json:"willSave"`
	WillSaveWaitUntil bool `json:"willSaveWaitUntil"`
}

type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
}

type ServerCapabilities struct {
	TextDocumentSync           int                `json:"textDocumentSync"`
	HoverProvider              bool               `json:"hoverProvider"`
	DefinitionProvider         bool               `json:"definitionProvider"`
	CompletionProvider         *CompletionOptions `json:"completionProvider,omitempty"`
	DocumentFormattingProvider bool               `json:"documentFormattingProvider"`
}

type CompletionOptions struct {
	ResolveProvider   bool     `json:"resolveProvider"`
	TriggerCharacters []string `json:"triggerCharacters"`
}

// TextDocument synchronization
type DidOpenTextDocumentParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

type DidChangeTextDocumentParams struct {
	TextDocument   VersionedTextDocumentIdentifier  `json:"textDocument"`
	ContentChanges []TextDocumentContentChangeEvent `json:"contentChanges"`
}

type DidCloseTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type TextDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

type VersionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

type TextDocumentContentChangeEvent struct {
	Range       *Range `json:"range,omitempty"`
	RangeLength *int   `json:"rangeLength,omitempty"`
	Text        string `json:"text"`
}

// PublishDiagnostics
type PublishDiagnosticsParams struct {
	URI         string       `json:"uri"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

type Diagnostic struct {
	Range    Range              `json:"range"`
	Severity DiagnosticSeverity `json:"severity"`
	Code     interface{}        `json:"code,omitempty"`
	Message  string             `json:"message"`
	Source   string             `json:"source"`
}

type DiagnosticSeverity int

const (
	SeverityError   DiagnosticSeverity = 1
	SeverityWarning DiagnosticSeverity = 2
	SeverityInfo    DiagnosticSeverity = 3
	SeverityHint    DiagnosticSeverity = 4
)

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Hover request
type HoverParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

type Hover struct {
	Contents MarkupContent `json:"contents"`
	Range    *Range        `json:"range,omitempty"`
}

type MarkupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

// Definition request
type DefinitionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// Completion request
type CompletionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
	Context      *CompletionContext     `json:"context,omitempty"`
}

type CompletionContext struct {
	TriggerKind      CompletionTriggerKind `json:"triggerKind"`
	TriggerCharacter *string               `json:"triggerCharacter,omitempty"`
}

type CompletionTriggerKind int

const (
	TriggerKindInvoked                         CompletionTriggerKind = 1
	TriggerKindTriggerCharacter                CompletionTriggerKind = 2
	TriggerKindTriggerForIncompleteCompletions CompletionTriggerKind = 3
)

type CompletionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []CompletionItem `json:"items"`
}

type CompletionItem struct {
	Label         string             `json:"label"`
	Kind          CompletionItemKind `json:"kind,omitempty"`
	Detail        string             `json:"detail,omitempty"`
	Documentation *MarkupContent     `json:"documentation,omitempty"`
}

// Formatting request
type DocumentFormattingParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Options      FormattingOptions      `json:"options"`
}

type FormattingOptions struct {
	TabSize      int  `json:"tabSize"`
	InsertSpaces bool `json:"insertSpaces"`
}

type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

type CompletionItemKind int

const (
	CompletionItemText          CompletionItemKind = 1
	CompletionItemMethod        CompletionItemKind = 2
	CompletionItemFunction      CompletionItemKind = 3
	CompletionItemConstructor   CompletionItemKind = 4
	CompletionItemField         CompletionItemKind = 5
	CompletionItemVariable      CompletionItemKind = 6
	CompletionItemClass         CompletionItemKind = 7
	CompletionItemInterface     CompletionItemKind = 8
	CompletionItemModule        CompletionItemKind = 9
	CompletionItemProperty      CompletionItemKind = 10
	CompletionItemUnit          CompletionItemKind = 11
	CompletionItemValue         CompletionItemKind = 12
	CompletionItemEnum          CompletionItemKind = 13
	CompletionItemKeyword       CompletionItemKind = 14
	CompletionItemSnippet       CompletionItemKind = 15
	CompletionItemColor         CompletionItemKind = 16
	CompletionItemFile          CompletionItemKind = 17
	CompletionItemReference     CompletionItemKind = 18
	CompletionItemFolder        CompletionItemKind = 19
	CompletionItemEnumMember    CompletionItemKind = 20
	CompletionItemConstant      CompletionItemKind = 21
	CompletionItemStruct        CompletionItemKind = 22
	CompletionItemEvent         CompletionItemKind = 23
	CompletionItemOperator      CompletionItemKind = 24
	CompletionItemTypeParameter CompletionItemKind = 25
)
