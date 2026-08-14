package c2

import (
	"path/filepath"
	"testing"

	"cyberstrike-ai/internal/database"

	"go.uber.org/zap"
)

func TestIngestCheckIn_PreservesOperatorSleepOnHeartbeat(t *testing.T) {
	tmp := t.TempDir()
	db, err := database.NewDB(filepath.Join(tmp, "c2.sqlite"), zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	mgr := NewManager(db, zap.NewNop(), tmp)
	ln, err := mgr.CreateListener(CreateListenerInput{
		Name:     "t",
		Type:     string(ListenerTypeHTTPBeacon),
		BindHost: "127.0.0.1",
		BindPort: 18080,
	})
	if err != nil {
		t.Fatal(err)
	}

	first, err := mgr.IngestCheckIn(ln.ID, ImplantCheckInRequest{
		ImplantUUID:   "implant-uuid-1",
		Hostname:      "host1",
		Username:      "user",
		OS:            "darwin",
		Arch:          "amd64",
		SleepSeconds:  5,
		JitterPercent: 0,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := db.SetC2SessionSleep(first.ID, 30, 20); err != nil {
		t.Fatal(err)
	}

	second, err := mgr.IngestCheckIn(ln.ID, ImplantCheckInRequest{
		ImplantUUID:   "implant-uuid-1",
		Hostname:      "host1",
		Username:      "user",
		OS:            "darwin",
		Arch:          "amd64",
		SleepSeconds:  5,
		JitterPercent: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.SleepSeconds != 30 || second.JitterPercent != 20 {
		t.Fatalf("expected sleep=30 jitter=20, got sleep=%d jitter=%d", second.SleepSeconds, second.JitterPercent)
	}

	stored, err := db.GetC2Session(first.ID)
	if err != nil || stored == nil {
		t.Fatal(err)
	}
	if stored.SleepSeconds != 30 || stored.JitterPercent != 20 {
		t.Fatalf("db: expected sleep=30 jitter=20, got sleep=%d jitter=%d", stored.SleepSeconds, stored.JitterPercent)
	}
}

func TestSetSessionSleep_UpdatesDBAndEnqueuesTask(t *testing.T) {
	tmp := t.TempDir()
	db, err := database.NewDB(filepath.Join(tmp, "c2.sqlite"), zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	mgr := NewManager(db, zap.NewNop(), tmp)
	ln, err := mgr.CreateListener(CreateListenerInput{
		Name:     "t2",
		Type:     string(ListenerTypeHTTPBeacon),
		BindHost: "127.0.0.1",
		BindPort: 18081,
	})
	if err != nil {
		t.Fatal(err)
	}
	sess, err := mgr.IngestCheckIn(ln.ID, ImplantCheckInRequest{
		ImplantUUID:  "implant-uuid-2",
		Hostname:     "host2",
		Username:     "user",
		OS:           "linux",
		Arch:         "amd64",
		SleepSeconds: 5,
	})
	if err != nil {
		t.Fatal(err)
	}

	task, err := mgr.SetSessionSleep(sess.ID, 15, 10)
	if err != nil {
		t.Fatal(err)
	}
	if task == nil || task.TaskType != string(TaskTypeSleep) {
		t.Fatalf("expected sleep task, got %#v", task)
	}

	stored, err := db.GetC2Session(sess.ID)
	if err != nil || stored == nil {
		t.Fatal(err)
	}
	if stored.SleepSeconds != 15 || stored.JitterPercent != 10 {
		t.Fatalf("expected sleep=15 jitter=10, got sleep=%d jitter=%d", stored.SleepSeconds, stored.JitterPercent)
	}
}
