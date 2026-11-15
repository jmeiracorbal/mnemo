package protocol

// OperationKind defines the delivery and idempotency semantics of a public
// Mnemo operation. All mutating RPC operations are durable commands. Events
// retain their own durable event stream and event identity.
type OperationKind string

const (
	KindQuery   OperationKind = "query"
	KindCommand OperationKind = "command"
	KindEvent   OperationKind = "event"
)

// OperationSpec is a copyable description of an operation in mnemo/1.
// It is not a controller action name or an authorization grant.
type OperationSpec struct {
	Name            Operation
	Kind            OperationKind
	RequiresPayload bool
}

// The list is the complete normative external-operation inventory in
// ADR-0006, not a claim that every handler is implemented yet.
const (
	OperationHealth Operation = "health"
	OperationLimitsGet Operation = "limits.get"
	OperationObservationCreate Operation = "observation.create"
	OperationObservationGet Operation = "observation.get"
	OperationObservationSearch Operation = "observation.search"
	OperationObservationUpdate Operation = "observation.update"
	OperationObservationDelete Operation = "observation.delete"
	OperationObservationTimeline Operation = "observation.timeline"
	OperationContextBuild Operation = "context.build"
	OperationSessionResolve Operation = "session.resolve"
	OperationSessionClose Operation = "session.close"
	OperationSessionBindExecution Operation = "session.bind_execution"
	OperationSessionExists Operation = "session.exists"
	OperationSessionObservationCount Operation = "session.observation_count"
	OperationSessionProjectObservationCount Operation = "session.project_observation_count"
	OperationPromptCreate Operation = "prompt.create"
	OperationCapturePassive Operation = "capture.passive"
	OperationTagList Operation = "tag.list"
	OperationTagStats Operation = "tag.stats"
	OperationTagRelated Operation = "tag.related"
	OperationTagMerge Operation = "tag.merge"
	OperationMemoryReview Operation = "memory.review"
	OperationMemoryMarkReviewed Operation = "memory.mark_reviewed"
	OperationMemoryMarkStale Operation = "memory.mark_stale"
	OperationMemorySupersede Operation = "memory.supersede"
	OperationMemoryConsolidationPlan Operation = "memory.consolidation.plan"
	OperationMemoryConsolidate Operation = "memory.consolidate"
	OperationProjectList Operation = "project.list"
	OperationProjectMergePlan Operation = "project.merge.plan"
	OperationProjectMerge Operation = "project.merge"
	OperationProjectRenamePlan Operation = "project.rename.plan"
	OperationProjectRename Operation = "project.rename"
	OperationProjectIdentityMigrate Operation = "project.identity.migrate"
	OperationStatsGet Operation = "stats.get"
	OperationArchiveExport Operation = "archive.export"
	OperationArchiveImport Operation = "archive.import"
	OperationEventPublish Operation = "event.publish"
)

var operationCatalog = [...]OperationSpec{
	{Name: OperationHealth, Kind: KindQuery, RequiresPayload: false},
	{Name: OperationLimitsGet, Kind: KindQuery, RequiresPayload: false},
	{Name: OperationObservationCreate, Kind: KindCommand, RequiresPayload: true},
	{Name: OperationObservationGet, Kind: KindQuery, RequiresPayload: true},
	{Name: OperationObservationSearch, Kind: KindQuery, RequiresPayload: true},
	{Name: OperationObservationUpdate, Kind: KindCommand, RequiresPayload: true},
	{Name: OperationObservationDelete, Kind: KindCommand, RequiresPayload: true},
	{Name: OperationObservationTimeline, Kind: KindQuery, RequiresPayload: true},
	{Name: OperationContextBuild, Kind: KindQuery, RequiresPayload: true},
	{Name: OperationSessionResolve, Kind: KindCommand, RequiresPayload: true},
	{Name: OperationSessionClose, Kind: KindCommand, RequiresPayload: true},
	{Name: OperationSessionBindExecution, Kind: KindCommand, RequiresPayload: true},
	{Name: OperationSessionExists, Kind: KindQuery, RequiresPayload: true},
	{Name: OperationSessionObservationCount, Kind: KindQuery, RequiresPayload: true},
	{Name: OperationSessionProjectObservationCount, Kind: KindQuery, RequiresPayload: true},
	{Name: OperationPromptCreate, Kind: KindCommand, RequiresPayload: true},
	{Name: OperationCapturePassive, Kind: KindCommand, RequiresPayload: true},
	{Name: OperationTagList, Kind: KindQuery, RequiresPayload: true},
	{Name: OperationTagStats, Kind: KindQuery, RequiresPayload: true},
	{Name: OperationTagRelated, Kind: KindQuery, RequiresPayload: true},
	{Name: OperationTagMerge, Kind: KindCommand, RequiresPayload: true},
	{Name: OperationMemoryReview, Kind: KindQuery, RequiresPayload: true},
	{Name: OperationMemoryMarkReviewed, Kind: KindCommand, RequiresPayload: true},
	{Name: OperationMemoryMarkStale, Kind: KindCommand, RequiresPayload: true},
	{Name: OperationMemorySupersede, Kind: KindCommand, RequiresPayload: true},
	{Name: OperationMemoryConsolidationPlan, Kind: KindQuery, RequiresPayload: true},
	{Name: OperationMemoryConsolidate, Kind: KindCommand, RequiresPayload: true},
	{Name: OperationProjectList, Kind: KindQuery, RequiresPayload: false},
	{Name: OperationProjectMergePlan, Kind: KindQuery, RequiresPayload: true},
	{Name: OperationProjectMerge, Kind: KindCommand, RequiresPayload: true},
	{Name: OperationProjectRenamePlan, Kind: KindQuery, RequiresPayload: true},
	{Name: OperationProjectRename, Kind: KindCommand, RequiresPayload: true},
	{Name: OperationProjectIdentityMigrate, Kind: KindCommand, RequiresPayload: true},
	{Name: OperationStatsGet, Kind: KindQuery, RequiresPayload: false},
	{Name: OperationArchiveExport, Kind: KindQuery, RequiresPayload: false},
	{Name: OperationArchiveImport, Kind: KindCommand, RequiresPayload: true},
	{Name: OperationEventPublish, Kind: KindEvent, RequiresPayload: true},
}

// Operations returns the published operation catalog as an independent slice.
func Operations() []OperationSpec {
	return append([]OperationSpec(nil), operationCatalog[:]...)
}

// Spec returns the normative kind and input requirements of an operation.
func (o Operation) Spec() (OperationSpec, bool) {
	for _, spec := range operationCatalog {
		if spec.Name == o {
			return spec, true
		}
	}
	return OperationSpec{}, false
}
