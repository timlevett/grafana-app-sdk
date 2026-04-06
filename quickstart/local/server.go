// Package local provides an embedded API server backed by SQLite for local
// development of Grafana app-platform apps without Kubernetes.
package local

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// AdmissionOp describes the operation being admitted.
type AdmissionOp string

const (
	AdmissionCreate AdmissionOp = "CREATE"
	AdmissionUpdate AdmissionOp = "UPDATE"
	AdmissionDelete AdmissionOp = "DELETE"
)

// AdmissionRequest is passed to admission hooks before a resource is persisted.
type AdmissionRequest struct {
	Op          AdmissionOp
	GVR         GroupVersionResource
	Resource    Resource
	OldResource *Resource // non-nil for UPDATE and DELETE
}

// AdmissionResponse is returned by admission hooks.
// Set Allowed=false to reject the request. MutatedResource replaces the
// incoming resource (mutation webhooks only; ignored for DELETE).
type AdmissionResponse struct {
	Allowed         bool
	Message         string
	MutatedResource *Resource
}

// AdmissionHook is called before a resource is persisted.
type AdmissionHook func(req AdmissionRequest) AdmissionResponse

// EventType describes the type of a reconcile event.
type EventType string

const (
	EventAdded    EventType = "ADDED"
	EventModified EventType = "MODIFIED"
	EventDeleted  EventType = "DELETED"
)

// ReconcileEvent describes a change to a resource.
type ReconcileEvent struct {
	Type     EventType
	GVR      GroupVersionResource
	Resource Resource
}

// ReconcileFunc is called after a resource change is committed to storage.
type ReconcileFunc func(event ReconcileEvent)

// Server is a lightweight K8s-compatible REST API server backed by SQLite.
type Server struct {
	db          *sql.DB
	mux         *http.ServeMux
	mu          sync.RWMutex
	groups      map[string]GroupVersionResource
	admission   []AdmissionHook
	reconcilers []ReconcileFunc
}

// GroupVersionResource describes a registered API resource.
type GroupVersionResource struct {
	Group    string
	Version  string
	Resource string // plural, lowercase
	Kind     string // singular, PascalCase
}

// NewServer creates a new local API server with the given SQLite DSN.
// Use ":memory:" for ephemeral storage or a file path for persistence.
func NewServer(dsn string) (*Server, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	s := &Server{
		db:     db,
		mux:    http.NewServeMux(),
		groups: make(map[string]GroupVersionResource),
	}

	s.mux.HandleFunc("/apis/", s.handleAPI)
	s.mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})

	return s, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS resources (
			kind       TEXT NOT NULL,
			namespace  TEXT NOT NULL DEFAULT 'default',
			name       TEXT NOT NULL,
			version    INTEGER NOT NULL DEFAULT 1,
			spec       JSON NOT NULL,
			status     JSON,
			labels     JSON,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (kind, namespace, name)
		)
	`)
	return err
}

// RegisterResource makes the server aware of a resource type so it can
// route requests to the correct handler.
func (s *Server) RegisterResource(gvr GroupVersionResource) {
	key := gvr.Group + "/" + gvr.Version + "/" + strings.ToLower(gvr.Resource)
	s.mu.Lock()
	s.groups[key] = gvr
	s.mu.Unlock()
}

// RegisterAdmissionHook registers a hook that is called before every
// create, update, or delete. Hooks run in registration order. If any hook
// returns Allowed=false the request is rejected with 422.
func (s *Server) RegisterAdmissionHook(h AdmissionHook) {
	s.mu.Lock()
	s.admission = append(s.admission, h)
	s.mu.Unlock()
}

// RegisterReconciler registers a function that is called after every
// successful create, update, or delete. Reconcilers run synchronously in
// registration order; panics are recovered and logged.
func (s *Server) RegisterReconciler(fn ReconcileFunc) {
	s.mu.Lock()
	s.reconcilers = append(s.reconcilers, fn)
	s.mu.Unlock()
}

// admit runs all registered admission hooks for the given request.
// Returns (resource to persist, ok). resource may be mutated by hooks.
func (s *Server) admit(req AdmissionRequest) (Resource, bool, string) {
	s.mu.RLock()
	hooks := make([]AdmissionHook, len(s.admission))
	copy(hooks, s.admission)
	s.mu.RUnlock()

	res := req.Resource
	for _, h := range hooks {
		resp := h(req)
		if !resp.Allowed {
			return res, false, resp.Message
		}
		if resp.MutatedResource != nil {
			res = *resp.MutatedResource
			req.Resource = res // subsequent hooks see mutated version
		}
	}
	return res, true, ""
}

// reconcile dispatches an event to all registered reconcilers.
func (s *Server) reconcile(evt ReconcileEvent) {
	s.mu.RLock()
	fns := make([]ReconcileFunc, len(s.reconcilers))
	copy(fns, s.reconcilers)
	s.mu.RUnlock()

	for _, fn := range fns {
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("reconciler panic: %v", r)
				}
			}()
			fn(evt)
		}()
	}
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// ListenAndServe starts the server on the given address.
func (s *Server) ListenAndServe(addr string) error {
	log.Printf("local api server listening on %s", addr)
	return http.ListenAndServe(addr, s)
}

// Close closes the underlying database.
func (s *Server) Close() error {
	return s.db.Close()
}

// handleAPI routes /apis/{group}/{version}/namespaces/{ns}/{resource}[/{name}]
func (s *Server) handleAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/apis/"), "/")
	// Expect: group/version/namespaces/ns/resource[/name]
	if len(parts) < 5 || parts[2] != "namespaces" {
		writeError(w, http.StatusNotFound, "invalid API path")
		return
	}

	group := parts[0]
	version := parts[1]
	ns := parts[3]
	resource := parts[4]
	name := ""
	if len(parts) > 5 {
		name = parts[5]
	}

	key := group + "/" + version + "/" + resource
	s.mu.RLock()
	gvr, ok := s.groups[key]
	s.mu.RUnlock()
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("unknown resource %s", key))
		return
	}

	switch r.Method {
	case http.MethodGet:
		if name == "" {
			s.handleList(w, r, gvr, ns)
		} else {
			s.handleGet(w, r, gvr, ns, name)
		}
	case http.MethodPost:
		s.handleCreate(w, r, gvr, ns)
	case http.MethodPut:
		s.handleUpdate(w, r, gvr, ns, name)
	case http.MethodDelete:
		s.handleDelete(w, r, gvr, ns, name)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// Resource is a K8s-compatible resource object.
type Resource struct {
	APIVersion string           `json:"apiVersion"`
	Kind       string           `json:"kind"`
	Metadata   ResourceMetadata `json:"metadata"`
	Spec       json.RawMessage  `json:"spec"`
	Status     json.RawMessage  `json:"status,omitempty"`
}

// ResourceMetadata holds K8s-compatible metadata.
type ResourceMetadata struct {
	Name            string            `json:"name"`
	Namespace       string            `json:"namespace"`
	ResourceVersion string            `json:"resourceVersion,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	CreationTime    string            `json:"creationTimestamp,omitempty"`
}

// ResourceList is a K8s-compatible list response.
type ResourceList struct {
	APIVersion string     `json:"apiVersion"`
	Kind       string     `json:"kind"`
	Items      []Resource `json:"items"`
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request, gvr GroupVersionResource, ns string) {
	rows, err := s.db.Query(
		"SELECT name, namespace, version, spec, status, labels, created_at FROM resources WHERE kind = ? AND namespace = ?",
		gvr.Kind, ns,
	)
	if err != nil {
		log.Printf("list %s/%s: query error: %v", ns, gvr.Kind, err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer rows.Close()

	items := make([]Resource, 0)
	for rows.Next() {
		var name, namespace, createdAt string
		var version int
		var spec, status, labels []byte
		if err := rows.Scan(&name, &namespace, &version, &spec, &status, &labels, &createdAt); err != nil {
			log.Printf("list %s/%s: scan error: %v", ns, gvr.Kind, err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		meta := ResourceMetadata{
			Name:            name,
			Namespace:       namespace,
			ResourceVersion: strconv.Itoa(version),
			CreationTime:    createdAt,
		}
		if labels != nil {
			if err := json.Unmarshal(labels, &meta.Labels); err != nil {
				log.Printf("list: failed to unmarshal labels for %s/%s: %v", namespace, name, err)
				writeError(w, http.StatusInternalServerError, "internal server error")
				return
			}
		}
		item := Resource{
			APIVersion: gvr.Group + "/" + gvr.Version,
			Kind:       gvr.Kind,
			Metadata:   meta,
			Spec:       spec,
		}
		if status != nil {
			item.Status = status
		}
		items = append(items, item)
	}

	writeJSON(w, http.StatusOK, ResourceList{
		APIVersion: gvr.Group + "/" + gvr.Version,
		Kind:       gvr.Kind + "List",
		Items:      items,
	})
}

func (s *Server) handleGet(w http.ResponseWriter, _ *http.Request, gvr GroupVersionResource, ns, name string) {
	var version int
	var spec, status, labels []byte
	var createdAt string
	err := s.db.QueryRow(
		"SELECT version, spec, status, labels, created_at FROM resources WHERE kind = ? AND namespace = ? AND name = ?",
		gvr.Kind, ns, name,
	).Scan(&version, &spec, &status, &labels, &createdAt)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, fmt.Sprintf("%s %q not found", gvr.Kind, name))
		return
	}
	if err != nil {
		log.Printf("get %s/%s/%s: query error: %v", ns, gvr.Kind, name, err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	meta := ResourceMetadata{
		Name:            name,
		Namespace:       ns,
		ResourceVersion: strconv.Itoa(version),
		CreationTime:    createdAt,
	}
	if labels != nil {
		if err := json.Unmarshal(labels, &meta.Labels); err != nil {
			log.Printf("get: failed to unmarshal labels for %s/%s: %v", ns, name, err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
	}

	res := Resource{
		APIVersion: gvr.Group + "/" + gvr.Version,
		Kind:       gvr.Kind,
		Metadata:   meta,
		Spec:       spec,
	}
	if status != nil {
		res.Status = status
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request, gvr GroupVersionResource, ns string) {
	var res Resource
	if err := json.NewDecoder(r.Body).Decode(&res); err != nil {
		log.Printf("create %s/%s: decode error: %v", ns, gvr.Kind, err)
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if res.Metadata.Name == "" {
		writeError(w, http.StatusBadRequest, "metadata.name is required")
		return
	}

	// Admission
	admitted, ok, msg := s.admit(AdmissionRequest{Op: AdmissionCreate, GVR: gvr, Resource: res})
	if !ok {
		writeError(w, http.StatusUnprocessableEntity, msg)
		return
	}
	res = admitted

	labelsJSON, err := json.Marshal(res.Metadata.Labels)
	if err != nil {
		log.Printf("create %s/%s/%s: marshal labels error: %v", ns, gvr.Kind, res.Metadata.Name, err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)

	_, err = s.db.Exec(
		"INSERT INTO resources (kind, namespace, name, spec, status, labels, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		gvr.Kind, ns, res.Metadata.Name, string(res.Spec), nullableJSON(res.Status), string(labelsJSON), now, now,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			writeError(w, http.StatusConflict, fmt.Sprintf("%s %q already exists", gvr.Kind, res.Metadata.Name))
			return
		}
		log.Printf("create %s/%s/%s: insert error: %v", ns, gvr.Kind, res.Metadata.Name, err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	res.APIVersion = gvr.Group + "/" + gvr.Version
	res.Kind = gvr.Kind
	res.Metadata.Namespace = ns
	res.Metadata.ResourceVersion = "1"
	res.Metadata.CreationTime = now

	go s.reconcile(ReconcileEvent{Type: EventAdded, GVR: gvr, Resource: res})
	writeJSON(w, http.StatusCreated, res)
}

func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request, gvr GroupVersionResource, ns, name string) {
	var res Resource
	if err := json.NewDecoder(r.Body).Decode(&res); err != nil {
		log.Printf("update %s/%s/%s: decode error: %v", ns, gvr.Kind, name, err)
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Fetch old version for admission hooks (best-effort; ignore if not found yet)
	var old *Resource
	if len(s.admission) > 0 || len(s.reconcilers) > 0 {
		var oldVersion int
		var oldSpec, oldStatus, oldLabels []byte
		var oldCreatedAt string
		err := s.db.QueryRow(
			"SELECT version, spec, status, labels, created_at FROM resources WHERE kind = ? AND namespace = ? AND name = ?",
			gvr.Kind, ns, name,
		).Scan(&oldVersion, &oldSpec, &oldStatus, &oldLabels, &oldCreatedAt)
		if err == nil {
			oldMeta := ResourceMetadata{Name: name, Namespace: ns, ResourceVersion: strconv.Itoa(oldVersion), CreationTime: oldCreatedAt}
			if oldLabels != nil {
				_ = json.Unmarshal(oldLabels, &oldMeta.Labels)
			}
			o := Resource{APIVersion: gvr.Group + "/" + gvr.Version, Kind: gvr.Kind, Metadata: oldMeta, Spec: oldSpec}
			if oldStatus != nil {
				o.Status = oldStatus
			}
			old = &o
		}
	}

	// Admission
	admitted, ok, msg := s.admit(AdmissionRequest{Op: AdmissionUpdate, GVR: gvr, Resource: res, OldResource: old})
	if !ok {
		writeError(w, http.StatusUnprocessableEntity, msg)
		return
	}
	res = admitted

	now := time.Now().UTC().Format(time.RFC3339)
	labelsJSON, err := json.Marshal(res.Metadata.Labels)
	if err != nil {
		log.Printf("update %s/%s/%s: marshal labels error: %v", ns, gvr.Kind, name, err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	result, err := s.db.Exec(
		"UPDATE resources SET spec = ?, status = ?, labels = ?, version = version + 1, updated_at = ? WHERE kind = ? AND namespace = ? AND name = ?",
		string(res.Spec), nullableJSON(res.Status), string(labelsJSON), now, gvr.Kind, ns, name,
	)
	if err != nil {
		log.Printf("update %s/%s/%s: exec error: %v", ns, gvr.Kind, name, err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	rows, err := result.RowsAffected()
	if err != nil {
		log.Printf("update %s/%s/%s: rows affected error: %v", ns, gvr.Kind, name, err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if rows == 0 {
		writeError(w, http.StatusNotFound, fmt.Sprintf("%s %q not found", gvr.Kind, name))
		return
	}

	// Read back the updated version
	s.handleGet(w, r, gvr, ns, name)

	// Dispatch reconcile event with the updated resource (re-read from response)
	if len(s.reconcilers) > 0 {
		updated := res
		updated.Metadata.Namespace = ns
		updated.Metadata.Name = name
		go s.reconcile(ReconcileEvent{Type: EventModified, GVR: gvr, Resource: updated})
	}
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request, gvr GroupVersionResource, ns, name string) {
	// Fetch resource for admission hooks
	var existing *Resource
	if len(s.admission) > 0 || len(s.reconcilers) > 0 {
		var version int
		var spec, status, labels []byte
		var createdAt string
		err := s.db.QueryRow(
			"SELECT version, spec, status, labels, created_at FROM resources WHERE kind = ? AND namespace = ? AND name = ?",
			gvr.Kind, ns, name,
		).Scan(&version, &spec, &status, &labels, &createdAt)
		if err == nil {
			meta := ResourceMetadata{Name: name, Namespace: ns, ResourceVersion: strconv.Itoa(version), CreationTime: createdAt}
			if labels != nil {
				_ = json.Unmarshal(labels, &meta.Labels)
			}
			e := Resource{APIVersion: gvr.Group + "/" + gvr.Version, Kind: gvr.Kind, Metadata: meta, Spec: spec}
			if status != nil {
				e.Status = status
			}
			existing = &e
		}
	}

	// Admission (deletion)
	if existing != nil {
		_, ok, msg := s.admit(AdmissionRequest{Op: AdmissionDelete, GVR: gvr, Resource: *existing, OldResource: existing})
		if !ok {
			writeError(w, http.StatusUnprocessableEntity, msg)
			return
		}
	}

	result, err := s.db.Exec(
		"DELETE FROM resources WHERE kind = ? AND namespace = ? AND name = ?",
		gvr.Kind, ns, name,
	)
	if err != nil {
		log.Printf("delete %s/%s/%s: exec error: %v", ns, gvr.Kind, name, err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	rows, err := result.RowsAffected()
	if err != nil {
		log.Printf("delete %s/%s/%s: rows affected error: %v", ns, gvr.Kind, name, err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if rows == 0 {
		writeError(w, http.StatusNotFound, fmt.Sprintf("%s %q not found", gvr.Kind, name))
		return
	}

	if existing != nil {
		go s.reconcile(ReconcileEvent{Type: EventDeleted, GVR: gvr, Resource: *existing})
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"kind":       gvr.Kind,
		"apiVersion": gvr.Group + "/" + gvr.Version,
		"status":     "Success",
	})
}

func nullableJSON(data json.RawMessage) interface{} {
	if data == nil || string(data) == "null" {
		return nil
	}
	return string(data)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"kind":    "Status",
		"status":  "Failure",
		"message": msg,
		"code":    status,
	})
}
