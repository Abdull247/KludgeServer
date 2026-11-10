package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultTxnTTL     = 300 * time.Second
	defaultHandoffTTL = 60 * time.Second
)

type HandoffStore struct {
	pool       *pgxpool.Pool
	txnTTL     time.Duration
	handoffTTL time.Duration
	mu         sync.Mutex
	txns       map[string]memoryTxn
	handoffs   map[string]memoryHandoff
}

type memoryTxn struct {
	provider    string
	client      string
	sessionData string
	expiresAt   time.Time
}

type memoryHandoff struct {
	userID    uuid.UUID
	expiresAt time.Time
	consumed  bool
}

func NewHandoffStore(pool *pgxpool.Pool, txnTTLSeconds, handoffTTLSeconds int) *HandoffStore {
	txnTTL := time.Duration(txnTTLSeconds) * time.Second
	if txnTTL <= 0 {
		txnTTL = defaultTxnTTL
	}
	handoffTTL := time.Duration(handoffTTLSeconds) * time.Second
	if handoffTTL <= 0 {
		handoffTTL = defaultHandoffTTL
	}
	return &HandoffStore{
		pool:       pool,
		txnTTL:     txnTTL,
		handoffTTL: handoffTTL,
		txns:       map[string]memoryTxn{},
		handoffs:   map[string]memoryHandoff{},
	}
}

func (s *HandoffStore) PutTransaction(ctx context.Context, state, provider, client, sessionData string) error {
	hash := sha256Hex(state)
	expires := time.Now().Add(s.txnTTL)
	if s.pool == nil {
		s.mu.Lock()
		s.txns[hash] = memoryTxn{provider: provider, client: client, sessionData: sessionData, expiresAt: expires}
		s.mu.Unlock()
		return nil
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO oauth_transactions (state_hash, provider, client, session_data, expires_at)
		VALUES ($1, $2, $3, $4, $5)
	`, hash, provider, client, sessionData, expires)
	if err != nil {
		return fmt.Errorf("insert oauth transaction: %w", err)
	}
	return nil
}

func (s *HandoffStore) TakeTransaction(ctx context.Context, state, provider string) (client, sessionData string, ok bool, err error) {
	hash := sha256Hex(state)
	if s.pool == nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		txn, found := s.txns[hash]
		if !found || txn.provider != provider || time.Now().After(txn.expiresAt) {
			delete(s.txns, hash)
			return "", "", false, nil
		}
		delete(s.txns, hash)
		return txn.client, txn.sessionData, true, nil
	}
	err = s.pool.QueryRow(ctx, `
		DELETE FROM oauth_transactions
		WHERE state_hash = $1 AND provider = $2 AND expires_at > now()
		RETURNING client, session_data
	`, hash, provider).Scan(&client, &sessionData)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, fmt.Errorf("take oauth transaction: %w", err)
	}
	return client, sessionData, true, nil
}

func (s *HandoffStore) PutHandoff(ctx context.Context, userID uuid.UUID) (ticket string, err error) {
	ticket, err = randomToken(32)
	if err != nil {
		return "", err
	}
	hash := sha256Hex(ticket)
	expires := time.Now().Add(s.handoffTTL)
	if s.pool == nil {
		s.mu.Lock()
		s.handoffs[hash] = memoryHandoff{userID: userID, expiresAt: expires}
		s.mu.Unlock()
		return ticket, nil
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO oauth_handoffs (ticket_hash, user_id, expires_at)
		VALUES ($1, $2, $3)
	`, hash, userID, expires)
	if err != nil {
		return "", fmt.Errorf("insert oauth handoff: %w", err)
	}
	return ticket, nil
}

func (s *HandoffStore) ConsumeHandoff(ctx context.Context, ticket string) (uuid.UUID, bool, error) {
	hash := sha256Hex(ticket)
	if s.pool == nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		h, found := s.handoffs[hash]
		if !found || h.consumed || time.Now().After(h.expiresAt) {
			return uuid.Nil, false, nil
		}
		h.consumed = true
		s.handoffs[hash] = h
		return h.userID, true, nil
	}
	var userID uuid.UUID
	err := s.pool.QueryRow(ctx, `
		UPDATE oauth_handoffs
		SET consumed_at = now()
		WHERE ticket_hash = $1 AND consumed_at IS NULL AND expires_at > now()
		RETURNING user_id
	`, hash).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("consume oauth handoff: %w", err)
	}
	return userID, true, nil
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func randomToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return strings.TrimRight(base64.RawURLEncoding.EncodeToString(buf), "="), nil
}
