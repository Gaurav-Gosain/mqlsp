package parser

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/adrg/xdg"
)

func keyvals(m map[string]string) []interface{} {
	var keyvals []interface{}
	for k, v := range m {
		keyvals = append(keyvals, k, v)
	}
	return keyvals
}

func DecodeUTF16(b []byte) (string, error) {
	if len(b)%2 != 0 {
		return "", fmt.Errorf("must have even length byte slice")
	}

	u16s := make([]uint16, 1)

	ret := &bytes.Buffer{}

	b8buf := make([]byte, 4)

	lb := len(b)
	for i := 0; i < lb; i += 2 {
		u16s[0] = uint16(b[i]) + (uint16(b[i+1]) << 8)
		r := utf16.Decode(u16s)
		n := utf8.EncodeRune(b8buf, r[0])
		ret.Write(b8buf[:n])
	}

	return ret.String(), nil
}

func compile(target, logfile string, logger *log.Logger) (outputStr string, status int) {
	compileScript := filepath.Join(xdg.DataHome, "mqlsp", "compile.sh")

	// check if the compile script exists
	if _, err := os.Stat(compileScript); os.IsNotExist(err) {
		scriptContent := `#!/bin/bash

# Compile the MetaTrader 4 script

# Get the script name
SCRIPT_NAME=$1

# Get the log file name
LOG_FILE_NAME=$2

# Compile the script
eval $METAEDITOR_PATH /compile:"$SCRIPT_NAME" /log:"$LOG_FILE_NAME" /s`

		if err := os.WriteFile(compileScript, []byte(scriptContent), 0755); err != nil {
			panic(err)
		}
	}

	logger.Printf("target: %s", target)
	logger.Printf("logfile: %s", logfile)

	cmd := exec.Command(compileScript, target, logfile)

	logger.Printf("metaeditor command: %s", cmd.String())

	// check the status of the command
	cmd.Run()

	// read the log file
	logFile, err := os.ReadFile(logfile)
	if err != nil {
		fmt.Println("failed to read log file")
		return "", 1
	}

	logFileUTF8, err := DecodeUTF16(logFile)
	if err != nil {
		return "", 1
	}

	logger.Println("hey, I compiled!")

	return logFileUTF8, 0
}

type Diagnostic struct {
	ScriptName string
	Type       string
	Message    string
	FileName   string
	Line       int
	Char       int
	Code       int
}

func Parse(target string, logger *log.Logger) (diagnostics []Diagnostic, err error) {
	// strip the file:// from the target
	target = strings.Replace(target, "file://", "", 1)

	// remove getwd from the target
	pwd, _ := os.Getwd()
	target = strings.Replace(target, pwd, "", 1)

	// remove "/" prefix
	target = strings.TrimPrefix(target, "/")

	// replace .mq4 with .log
	logfile := filepath.Join(
		xdg.DataHome,
		"mqlsp",
		"lsp.log",
	)

	if _, err := os.Stat(logfile); os.IsNotExist(err) {
		// create the chat history file
		file, err := os.Create(logfile)
		if err != nil {
			panic(err)
		}
		file.Close() //nolint:errcheck
	}

	outputStr, msg := compile(target, logfile, logger)

	if msg != 0 {
		return nil, fmt.Errorf("failed to compile %s", target)
	}

	scanner := bufio.NewScanner(strings.NewReader(outputStr))
	for scanner.Scan() {
		line := scanner.Text()

		// if empty line, skip
		if strings.TrimSpace(line) == "" {
			continue
		}
		info := Diagnostic{}
		if !strings.Contains(line, "information:") {
			re := regexp.MustCompile(`^(.*)\((\d+),(\d+)\) : (\w+) (\d+): (.*)$`)
			matches := re.FindStringSubmatch(line)
			if len(matches) == 7 {
				info.ScriptName = matches[1]
				if info.ScriptName != target {
					continue
				}
				fmt.Sscanf(matches[2], "%d", &info.Line)
				info.Line = max(info.Line-1, 0)
				fmt.Sscanf(matches[3], "%d", &info.Char)
				info.Char = max(info.Char-1, 0)
				info.Type = matches[4]
				fmt.Sscanf(matches[5], "%d", &info.Code)
				info.Message = matches[6]
				diagnostics = append(diagnostics, info)
			}
		}
	}

	return diagnostics, nil
}
