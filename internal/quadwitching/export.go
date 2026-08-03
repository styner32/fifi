package quadwitching

import (
	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/fileio"
)

type EndpointSnapshot struct {
	Status      string         `json:"status"`
	MessageCode string         `json:"message_code,omitempty"`
	Message     string         `json:"message,omitempty"`
	Error       string         `json:"error,omitempty"`
	ParseError  string         `json:"parse_error,omitempty"`
	RawBody     string         `json:"raw_body,omitempty"`
	Body        map[string]any `json:"body,omitempty"`
}

type SnapshotExport struct {
	GeneratedAt    string                      `json:"generated_at"`
	BusinessDate   string                      `json:"business_date,omitempty"`
	FuturesCode    string                      `json:"futures_code,omitempty"`
	FuturesName    string                      `json:"futures_name,omitempty"`
	MasterCache    string                      `json:"master_cache,omitempty"`
	MasterJSON     string                      `json:"master_json,omitempty"`
	EndpointStates map[string]EndpointSnapshot `json:"endpoint_states,omitempty"`
	Notes          []string                    `json:"notes,omitempty"`
}

func NewEndpointSnapshot(resp *auth.RESTResponse, err error) EndpointSnapshot {
	snapshot := EndpointSnapshot{}
	if err != nil {
		snapshot.Status = "error"
		snapshot.Error = err.Error()
		if resp != nil {
			snapshot.MessageCode = resp.MessageCode()
			snapshot.Message = resp.Message()
			snapshot.ParseError = resp.ParseError
			snapshot.RawBody = string(resp.RawBody)
			snapshot.Body = resp.Body
		}
		return snapshot
	}
	if resp == nil {
		return EndpointSnapshot{Status: "nil", Error: "response is nil"}
	}

	if resp.IsOK() {
		snapshot.Status = "ok"
	} else {
		snapshot.Status = "business_error"
	}
	snapshot.MessageCode = resp.MessageCode()
	snapshot.Message = resp.Message()
	snapshot.ParseError = resp.ParseError
	snapshot.RawBody = string(resp.RawBody)
	snapshot.Body = resp.Body
	return snapshot
}

func WriteSnapshot(path string, payload SnapshotExport) error {
	return fileio.WriteJSONAtomic(path, payload)
}
