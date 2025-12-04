package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"gantt-backend-go/common"
	"gantt-backend-go/data"

	"github.com/go-chi/chi"
)

func TestPutTasksUpdate(t *testing.T) {
	dao := newTestDAO(t)
	taskID := addTask(t, dao, "Original Task", 0)
	router := newTestRouter(dao)

	payload := map[string]any{
		"text":     "Updated Task",
		"duration": 15,
		"progress": 45,
		"parent":   0,
	}

	resp := performJSONRequest(t, router, http.MethodPut, fmt.Sprintf("/tasks/%d", taskID), payload)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var body Response
	decodeResponse(t, resp, &body)
	if body.ID != taskID {
		t.Fatalf("expected response ID %d, got %d", taskID, body.ID)
	}

	updated, err := dao.Tasks.GetOne(taskID)
	if err != nil {
		t.Fatalf("failed to load updated task: %v", err)
	}
	if updated.Text != "Updated Task" {
		t.Fatalf("expected text 'Updated Task', got %q", updated.Text)
	}
	if updated.Progress != 45 {
		t.Fatalf("expected progress 45, got %d", updated.Progress)
	}
}

func TestPutTasksMove(t *testing.T) {
	dao := newTestDAO(t)
	parentA := addTask(t, dao, "Parent A", 0)
	parentB := addTask(t, dao, "Parent B", 0)
	child := addTask(t, dao, "Child", parentA)
	router := newTestRouter(dao)

	payload := map[string]any{
		"operation": "move",
		"target":    parentB,
		"mode":      "child",
	}

	resp := performJSONRequest(t, router, http.MethodPut, fmt.Sprintf("/tasks/%d", child), payload)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var body Response
	decodeResponse(t, resp, &body)
	if body.ID != child {
		t.Fatalf("expected moved task ID %d, got %d", child, body.ID)
	}

	moved, err := dao.Tasks.GetOne(child)
	if err != nil {
		t.Fatalf("failed to load moved task: %v", err)
	}
	if moved.Parent != parentB {
		t.Fatalf("expected parent %d, got %d", parentB, moved.Parent)
	}
}

func TestPutTasksCopyNested(t *testing.T) {
	dao := newTestDAO(t)
	sourceParent := addTask(t, dao, "Copy Source", 0)
	sourceChild := addTask(t, dao, "Source Child", sourceParent)
	external := addTask(t, dao, "External", 0)
	_, err := dao.Links.Add(data.LinkUpdate{
		Source: common.FuzzyInt(sourceChild),
		Target: common.FuzzyInt(external),
		Type:   "fs",
	})
	if err != nil {
		t.Fatalf("failed to seed link: %v", err)
	}
	target := addTask(t, dao, "Target", 0)
	router := newTestRouter(dao)

	payload := map[string]any{
		"operation": "copy",
		"target":    target,
		"mode":      "after",
		"nested":    true,
	}

	resp := performJSONRequest(t, router, http.MethodPut, fmt.Sprintf("/tasks/%d", sourceParent), payload)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var body Response
	decodeResponse(t, resp, &body)
	if body.ID == sourceParent {
		t.Fatal("expected copy to return new task ID")
	}

	copied, err := dao.Tasks.GetOne(body.ID)
	if err != nil {
		t.Fatalf("failed to load copied task: %v", err)
	}
	if copied.Text != "Copy Source" {
		t.Fatalf("copied task text mismatch: expected 'Copy Source', got %q", copied.Text)
	}

	newChild := findTaskByTextAndParent(t, dao, "Source Child", body.ID)
	if newChild.ID == 0 {
		t.Fatal("expected new child task to exist")
	}

	links, err := dao.Links.GetAll()
	if err != nil {
		t.Fatalf("failed to load links: %v", err)
	}
	found := false
	for _, l := range links {
		if l.Source == newChild.ID && l.Target == external {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected link to be copied for nested child")
	}
}

func newTestDAO(t *testing.T) *data.DAO {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	dao := data.NewDAO(data.DBConfig{Path: dbPath, ResetOnStart: false}, "")
	t.Cleanup(func() {
		sqlDB, err := dao.GetDB().DB()
		if err == nil {
			sqlDB.Close()
		}
	})
	return dao
}

func newTestRouter(dao *data.DAO) http.Handler {
	r := chi.NewRouter()
	initRoutes(r, dao)
	return r
}

func addTask(t *testing.T, dao *data.DAO, text string, parent int) int {
	t.Helper()
	id, err := dao.Tasks.Add(data.TaskUpdate{
		Text:   text,
		Parent: common.FuzzyInt(parent),
	})
	if err != nil {
		t.Fatalf("failed to add task %q: %v", text, err)
	}
	return id
}

func performJSONRequest(t *testing.T, handler http.Handler, method, path string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	var bodyBytes []byte
	var err error
	if payload != nil {
		bodyBytes, err = json.Marshal(payload)
		if err != nil {
			t.Fatalf("failed to marshal payload: %v", err)
		}
	}

	req := httptest.NewRequest(method, path, bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	return resp
}

func decodeResponse(t *testing.T, resp *httptest.ResponseRecorder, out interface{}) {
	t.Helper()
	if err := json.Unmarshal(resp.Body.Bytes(), out); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
}

func findTaskByTextAndParent(t *testing.T, dao *data.DAO, text string, parent int) data.Task {
	t.Helper()
	tasks, err := dao.Tasks.GetAll()
	if err != nil {
		t.Fatalf("failed to enumerate tasks: %v", err)
	}
	for _, task := range tasks {
		if task.Text == text && task.Parent == parent {
			return task
		}
	}
	return data.Task{}
}
