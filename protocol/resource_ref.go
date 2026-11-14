package protocol

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// ResourceRef identifies a persisted Mnemo observation without exposing a
// local SQLite row ID or the identity of the calling agent/harness.
type ResourceRef struct {
	ProjectID string
	SyncID    string
}

// NewObservationRef constructs a canonical reference for an existing
// observation. projectID is the identity managed by Mnemo.
func NewObservationRef(projectID, syncID string) (ResourceRef, error) {
	ref := ResourceRef{ProjectID: projectID, SyncID: syncID}
	if err := ref.Validate(); err != nil {
		return ResourceRef{}, err
	}
	return ref, nil
}

// ParseResourceRef accepts only the canonical URI form:
//
//	mnemo://<project-id>/observations/<sync-id>
//
// It rejects queries, fragments, user information, encoded path components,
// alternative resource kinds, and noncanonical URI representations.
func ParseResourceRef(raw string) (ResourceRef, error) {
	uri, err := url.Parse(raw)
	if err != nil {
		return ResourceRef{}, invalidReference("parse resource URI: %v", err)
	}
	if uri.Scheme != "mnemo" || uri.Opaque != "" || uri.User != nil ||
		uri.RawQuery != "" || uri.Fragment != "" || uri.ForceQuery ||
		uri.RawFragment != "" || uri.RawPath != "" ||
		uri.Host == "" || uri.Host != uri.Hostname() {
		return ResourceRef{}, invalidReference("expected canonical mnemo observation URI")
	}
	parts := strings.Split(uri.Path, "/")
	if len(parts) != 3 || parts[0] != "" || parts[1] != "observations" {
		return ResourceRef{}, invalidReference("unsupported resource path")
	}
	ref, err := NewObservationRef(uri.Host, parts[2])
	if err != nil {
		return ResourceRef{}, err
	}
	if raw != ref.String() {
		return ResourceRef{}, invalidReference("noncanonical resource URI")
	}
	return ref, nil
}

// Validate rejects delimiters and escapes so a reference is unambiguous.
func (r ResourceRef) Validate() error {
	if !validResourceToken(r.ProjectID) {
		return invalidReference("invalid Mnemo project ID")
	}
	if !validResourceToken(r.SyncID) {
		return invalidReference("invalid Mnemo observation sync ID")
	}
	return nil
}

func validResourceToken(value string) bool {
	if value == "" || value == "." || value == ".." {
		return false
	}
	for _, char := range value {
		switch {
		case char >= 'a' && char <= 'z':
		case char >= 'A' && char <= 'Z':
		case char >= '0' && char <= '9':
		case char == '-' || char == '_' || char == '.':
		default:
			return false
		}
	}
	return true
}

// String returns the canonical observation URI. Call Validate before using a
// ResourceRef assembled directly through a struct literal.
func (r ResourceRef) String() string {
	return "mnemo://" + r.ProjectID + "/observations/" + r.SyncID
}

// MarshalText and UnmarshalText make the canonical reference usable in
// application contracts without exposing separate internal identifiers.
func (r ResourceRef) MarshalText() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	return []byte(r.String()), nil
}

func (r *ResourceRef) UnmarshalText(data []byte) error {
	if r == nil {
		return invalidReference("nil resource reference")
	}
	ref, err := ParseResourceRef(string(data))
	if err != nil {
		return err
	}
	*r = ref
	return nil
}

func (r ResourceRef) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(r.String())
}

func (r *ResourceRef) UnmarshalJSON(data []byte) error {
	if r == nil {
		return invalidReference("nil resource reference")
	}
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return invalidReference("resource reference must be a URI string: %v", err)
	}
	ref, err := ParseResourceRef(raw)
	if err != nil {
		return err
	}
	*r = ref
	return nil
}

func invalidReference(format string, args ...any) error {
	return &Error{Code: CodeInvalidReference, Message: fmt.Sprintf(format, args...)}
}
