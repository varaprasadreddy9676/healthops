package monitoring

import "time"

func buildDashboardSnapshot(state State) DashboardSnapshot {
	return DashboardSnapshot{
		State:       cloneState(state),
		Summary:     buildSummary(state.Checks, state.Results, &state.LastRunAt),
		GeneratedAt: time.Now().UTC(),
	}
}
