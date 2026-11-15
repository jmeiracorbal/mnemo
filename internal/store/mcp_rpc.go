package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

		"github.com/jmeiracorbal/mnemo/protocol"
	dbgen "github.com/jmeiracorbal/mnemo/internal/db/generated"
)

// ApplyMCPCommand executes one durable MCP mutation and stores its serialized
// result in the same SQLite transaction. A JetStream redelivery therefore
// receives the original result without applying the mutation again.
func (s *Store) ApplyMCPCommand(id, action string, payload json.RawMessage) (json.RawMessage, error) {
	// JetStream delivery IDs change on each send. The public request ID does not:
	// it identifies the mutation even when the original response was lost.
	if action != protocol.Version {
		return protocolFailure(protocol.CodeInvalidRequest, "unsupported controller command"), nil
	}
	key := id
	var request protocol.Request
	if action == protocol.Version {
		if err := json.Unmarshal(payload, &request); err != nil {
			return protocolFailure(protocol.CodeInvalidRequest, "invalid protocol request"), nil
		}
		if err := request.Validate(); err != nil {
			var coded *protocol.Error
			if errors.As(err, &coded) {
				return protocolFailure(coded.Code, coded.Message), nil
			}
			return protocolFailure(protocol.CodeInvalidRequest, err.Error()), nil
		}
		spec, ok := request.Operation.Spec()
		if !ok || spec.Kind != protocol.KindCommand {
			return protocolFailure(protocol.CodeInvalidRequest, "non-mutating protocol operation submitted as durable command"), nil
		}
		key = protocol.Version + ":" + request.RequestID
	}

	// Include action and payload to reject accidental request ID reuse.
	hash := sha256.Sum256(append([]byte(action+"\x00"), payload...))
	fingerprint := hex.EncodeToString(hash[:])
	var result json.RawMessage
	err := s.withTx(func(tx *sql.Tx) error {
		q := s.q.WithTx(tx)
		prior, err := q.GetProcessedMCPCommandResult(context.Background(), key)
		if err == nil {
			if prior.Action != action || prior.RequestHash != fingerprint {
				if action == protocol.Version {
					result = protocolFailure(protocol.CodeConflict, "request_id already used with a different operation or payload")
					return nil
				}
				return fmt.Errorf("durable command ID already used with different content")
			}
			result = json.RawMessage(prior.ResultJson)
			return nil
		}
		if err != sql.ErrNoRows {
			return err
		}
		transactional := *s
		transactional.activeTx = tx
		transactional.q = q
		computed, err := transactional.ExecuteProtocolRequest(context.Background(), payload)
		if err != nil {
			return err
		}
		if err := q.InsertProcessedMCPCommand(context.Background(), dbgen.InsertProcessedMCPCommandParams{
			ID: key, Action: action, ResultJson: string(computed), RequestHash: fingerprint,
		}); err != nil {
			return err
		}
		result = computed
		return nil
	})
	return result, err
}

// ExecuteMCPAction is the controller-only RPC surface for MCP operations.
// Keeping this dispatch next to Store makes the controller the only process
// that can translate an MCP request into SQLite access.
