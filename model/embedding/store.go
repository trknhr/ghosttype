package embedding

import (
	"bytes"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/trknhr/ghosttype/internal/logger.go"
	"github.com/trknhr/ghosttype/model"
	_ "github.com/tursodatabase/go-libsql"
)

type EmbeddingStore struct {
	DB *sql.DB
}

func (s *EmbeddingStore) Save(source, text string, vec []float32) error {
	vecJSON, err := json.Marshal(vec)

	if err != nil {
		return fmt.Errorf("failed to marshal vector: %w", err)
	}

	_, err = s.DB.Exec(`
		INSERT INTO embeddings (source, text, emb)
		VALUES (?, ?, vector32(?))
	`, source, text, string(vecJSON))
	if err != nil {
		return fmt.Errorf("failed to insert embedding: %w", err)
	}
	return nil

}

func (s *EmbeddingStore) SearchBySource(source string) ([]string, [][]float32, error) {
	rows, err := s.DB.Query(`
		SELECT text, emb FROM embeddings WHERE source = ?
	`, source)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var texts []string
	var vectors [][]float32

	for rows.Next() {
		var text string
		var blob []byte
		if err := rows.Scan(&text, &blob); err != nil {
			continue
		}
		reader := bytes.NewReader(blob)
		vec := make([]float32, len(blob)/4)
		if err := binary.Read(reader, binary.LittleEndian, vec); err != nil {
			continue
		}
		texts = append(texts, text)
		vectors = append(vectors, vec)
	}
	return texts, vectors, nil
}

func (s *EmbeddingStore) SearchSimilar(inputVec []float32, source string, topK int, threshold float64) ([]model.Suggestion, error) {
	vecJSON, err := json.Marshal(inputVec)

	if err != nil {
		return nil, fmt.Errorf("failed to marshal input vector: %w", err)
	}

	rows, err := s.DB.Query(`
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

	var results []model.Suggestion
	for rows.Next() {
		var text string
		var score float64
		if err := rows.Scan(&text, &score); err != nil {
			continue
		}
		if score >= threshold {
			results = append(results, model.Suggestion{
				Text:   text,
				Score:  score,
				Source: source,
			})
		}
	}
	return results, nil
}

func (s *EmbeddingStore) Exists(source, text string) bool {
	var count int
	err := s.DB.QueryRow(`
		SELECT COUNT(1) FROM embeddings
		WHERE source = ? AND text = ?
	`, source, text).Scan(&count)

	if err != nil {
		return false // or log if needed
	}

	return count > 0
}
