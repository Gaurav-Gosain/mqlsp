package main

import (
	"bufio"
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
	"github.com/gaurav-gosain/mqlsp/analysis"
	"github.com/gaurav-gosain/mqlsp/lsp"
	"github.com/gaurav-gosain/mqlsp/rpc"
)

func main() {
	// create the mqlsp directory if it doesn't exist
	if _, err := os.Stat(filepath.Join(xdg.DataHome, "mqlsp")); os.IsNotExist(err) {
		os.Mkdir(filepath.Join(xdg.DataHome, "mqlsp"), 0755)
	}

	logfile := filepath.Join(
		xdg.DataHome,
		"mqlsp",
		"mqlsp.log",
	)

	if _, err := os.Stat(logfile); os.IsNotExist(err) {
		// create the chat history file
		file, err := os.Create(logfile)
		if err != nil {
			panic(err)
		}
		file.Close() //nolint:errcheck
	}

	logger := getLogger(logfile)
	logger.Println("Hey, I started!")

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Split(rpc.Split)

	state := analysis.NewState()
	writer := os.Stdout

	for scanner.Scan() {
		msg := scanner.Bytes()
		method, contents, err := rpc.DecodeMessage(msg)
		if err != nil {
			logger.Printf("Got an error: %s", err)
			continue
		}

		handleMessage(logger, writer, state, method, contents)
	}
}

func handleMessage(logger *log.Logger, writer io.Writer, state analysis.State, method string, contents []byte) {
	logger.Printf("Received msg with method: %s", method)

	switch method {
	case "initialize":
		var request lsp.InitializeRequest
		if err := json.Unmarshal(contents, &request); err != nil {
			logger.Printf("Hey, we couldn't parse this: %s", err)
		}

		logger.Printf("Connected to: %s %s",
			request.Params.ClientInfo.Name,
			request.Params.ClientInfo.Version)

		// hey... let's reply!
		msg := lsp.NewInitializeResponse(request.ID)
		writeResponse(writer, msg)

		logger.Print("Sent the reply")
	case "textDocument/didOpen":
		var request lsp.DidOpenTextDocumentNotification
		if err := json.Unmarshal(contents, &request); err != nil {
			logger.Printf("textDocument/didOpen: %s", err)
			return
		}

		logger.Printf("Opened: %s", request.Params.TextDocument.URI)
		diagnostics := state.OpenDocument(request.Params.TextDocument.URI, request.Params.TextDocument.Text, logger)
		writeResponse(writer, lsp.PublishDiagnosticsNotification{
			Notification: lsp.Notification{
				RPC:    "2.0",
				Method: "textDocument/publishDiagnostics",
			},
			Params: lsp.PublishDiagnosticsParams{
				URI:         request.Params.TextDocument.URI,
				Diagnostics: diagnostics,
			},
		})
	case "textDocument/didSave":
		var request lsp.TextDocumentDidChangeNotification
		if err := json.Unmarshal(contents, &request); err != nil {
			logger.Printf("textDocument/didChange: %s", err)
			return
		}

		logger.Printf("Changed: %s", request.Params.TextDocument.URI)
		for _, change := range request.Params.ContentChanges {
			diagnostics := state.UpdateDocument(request.Params.TextDocument.URI, change.Text, logger)
			writeResponse(writer, lsp.PublishDiagnosticsNotification{
				Notification: lsp.Notification{
					RPC:    "2.0",
					Method: "textDocument/publishDiagnostics",
				},
				Params: lsp.PublishDiagnosticsParams{
					URI:         request.Params.TextDocument.URI,
					Diagnostics: diagnostics,
				},
			})
		}
	case "textDocument/didChange":
		var request lsp.TextDocumentDidChangeNotification
		if err := json.Unmarshal(contents, &request); err != nil {
			logger.Printf("textDocument/didChange: %s", err)
			return
		}

		logger.Printf("Changed: %s", request.Params.TextDocument.URI)
		for _, change := range request.Params.ContentChanges {
			diagnostics := state.UpdateDocument(request.Params.TextDocument.URI, change.Text, logger)
			writeResponse(writer, lsp.PublishDiagnosticsNotification{
				Notification: lsp.Notification{
					RPC:    "2.0",
					Method: "textDocument/publishDiagnostics",
				},
				Params: lsp.PublishDiagnosticsParams{
					URI:         request.Params.TextDocument.URI,
					Diagnostics: diagnostics,
				},
			})
		}
	}
}

func writeResponse(writer io.Writer, msg any) {
	reply := rpc.EncodeMessage(msg)
	writer.Write([]byte(reply))
}

func getLogger(filename string) *log.Logger {
	logfile, err := os.OpenFile(filename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
	if err != nil {
		panic("hey, you didnt give me a good file")
	}

	return log.New(logfile, "[mqlsp]", log.Ldate|log.Ltime|log.Lshortfile)
}
