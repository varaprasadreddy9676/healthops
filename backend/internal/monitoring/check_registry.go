package monitoring

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// CheckExecutor is the plugin contract for a check type.
//
// New check types are added by:
//  1. Implementing this interface in a file like `check_<type>.go`.
//  2. Registering an instance via RegisterCheckExecutor() in an init() function.
//  3. (Optional) Adding a typed config block to CheckConfig if the check needs
//     structured per-type options. Simple checks can reuse existing fields like
//     Target/Host/Port.
//
// The registry is the single source of truth for all check types.
// Built-in types (api, tcp, process, command, log, mysql, ssh, ssl, dns,
// ping, domain, heartbeat) are registered via init() functions in their
// respective check_*.go files.
type CheckExecutor interface {
	// Type returns the lowercase string used in CheckConfig.Type (e.g. "ssl").
	// Must be stable and unique across the binary.
	Type() string

	// ApplyDefaults populates default values on the check. Called during config
	// load and API mutations. Must be safe to call multiple times.
	ApplyDefaults(check *CheckConfig)

	// Validate verifies the check is well-formed. Called during config load and
	// on API mutations. Should return a clear, user-facing error.
	Validate(check *CheckConfig, cfg *Config) error

	// Execute runs the check. Implementations should populate result.Metrics
	// (gauge-style float64 values) and return a non-nil error to mark the
	// check critical. To produce a warning instead, set result.Status="warning"
	// and result.Healthy=false and return nil.
	//
	// The runner is passed so executors can access shared infrastructure
	// (e.g. r.resolveServer(), r.client). Executors must not mutate runner state.
	Execute(ctx context.Context, r *Runner, check CheckConfig, result *CheckResult) error
}

var (
	checkRegistryMu sync.RWMutex
	checkRegistry   = map[string]CheckExecutor{}
)

// RegisterCheckExecutor registers a check executor. Safe to call from init().
// Panics on nil, empty type, or duplicate registration — these are programmer
// errors that should fail at process startup.
func RegisterCheckExecutor(exec CheckExecutor) {
	if exec == nil {
		panic("monitoring: RegisterCheckExecutor called with nil executor")
	}
	t := exec.Type()
	if t == "" {
		panic("monitoring: CheckExecutor.Type() returned empty string")
	}
	checkRegistryMu.Lock()
	defer checkRegistryMu.Unlock()
	if _, exists := checkRegistry[t]; exists {
		panic(fmt.Sprintf("monitoring: check executor %q already registered", t))
	}
	checkRegistry[t] = exec
}

// LookupCheckExecutor returns the registered executor for a type, if any.
func LookupCheckExecutor(checkType string) (CheckExecutor, bool) {
	checkRegistryMu.RLock()
	defer checkRegistryMu.RUnlock()
	exec, ok := checkRegistry[checkType]
	return exec, ok
}

// RegisteredCheckTypes returns a sorted list of registered check type names.
// Useful for diagnostics and UI dropdowns.
func RegisteredCheckTypes() []string {
	checkRegistryMu.RLock()
	defer checkRegistryMu.RUnlock()
	out := make([]string, 0, len(checkRegistry))
	for t := range checkRegistry {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}
