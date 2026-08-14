package database

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestWorkflowPackageApplyOverwriteIsTransactionalAndIdempotent(t *testing.T) {
	db, err := NewDB(filepath.Join(t.TempDir(), "workflow-package.db"), zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	current := &WorkflowDefinition{ID: "wf-1", Name: "Local", Version: 12, GraphJSON: `{"nodes":[]}`, Enabled: true}
	if err := db.UpsertWorkflowDefinition(current); err != nil {
		t.Fatal(err)
	}
	current, _ = db.GetWorkflowDefinition("wf-1")
	content, graph := workflowDefinitionPackageHashes(current)
	payload, _ := json.Marshal(map[string]any{"id": "wf-1", "name": "Imported", "description": "new", "version": 18, "graph_json": `{"nodes":[]}`, "enabled": false})
	now := time.Now().UTC()
	inspection := &WorkflowPackageInspection{ID: "wpi_test", PackageHash: "sha256:pkg", ManifestJSON: "{}", WorkflowPayloadJSON: string(payload), InspectionJSON: "{}", SourceWorkflowID: "wf-1", SourceRevision: 18, SourceContentHash: "sha256:src", SourceGraphHash: "sha256:graph", LocalConflictState: "id_conflict", LocalWorkflowID: "wf-1", LocalContentHash: content, LocalGraphHash: graph, CreatedBy: "user-1", CreatedAt: now, ExpiresAt: now.Add(time.Minute)}
	if err := db.CreateWorkflowPackageInspection(inspection); err != nil {
		t.Fatal(err)
	}
	req := WorkflowPackageApplyRequest{InspectionID: inspection.ID, RequestHash: "sha256:req", IdempotencyKey: "key-1", ActorUserID: "user-1", Action: "overwrite", ConfirmOverwrite: true}
	imp, replayed, err := db.ApplyWorkflowPackageImport(context.Background(), req)
	if err != nil || replayed || imp.Result != "overwritten" {
		t.Fatalf("apply = %#v replay=%v err=%v", imp, replayed, err)
	}
	updated, _ := db.GetWorkflowDefinition("wf-1")
	if updated.Version != 13 || updated.Name != "Imported" || updated.Enabled {
		t.Fatalf("updated workflow = %#v", updated)
	}
	replay, replayed, err := db.ApplyWorkflowPackageImport(context.Background(), req)
	if err != nil || !replayed || replay.ID != imp.ID {
		t.Fatalf("replay = %#v replay=%v err=%v", replay, replayed, err)
	}
	gotInspection, err := db.GetWorkflowPackageInspection(inspection.ID, "user-1")
	if err != nil || gotInspection.Status != "consumed" {
		t.Fatalf("inspection=%#v err=%v", gotInspection, err)
	}
}

func TestWorkflowPackageApplyRejectsChangedConflictSnapshot(t *testing.T) {
	db, err := NewDB(filepath.Join(t.TempDir(), "workflow-package-conflict.db"), zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.UpsertWorkflowDefinition(&WorkflowDefinition{ID: "wf-2", Name: "Local", GraphJSON: `{"nodes":[]}`, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	local, _ := db.GetWorkflowDefinition("wf-2")
	content, graph := workflowDefinitionPackageHashes(local)
	payload, _ := json.Marshal(map[string]any{"id": "wf-2", "name": "Imported", "version": 2, "graph_json": `{"nodes":[]}`, "enabled": true})
	now := time.Now().UTC()
	inspection := &WorkflowPackageInspection{ID: "wpi_changed", PackageHash: "sha256:pkg", ManifestJSON: "{}", WorkflowPayloadJSON: string(payload), InspectionJSON: "{}", SourceWorkflowID: "wf-2", SourceRevision: 2, SourceContentHash: "sha256:src", SourceGraphHash: "sha256:graph", LocalConflictState: "id_conflict", LocalWorkflowID: "wf-2", LocalContentHash: content, LocalGraphHash: graph, CreatedBy: "user-1", CreatedAt: now, ExpiresAt: now.Add(time.Minute)}
	if err := db.CreateWorkflowPackageInspection(inspection); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertWorkflowDefinition(&WorkflowDefinition{ID: "wf-2", Name: "Changed", GraphJSON: `{"nodes":[]}`, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	_, _, err = db.ApplyWorkflowPackageImport(context.Background(), WorkflowPackageApplyRequest{InspectionID: inspection.ID, RequestHash: "sha256:req", IdempotencyKey: "key-2", ActorUserID: "user-1", Action: "overwrite", ConfirmOverwrite: true})
	if e, ok := err.(*WorkflowPackageStoreError); !ok || e.Code != "WFPKG_CONFLICT_CHANGED" {
		t.Fatalf("err=%v", err)
	}
}
