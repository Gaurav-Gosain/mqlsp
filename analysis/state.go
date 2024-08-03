package analysis

import (
	"log"

	"github.com/gaurav-gosain/mqlsp/lsp"
	"github.com/gaurav-gosain/mqlsp/parser"
)

type State struct {
	// Map of file names to contents
	Documents map[string]string
}

func NewState() State {
	return State{Documents: map[string]string{}}
}

func getDiagnosticsForFile(uri, text string, logger *log.Logger) []lsp.Diagnostic {
	mqlDiagnostics, err := parser.Parse(uri, logger)
	if err != nil {
		return []lsp.Diagnostic{}
	}

	diagnostics := []lsp.Diagnostic{}

	for _, diag := range mqlDiagnostics {

		severity := 1
		if diag.Type == "warning" {
			severity = 2
		}

		diagnostics = append(diagnostics, lsp.Diagnostic{
			Range:    LineRange(diag.Line, diag.Char, diag.Char+len(diag.Message)),
			Severity: severity,
			Source:   "MQLSP",
			Message:  diag.Message,
		})
	}

	return diagnostics
}

func (s *State) OpenDocument(uri, text string, logger *log.Logger) []lsp.Diagnostic {
	s.Documents[uri] = text

	return getDiagnosticsForFile(uri, text, logger)
}

func (s *State) UpdateDocument(uri, text string, logger *log.Logger) []lsp.Diagnostic {
	s.Documents[uri] = text

	return getDiagnosticsForFile(uri, text, logger)
}

func LineRange(line, start, end int) lsp.Range {
	return lsp.Range{
		Start: lsp.Position{
			Line:      line,
			Character: start,
		},
		End: lsp.Position{
			Line:      line,
			Character: end,
		},
	}
}
