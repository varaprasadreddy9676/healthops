// Package evidence implements the Evidence Backbone for AI-native incident
// intelligence. It defines the common signal envelope (SignalEvent), incident
// timeline events (IncidentEvent), the EvidenceProvider registry, the AI
// context builder, and the deterministic confidence scorer.
//
// Phase 1 ships providers for: check results, MySQL metrics, server metrics,
// audit events, and incident history. Later phases add logs, deployments,
// SLO burn, and similar-incident search without modifying the context builder.
package evidence
