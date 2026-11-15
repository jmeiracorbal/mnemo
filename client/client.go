// Package client exposes Mnemo's versioned public Go API. It communicates
// exclusively with the existing installation-wide controller.
package client

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jmeiracorbal/mnemo/internal/events"
	"github.com/jmeiracorbal/mnemo/protocol"
)

type Client struct {
	config events.Config
}

// New loads the installed Mnemo controller settings from the Mnemo data root.
// Consumers never open SQLite or connect to JetStream themselves.
func New(dataDir string) (*Client, error) {
	config, err := events.LoadConfig(dataDir)
	if err != nil {
		return nil, err
	}
	return &Client{config: config}, nil
}

func (c *Client) call(ctx context.Context, request protocol.Request, output any) error {
	if c == nil {
		return fmt.Errorf("mnemo client is nil")
	}
	if err := request.Validate(); err != nil {
		return err
	}
	var response protocol.Response
	if err := events.Call(ctx, c.config, protocol.Version, request, &response); err != nil {
		return fmt.Errorf("mnemo controller request: %w", err)
	}
	if response.Version != protocol.Version {
		return &protocol.Error{Code: protocol.CodeUnsupportedVersion, Message: "invalid controller protocol version"}
	}
	if response.Error != nil {
		return response.Error
	}
	if response.RequestID != request.RequestID {
		return &protocol.Error{Code: protocol.CodeInternal, Message: "response request_id mismatch"}
	}
	if output == nil {
		return nil
	}
	if len(response.Payload) == 0 {
		return &protocol.Error{Code: protocol.CodeInternal, Message: "controller returned no payload"}
	}
	if err := json.Unmarshal(response.Payload, output); err != nil {
		return fmt.Errorf("decode Mnemo protocol response: %w", err)
	}
	return nil
}

func (c *Client) Health(ctx context.Context) error {
	request, err := protocol.NewRequest(protocol.OperationHealth, "", nil)
	if err != nil {
		return err
	}
	var status protocol.HealthOutput
	if err := c.call(ctx, request, &status); err != nil {
		return err
	}
	if !status.Ready {
		return &protocol.Error{Code: protocol.CodeUnavailable, Message: "controller is not ready"}
	}
	return nil
}

func (c *Client) Get(ctx context.Context, ref protocol.ResourceRef) (protocol.Observation, error) {
	if err := ref.Validate(); err != nil {
		return protocol.Observation{}, err
	}
	request, err := protocol.NewRequest(protocol.OperationObservationGet, "", protocol.ObservationGetInput{Ref: ref})
	if err != nil {
		return protocol.Observation{}, err
	}
	var result protocol.ObservationGetOutput
	if err := c.call(ctx, request, &result); err != nil {
		return protocol.Observation{}, err
	}
	if result.Observation.Ref != ref {
		return protocol.Observation{}, &protocol.Error{Code: protocol.CodeInternal, Message: "controller returned a different observation"}
	}
	return result.Observation, nil
}

// Create requires the caller to supply a stable ID for this logical write.
// Retrying after any timeout or lost reply MUST reuse the same requestID and
// input; a new request ID would represent a new mutation.
func (c *Client) Create(ctx context.Context, requestID string, input protocol.ObservationCreateInput) (protocol.ResourceRef, error) {
	if err := input.Validate(); err != nil {
		return protocol.ResourceRef{}, err
	}
	request, err := protocol.NewRequest(protocol.OperationObservationCreate, requestID, input)
	if err != nil {
		return protocol.ResourceRef{}, err
	}
	var result protocol.ObservationCreateOutput
	if err := c.call(ctx, request, &result); err != nil {
		return protocol.ResourceRef{}, err
	}
	if result.Ref.ProjectID != input.ProjectID {
		return protocol.ResourceRef{}, &protocol.Error{Code: protocol.CodeInternal, Message: "controller returned an observation from another project"}
	}
	if err := result.Ref.Validate(); err != nil {
		return protocol.ResourceRef{}, err
	}
	return result.Ref, nil
}

func (c *Client) invoke(ctx context.Context, operation protocol.Operation, requestID string, input, output any) error {
	request, err := protocol.NewRequest(operation, requestID, input)
	if err != nil { return err }
	return c.call(ctx, request, output)
}

// Search returns canonical observation references and rank metadata.
func (c *Client) Search(ctx context.Context, input protocol.ObservationSearchInput) ([]protocol.ObservationMatch, error) {
	var output protocol.ObservationSearchOutput
	if err := c.invoke(ctx, protocol.OperationObservationSearch, "", input, &output); err != nil {
		return nil, err
	}
	return output.Matches, nil
}

func (c *Client) Update(ctx context.Context, requestID string, input protocol.ObservationUpdateInput) (protocol.Observation, error) {
	if err := input.Ref.Validate(); err != nil { return protocol.Observation{}, err }
	var output protocol.ObservationGetOutput
	if err := c.invoke(ctx, protocol.OperationObservationUpdate, requestID, input, &output); err != nil {
		return protocol.Observation{}, err
	}
	if output.Observation.Ref != input.Ref {
		return protocol.Observation{}, &protocol.Error{Code: protocol.CodeInternal, Message: "controller returned a different observation"}
	}
	return output.Observation, nil
}

func (c *Client) Delete(ctx context.Context, requestID string, ref protocol.ResourceRef) error {
	if err := ref.Validate(); err != nil { return err }
	return c.invoke(ctx, protocol.OperationObservationDelete, requestID, protocol.ObservationDeleteInput{Ref: ref}, nil)
}

func (c *Client) Timeline(ctx context.Context, input protocol.ObservationTimelineInput) (protocol.ObservationTimelineOutput, error) {
	var output protocol.ObservationTimelineOutput
	if err := input.Ref.Validate(); err != nil { return output, err }
	if err := c.invoke(ctx, protocol.OperationObservationTimeline, "", input, &output); err != nil { return output, err }
	return output, nil
}

func (c *Client) Context(ctx context.Context, input protocol.ContextBuildInput) (string, error) {
	var output protocol.ContextBuildOutput
	if err := c.invoke(ctx, protocol.OperationContextBuild, "", input, &output); err != nil { return "", err }
	return output.Text, nil
}

func (c *Client) Limits(ctx context.Context) (protocol.LimitsOutput, error) {
	var output protocol.LimitsOutput
	if err := c.invoke(ctx, protocol.OperationLimitsGet, "", nil, &output); err != nil { return output, err }
	return output, nil
}

func (c *Client) Stats(ctx context.Context) (protocol.StatsOutput, error) {
	var output protocol.StatsOutput
	if err := c.invoke(ctx, protocol.OperationStatsGet, "", nil, &output); err != nil { return output, err }
	return output, nil
}
