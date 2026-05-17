package monitoring

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// MaintenanceAPIHandler handles maintenance window CRUD.
type MaintenanceAPIHandler struct {
	store *MaintenanceStore
}

// NewMaintenanceAPIHandler creates a maintenance API handler.
func NewMaintenanceAPIHandler(store *MaintenanceStore) *MaintenanceAPIHandler {
	return &MaintenanceAPIHandler{store: store}
}

// RegisterRoutes registers maintenance window routes.
func (h *MaintenanceAPIHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/maintenance", h.handleList)
	mux.HandleFunc("POST /api/v1/maintenance", h.handleCreate)
	mux.HandleFunc("GET /api/v1/maintenance/{id}", h.handleGet)
	mux.HandleFunc("PUT /api/v1/maintenance/{id}", h.handleUpdate)
	mux.HandleFunc("DELETE /api/v1/maintenance/{id}", h.handleDelete)
	mux.HandleFunc("GET /api/v1/maintenance/active", h.handleListActive)
}

func (h *MaintenanceAPIHandler) handleList(w http.ResponseWriter, r *http.Request) {
	windows := h.store.List()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"maintenanceWindows": windows,
		"total":              len(windows),
	})
}

func (h *MaintenanceAPIHandler) handleListActive(w http.ResponseWriter, r *http.Request) {
	windows := h.store.ListActive()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"maintenanceWindows": windows,
		"total":              len(windows),
	})
}

func (h *MaintenanceAPIHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 64*1024))
	if err != nil {
		http.Error(w, `{"error":"failed to read body"}`, http.StatusBadRequest)
		return
	}

	var mw MaintenanceWindow
	if err := json.Unmarshal(body, &mw); err != nil {
		http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
		return
	}

	// Validate
	if mw.Name == "" {
		http.Error(w, `{"error":"name is required"}`, http.StatusBadRequest)
		return
	}
	if mw.StartTime.IsZero() || mw.EndTime.IsZero() {
		http.Error(w, `{"error":"startTime and endTime are required"}`, http.StatusBadRequest)
		return
	}
	if !mw.EndTime.After(mw.StartTime) {
		http.Error(w, `{"error":"endTime must be after startTime"}`, http.StatusBadRequest)
		return
	}

	mw.Enabled = true
	mw.CreatedAt = time.Now().UTC()

	if err := h.store.Create(mw); err != nil {
		http.Error(w, `{"error":"failed to create maintenance window"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":                true,
		"maintenanceWindow": mw,
	})
}

func (h *MaintenanceAPIHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	mw, err := h.store.Get(id)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mw)
}

func (h *MaintenanceAPIHandler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	body, err := io.ReadAll(io.LimitReader(r.Body, 64*1024))
	if err != nil {
		http.Error(w, `{"error":"failed to read body"}`, http.StatusBadRequest)
		return
	}

	// Decode into a partial update struct with pointer bools so we can tell
	// "field omitted" apart from "field explicitly set to false".
	var update struct {
		Name           string    `json:"name"`
		Description    string    `json:"description"`
		StartTime      time.Time `json:"startTime"`
		EndTime        time.Time `json:"endTime"`
		CheckIDs       []string  `json:"checkIds"`
		Tags           []string  `json:"tags"`
		Servers        []string  `json:"servers"`
		Recurring      *bool     `json:"recurring"`
		RecurrenceRule *string   `json:"recurrenceRule"`
		Enabled        *bool     `json:"enabled"`
	}
	if err := json.Unmarshal(body, &update); err != nil {
		http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
		return
	}

	err = h.store.Update(id, func(mw *MaintenanceWindow) error {
		if update.Name != "" {
			mw.Name = update.Name
		}
		if update.Description != "" {
			mw.Description = update.Description
		}
		if !update.StartTime.IsZero() {
			mw.StartTime = update.StartTime
		}
		if !update.EndTime.IsZero() {
			mw.EndTime = update.EndTime
		}
		if update.CheckIDs != nil {
			mw.CheckIDs = update.CheckIDs
		}
		if update.Tags != nil {
			mw.Tags = update.Tags
		}
		if update.Servers != nil {
			mw.Servers = update.Servers
		}
		if update.Recurring != nil {
			mw.Recurring = *update.Recurring
		}
		if update.RecurrenceRule != nil {
			mw.RecurrenceRule = *update.RecurrenceRule
		}
		if update.Enabled != nil {
			mw.Enabled = *update.Enabled
		}
		if mw.Name == "" {
			return fmt.Errorf("name is required")
		}
		if mw.StartTime.IsZero() || mw.EndTime.IsZero() {
			return fmt.Errorf("startTime and endTime are required")
		}
		if !mw.EndTime.After(mw.StartTime) {
			return fmt.Errorf("endTime must be after startTime")
		}
		return nil
	})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	mw, _ := h.store.Get(id)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":                true,
		"maintenanceWindow": mw,
	})
}

func (h *MaintenanceAPIHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.Delete(id); err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"deleted": id,
	})
}
