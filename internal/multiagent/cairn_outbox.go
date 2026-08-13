package multiagent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"cyberstrike-ai/internal/database"
)

// Cairn outbox 投影 worker：把 canonical 事件异步投影到 legacy project_facts 与 YAML 导出。
// 运行链本身不直接双写，保证 DB 事务是唯一真相，投影失败可重试且不阻塞主链。

const (
	cairnOutboxConsumerProjectFact = "project_fact_projection"
	cairnOutboxConsumerYAMLExport  = "yaml_export"
	cairnOutboxBatchLimit          = 50
)

// CairnOutboxWorker 周期性 drain cairn_outbox。
type CairnOutboxWorker struct {
	db       *database.DB
	stateDir string
	logger   *zap.Logger
	interval time.Duration
}

// NewCairnOutboxWorker 构建 worker。interval<=0 时默认 3 秒。
func NewCairnOutboxWorker(db *database.DB, stateDir string, logger *zap.Logger, interval time.Duration) *CairnOutboxWorker {
	if interval <= 0 {
		interval = 3 * time.Second
	}
	return &CairnOutboxWorker{db: db, stateDir: strings.TrimSpace(stateDir), logger: logger, interval: interval}
}

// Start 启动后台循环，返回 stop 函数。调用方应在 canonical enabled 时才启动。
func (w *CairnOutboxWorker) Start(ctx context.Context) (stop func()) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.drainOnce(ctx)
			}
		}
	}()
	return func() { <-done }
}

// DrainOnce 立即执行一轮 drain（测试用）。
func (w *CairnOutboxWorker) DrainOnce(ctx context.Context) {
	w.drainOnce(ctx)
}

func (w *CairnOutboxWorker) drainOnce(ctx context.Context) {
	for _, consumer := range []string{cairnOutboxConsumerProjectFact, cairnOutboxConsumerYAMLExport} {
		items, err := w.db.ClaimCairnOutboxItems(ctx, consumer, cairnOutboxBatchLimit)
		if err != nil {
			if w.logger != nil {
				w.logger.Warn("cairn outbox claim failed", zap.String("consumer", consumer), zap.Error(err))
			}
			continue
		}
		for _, item := range items {
			if err := w.processItem(ctx, &item); err != nil {
				if nerr := w.db.NackCairnOutboxItem(ctx, item.EventID, consumer, err.Error()); nerr != nil && w.logger != nil {
					w.logger.Error("cairn outbox nack failed", zap.String("event_id", item.EventID), zap.Error(nerr))
				}
				continue
			}
			if err := w.db.AckCairnOutboxItem(ctx, item.EventID, consumer); err != nil && w.logger != nil {
				w.logger.Error("cairn outbox ack failed", zap.String("event_id", item.EventID), zap.Error(err))
			}
		}
	}
}

func (w *CairnOutboxWorker) processItem(ctx context.Context, item *database.CairnOutboxItem) error {
	if item == nil || item.EventID == "" {
		return fmt.Errorf("cairn outbox item is empty")
	}
	event, err := w.db.GetCairnEventByID(ctx, item.EventID)
	if err != nil {
		return err
	}
	switch item.Consumer {
	case cairnOutboxConsumerProjectFact:
		if event.EventType != "fact_created" {
			return nil // 其他事件无需投影，ack 跳过
		}
		return projectCairnFactEvent(ctx, w.db, event)
	case cairnOutboxConsumerYAMLExport:
		return w.exportCairnYAML(ctx, event)
	default:
		return nil
	}
}

// projectCairnFactEvent 把 canonical fact 投影到 project_facts（黑板）。
func projectCairnFactEvent(ctx context.Context, db *database.DB, event *database.CairnEvent) error {
	if event == nil {
		return fmt.Errorf("cairn fact event is nil")
	}
	var payload struct {
		FactID   string `json:"fact_id"`
		IntentID string `json:"intent_id"`
	}
	if err := json.Unmarshal([]byte(event.PayloadJSON), &payload); err != nil {
		return fmt.Errorf("cairn fact event payload parse: %w", err)
	}
	if payload.FactID == "" {
		return fmt.Errorf("cairn fact event missing fact_id")
	}
	fact, err := db.GetCairnFactByID(ctx, payload.FactID)
	if err != nil {
		return err
	}
	run, err := db.GetCairnRunByID(ctx, event.RunID)
	if err != nil {
		return err
	}
	key := stableCairnID("cairn", run.ConversationID, fact.SourceEventID)
	f := &database.ProjectFact{
		ProjectID:            run.ProjectID,
		FactKey:              key,
		Category:             "cairn_fact",
		Summary:              fact.Statement,
		Body:                 fmt.Sprintf("source_intent_id: %s\nobservation_type: %s\nrun_id: %s", fact.SourceIntentID, fact.ObservationType, run.ID),
		Confidence:           fact.Confidence,
		SourceConversationID: run.ConversationID,
		Pinned:               false,
	}
	if _, err := db.UpsertProjectFact(f); err != nil {
		return fmt.Errorf("cairn project fact projection: %w", err)
	}
	return nil
}

// exportCairnYAML 把当前 canonical snapshot 导出为与 legacy 兼容的 YAML（只读投影，供观察/降级）。
func (w *CairnOutboxWorker) exportCairnYAML(ctx context.Context, event *database.CairnEvent) error {
	if event == nil {
		return fmt.Errorf("cairn export event is nil")
	}
	run, err := w.db.GetCairnRunByID(ctx, event.RunID)
	if err != nil {
		return err
	}
	snapshot, err := w.db.GetLatestCairnStateSnapshot(ctx, run.ProjectID, run.ConversationID)
	if err != nil {
		return err
	}
	st := cairnCanonicalStateForPrompt(snapshot)
	if err := SaveCairnState(StateFilePath(w.stateDir, run.ProjectID, run.ConversationID), st); err != nil {
		return fmt.Errorf("cairn yaml export: %w", err)
	}
	return nil
}
