package evidence

// ComputeConfidence calculates the deterministic, evidence-weighted confidence
// score per the roadmap formula:
//
//	confidence = clip(
//	    0.2 * has_deployment_correlation
//	  + 0.2 * has_metric_anomaly
//	  + 0.2 * has_log_fingerprint_spike
//	  + 0.2 * has_similar_past_incident_with_same_root_cause
//	  + 0.2 * evidence_count_normalized,
//	  0, 1
//	)
//
// Phase 1 can only contribute metric anomaly, incident history, and evidence
// count. Deployment correlation and log fingerprint spike arrive in later
// phases, reducing the max achievable score proportionally.
func ComputeConfidence(evidence *CollectedEvidence) ConfidenceScore {
	bd := ConfidenceBreakdown{
		EvidenceCount:           len(evidence.Events),
		AvailableCategories:     len(evidence.AvailableCategories),
		TotalPossibleCategories: len(evidence.AvailableCategories) + len(evidence.MissingCategories),
	}

	// evidence_count_normalized = min(evidence_count / 10, 1.0)
	ecn := float64(bd.EvidenceCount) / 10.0
	if ecn > 1.0 {
		ecn = 1.0
	}
	bd.EvidenceCountNormalized = ecn

	// Check for metric anomaly (server or MySQL with warning/critical severity)
	for _, e := range evidence.Events {
		if (e.Type == SignalTypeServer || e.Type == SignalTypeMySQL) &&
			(e.Severity == "warning" || e.Severity == "critical") {
			bd.HasMetricAnomaly = true
			break
		}
	}

	// Check for similar past incidents
	for _, e := range evidence.Events {
		if e.Type == SignalTypeCheck && e.Source == "incident_manager" {
			bd.HasSimilarPastIncident = true
			break
		}
	}

	// Deployment correlation — not available until Phase 3
	bd.HasDeploymentCorrelation = false

	// Log fingerprint spike — not available until Phase 2
	bd.HasLogFingerprintSpike = false

	// Compute score
	score := 0.0
	if bd.HasDeploymentCorrelation {
		score += 0.2
	}
	if bd.HasMetricAnomaly {
		score += 0.2
	}
	if bd.HasLogFingerprintSpike {
		score += 0.2
	}
	if bd.HasSimilarPastIncident {
		score += 0.2
	}
	score += 0.2 * bd.EvidenceCountNormalized

	// Clip to [0, 1]
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	// Map to band
	band := "low"
	if score > 0.75 {
		band = "high"
	} else if score >= 0.4 {
		band = "medium"
	}

	return ConfidenceScore{
		Score:     score,
		Band:      band,
		Breakdown: bd,
	}
}
