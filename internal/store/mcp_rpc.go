package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	
	dbgen "github.com/jmeiracorbal/mnemo/internal/db/generated"
	"github.com/jmeiracorbal/mnemo/protocol"
)

// ApplyMCPCommand executes one durable mnemo/1 mutation and stores the result
// in the same transaction, so redelivery never repeats the logical mutation.
func (s *Store) ApplyMCPCommand(id, action string, payload json.RawMessage) (json.RawMessage, error) {
	// JetStream delivery IDs change on each send. The public request ID does not:
	// it identifies the mutation even when the original response was lost.
	if action != protocol.Version {
		return protocolFailure(protocol.CodeInvalidRequest, "unsupported controller command"), nil
	}
	var request protocol.Request
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
	key := protocol.Version + ":" + request.RequestID

	// Include action and payload to reject accidental request ID reuse.
	hash := sha256.Sum256(append([]byte(action+"\x00"), payload...))
	fingerprint := hex.EncodeToString(hash[:])
	var result json.RawMessage
	err := s.withTx(func(tx *sql.Tx) error {
		q := s.q.WithTx(tx)
		prior, err := q.GetProcessedMCPCommandResult(context.Background(), key)
		if err == nil {
			if prior.Action != action || prior.RequestHash != fingerprint {
				result = protocolFailure(protocol.CodeConflict, "request_id already used with a different operation or payload")
				return nil
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
