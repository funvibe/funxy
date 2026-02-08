package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
)

// Language Server implementation
type LanguageServer struct {
	documents map[string]*DocumentState // URI -> document state
	mu        sync.RWMutex              // Mutex to protect the documents map
	writer    io.Writer                 // Output stream for JSON-RPC responses
	rootPath  string                    // Workspace root for resolving imports
}

func NewLanguageServer(writer io.Writer) *LanguageServer {
	if writer == nil {
		writer = os.Stdout
	}
	return &LanguageServer{
		documents: make(map[string]*DocumentState),
		writer:    writer,
	}
}

func (s *LanguageServer) Start() {
	// Use a bufio.Reader instead of Scanner to handle arbitrary buffer sizes and raw reads
	reader := bufio.NewReader(os.Stdin)

	for {
		// Read header line
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				log.Printf("Error reading header: %v", err)
			}
			break
		}

		// Remove trailing CR/LF
		line = strings.TrimRight(line, "\r\n")

		if line == "" {
			continue // Skip empty lines between messages or before headers
		}

		if strings.HasPrefix(line, "Content-Length: ") {
			contentLengthStr := strings.TrimPrefix(line, "Content-Length: ")
			contentLength, err := strconv.Atoi(contentLengthStr)
			if err != nil {
				log.Printf("Error parsing Content-Length: %v", err)
				continue
			}

			// The next line must be an empty line (\r\n)
			// Read until we find the empty line separator
			for {
				emptyLine, err := reader.ReadString('\n')
				if err != nil {
					log.Printf("Error reading separator: %v", err)
					return
				}
				emptyLine = strings.TrimRight(emptyLine, "\r\n")
				if emptyLine == "" {
					break
				}
			}

			// Read content
			content := make([]byte, contentLength)
			_, err = io.ReadFull(reader, content)
			if err != nil {
				log.Printf("Error reading content: %v", err)
				break // Should verify if we can recover or just exit
			}

			// Process message
			if err := s.handleMessage(content); err != nil {
				log.Printf("Error handling message: %v", err)
			}
		}
	}
}

func (s *LanguageServer) handleMessage(content []byte) error {
	log.Printf("Received message: %s", string(content))

	var baseMessage struct {
		Jsonrpc string      `json:"jsonrpc"`
		ID      interface{} `json:"id,omitempty"`
		Method  string      `json:"method"`
		Params  interface{} `json:"params,omitempty"`
	}

	if err := json.Unmarshal(content, &baseMessage); err != nil {
		log.Printf("Failed to unmarshal message: %v", err)
		return fmt.Errorf("failed to unmarshal message: %v", err)
	}

	log.Printf("Parsed message - Method: %s, ID: %v", baseMessage.Method, baseMessage.ID)

	// Check if this is a request (has ID) or notification (no ID)
	if baseMessage.ID != nil {
		return s.handleRequest(baseMessage, content)
	} else {
		return s.handleNotification(baseMessage, content)
	}
}

func (s *LanguageServer) handleRequest(baseMessage struct {
	Jsonrpc string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}, content []byte) error {

	switch baseMessage.Method {
	case "initialize":
		var params InitializeParams
		if err := json.Unmarshal(content, &RequestMessage{Params: &params}); err != nil {
			return err
		}
		return s.handleInitialize(baseMessage.ID, params)

	case "shutdown":
		return s.handleShutdown(baseMessage.ID)

	case "textDocument/hover":
		var params HoverParams
		if err := json.Unmarshal(content, &RequestMessage{Params: &params}); err != nil {
			return err
		}
		return s.handleHover(baseMessage.ID, params)

	case "textDocument/definition":
		var params DefinitionParams
		if err := json.Unmarshal(content, &RequestMessage{Params: &params}); err != nil {
			return err
		}
		return s.handleDefinition(baseMessage.ID, params)

	case "textDocument/completion":
		var params CompletionParams
		if err := json.Unmarshal(content, &RequestMessage{Params: &params}); err != nil {
			return err
		}
		return s.handleCompletion(baseMessage.ID, params)

	case "textDocument/formatting":
		// Formatting is currently disabled
		response := ResponseMessage{
			Jsonrpc: "2.0",
			ID:      baseMessage.ID,
			Error: &Error{
				Code:    -32601,
				Message: fmt.Sprintf("Method not found: %s", baseMessage.Method),
			},
		}
		return s.sendResponse(response)

	default:
		// Method not implemented
		response := ResponseMessage{
			Jsonrpc: "2.0",
			ID:      baseMessage.ID,
			Error: &Error{
				Code:    -32601,
				Message: fmt.Sprintf("Method not found: %s", baseMessage.Method),
			},
		}
		return s.sendResponse(response)
	}
}

func (s *LanguageServer) handleNotification(baseMessage struct {
	Jsonrpc string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}, content []byte) error {

	switch baseMessage.Method {
	case "initialized":
		// Client has finished initialization
		return nil

	case "textDocument/didOpen":
		var params DidOpenTextDocumentParams
		if err := json.Unmarshal(content, &NotificationMessage{Params: &params}); err != nil {
			return err
		}
		return s.handleDidOpen(params)

	case "textDocument/didChange":
		var params DidChangeTextDocumentParams
		if err := json.Unmarshal(content, &NotificationMessage{Params: &params}); err != nil {
			return err
		}
		return s.handleDidChange(params)

	case "textDocument/didClose":
		var params DidCloseTextDocumentParams
		if err := json.Unmarshal(content, &NotificationMessage{Params: &params}); err != nil {
			return err
		}
		return s.handleDidClose(params)

	case "exit":
		os.Exit(0)
		return nil

	default:
		// Unknown notification, ignore
		return nil
	}
}

func (s *LanguageServer) sendResponse(response ResponseMessage) error {
	return s.sendMessage(response)
}

func (s *LanguageServer) sendNotification(notification NotificationMessage) error {
	return s.sendMessage(notification)
}

func (s *LanguageServer) sendMessage(message interface{}) error {
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}

	content := string(data)
	msg := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(content), content)

	_, err = fmt.Fprint(s.writer, msg)
	return err
}
