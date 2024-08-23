package analysis

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"

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

func (s *State) TextDocumentCodeAction(id int, uri string, lineNumber int) lsp.TextDocumentCodeActionResponse {
	text := s.Documents[uri]

	actions := []lsp.CodeAction{}

	lines := strings.Split(text, "\n")

	line := lines[lineNumber]

	if hasArguments(line) {
		// split the arguments into separate lines
		argsRaw := line[strings.Index(line, "(")+1:]

		// remove the closing parenthesis
		argsRaw = argsRaw[:strings.Index(argsRaw, ")")]

		args := strings.Split(argsRaw, ",")
		actions = append(actions, lsp.CodeAction{
			Title: "Split arguments into separate lines",
			Edit: &lsp.WorkspaceEdit{
				Changes: map[string][]lsp.TextEdit{
					uri: {
						{
							Range:   LineRange(lineNumber, strings.Index(line, "(")+1, strings.Index(line, "(")+1+len(strings.Join(args, ","))),
							NewText: "\n" + strings.Join(args, ",\n") + "\n",
						},
					},
				},
			},
		})
	}

	if isStringWithSpaces(line) {
		// split the arguments into separate lines
		argsRaw := line[strings.Index(line, `"`)+1:]

		// remove the closing quotation mark
		argsRaw = argsRaw[:strings.Index(argsRaw, `"`)]

		args := strings.Split(argsRaw, " ")

		finalString := ""
		for _, arg := range args {
			finalString += `"` + arg + `"` + "\n"
		}

		actions = append(actions, lsp.CodeAction{
			Title: "Split string into multiple lines",
			Edit: &lsp.WorkspaceEdit{
				Changes: map[string][]lsp.TextEdit{
					uri: {
						{
							Range:   LineRange(lineNumber, strings.Index(line, `"`), strings.Index(line, `"`)+2+len(strings.Join(args, ","))),
							NewText: finalString,
						},
					},
				},
			},
		})
	}

	// check if the line is the start of a multiline comment
	if strings.HasPrefix(line, "/*") {
		lastLine := lineNumber
		// get the whole comment
		comment := lines[lineNumber+1]
		for i := lineNumber + 2; i < len(lines); i++ {
			comment += lines[i] + "\n"
			lastLine++
			if strings.Contains(lines[i], "*/") {
				lastLine++
				break
			}
		}

		// remove the */ from the end
		comment = comment[:strings.Index(comment, "*/")]

		actions = append(actions, lsp.CodeAction{
			Title: "Convert JSON to Struct",
			Edit: &lsp.WorkspaceEdit{
				Changes: map[string][]lsp.TextEdit{
					uri: {
						{
							Range:   LineRange(lastLine, strings.Index(lines[lastLine], "*/"), len(lines[lastLine])),
							NewText: "*/\n\n" + convertJSONToStruct(comment),
						},
					},
				},
			},
		})

	}

	response := lsp.TextDocumentCodeActionResponse{
		Response: lsp.Response{
			RPC: "2.0",
			ID:  &id,
		},
		Result: actions,
	}

	return response
}

func isStringWithSpaces(line string) bool {
	// check if the line has a string with spaces
	re := regexp.MustCompile(`".*\s.*"`)
	return re.MatchString(line)
}

func hasArguments(line string) bool {
	re := regexp.MustCompile(`.*\w+\s*\(.*,.*\)\s*\{?`)
	return re.MatchString(line)
}

func convertJSONToStruct(line string) string {
	// parse the line into a JSON object
	jsonObj := map[string]interface{}{}
	err := json.Unmarshal([]byte(line), &jsonObj)
	if err != nil {
		return line
	}

	// convert the JSON object to a struct
	structObj := structToString(jsonObj)

	return structObj
}

func structToString(obj map[string]interface{}) string {
	// extract the keys and determine the type of each key
	keys := []string{}
	for key := range obj {
		keys = append(keys, key)
	}

	types := map[string]string{}

	for _, key := range keys {
		switch obj[key].(type) {
		case string:
			types[key] = "string"
		case float64:
			if strings.Contains(fmt.Sprint(obj[key]), ".") {
				types[key] = "double"
			} else {
				types[key] = "int"
			}
		case bool:
			types[key] = "bool"
		case map[string]interface{}:
			types[key] = "map[string]interface{}"
		case []interface{}:
			types[key] = "interface{}"
			// figure out the type of the elements in the array
			for _, element := range obj[key].([]interface{}) {
				switch element.(type) {
				case string:
					types[key] = "string[]"
				case float64:
					if strings.Contains(fmt.Sprint(element), ".") {
						types[key] = "double[]"
					} else {
						types[key] = "int[]"
					}
				case bool:
					types[key] = "bool[]"
				}
			}
		}
	}

	var structStr string

	structStr += "struct change_me {\n"

	for key, value := range types {
		if strings.HasSuffix(value, "[]") {
			value = value[:len(value)-2]
			structStr += strings.Repeat(" ", 4) + value + " " + key + "[];\n"
		} else {
			structStr += strings.Repeat(" ", 4) + value + " " + key + ";\n"
		}
	}

	structStr += "};"

	return structStr
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

func MultiLineRange(startLine, startChar, endLine, endChar int) lsp.Range {
	return lsp.Range{
		Start: lsp.Position{
			Line:      startLine,
			Character: startChar,
		},
		End: lsp.Position{
			Line:      endLine,
			Character: endChar,
		},
	}
}
