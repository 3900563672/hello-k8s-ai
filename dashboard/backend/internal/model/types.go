package model

import (
	"encoding/json"
	"time"
)

type ResourceRef struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Namespace  string `json:"namespace,omitempty"`
	Name       string `json:"name"`
	UID        string `json:"uid,omitempty"`
}

type Condition struct {
	Type               string     `json:"type"`
	Status             string     `json:"status"`
	Reason             string     `json:"reason,omitempty"`
	Message            string     `json:"message,omitempty"`
	ObservedGeneration int64      `json:"observedGeneration,omitempty"`
	LastTransitionTime *time.Time `json:"lastTransitionTime,omitempty"`
}

type ResourceMetadata struct {
	Generation        int64             `json:"generation"`
	ResourceVersion   string            `json:"resourceVersion"`
	CreatedAt         *time.Time        `json:"createdAt,omitempty"`
	DeletionTimestamp *time.Time        `json:"deletionTimestamp,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty"`
}

type PlatformResource struct {
	Ref        ResourceRef      `json:"ref"`
	Metadata   ResourceMetadata `json:"metadata"`
	Spec       map[string]any   `json:"spec"`
	Status     map[string]any   `json:"status"`
	Conditions []Condition      `json:"conditions"`
	Derived    map[string]any   `json:"derived,omitempty"`
}

type PolicySet struct {
	TenantModel []PlatformResource `json:"tenantModel"`
	TenantNode  []PlatformResource `json:"tenantNode"`
	ModelNode   []PlatformResource `json:"modelNode"`
}

type Configuration struct {
	AsOf               time.Time          `json:"asOf"`
	Availability       string             `json:"availability"`
	Models             []PlatformResource `json:"models"`
	WorkerNodes        []PlatformResource `json:"workerNodes"`
	Tenants            []PlatformResource `json:"tenants"`
	Policies           PolicySet          `json:"policies"`
	Orchestrators      []PlatformResource `json:"orchestrators"`
	SimulationClocks   []PlatformResource `json:"simulationClocks"`
	SimulatorInstances []PlatformResource `json:"simulatorInstances"`
	TenantPerformance  []PlatformResource `json:"tenantPerformance"`
	TenantRuntimes     []PlatformResource `json:"tenantRuntimes"`
}

type ClusterNode struct {
	Ref         ResourceRef       `json:"ref"`
	Role        string            `json:"role"`
	Ready       bool              `json:"ready"`
	Phase       string            `json:"phase"`
	Schedulable bool              `json:"schedulable"`
	Zone        string            `json:"zone,omitempty"`
	Version     string            `json:"version,omitempty"`
	InternalIP  string            `json:"internalIP,omitempty"`
	Conditions  []Condition       `json:"conditions"`
	ObservedAt  time.Time         `json:"observedAt"`
	Capacity    map[string]string `json:"capacity,omitempty"`
	Allocatable map[string]string `json:"allocatable,omitempty"`
}

type ContainerState struct {
	Name         string `json:"name"`
	Ready        bool   `json:"ready"`
	RestartCount int32  `json:"restartCount"`
	State        string `json:"state"`
	Reason       string `json:"reason,omitempty"`
}

type Pod struct {
	Ref               ResourceRef      `json:"ref"`
	Phase             string           `json:"phase"`
	Ready             bool             `json:"ready"`
	NodeName          string           `json:"nodeName,omitempty"`
	PodIP             string           `json:"podIP,omitempty"`
	StartTime         *time.Time       `json:"startTime,omitempty"`
	Conditions        []Condition      `json:"conditions"`
	Containers        []ContainerState `json:"containers"`
	SimulatorInstance string           `json:"simulatorInstance,omitempty"`
	Tenant            string           `json:"tenant,omitempty"`
	Model             string           `json:"model,omitempty"`
}

type Deployment struct {
	Ref                 ResourceRef `json:"ref"`
	DesiredReplicas     int32       `json:"desiredReplicas"`
	UpdatedReplicas     int32       `json:"updatedReplicas"`
	ReadyReplicas       int32       `json:"readyReplicas"`
	AvailableReplicas   int32       `json:"availableReplicas"`
	UnavailableReplicas int32       `json:"unavailableReplicas"`
	Conditions          []Condition `json:"conditions"`
	SimulatorInstance   string      `json:"simulatorInstance,omitempty"`
	Tenant              string      `json:"tenant,omitempty"`
	Model               string      `json:"model,omitempty"`
}

type Service struct {
	Ref       ResourceRef       `json:"ref"`
	Type      string            `json:"type"`
	ClusterIP string            `json:"clusterIP,omitempty"`
	Ports     []map[string]any  `json:"ports"`
	Selector  map[string]string `json:"selector,omitempty"`
}

type Lease struct {
	Ref            ResourceRef `json:"ref"`
	HolderIdentity string      `json:"holderIdentity,omitempty"`
	RenewTime      *time.Time  `json:"renewTime,omitempty"`
	LeaseDuration  int32       `json:"leaseDurationSeconds,omitempty"`
	Fresh          bool        `json:"fresh"`
	AgeMs          int64       `json:"ageMs,omitempty"`
}

type Event struct {
	ID                  string      `json:"id"`
	EventTime           time.Time   `json:"eventTime"`
	Type                string      `json:"type"`
	Reason              string      `json:"reason"`
	Message             string      `json:"message"`
	Count               int32       `json:"count"`
	FirstSeen           *time.Time  `json:"firstSeen,omitempty"`
	LastSeen            *time.Time  `json:"lastSeen,omitempty"`
	Regarding           ResourceRef `json:"regarding"`
	ReportingController string      `json:"reportingController,omitempty"`
	Source              string      `json:"source,omitempty"`
}

type Workloads struct {
	Nodes       []ClusterNode `json:"nodes"`
	Pods        []Pod         `json:"pods"`
	Deployments []Deployment  `json:"deployments"`
	Services    []Service     `json:"services"`
	Leases      []Lease       `json:"leases"`
	Events      []Event       `json:"events"`
}

type Performance struct {
	AvgTTFT     *NumberValue `json:"avgTTFT,omitempty"`
	AvgQueue    *NumberValue `json:"avgQueue,omitempty"`
	SampleCount int          `json:"sampleCount"`
	ObservedAt  *time.Time   `json:"observedAt,omitempty"`
	Freshness   string       `json:"freshness"`
	Phase       string       `json:"phase,omitempty"`
}

type NumberValue struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

type TrafficInstance struct {
	Name              string     `json:"name"`
	Model             string     `json:"model"`
	DesiredReplicas   int        `json:"desiredReplicas"`
	AvailableReplicas int        `json:"availableReplicas"`
	AssignedQPS       int        `json:"assignedQPS"`
	Score             *int64     `json:"score,omitempty"`
	EffectiveScore    *int64     `json:"effectiveScore,omitempty"`
	Phase             string     `json:"phase,omitempty"`
	ObservedAt        *time.Time `json:"observedAt,omitempty"`
	Freshness         string     `json:"freshness"`
	Pods              []Pod      `json:"pods"`
}

type TenantTraffic struct {
	Tenant             ResourceRef       `json:"tenant"`
	DisplayName        string            `json:"displayName"`
	Priority           string            `json:"priority"`
	RequestedQPS       int               `json:"requestedQPS"`
	AllocatedQPS       int               `json:"allocatedQPS"`
	AllocationBalanced bool              `json:"allocationBalanced"`
	Performance        Performance       `json:"performance"`
	ReadyReplicaCount  int               `json:"readyReplicaCount"`
	RuntimePhase       string            `json:"runtimePhase,omitempty"`
	Instances          []TrafficInstance `json:"instances"`
}

type Traffic struct {
	AsOf    time.Time       `json:"asOf"`
	Tenants []TenantTraffic `json:"tenants"`
}

type ClockCapabilities struct {
	CanSetRate            bool `json:"canSetRate"`
	CanPause              bool `json:"canPause"`
	CanSeek               bool `json:"canSeek"`
	SimulatorAcceleration bool `json:"simulatorAcceleration"`
}

type ClockState struct {
	ClockID               string            `json:"clockId"`
	Mode                  string            `json:"mode"`
	State                 string            `json:"state"`
	ServerTime            time.Time         `json:"serverTime"`
	ActualTime            time.Time         `json:"actualTime"`
	LogicalTime           time.Time         `json:"logicalTime"`
	SimulationTime        *time.Time        `json:"simulationTime"`
	ActualAnchor          time.Time         `json:"actualAnchor"`
	LogicalAnchor         time.Time         `json:"logicalAnchor"`
	OffsetMs              int64             `json:"offsetMs"`
	Rate                  float64           `json:"rate"`
	AppliedRate           float64           `json:"appliedRate"`
	ResourceVersion       string            `json:"resourceVersion,omitempty"`
	Converged             bool              `json:"converged"`
	SynchronizedInstances int               `json:"synchronizedInstances"`
	TotalInstances        int               `json:"totalInstances"`
	Version               string            `json:"version"`
	Authoritative         bool              `json:"authoritative"`
	MaxClientDriftMs      int64             `json:"maxClientDriftMs"`
	Capabilities          ClockCapabilities `json:"capabilities"`
}

type ProviderState struct {
	State      string    `json:"state"`
	ObservedAt time.Time `json:"observedAt"`
	Error      string    `json:"error,omitempty"`
	Retention  string    `json:"retention,omitempty"`
	Storage    string    `json:"storage,omitempty"`
}

type TimelineItem struct {
	ID        string         `json:"id"`
	Timestamp time.Time      `json:"timestamp"`
	Weight    int            `json:"weight"`
	Type      string         `json:"type"`
	Trigger   string         `json:"trigger"`
	Domain    string         `json:"domain"`
	Severity  string         `json:"severity"`
	Title     string         `json:"title"`
	Summary   string         `json:"summary"`
	Source    string         `json:"source"`
	Impact    map[string]int `json:"impact"`
	Tags      []string       `json:"tags"`
}

type Bootstrap struct {
	Cluster   ClusterInfo              `json:"cluster"`
	Clock     ClockState               `json:"clock"`
	Counts    map[string]int           `json:"counts"`
	Nodes     []ClusterNode            `json:"nodes"`
	Providers map[string]ProviderState `json:"providers"`
	Timeline  []TimelineItem           `json:"timeline"`
}

type ClusterInfo struct {
	Name          string    `json:"name"`
	Context       string    `json:"context,omitempty"`
	Version       string    `json:"version,omitempty"`
	Connected     bool      `json:"connected"`
	CacheSynced   bool      `json:"cacheSynced"`
	CacheSyncedAt time.Time `json:"cacheSyncedAt,omitempty"`
	NodeCount     int       `json:"nodeCount"`
	ReadyNodes    int       `json:"readyNodes"`
}

type CurrentSnapshot struct {
	CapturedAt    time.Time     `json:"capturedAt"`
	Configuration Configuration `json:"configuration"`
	Traffic       Traffic       `json:"traffic"`
	Workloads     Workloads     `json:"workloads"`
}

type StoredSnapshot struct {
	ID          string          `json:"id"`
	CapturedAt  time.Time       `json:"capturedAt"`
	LogicalTime time.Time       `json:"logicalTime"`
	Payload     json.RawMessage `json:"payload"`
}

type Overview struct {
	Availability  string                   `json:"availability"`
	AsOf          time.Time                `json:"asOf"`
	SnapshotID    string                   `json:"snapshotId,omitempty"`
	Clock         ClockState               `json:"clock"`
	Configuration Configuration            `json:"configuration"`
	Traffic       Traffic                  `json:"traffic"`
	Workloads     Workloads                `json:"workloads"`
	Metrics       map[string]MetricResult  `json:"metrics"`
	Traces        []TraceSummary           `json:"traces"`
	Freshness     map[string]ProviderState `json:"freshness"`
}

type MetricPoint struct {
	Time  time.Time `json:"time"`
	Value float64   `json:"value"`
}

type MetricSeries struct {
	Labels map[string]string `json:"labels"`
	Points []MetricPoint     `json:"points"`
}

type MetricResult struct {
	MetricID    string         `json:"metricId"`
	Unit        string         `json:"unit"`
	Start       time.Time      `json:"start"`
	End         time.Time      `json:"end"`
	StepSeconds int64          `json:"stepSeconds"`
	Series      []MetricSeries `json:"series"`
	ResultType  string         `json:"resultType"`
	Warnings    []string       `json:"warnings"`
	QueriedAt   time.Time      `json:"queriedAt"`
}

type TraceSummary struct {
	TraceID        string            `json:"traceId"`
	RootService    string            `json:"rootService"`
	RootOperation  string            `json:"rootOperation"`
	StartTime      time.Time         `json:"startTime"`
	DurationMs     float64           `json:"durationMs"`
	SpanCount      int               `json:"spanCount"`
	ErrorSpanCount int               `json:"errorSpanCount"`
	Entities       map[string]string `json:"entities"`
}

type SpanEvent struct {
	Name       string         `json:"name"`
	Time       time.Time      `json:"time"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

type Span struct {
	SpanID       string         `json:"spanId"`
	ParentSpanID string         `json:"parentSpanId,omitempty"`
	Service      string         `json:"service"`
	Operation    string         `json:"operation"`
	StartTime    time.Time      `json:"startTime"`
	DurationMs   float64        `json:"durationMs"`
	Status       string         `json:"status"`
	Attributes   map[string]any `json:"attributes"`
	Events       []SpanEvent    `json:"events"`
}

type TraceDetail struct {
	TraceID     string        `json:"traceId"`
	Spans       []Span        `json:"spans"`
	EntityLinks []ResourceRef `json:"entityLinks"`
}

type OperationReceipt struct {
	OperationID string                    `json:"operationId"`
	AcceptedAt  time.Time                 `json:"acceptedAt"`
	State       string                    `json:"state"`
	Results     []OperationResourceResult `json:"results"`
}

type OperationResourceResult struct {
	Ref             ResourceRef `json:"ref"`
	Action          string      `json:"action"`
	ResourceVersion string      `json:"resourceVersion,omitempty"`
	Convergence     string      `json:"convergence"`
	Error           string      `json:"error,omitempty"`
}

type ResourceChange struct {
	EventID         string          `json:"eventId"`
	OccurredAt      time.Time       `json:"occurredAt"`
	Operation       string          `json:"operation"`
	Ref             ResourceRef     `json:"ref"`
	ResourceVersion string          `json:"resourceVersion,omitempty"`
	Generation      int64           `json:"generation,omitempty"`
	Payload         json.RawMessage `json:"payload,omitempty"`
}

type AuditRecord struct {
	OperationID string          `json:"operationId"`
	OccurredAt  time.Time       `json:"occurredAt"`
	Actor       string          `json:"actor"`
	Action      string          `json:"action"`
	Ref         ResourceRef     `json:"ref"`
	Outcome     string          `json:"outcome"`
	RequestID   string          `json:"requestId"`
	Details     json.RawMessage `json:"details,omitempty"`
}
