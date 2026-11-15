package protocol

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestResourceRefRoundTrip(t *testing.T) {
	want, err := NewObservationRef("project-6f9a", "obs-1abcdef234567890")
	if err != nil {
		t.Fatal(err)
	}
	raw := "mnemo://project-6f9a/observations/obs-1abcdef234567890"
	if got := want.String(); got != raw {
		t.Fatalf("ref = %q, want %q", got, raw)
	}
	parsed, err := ParseResourceRef(raw)
	if err != nil || parsed != want {
		t.Fatalf("parse: got %+v, err %v", parsed, err)
	}
	encoded, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != `"mnemo://project-6f9a/observations/obs-1abcdef234567890"` {
		t.Fatalf("JSON resource ref = %s", encoded)
	}
	var decoded ResourceRef
	if err := json.Unmarshal(encoded, &decoded); err != nil || decoded != want {
		t.Fatalf("JSON decode: got %+v, err %v", decoded, err)
	}
	text, err := want.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	var fromText ResourceRef
	if err := fromText.UnmarshalText(text); err != nil || fromText != want {
		t.Fatalf("text decode: got %+v, err %v", fromText, err)
	}
}

func TestResourceRefRejectsNoncanonicalInput(t *testing.T) {
	tests := []string{
		"", "mnemo://", "mnemo://project-1/observations/",
		"mnemo://project-1/observations/..",
		"mnemo://project-1/observations/obs-1/extra",
		"mnemo://project-1/prompts/obs-1",
		"mnemo://project-1/observations/obs-1?x=y",
		"mnemo://project-1/observations/obs-1#fragment",
		"mnemo://alice@project-1/observations/obs-1",
		"mnemo://project-1:4222/observations/obs-1",
		"mnemo://project-1/observations/obs%2D1",
		"mnemo://project-1/observations/obs%2F1",
		"MNEMO://project-1/observations/obs-1",
		"mnemo://project-1/observations/obs 1",
		"mnemo://project-1//observations/obs-1",
		"https://project-1/observations/obs-1",
	}
	for _, raw := range tests {
		t.Run(raw, func(t *testing.T) {
			_, err := ParseResourceRef(raw)
			if err == nil {
				t.Fatal("accepted invalid resource reference")
			}
			var protoErr *Error
			if !errors.As(err, &protoErr) || protoErr.Code != CodeInvalidReference {
				t.Fatalf("error = %v, want invalid_reference", err)
			}
		})
	}
}

func TestRequestVersionAndStableId(t *testing.T) {
	id := "request-47"
	input := ObservationCreateInput{ProjectID: "project-1", Type: "instruction", Title: "Plan", Content: "Do the work"}
	first, err := NewRequest(OperationObservationCreate, id, input)
	if err != nil {
		t.Fatal(err)
	}
	retry, err := NewRequest(OperationObservationCreate, id, input)
	if err != nil {
		t.Fatal(err)
	}
	if first.RequestID != retry.RequestID || string(first.Payload) != string(retry.Payload) {
		t.Fatal("retry changed its logical mutation identity")
	}
	if first.Version != "mnemo/1" {
		t.Fatalf("version = %q", first.Version)
	}
	if first.Operation != OperationObservationCreate {
		t.Fatalf("operation = %q", first.Operation)
	}
}

func TestRequestValidation(t *testing.T) {
	cases := []struct {
		name string
		req  Request
		code ErrorCode
	}{
		{"missing request ID", Request{Version: Version, Operation: OperationObservationCreate, Payload: []byte(`{}`)}, CodeInvalidRequest},
		{"missing create payload", Request{Version: Version, Operation: OperationObservationCreate, RequestID: "r-1"}, CodeInvalidRequest},
		{"missing get payload", Request{Version: Version, Operation: OperationObservationGet}, CodeInvalidRequest},
		{"invalid JSON", Request{Version: Version, Operation: OperationObservationCreate, RequestID: "r-1", Payload: []byte(`{`)}, CodeInvalidRequest},
		{"unsupported version", Request{Version: "mnemo/2", Operation: OperationHealth}, CodeUnsupportedVersion},
		{"unsupported operation", Request{Version: Version, Operation: "get_observation"}, CodeInvalidRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.req.Validate()
			var coded *Error
			if !errors.As(err, &coded) || coded.Code != tc.code {
				t.Fatalf("Validate error = %v, want code %s", err, tc.code)
			}
		})
	}
	if err := (Request{Version: Version, Operation: OperationHealth}).Validate(); err != nil {
		t.Fatalf("health: %v", err)
	}
	if err := (Request{Version: Version, Operation: OperationObservationGet, Payload: []byte(`{"ref":"mnemo://p/observations/o"}`)}).Validate(); err != nil {
		t.Fatalf("observation.get: %v", err)
	}
}

func TestCreateInputValidation(t *testing.T) {
	input := ObservationCreateInput{ProjectID: "project-1", Type: "instruction", Title: "Plan", Content: "Contents"}
	if err := input.Validate(); err != nil {
		t.Fatal(err)
	}
	input.ProjectID = "../other"
	if err := input.Validate(); err == nil {
		t.Fatal("accepted invalid project ID")
	}
	input.ProjectID = "project-1"
	input.Content = ""
	if err := input.Validate(); err == nil {
		t.Fatal("accepted empty content")
	}
}
