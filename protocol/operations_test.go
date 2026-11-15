package protocol

import (
	"errors"
	"testing"
)

func TestPublicOperationCatalogIsUniqueAndClosed(t *testing.T) {
	specs := Operations()
	if len(specs) != 37 {
		t.Fatalf("catalog operations = %d, want 37", len(specs))
	}
	seen := map[Operation]bool{}
	for _, spec := range specs {
		if seen[spec.Name] {
			t.Fatalf("duplicated public operation: %q", spec.Name)
		}
		seen[spec.Name] = true
		found, ok := spec.Name.Spec()
		if !ok || found != spec {
			t.Fatalf("invalid operation metadata for %q", spec.Name)
		}
		if spec.Kind != KindQuery && spec.Kind != KindCommand && spec.Kind != KindEvent {
			t.Fatalf("unknown delivery semantics for %q", spec.Name)
		}
		if spec.Kind == KindCommand && !spec.RequiresPayload {
			t.Fatalf("command %q does not require explicit payload", spec.Name)
		}
	}
	if _, ok := Operation("get_observation").Spec(); ok {
		t.Fatal("internal controller action exposed as public operation")
	}
	// Callers must not be able to mutate the normative catalog.
	specs[0].Name = "tampered"
	if other := Operations()[0].Name; other != OperationHealth {
		t.Fatalf("caller modified public operation catalog: %q", other)
	}
}

func TestPublicOperationMutationIdentity(t *testing.T) {
	for _, spec := range Operations() {
		if spec.Kind == KindEvent {
			req := Request{Version: Version, Operation: spec.Name, RequestID: "r-1", Payload: []byte("{}")}
			var coded *Error
			if !errors.As(req.Validate(), &coded) || coded.Code != CodeInvalidRequest {
				t.Fatalf("event %s incorrectly accepted in RPC envelope", spec.Name)
			}
			continue
		}
		req := Request{Version: Version, Operation: spec.Name}
		if spec.RequiresPayload {
			req.Payload = []byte("{}")
		}
		if spec.Kind == KindCommand {
			var coded *Error
			if !errors.As(req.Validate(), &coded) || coded.Code != CodeInvalidRequest {
				t.Fatalf("command %s accepted without stable request_id", spec.Name)
			}
			req.RequestID = "r-1"
		}
		if err := req.Validate(); err != nil {
			t.Fatalf("valid catalog request %s rejected: %v", spec.Name, err)
		}
	}
}
