package remediation

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
)

// Handler serves the remediation API.
type Handler struct {
	engine *Engine
	logger *log.Logger
}

// NewHandler creates a new remediation handler.
func NewHandler(engine *Engine, logger *log.Logger) *Handler {
	if logger == nil {
		logger = log.Default()
	}
	return &Handler{engine: engine, logger: logger}
}

// RegisterRoutes implements RouteRegistrar.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Global config
	mux.HandleFunc("GET /api/v1/remediation/config", h.handleGetConfig)
	mux.HandleFunc("PUT /api/v1/remediation/config", h.handleSaveConfig)

	// Allowed actions registry
	mux.HandleFunc("GET /api/v1/remediation/actions", h.handleListActions)
	mux.HandleFunc("POST /api/v1/remediation/actions", h.handleCreateAction)
	mux.HandleFunc("GET /api/v1/remediation/actions/{id}", h.handleGetAction)
	mux.HandleFunc("PUT /api/v1/remediation/actions/{id}", h.handleUpdateAction)
	mux.HandleFunc("DELETE /api/v1/remediation/actions/{id}", h.handleDeleteAction)

	// Attempts
	mux.HandleFunc("GET /api/v1/remediation/attempts", h.handleListAttempts)
	mux.HandleFunc("GET /api/v1/remediation/attempts/{id}", h.handleGetAttempt)

	// Per-check and per-incident views
	mux.HandleFunc("GET /api/v1/checks/{id}/remediations", h.handleCheckAttempts)
	mux.HandleFunc("GET /api/v1/incidents/{id}/remediations", h.handleIncidentAttempts)

	// Manual trigger
	mux.HandleFunc("POST /api/v1/checks/{id}/remediate", h.handleManualRemediate)

	// AI suggest
	mux.HandleFunc("POST /api/v1/remediation/suggest-command", h.handleSuggestCommand)
}

// ---------- Config ----------

func (h *Handler) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.engine.repo.GetConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "CONFIG_ERROR", err.Error())
		return
	}
	writeSuccess(w, http.StatusOK, cfg)
}

func (h *Handler) handleSaveConfig(w http.ResponseWriter, r *http.Request) {
	var cfg GlobalConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "invalid request body")
		return
	}
	if cfg.MaxConcurrent < 0 {
		writeError(w, http.StatusBadRequest, "VALIDATION", "maxConcurrent must be >= 0")
		return
	}
	if cfg.OutputLimitBytes < 0 {
		writeError(w, http.StatusBadRequest, "VALIDATION", "outputLimitBytes must be >= 0")
		return
	}
	if err := h.engine.repo.SaveConfig(cfg); err != nil {
		writeError(w, http.StatusInternalServerError, "SAVE_ERROR", err.Error())
		return
	}
	writeSuccess(w, http.StatusOK, cfg)
}

// ---------- Actions ----------

func (h *Handler) handleListActions(w http.ResponseWriter, r *http.Request) {
	actions, err := h.engine.repo.ListActions()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "LIST_ERROR", err.Error())
		return
	}
	writeSuccess(w, http.StatusOK, map[string]interface{}{
		"actions": actions,
		"total":   len(actions),
	})
}

func (h *Handler) handleCreateAction(w http.ResponseWriter, r *http.Request) {
	var action AllowedAction
	if err := json.NewDecoder(r.Body).Decode(&action); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "invalid request body")
		return
	}

	// Validate required fields
	if action.ID == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION", "id is required")
		return
	}
	if action.Name == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION", "name is required")
		return
	}
	if action.Type == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION", "type is required (command, ssh_command, http)")
		return
	}
	if action.Type != ActionCommand && action.Type != ActionSSHCommand && action.Type != ActionHTTP {
		writeError(w, http.StatusBadRequest, "VALIDATION", "type must be command, ssh_command, or http")
		return
	}
	if action.Type != ActionHTTP && action.Command == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION", "command is required for command/ssh_command types")
		return
	}
	if action.Type == ActionHTTP && action.URL == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION", "url is required for http type")
		return
	}
	if action.Risk == "" {
		action.Risk = RiskMedium
	}
	if action.TimeoutSeconds <= 0 {
		action.TimeoutSeconds = 30
	}

	if err := h.engine.repo.CreateAction(action); err != nil {
		writeError(w, http.StatusInternalServerError, "CREATE_ERROR", err.Error())
		return
	}
	writeSuccess(w, http.StatusCreated, action)
}

func (h *Handler) handleGetAction(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	action, err := h.engine.repo.GetAction(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	writeSuccess(w, http.StatusOK, action)
}

func (h *Handler) handleUpdateAction(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var action AllowedAction
	if err := json.NewDecoder(r.Body).Decode(&action); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "invalid request body")
		return
	}
	action.ID = id

	if action.Type != "" && action.Type != ActionCommand && action.Type != ActionSSHCommand && action.Type != ActionHTTP {
		writeError(w, http.StatusBadRequest, "VALIDATION", "type must be command, ssh_command, or http")
		return
	}

	if err := h.engine.repo.UpdateAction(action); err != nil {
		writeError(w, http.StatusInternalServerError, "UPDATE_ERROR", err.Error())
		return
	}
	writeSuccess(w, http.StatusOK, action)
}

func (h *Handler) handleDeleteAction(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.engine.repo.DeleteAction(id); err != nil {
		writeError(w, http.StatusInternalServerError, "DELETE_ERROR", err.Error())
		return
	}
	writeSuccess(w, http.StatusNoContent, nil)
}

// ---------- Attempts ----------

func (h *Handler) handleListAttempts(w http.ResponseWriter, r *http.Request) {
	filter := parseAttemptFilter(r)
	attempts, err := h.engine.repo.ListAttempts(filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "LIST_ERROR", err.Error())
		return
	}
	writeSuccess(w, http.StatusOK, map[string]interface{}{
		"attempts": attempts,
		"total":    len(attempts),
	})
}

func (h *Handler) handleGetAttempt(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	attempt, err := h.engine.repo.GetAttempt(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	writeSuccess(w, http.StatusOK, attempt)
}

func (h *Handler) handleCheckAttempts(w http.ResponseWriter, r *http.Request) {
	checkID := r.PathValue("id")
	filter := parseAttemptFilter(r)
	filter.CheckID = checkID
	attempts, err := h.engine.repo.ListAttempts(filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "LIST_ERROR", err.Error())
		return
	}
	writeSuccess(w, http.StatusOK, map[string]interface{}{
		"attempts": attempts,
		"total":    len(attempts),
	})
}

func (h *Handler) handleIncidentAttempts(w http.ResponseWriter, r *http.Request) {
	incidentID := r.PathValue("id")
	filter := parseAttemptFilter(r)
	filter.IncidentID = incidentID
	attempts, err := h.engine.repo.ListAttempts(filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "LIST_ERROR", err.Error())
		return
	}
	writeSuccess(w, http.StatusOK, map[string]interface{}{
		"attempts": attempts,
		"total":    len(attempts),
	})
}

// ---------- Manual Trigger ----------

func (h *Handler) handleManualRemediate(w http.ResponseWriter, r *http.Request) {
	checkID := r.PathValue("id")

	var req struct {
		IncidentID string `json:"incidentId"`
		Actor      string `json:"actor"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "invalid request body")
		return
	}
	if req.Actor == "" {
		req.Actor = "manual"
	}

	// The caller must supply enough info. In practice this is called from the
	// frontend which builds CheckInfo from the existing check config.
	// For the handler we need the check resolver to be wired in.
	if h.engine.checkResolver == nil {
		writeError(w, http.StatusInternalServerError, "NOT_CONFIGURED", "check resolver not wired")
		return
	}

	info, err := h.engine.checkResolver(checkID)
	if err != nil {
		writeError(w, http.StatusNotFound, "CHECK_NOT_FOUND", err.Error())
		return
	}
	if info.Ref.ActionRef == "" {
		writeError(w, http.StatusBadRequest, "NO_REMEDIATION", "check has no remediation action configured")
		return
	}
	if req.IncidentID != "" {
		info.IncidentID = req.IncidentID
	}

	attempt, err := h.engine.ManualRemediate(info, req.Actor)
	if err != nil {
		writeError(w, http.StatusConflict, "REMEDIATION_BLOCKED", err.Error())
		return
	}
	writeSuccess(w, http.StatusOK, attempt)
}

// ---------- AI Suggest ----------

func (h *Handler) handleSuggestCommand(w http.ResponseWriter, r *http.Request) {
	var req SuggestCommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "invalid request body")
		return
	}
	if req.CheckType == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION", "checkType is required")
		return
	}

	suggestion, err := h.engine.SuggestCommand(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "AI_ERROR", err.Error())
		return
	}
	writeSuccess(w, http.StatusOK, map[string]string{
		"command": suggestion,
	})
}

// ---------- Helpers ----------

func parseAttemptFilter(r *http.Request) AttemptFilter {
	q := r.URL.Query()
	f := AttemptFilter{
		CheckID:    q.Get("checkId"),
		IncidentID: q.Get("incidentId"),
	}
	if v, err := strconv.Atoi(q.Get("limit")); err == nil && v > 0 {
		f.Limit = v
	}
	if v, err := strconv.Atoi(q.Get("offset")); err == nil && v >= 0 {
		f.Offset = v
	}
	return f
}

func writeSuccess(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data == nil {
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    data,
	})
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
