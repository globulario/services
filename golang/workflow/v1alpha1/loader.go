package v1alpha1

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Loader struct{}

func NewLoader() *Loader { return &Loader{} }

// MinIOFetcher is an optional callback that loads a workflow definition
// by name from MinIO (globular-config bucket). When set, LoadFile tries
// MinIO first and falls back to disk if the definition isn't found.
// This indirection avoids a hard dependency of v1alpha1 on the config package.
var MinIOFetcher func(name string) ([]byte, error)

// workflowNameFromPath extracts the workflow name from a disk path like
// "/var/lib/globular/workflow/definitions/day0.bootstrap.yaml" → "day0.bootstrap"
func workflowNameFromPath(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(strings.TrimSuffix(base, ".yaml"), ".yml")
}

func (l *Loader) LoadFile(path string) (*WorkflowDefinition, error) {
	// Try MinIO first if a fetcher is configured (single source of truth)
	if MinIOFetcher != nil {
		name := workflowNameFromPath(path)
		if b, err := MinIOFetcher(name); err == nil && len(b) > 0 {
			def, derr := l.LoadBytes(b)
			if derr == nil {
				return def, nil
			}
			// MinIO returned bad data — fall through to disk
		}
	}
	// Fallback: read from disk
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read workflow definition %q: %w", path, err)
	}
	def, err := l.LoadBytes(b)
	if err != nil {
		return nil, fmt.Errorf("load workflow definition %q: %w", path, err)
	}
	return def, nil
}

func (l *Loader) LoadReader(r io.Reader) (*WorkflowDefinition, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read workflow definition: %w", err)
	}
	return l.LoadBytes(b)
}

func (l *Loader) LoadBytes(b []byte) (*WorkflowDefinition, error) {
	var def WorkflowDefinition
	trimmed := strings.TrimSpace(string(b))
	if trimmed == "" {
		return nil, fmt.Errorf("workflow definition is empty")
	}

	first := trimmed[0]
	switch first {
	case '{', '[':
		if err := json.Unmarshal(b, &def); err != nil {
			return nil, fmt.Errorf("decode workflow JSON: %w", err)
		}
	default:
		if err := yaml.Unmarshal(b, &def); err != nil {
			return nil, fmt.Errorf("decode workflow YAML: %w", err)
		}
	}

	if err := ValidateDefinition(&def); err != nil {
		return nil, err
	}
	return &def, nil
}

func LoadDirectory(dir string) ([]*WorkflowDefinition, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read definition dir %q: %w", dir, err)
	}

	loader := NewLoader()
	defs := make([]*WorkflowDefinition, 0)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext != ".yaml" && ext != ".yml" && ext != ".json" {
			continue
		}
		def, err := loader.LoadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		defs = append(defs, def)
	}

	sort.Slice(defs, func(i, j int) bool { return defs[i].Metadata.Name < defs[j].Metadata.Name })
	return defs, nil
}

func ValidateDefinition(def *WorkflowDefinition) error {
	if def == nil {
		return fmt.Errorf("workflow definition is nil")
	}
	if def.APIVersion != APIVersion {
		return fmt.Errorf("apiVersion must be %q, got %q", APIVersion, def.APIVersion)
	}
	if def.Kind != Kind {
		return fmt.Errorf("kind must be %q, got %q", Kind, def.Kind)
	}
	if strings.TrimSpace(def.Metadata.Name) == "" {
		return fmt.Errorf("metadata.name is required")
	}
	if len(def.Spec.Steps) == 0 {
		return fmt.Errorf("spec.steps must contain at least one step")
	}
	if err := validateStrategy(def.Spec.Strategy); err != nil {
		return err
	}
	if err := validateHooks(def); err != nil {
		return err
	}
	if err := validateSteps(def.Spec.Steps); err != nil {
		return err
	}
	return nil
}

func validateStrategy(s ExecutionStrategy) error {
	switch s.Mode {
	case StrategySingle, StrategyForeach, StrategyDAG:
	default:
		return fmt.Errorf("spec.strategy.mode must be one of %q, %q, %q", StrategySingle, StrategyForeach, StrategyDAG)
	}

	if s.Mode == StrategyForeach {
		if s.Collection == nil || strings.TrimSpace(s.Collection.String()) == "" {
			return fmt.Errorf("spec.strategy.collection is required for foreach mode")
		}
		if s.ItemName == nil || strings.TrimSpace(s.ItemName.String()) == "" {
			return fmt.Errorf("spec.strategy.itemName is required for foreach mode")
		}
	}
	if s.Concurrency != nil {
		if n, ok := s.Concurrency.IntValue(); ok && n <= 0 {
			return fmt.Errorf("spec.strategy.concurrency must be > 0")
		}
	}
	return nil
}

func validateHooks(def *WorkflowDefinition) error {
	for name, h := range map[string]*WorkflowHook{"onFailure": def.Spec.OnFailure, "onSuccess": def.Spec.OnSuccess} {
		if h == nil {
			continue
		}
		if err := validateActorAndAction(h.Actor, h.Action, name); err != nil {
			return err
		}
	}
	return nil
}

func validateSteps(steps []WorkflowStepSpec) error {
	ids := make(map[string]WorkflowStepSpec, len(steps))
	for i, s := range steps {
		loc := fmt.Sprintf("spec.steps[%d]", i)
		if strings.TrimSpace(s.ID) == "" {
			return fmt.Errorf("%s.id is required", loc)
		}
		if _, exists := ids[s.ID]; exists {
			return fmt.Errorf("duplicate step id %q", s.ID)
		}
		// Foreach group steps (with nested Steps) don't require actor/action.
		if len(s.Steps) == 0 {
			if err := validateActorAndAction(s.Actor, s.Action, loc); err != nil {
				return err
			}
		}
		if s.Retry != nil {
			if s.Retry.MaxAttempts <= 0 {
				return fmt.Errorf("%s.retry.maxAttempts must be > 0", loc)
			}
			if err := validateDurationScalar(loc+".retry.backoff", s.Retry.Backoff); err != nil {
				return err
			}
		}
		if err := validateDurationScalar(loc+".timeout", s.Timeout); err != nil {
			return err
		}
		if s.WaitFor != nil {
			if strings.TrimSpace(s.WaitFor.Condition) == "" {
				return fmt.Errorf("%s.waitFor.condition is required", loc)
			}
			if err := validateDurationScalar(loc+".waitFor.timeout", s.WaitFor.Timeout); err != nil {
				return err
			}
		}
		if s.When != nil {
			if err := validateCondition(*s.When, loc+".when"); err != nil {
				return err
			}
		}
		ids[s.ID] = s
	}

	for _, s := range steps {
		for _, dep := range s.DependsOn {
			if dep == s.ID {
				return fmt.Errorf("step %q cannot depend on itself", s.ID)
			}
			if _, ok := ids[dep]; !ok {
				return fmt.Errorf("step %q depends on unknown step %q", s.ID, dep)
			}
		}
	}
	if err := validateAcyclicGraph(ids); err != nil {
		return err
	}
	return nil
}

func validateCondition(c StepCondition, loc string) error {
	branches := 0
	if strings.TrimSpace(c.Expr) != "" {
		branches++
	}
	if len(c.AnyOf) > 0 {
		branches++
		for i, child := range c.AnyOf {
			if err := validateCondition(child, fmt.Sprintf("%s.anyOf[%d]", loc, i)); err != nil {
				return err
			}
		}
	}
	if len(c.AllOf) > 0 {
		branches++
		for i, child := range c.AllOf {
			if err := validateCondition(child, fmt.Sprintf("%s.allOf[%d]", loc, i)); err != nil {
				return err
			}
		}
	}
	if c.Not != nil {
		branches++
		if err := validateCondition(*c.Not, loc+".not"); err != nil {
			return err
		}
	}
	if branches == 0 {
		return fmt.Errorf("%s must define expr, anyOf, allOf or not", loc)
	}
	return nil
}

func validateDurationScalar(field string, s *ScalarString) error {
	if s == nil || strings.TrimSpace(s.String()) == "" {
		return nil
	}
	if s.IsExpression() {
		return nil
	}
	if _, err := time.ParseDuration(s.String()); err != nil {
		return fmt.Errorf("%s must be a valid duration or expression, got %q", field, s.String())
	}
	return nil
}

func validateActorAndAction(actor ActorType, action, loc string) error {
	switch actor {
	case ActorWorkflowService, ActorClusterController, ActorClusterDoctor, ActorNodeAgent, ActorInstaller, ActorRepository, ActorOperator, ActorCompute:
	default:
		return fmt.Errorf("%s.actor %q is not supported", loc, actor)
	}
	if strings.TrimSpace(action) == "" {
		return fmt.Errorf("%s.action is required", loc)
	}
	return nil
}

func validateAcyclicGraph(steps map[string]WorkflowStepSpec) error {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int, len(steps))
	var visit func(string) error
	visit = func(id string) error {
		switch color[id] {
		case gray:
			return fmt.Errorf("dependency cycle detected at step %q", id)
		case black:
			return nil
		}
		color[id] = gray
		for _, dep := range steps[id].DependsOn {
			if err := visit(dep); err != nil {
				return err
			}
		}
		color[id] = black
		return nil
	}
	for id := range steps {
		if color[id] == white {
			if err := visit(id); err != nil {
				return err
			}
		}
	}
	return nil
}
