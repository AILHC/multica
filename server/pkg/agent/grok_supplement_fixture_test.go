package agent

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

const grokSupplementHelperModeEnv = "MULTICA_GROK_SUPPLEMENT_HELPER"

func grokSupplementFixture(t *testing.T, env map[string]string) (string, map[string]string) {
	t.Helper()
	fixtureEnv := make(map[string]string, len(env)+1)
	for key, value := range env {
		fixtureEnv[key] = value
	}
	if runtime.GOOS != "windows" {
		path := filepath.Join(t.TempDir(), "grok")
		writeTestExecutable(t, path, []byte(fakeGrokACPScript()))
		return path, fixtureEnv
	}

	path, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve test executable: %v", err)
	}
	fixtureEnv[grokSupplementHelperModeEnv] = "1"
	return path, fixtureEnv
}

func runFakeGrokSupplementHelper() error {
	var record *os.File
	if path := os.Getenv("GROK_REQUESTS_FILE"); path != "" {
		var err error
		record, err = os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return fmt.Errorf("open request record: %w", err)
		}
		defer record.Close()
	}

	out := bufio.NewWriter(os.Stdout)
	flush := func(format string, args ...any) error {
		if _, err := fmt.Fprintf(out, format+"\n", args...); err != nil {
			return err
		}
		return out.Flush()
	}

	var promptID json.RawMessage
	ignoredFirstInterject := false
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := append([]byte(nil), scanner.Bytes()...)
		if record != nil {
			if _, err := fmt.Fprintln(record, string(line)); err != nil {
				return fmt.Errorf("record request: %w", err)
			}
		}
		var request struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if err := json.Unmarshal(line, &request); err != nil {
			return fmt.Errorf("decode request: %w", err)
		}

		switch request.Method {
		case "initialize":
			if err := flush(`{"jsonrpc":"2.0","id":%s,"result":{"protocolVersion":1,"authMethods":[{"id":"cached_token","name":"Cached login"}],"agentCapabilities":{"loadSession":true}}}`, request.ID); err != nil {
				return err
			}
		case "authenticate":
			if err := flush(`{"jsonrpc":"2.0","id":%s,"result":{}}`, request.ID); err != nil {
				return err
			}
		case "session/new":
			if err := flush(`{"jsonrpc":"2.0","id":%s,"result":{"sessionId":"ses_new","models":{"availableModels":[{"modelId":"grok-4.6","name":"Grok 4.6"}],"currentModelId":"grok-4.6"}}}`, request.ID); err != nil {
				return err
			}
		case "session/prompt":
			promptID = append(promptID[:0], request.ID...)
			if os.Getenv("GROK_NO_OUTPUT_BEFORE_INTERJECT") == "" {
				if err := flush(`{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"ses_new","update":{"sessionUpdate":"agent_thought_chunk","content":{"type":"text","text":"working"}}}}`); err != nil {
					return err
				}
			}
		case "_x.ai/interject":
			if os.Getenv("GROK_INTERJECT_NO_FIRST_RESPONSE") != "" && !ignoredFirstInterject {
				ignoredFirstInterject = true
				continue
			}
			switch {
			case os.Getenv("GROK_INTERJECT_UNSUPPORTED") != "":
				if err := flush(`{"jsonrpc":"2.0","id":%s,"error":{"code":-32601,"message":"method not found"}}`, request.ID); err != nil {
					return err
				}
			case os.Getenv("GROK_INTERJECT_EXTENSION_ERROR") != "":
				if err := flush(`{"jsonrpc":"2.0","id":%s,"result":{"result":null,"error":"delivery failed"}}`, request.ID); err != nil {
					return err
				}
			default:
				status := os.Getenv("GROK_INTERJECT_NESTED_STATUS")
				if status == "" {
					status = "queued"
				}
				if err := flush(`{"jsonrpc":"2.0","id":%s,"result":{"result":{"status":%q}}}`, request.ID, status); err != nil {
					return err
				}
			}
			if os.Getenv("GROK_NO_OUTPUT_BEFORE_INTERJECT") != "" {
				if err := flush(`{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"ses_new","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"111"}}}}`); err != nil {
					return err
				}
			}
			if len(promptID) == 0 {
				return fmt.Errorf("interject arrived before prompt")
			}
			if err := flush(`{"jsonrpc":"2.0","id":%s,"result":{"stopReason":"end_turn"}}`, promptID); err != nil {
				return err
			}
			return nil
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read request: %w", err)
	}
	return nil
}
