package agent

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

const (
	acpRecordingHelperModeEnv      = "MULTICA_ACP_RECORDING_HELPER_MODE"
	acpRecordingHelperRecordEnv    = "MULTICA_ACP_RECORDING_HELPER_RECORD"
	acpRecordingHelperSessionIDEnv = "MULTICA_ACP_RECORDING_HELPER_SESSION_ID"
	acpRecordingHelperCapsEnv      = "MULTICA_ACP_RECORDING_HELPER_CAPS"
)

func runFakeACPRecordingHelper() error {
	mode := os.Getenv(acpRecordingHelperModeEnv)
	recordPath := os.Getenv(acpRecordingHelperRecordEnv)
	if recordPath == "" {
		return fmt.Errorf("missing %s", acpRecordingHelperRecordEnv)
	}
	record, err := os.OpenFile(recordPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open RPC record: %w", err)
	}
	defer record.Close()

	sessionID := os.Getenv(acpRecordingHelperSessionIDEnv)
	if sessionID == "" {
		sessionID = "ses_fake"
	}
	caps := os.Getenv(acpRecordingHelperCapsEnv)
	if caps == "" {
		caps = `{}`
	}
	if mode == "qoder" && os.Getenv("QODER_INIT_MCP_SSE") == "1" {
		caps = `{"mcpCapabilities":{"sse":true}}`
	}

	out := bufio.NewWriter(os.Stdout)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if _, err := fmt.Fprintln(record, string(line)); err != nil {
			return fmt.Errorf("record RPC frame: %w", err)
		}
		var request struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if err := json.Unmarshal(line, &request); err != nil {
			return fmt.Errorf("decode RPC frame: %w", err)
		}

		var result string
		switch request.Method {
		case "initialize":
			result = `{"protocolVersion":1,"agentCapabilities":` + caps + `}`
		case "session/new", "session/resume", "session/load":
			result = `{"sessionId":` + fmt.Sprintf("%q", sessionID) + `}`
		case "session/set_model":
			result = `{}`
		case "session/prompt":
			if mode == "qoder" {
				if _, err := fmt.Fprintf(out, `{"jsonrpc":"2.0","method":"session/notification","params":{"sessionId":%q,"update":{"type":"AgentMessageChunk","content":{"type":"text","text":"ok"}}}}`+"\n", sessionID); err != nil {
					return fmt.Errorf("write notification: %w", err)
				}
				result = `{"stopReason":"end_turn","usage":{"inputTokens":1,"outputTokens":2}}`
			} else {
				result = `{"stopReason":"end_turn"}`
			}
		default:
			continue
		}

		if _, err := fmt.Fprintf(out, `{"jsonrpc":"2.0","id":%s,"result":%s}`+"\n", request.ID, result); err != nil {
			return fmt.Errorf("write RPC response: %w", err)
		}
		if err := out.Flush(); err != nil {
			return fmt.Errorf("flush RPC response: %w", err)
		}
		if request.Method == "session/prompt" {
			return nil
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read RPC frame: %w", err)
	}
	return nil
}
