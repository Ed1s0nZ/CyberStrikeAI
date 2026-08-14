package knowledge

import "testing"

func TestIndexerRejectsConcurrentIndexRuns(t *testing.T) {
	idx := &Indexer{}

	if err := idx.beginIndexRun(); err != nil {
		t.Fatalf("first index run should start: %v", err)
	}
	if err := idx.beginIndexRun(); err == nil {
		t.Fatal("second index run should be rejected while one is active")
	}

	idx.FinishIndexRun()
	if err := idx.beginIndexRun(); err != nil {
		t.Fatalf("index run should start again after finish: %v", err)
	}
	idx.FinishIndexRun()
}
