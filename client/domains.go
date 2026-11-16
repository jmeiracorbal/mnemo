package client

import (
	"context"

	"github.com/jmeiracorbal/mnemo/v2/protocol"
)

func (c *Client) CreatePrompt(ctx context.Context, requestID string, input protocol.PromptCreateInput) error {
	return c.invoke(ctx, protocol.OperationPromptCreate, requestID, input, nil)
}

func (c *Client) CapturePassive(ctx context.Context, requestID string, input protocol.CapturePassiveInput) (protocol.CapturePassiveOutput, error) {
	var output protocol.CapturePassiveOutput
	err := c.invoke(ctx, protocol.OperationCapturePassive, requestID, input, &output)
	return output, err
}

func (c *Client) ListProjects(ctx context.Context) ([]protocol.ProjectSummary, error) {
	var output protocol.ProjectListOutput
	err := c.invoke(ctx, protocol.OperationProjectList, "", nil, &output)
	return output.Projects, err
}

func (c *Client) PlanProjectMerge(ctx context.Context, input protocol.ProjectPairInput) (protocol.ProjectMergePlan, error) {
	var output protocol.ProjectMergePlan
	err := c.invoke(ctx, protocol.OperationProjectMergePlan, "", input, &output)
	return output, err
}

func (c *Client) MergeProjects(ctx context.Context, requestID string, input protocol.ProjectPairInput) (protocol.ProjectMergeOutput, error) {
	var output protocol.ProjectMergeOutput
	err := c.invoke(ctx, protocol.OperationProjectMerge, requestID, input, &output)
	return output, err
}

func (c *Client) PlanProjectRename(ctx context.Context, input protocol.ProjectRenameInput) (protocol.ProjectRenamePlan, error) {
	var output protocol.ProjectRenamePlan
	err := c.invoke(ctx, protocol.OperationProjectRenamePlan, "", input, &output)
	return output, err
}

func (c *Client) RenameProject(ctx context.Context, requestID string, input protocol.ProjectRenameInput) (protocol.ProjectRenameOutput, error) {
	var output protocol.ProjectRenameOutput
	err := c.invoke(ctx, protocol.OperationProjectRename, requestID, input, &output)
	return output, err
}

func (c *Client) MigrateProjectIdentity(ctx context.Context, requestID string, input protocol.ProjectIdentityMigrateInput) (protocol.ProjectIdentityMigrateOutput, error) {
	var output protocol.ProjectIdentityMigrateOutput
	err := c.invoke(ctx, protocol.OperationProjectIdentityMigrate, requestID, input, &output)
	return output, err
}

func (c *Client) ReviewMemories(ctx context.Context, input protocol.MemoryReviewInput) (protocol.MemoryReviewOutput, error) {
	var output protocol.MemoryReviewOutput
	err := c.invoke(ctx, protocol.OperationMemoryReview, "", input, &output)
	return output, err
}

func (c *Client) MarkMemoryReviewed(ctx context.Context, requestID string, input protocol.MemoryMarkInput) error {
	return c.invoke(ctx, protocol.OperationMemoryMarkReviewed, requestID, input, nil)
}

func (c *Client) MarkMemoryStale(ctx context.Context, requestID string, input protocol.MemoryMarkInput) error {
	return c.invoke(ctx, protocol.OperationMemoryMarkStale, requestID, input, nil)
}

func (c *Client) SupersedeMemory(ctx context.Context, requestID string, input protocol.MemorySupersedeInput) error {
	return c.invoke(ctx, protocol.OperationMemorySupersede, requestID, input, nil)
}

func (c *Client) PlanMemoryConsolidation(ctx context.Context, input protocol.MemoryConsolidationInput) (protocol.MemoryConsolidationOutput, error) {
	var output protocol.MemoryConsolidationOutput
	err := c.invoke(ctx, protocol.OperationMemoryConsolidationPlan, "", input, &output)
	return output, err
}

func (c *Client) ConsolidateMemories(ctx context.Context, requestID string, input protocol.MemoryConsolidationInput) (protocol.MemoryConsolidationOutput, error) {
	var output protocol.MemoryConsolidationOutput
	err := c.invoke(ctx, protocol.OperationMemoryConsolidate, requestID, input, &output)
	return output, err
}

func (c *Client) Export(ctx context.Context) (protocol.ArchiveExportOutput, error) {
	var output protocol.ArchiveExportOutput
	err := c.invoke(ctx, protocol.OperationArchiveExport, "", nil, &output)
	return output, err
}

func (c *Client) Import(ctx context.Context, requestID string, input protocol.ArchiveImportInput) (protocol.ArchiveImportOutput, error) {
	var output protocol.ArchiveImportOutput
	err := c.invoke(ctx, protocol.OperationArchiveImport, requestID, input, &output)
	return output, err
}
