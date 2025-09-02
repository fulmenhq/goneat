---
title: "Enterprise Architecture Patterns"
description: "Design patterns and best practices for enterprise-scale goneat deployments"
author: "@arch-eagle"
date: "2025-09-01"
last_updated: "2025-09-01"
status: "draft"
tags: ["architecture", "enterprise", "scalability", "security"]
---

# Enterprise Architecture Patterns for Goneat

This document outlines architectural patterns and best practices for deploying goneat in large enterprise environments with complex requirements around scale, security, and compliance.

## Overview

Enterprise deployments of goneat face unique challenges:

- **Scale**: Repositories with millions of lines of code across thousands of modules
- **Security**: Strict compliance requirements and audit trails
- **Integration**: Complex CI/CD pipelines and tool ecosystems
- **Governance**: Policy enforcement across diverse teams and projects

### JSON-First SSOT Architecture with JSON Schema 2020-12

As documented in the [README](../../README.md#json-first-ssot), goneat follows a **JSON-first Single Source of Truth** design principle, backed by rigorous **JSON Schema Draft 2020-12** specifications. This is fundamental to enterprise deployments:

- **Structured Data with Schema Validation**: All JSON output conforms to published schemas ([work manifest schema](../../schemas/work/work-manifest-v1.0.0.yaml), [config schema](../../schemas/config/goneat-config-v1.0.0.yaml))
- **Machine-Readable Primary Format**: Structured JSON with schema URIs enables automatic validation and type generation
- **Derived Human Formats**: Markdown, HTML, and console outputs are generated from the validated JSON ([formatter implementation](../../internal/assess/formatter.go))
- **AI/Human Parity**: Schema-defined structures ensure consistent interpretation by both AI agents and human developers
- **Automation-Ready**: CI/CD systems consume JSON with confidence in structure and type safety

This architecture enables:

1. **Schema-Driven Integration**: Downstream systems can validate inputs and generate type-safe clients
2. **Contract-Based Development**: JSON Schemas serve as contracts between goneat and consuming systems
3. **Versioned Data Evolution**: Schema versioning (`$version: 1.0.0`) enables backward-compatible changes
4. **Lossless Data Pipeline**: Full fidelity from assessment to reporting with schema validation at each step
5. **Efficient Agentic Work**: AI agents can leverage schemas for accurate data parsing and generation
6. **Audit Compliance**: Schema-validated data ensures consistency for compliance systems

The project uses:

- **JSON Schema Draft 2020-12**: Latest specification for maximum expressiveness
- **Schema URIs**: Published at `https://schemas.goneat.dev/` for discovery
- **Go JSON Schema Library**: `github.com/xeipuuv/gojsonschema` for runtime validation

See also:

- [Assessment types](../../internal/assess/types.go) - Go structs that map to JSON schemas
- [Assessment Workflow Architecture](./assess-workflow.md#4-dual-output-formats-json-first)
- [Concurrency Model](./concurrency-model.md) - JSON aggregation patterns
- [Security Architecture](./security-and-secrets-scanning-architecture.md) - normalized issue schema

## Core Architectural Principles

### 1. Distributed Execution Architecture

For massive monorepos and multi-repository scenarios, goneat supports distributed execution patterns:

```yaml
# Enterprise execution configuration
execution:
  mode: distributed # local | distributed | hybrid
  orchestrator:
    type: kubernetes # kubernetes | nomad | custom
    endpoint: "https://k8s-control.corp.example.com"
  worker_pools:
    - name: high-memory
      resources:
        memory: 32Gi
        cpu: 8
      selector:
        workload: "large-files"
    - name: standard
      resources:
        memory: 8Gi
        cpu: 4
      selector:
        workload: "default"
```

### 2. Schema-Driven Enterprise Integration

Enterprise systems require predictable, versioned data contracts:

```yaml
# Enterprise schema extension example
$schema: https://json-schema.org/draft/2020-12/schema
$id: https://schemas.corp.example.com/goneat-extensions/v1.0.0
$ref: https://schemas.goneat.dev/config/v1.0.0 # Extend base schema

# Add enterprise-specific properties
properties:
  enterprise:
    type: object
    properties:
      compliance:
        type: object
        properties:
          frameworks:
            type: array
            items:
              enum: ["sox", "pci-dss", "hipaa", "iso27001"]
          retention_days:
            type: integer
            minimum: 90
      integration:
        type: object
        properties:
          jira:
            type: object
            properties:
              enabled:
                type: boolean
              project_key:
                type: string
              issue_type:
                type: string
                enum: ["bug", "security-vulnerability", "technical-debt"]
```

This enables:

- **Type-safe client generation** in any language
- **Automatic API documentation** from schemas
- **Contract testing** between services
- **Schema registry** for governance

### 3. Hierarchical Configuration Management

Enterprise environments require layered configuration with clear precedence, all validated against schemas:

```
┌─────────────────────────────┐
│   Organization Config       │  (e.g., s3://corp-config/goneat/org.yaml)
└──────────────┬──────────────┘  Schema: corp-config-v1.0.0
               │
┌──────────────▼──────────────┐
│      Team Config           │  (e.g., git://configs/team-backend.yaml)
└──────────────┬──────────────┘  Schema: team-config-v1.0.0
               │
┌──────────────▼──────────────┐
│    Project Config          │  (e.g., .goneat.yaml in repo)
└──────────────┬──────────────┘  Schema: goneat-config-v1.0.0
               │
┌──────────────▼──────────────┐
│     User Config            │  (e.g., ~/.goneat/config.yaml)
└─────────────────────────────┘  Schema: user-preferences-v1.0.0
```

Each layer is validated against its schema before merging, ensuring configuration integrity.

### 4. Policy-as-Code Framework

Enterprise policy enforcement through declarative policies:

```yaml
# policies/security-baseline.yaml
apiVersion: policy.goneat.io/v1
kind: SecurityPolicy
metadata:
  name: enterprise-baseline
  namespace: global
spec:
  rules:
    - id: no-high-vulns-in-prod
      description: "Production code must have no high or critical vulnerabilities"
      condition:
        environment: production
      enforcement:
        security:
          fail_on: high
          required_tools: ["gosec", "govulncheck"]

    - id: format-consistency
      description: "All code must follow organization formatting standards"
      enforcement:
        format:
          required: true
          finalizer:
            ensure_eof: true
            normalize_line_endings: "lf"
```

### 5. Audit and Compliance Architecture

Comprehensive audit trail for compliance requirements, building on goneat's [JSON-first assessment types](../../internal/assess/types.go):

```go
// pkg/audit/types.go
type AuditEvent struct {
    ID           string    `json:"id"`
    Timestamp    time.Time `json:"timestamp"`
    Actor        Actor     `json:"actor"`
    Action       string    `json:"action"`
    Resource     Resource  `json:"resource"`
    Result       Result    `json:"result"`
    PolicyViolations []PolicyViolation `json:"policy_violations,omitempty"`
    Attestation  *Attestation      `json:"attestation,omitempty"`

    // Embed the full assessment report for complete audit trail
    AssessmentReport *assess.AssessmentReport `json:"assessment_report,omitempty"`
}

type Attestation struct {
    Signature    string `json:"signature"`
    Certificate  string `json:"certificate"`
    Algorithm    string `json:"algorithm"`
}
```

This ensures all audit events contain the complete JSON assessment data, maintaining the SSOT principle for compliance reporting.

### 6. Caching and Performance Optimization

Multi-tier caching architecture for large-scale deployments:

```yaml
# Cache configuration
cache:
  enabled: true
  tiers:
    - name: local
      type: filesystem
      path: "/var/cache/goneat"
      max_size: 10Gi
      ttl: 24h

    - name: shared
      type: redis
      endpoints:
        - "redis-cluster.corp.example.com:6379"
      ttl: 7d

    - name: persistent
      type: s3
      bucket: "goneat-cache-prod"
      prefix: "v1/cache/"
      ttl: 30d

  strategies:
    format_results:
      key_pattern: "format:{file_hash}:{config_hash}"
      tiers: ["local", "shared"]

    security_scans:
      key_pattern: "security:{module}:{tool}:{version}"
      tiers: ["local", "shared", "persistent"]
```

## Implementation Patterns

### 1. Sharded Execution for Large Repositories

Building on the existing [security runner sharding implementation](../../internal/assess/security_runner.go) which already demonstrates enterprise-scale patterns:

```go
// pkg/shard/strategy.go
type ShardingStrategy interface {
    // Partition work into shards for parallel execution
    Partition(ctx context.Context, workload Workload) ([]Shard, error)

    // Estimate optimal shard count based on workload characteristics
    EstimateShardCount(workload Workload) int

    // Merge results from multiple shards - maintains JSON structure
    MergeResults(results []ShardResult) (*assess.AssessmentReport, error)
}

// Implementations following security_runner.go patterns
type ModuleShardingStrategy struct{}  // Shard by Go modules (see listGoPackageDirs)
type SizeBasedShardingStrategy struct{} // Shard by file size distribution
type DependencyAwareShardingStrategy struct{} // Respect dependency boundaries
```

The security runner already implements:

- Multi-module discovery ([findModuleDirs](../../internal/assess/security_runner.go#L325))
- Package-level sharding ([listGoPackageDirs](../../internal/assess/security_runner.go#L301))
- Worker pool management with configurable concurrency
- `.goneatignore` pattern support

### 2. Remote Execution Framework

```go
// pkg/remote/executor.go
type RemoteExecutor interface {
    // Submit work to remote execution environment
    Submit(ctx context.Context, job Job) (JobID, error)

    // Monitor job progress
    Status(ctx context.Context, id JobID) (JobStatus, error)

    // Retrieve results
    Results(ctx context.Context, id JobID) (Result, error)

    // Cancel running job
    Cancel(ctx context.Context, id JobID) error
}

// Kubernetes implementation
type KubernetesExecutor struct {
    client    kubernetes.Interface
    namespace string
    image     string
}
```

### 3. Policy Enforcement Engine

```go
// pkg/policy/engine.go
type PolicyEngine struct {
    loader   PolicyLoader
    evaluator PolicyEvaluator
    enforcer PolicyEnforcer
}

func (e *PolicyEngine) Evaluate(ctx context.Context, target Target) (*PolicyResult, error) {
    // Load applicable policies
    policies, err := e.loader.LoadPolicies(ctx, target)
    if err != nil {
        return nil, err
    }

    // Evaluate policies
    violations := []Violation{}
    for _, policy := range policies {
        if result := e.evaluator.Evaluate(policy, target); !result.Compliant {
            violations = append(violations, result.Violations...)
        }
    }

    // Enforce based on policy configuration
    return e.enforcer.Enforce(violations)
}
```

## AI/ML Integration via JSON Schema

The JSON Schema 2020-12 foundation enables sophisticated AI/ML integrations:

### 1. Schema-Aware AI Agents

```typescript
// Generated TypeScript types from JSON Schema
import { AssessmentReport, Issue, IssueSeverity } from "@goneat/types";

class GoneatAIAgent {
  private schemaValidator: JSONSchema;

  async analyzeCodebase(path: string): Promise<AssessmentReport> {
    // Run goneat with JSON output
    const rawOutput = await exec("goneat assess --format=json", { cwd: path });

    // Validate against schema
    const report = this.schemaValidator.validate<AssessmentReport>(
      rawOutput,
      "https://schemas.goneat.dev/assessment-report/v1.0.0",
    );

    // AI can now work with strongly-typed data
    return this.prioritizeIssues(report);
  }

  prioritizeIssues(report: AssessmentReport): AssessmentReport {
    // ML model trained on schema-structured data
    const model = await tf.loadLayersModel("issue-priority-model");

    // Transform issues to feature vectors using schema properties
    const features = report.categories.security.issues.map((issue) => [
      this.severityToNumber(issue.severity),
      issue.auto_fixable ? 1 : 0,
      this.categoryToVector(issue.category),
      // ... other schema-defined properties
    ]);

    // Get predictions
    const priorities = model.predict(features);

    // Return modified report maintaining schema structure
    return { ...report, ml_priorities: priorities };
  }
}
```

### 2. Training Data Generation

```python
# Python schema-based training data generator
from jsonschema import validate
import goneat_schemas

class TrainingDataGenerator:
    def __init__(self):
        self.schema = goneat_schemas.load('assessment-report-v1.0.0')

    def generate_training_data(self, historical_reports: List[dict]) -> pd.DataFrame:
        """Convert schema-validated reports to ML training data"""

        validated_reports = []
        for report in historical_reports:
            # Validate each report against schema
            validate(instance=report, schema=self.schema)
            validated_reports.append(report)

        # Extract features using schema structure
        features = []
        for report in validated_reports:
            for category, result in report['categories'].items():
                for issue in result['issues']:
                    features.append({
                        'severity': issue['severity'],
                        'category': issue['category'],
                        'sub_category': issue.get('sub_category', ''),
                        'auto_fixable': issue['auto_fixable'],
                        'file_extension': Path(issue['file']).suffix,
                        'estimated_time_minutes': issue.get('estimated_time', 0) / 60,
                        # Schema guarantees these fields exist
                    })

        return pd.DataFrame(features)
```

### 3. Automated Remediation with Schema Validation

```go
// pkg/ai/remediation.go
type AIRemediationEngine struct {
    schemaValidator *gojsonschema.Schema
    llmClient       LLMClient
}

func (e *AIRemediationEngine) Generatefix(issue assess.Issue) (*Remediation, error) {
    // Generate fix using LLM with schema context
    prompt := fmt.Sprintf(`
        Given this issue (JSON Schema: %s):
        %s

        Generate a remediation that maintains schema compliance.
        The output must validate against the remediation schema.
    `, issue.SchemaURI(), issue.ToJSON())

    response := e.llmClient.Complete(prompt)

    // Validate LLM output against remediation schema
    result, err := e.schemaValidator.Validate(gojsonschema.NewStringLoader(response))
    if err != nil {
        return nil, fmt.Errorf("LLM output validation failed: %w", err)
    }

    if !result.Valid() {
        // Feed validation errors back to LLM for correction
        return e.retryWithSchemaErrors(response, result.Errors())
    }

    var remediation Remediation
    json.Unmarshal([]byte(response), &remediation)
    return &remediation, nil
}
```

This schema-driven approach enables:

- **Type-safe AI integrations** across languages
- **Consistent training data** for ML models
- **Validated LLM outputs** that conform to expected structures
- **Cross-tool compatibility** for AI/ML pipelines

## Enterprise Integration Patterns

### 1. CI/CD Pipeline Integration

Following the [hooks architecture](./hooks-command-architecture.md) and [environment variables SSOT](../environment-variables.md):

```yaml
# .gitlab-ci.yml example
goneat-assess:
  stage: quality
  image: registry.corp.example.com/goneat:enterprise-v1
  variables:
    GONEAT_HOOK_OUTPUT: json # JSON-first for CI parsing
    GONEAT_SECURITY_FAIL_ON: high
    GONEAT_MAX_ISSUES_DISPLAY: 100 # Limit console output, JSON has all
  script:
    - goneat assess --mode=distributed --policy=policies/prod-baseline.yaml --format=json > goneat-full.json
    - goneat assess --mode=distributed --policy=policies/prod-baseline.yaml --format=junit > goneat-junit.xml
  artifacts:
    reports:
      junit: goneat-junit.xml
    paths:
      - goneat-full.json # Complete JSON for downstream analysis
```

The JSON output can be consumed by:

- Dashboard systems for metrics aggregation
- Policy engines for compliance validation
- AI/ML systems for trend analysis
- Issue tracking systems for automated ticket creation

### 2. SIEM Integration

```go
// pkg/integrations/siem/client.go
type SIEMClient interface {
    // Send security events to SIEM
    SendEvent(event SecurityEvent) error

    // Batch send for efficiency
    SendBatch(events []SecurityEvent) error
}

// Splunk implementation
type SplunkClient struct {
    endpoint string
    token    string
    index    string
}
```

### 3. Service Mesh Integration

```yaml
# Envoy sidecar configuration for goneat service
static_resources:
  listeners:
    - name: goneat_listener
      address:
        socket_address:
          address: 0.0.0.0
          port_value: 8080
      filter_chains:
        - filters:
            - name: envoy.filters.network.http_connection_manager
              typed_config:
                "@type": type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager
                codec_type: AUTO
                stat_prefix: goneat_ingress
                route_config:
                  name: goneat_route
                  virtual_hosts:
                    - name: goneat_service
                      domains: ["*"]
                      routes:
                        - match:
                            prefix: "/api/v1/assess"
                          route:
                            cluster: goneat_cluster
                            timeout: 300s # Long timeout for large assessments
```

## Deployment Patterns

### 1. Kubernetes Operator Pattern

```go
// pkg/operator/controller.go
type GoneatController struct {
    client     client.Client
    scheme     *runtime.Scheme
    recorder   record.EventRecorder
}

func (r *GoneatController) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
    // Reconcile GoneatAssessment custom resources
    assessment := &v1.GoneatAssessment{}
    if err := r.client.Get(ctx, req.NamespacedName, assessment); err != nil {
        return reconcile.Result{}, client.IgnoreNotFound(err)
    }

    // Create or update assessment job
    job := r.constructJob(assessment)
    if err := r.client.Create(ctx, job); err != nil {
        return reconcile.Result{}, err
    }

    return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
}
```

### 2. Multi-Region Deployment

```yaml
# terraform/modules/goneat/main.tf
resource "kubernetes_deployment" "goneat_workers" {
for_each = var.regions

metadata {
name      = "goneat-worker-${each.key}"
namespace = "goneat-system"
labels = {
app    = "goneat"
region = each.key
tier   = "worker"
}
}

spec {
replicas = each.value.worker_count

selector {
match_labels = {
app    = "goneat"
region = each.key
}
}

template {
metadata {
labels = {
app    = "goneat"
region = each.key
}
}

spec {
container {
image = "goneat:enterprise-${var.version}"
name  = "worker"

resources {
limits = {
cpu    = each.value.cpu_limit
memory = each.value.memory_limit
}
requests = {
cpu    = each.value.cpu_request
memory = each.value.memory_request
}
}

env {
name  = "GONEAT_REGION"
value = each.key
}

env {
name  = "GONEAT_CACHE_ENDPOINT"
value = each.value.cache_endpoint
}
}
}
}
}
}
```

## Monitoring and Observability

### 1. Metrics Architecture

```go
// pkg/metrics/collector.go
type MetricsCollector struct {
    assessmentDuration *prometheus.HistogramVec
    issuesFound        *prometheus.CounterVec
    policyViolations   *prometheus.CounterVec
    cacheHitRate       *prometheus.GaugeVec
}

func NewMetricsCollector() *MetricsCollector {
    return &MetricsCollector{
        assessmentDuration: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name:    "goneat_assessment_duration_seconds",
                Help:    "Duration of assessments",
                Buckets: prometheus.ExponentialBuckets(1, 2, 10),
            },
            []string{"command", "category", "status"},
        ),
        // ... other metrics
    }
}
```

### 2. Distributed Tracing

```go
// pkg/tracing/tracer.go
func InitTracing(serviceName string) (opentracing.Tracer, io.Closer, error) {
    cfg := &config.Configuration{
        ServiceName: serviceName,
        Sampler: &config.SamplerConfig{
            Type:  "adaptive",
            Param: 1.0,
        },
        Reporter: &config.ReporterConfig{
            LogSpans:           true,
            CollectorEndpoint:  os.Getenv("JAEGER_COLLECTOR_ENDPOINT"),
            BufferFlushInterval: 1 * time.Second,
        },
    }

    return cfg.NewTracer(
        config.Logger(jaeger.StdLogger),
        config.Metrics(metrics.NullFactory),
    )
}
```

## Security Considerations

### 1. Supply Chain Security

```yaml
# .goneat/sbom-config.yaml
sbom:
  format: spdx # spdx | cyclonedx
  output: goneat-sbom.json
  sign: true
  signing_key: /secrets/sbom-signing-key
  include:
    - dependencies
    - build_tools
    - runtime_environment
```

### 2. Zero Trust Architecture

```go
// pkg/auth/validator.go
type TokenValidator struct {
    jwks   *keyfunc.JWKS
    issuer string
}

func (v *TokenValidator) Validate(tokenString string) (*Claims, error) {
    token, err := jwt.ParseWithClaims(tokenString, &Claims{}, v.jwks.Keyfunc)
    if err != nil {
        return nil, fmt.Errorf("invalid token: %w", err)
    }

    claims := token.Claims.(*Claims)
    if claims.Issuer != v.issuer {
        return nil, fmt.Errorf("invalid issuer")
    }

    return claims, nil
}
```

## Migration Strategies

### 1. Phased Rollout

1. **Phase 1**: Shadow mode - run alongside existing tools
2. **Phase 2**: Pilot teams - early adopters with support
3. **Phase 3**: Gradual migration - team by team
4. **Phase 4**: Full deployment - organization-wide

### 2. Compatibility Bridge

```go
// pkg/compat/adapter.go
type LegacyToolAdapter interface {
    // Convert legacy tool output to goneat format
    Convert(input io.Reader) (*assess.Report, error)

    // Validate compatibility
    IsCompatible(version string) bool
}
```

## Best Practices

1. **Start Small**: Begin with format/lint, add security scanning gradually
2. **Cache Aggressively**: Use distributed caching for repeated operations
3. **Monitor Everything**: Comprehensive metrics and tracing from day one
4. **Policy Gradual**: Start with warnings, move to enforcement over time
5. **Automate Migration**: Provide tools to convert existing configurations

## Roadmap

### Phase 1: Foundation (Current)

- [x] Basic sharding for security scans ([security_runner.go](../../internal/assess/security_runner.go))
- [x] JSON-first output ([types.go](../../internal/assess/types.go), [formatter.go](../../internal/assess/formatter.go))
- [x] Environment variable configuration ([environment-variables.md](../environment-variables.md))
- [x] Basic ignore patterns (`.goneatignore`)
- [ ] Hierarchical configuration ([hierarchy.go](../../pkg/config/hierarchy.go) - started)
- [ ] Basic policy framework

### Phase 2: Scale (Q1 2025)

- [ ] Distributed execution framework
- [ ] Advanced caching architecture ([config.go](../../pkg/config/config.go) has cache dir support)
- [ ] Kubernetes operator
- [ ] SIEM integration
- [ ] Extended sharding to format/lint operations

### Phase 3: Enterprise (Q2 2025)

- [ ] Multi-region support
- [ ] Advanced policy engine with [feature gates](../configuration/feature-gates.md)
- [ ] Supply chain security (SBOM generation)
- [ ] Enterprise support portal
- [ ] Remote configuration sources (S3, Git)

### Phase 4: Innovation (Q3 2025)

- [ ] ML-powered issue prediction
- [ ] Automated remediation with AI agents
- [ ] Cross-language unification (Python, TypeScript, Rust)
- [ ] Cloud-native SaaS offering

## Related Documentation

- [Assessment Workflow Architecture](./assess-workflow.md) - Core assessment patterns
- [Concurrency Model](./concurrency-model.md) - Parallel execution patterns
- [Security Architecture](./security-and-secrets-scanning-architecture.md) - Security scanning patterns
- [Hooks Architecture](./hooks-command-architecture.md) - Git hooks integration
- [Feature Gates](../configuration/feature-gates.md) - Configuration management
- [Environment Variables](../environment-variables.md) - SSOT for env config
- [User Guide](../user-guide/commands/) - Command documentation

---

Generated by @arch-eagle under supervision of @3leapsdave
