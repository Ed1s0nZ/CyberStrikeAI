package knowledge

import (
	"database/sql"
	"fmt"
)

// EnsureKnowledgeEmbeddingsSchema migrates knowledge_embeddings for sub_indexes + embedding metadata.
func EnsureKnowledgeEmbeddingsSchema(db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='knowledge_embeddings'`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		return nil
	}
	if err := addKnowledgeEmbeddingsColumnIfMissing(db, "sub_indexes",
		`ALTER TABLE knowledge_embeddings ADD COLUMN sub_indexes TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}
	if err := addKnowledgeEmbeddingsColumnIfMissing(db, "embedding_model",
		`ALTER TABLE knowledge_embeddings ADD COLUMN embedding_model TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}
	if err := addKnowledgeEmbeddingsColumnIfMissing(db, "embedding_dim",
		`ALTER TABLE knowledge_embeddings ADD COLUMN embedding_dim INTEGER NOT NULL DEFAULT 0`); err != nil {
		return err
	}
	return nil
}

func addKnowledgeEmbeddingsColumnIfMissing(db *sql.DB, column, alterSQL string) error {
	var colCount int
	q := `SELECT COUNT(*) FROM pragma_table_info('knowledge_embeddings') WHERE name = ?`
	if err := db.QueryRow(q, column).Scan(&colCount); err != nil {
		return err
	}
	if colCount > 0 {
		return nil
	}
	_, err := db.Exec(alterSQL)
	return err
}

// ensureKnowledgeEmbeddingsSubIndexesColumn 向后兼容；请使用 [EnsureKnowledgeEmbeddingsSchema]。
func ensureKnowledgeEmbeddingsSubIndexesColumn(db *sql.DB) error {
	return EnsureKnowledgeEmbeddingsSchema(db)
}
