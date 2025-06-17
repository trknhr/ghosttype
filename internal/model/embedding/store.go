package embedding

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/trknhr/ghosttype/internal/logger"
	"github.com/trknhr/ghosttype/internal/model/entity"

	_ "github.com/tursodatabase/go-libsql"
)

type EmbeddingStore interface {
	Exists(source, text string) bool
	Save(source, text string, vec []float32) error
	SearchSimilar(vec []float32, source string, topK int, threshold float64) ([]entity.Suggestion, error)
}

type embeddingStore struct {
	db *sql.DB
}

func NewEmbeddingStore(db *sql.DB) EmbeddingStore {
	return &embeddingStore{
		db,
	}
}

func (s *embeddingStore) Save(source, text string, vec []float32) error {
	vecJSON, err := json.Marshal(vec)

	if err != nil {
		return fmt.Errorf("failed to marshal vector: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO embeddings (source, text, emb)
		VALUES (?, ?, vector32(?))
	`, source, text, string(vecJSON))
	if err != nil {
		return fmt.Errorf("failed to insert embedding: %w", err)
	}
	return nil

}

func (s *embeddingStore) SearchSimilar(inputVec []float32, source string, topK int, threshold float64) ([]entity.Suggestion, error) {
	vecJSON, err := json.Marshal(inputVec)

	if err != nil {
		return nil, fmt.Errorf("failed to marshal input vector: %w", err)
	}

	rows, err := s.db.Query(`
WITH q AS (SELECT vector32(?) v)
SELECT e.text, 1 - vector_distance_cos(e.emb, q.v) AS score
FROM   q
JOIN vector_top_k('embeddings_idx', (SELECT v FROM q), ?) AS t
JOIN  embeddings e ON e.rowid = t.id
WHERE e.source = ?
ORDER  BY score DESC;`, string(vecJSON), topK, source)

	if err != nil {
		logger.Error("erro: %v", err)
		return nil, fmt.Errorf("failed to run vector search query: %w", err)
	}
	defer rows.Close()

	var results []entity.Suggestion
	for rows.Next() {
		var text string
		var score float64
		if err := rows.Scan(&text, &score); err != nil {
			continue
		}
		if score >= threshold {
			results = append(results, entity.Suggestion{
				Text:   text,
				Score:  score,
				Source: source,
			})
		}
	}
	return results, nil
}

func (s *embeddingStore) Exists(source, text string) bool {
	var count int
	err := s.db.QueryRow(`
		SELECT COUNT(1) FROM embeddings
		WHERE source = ? AND text = ?
	`, source, text).Scan(&count)

	if err != nil {
		return false // or log if needed
	}

	return count > 0
}
