package repo

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	db_domain "schma.ai/internal/domain/db"
	"schma.ai/internal/domain/speech"
	db "schma.ai/internal/infra/db/generated"
)

// compile-time check
var _ db_domain.TranscriptsRepo = (*TranscriptsRepo)(nil)

type TranscriptsRepo struct {
	q *db.Queries
}

func NewTranscriptsRepo(pool *pgxpool.Pool) *TranscriptsRepo {
	return &TranscriptsRepo{
		q: db.New(pool),
	}
}

// StoreTranscript stores a transcript in the database
func (r *TranscriptsRepo) StoreTranscript(ctx context.Context, sessionID pgtype.UUID, transcript speech.Transcript) (db.Transcript, error) {
	// Marshal words and turns to JSONB if they exist
	var wordsBytes []byte
	var turnsBytes []byte
	var phrasesBytes []byte
	var err error

	if len(transcript.Words) > 0 {
		wordsBytes, err = json.Marshal(transcript.Words)
		if err != nil {
			return db.Transcript{}, err
		}
	}

	if len(transcript.Turns) > 0 {
		turnsBytes, err = json.Marshal(transcript.Turns)
		if err != nil {
			return db.Transcript{}, err
		}
	}

	if len(transcript.Phrases) > 0 {
		phrasesBytes, err = json.Marshal(transcript.Phrases)
		if err != nil {
			return db.Transcript{}, err
		}
	}

	_, err = r.q.AddTranscript(ctx, db.AddTranscriptParams{
		SessionID:     sessionID,
		Text:          transcript.Text,
		IsFinal:       transcript.IsFinal,
		Confidence:    pgtype.Float4{Float32: transcript.Confidence, Valid: transcript.Confidence > 0},
		Stability:     pgtype.Float4{Float32: transcript.Stability, Valid: transcript.Stability > 0},
		ChunkDurSec:   pgtype.Float8{Float64: transcript.ChunkDurSec, Valid: transcript.ChunkDurSec > 0},
		Channel:       pgtype.Int4{Int32: int32(transcript.Channel), Valid: transcript.Channel > 0},
		Words:         wordsBytes,
		Turns:         turnsBytes,
		Phrases:       phrasesBytes,
		CreatedAt:     pgtype.Timestamp{Time: time.Now(), Valid: true},
	})

	return db.Transcript{}, err
}

// GetTranscriptsBySession retrieves all transcripts for a session
func (r *TranscriptsRepo) GetTranscriptsBySession(ctx context.Context, sessionID pgtype.UUID) ([]db.Transcript, error) {
	return r.q.ListTranscriptsBySession(ctx, sessionID)
}
