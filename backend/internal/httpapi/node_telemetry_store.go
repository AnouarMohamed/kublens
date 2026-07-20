package httpapi

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	storesql "kubelens-backend/internal/db"
	"kubelens-backend/internal/model"
)

type sqlNodeTelemetryStore struct {
	db      *sql.DB
	dialect storesql.Dialect
	max     int
	ttl     time.Duration
}

func newSQLNodeTelemetryStore(
	handle *sql.DB,
	dialect storesql.Dialect,
	maxItems int,
	ttl time.Duration,
) *sqlNodeTelemetryStore {
	if handle == nil {
		return nil
	}
	if maxItems <= 0 {
		maxItems = 256
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	if dialect == "" {
		dialect = storesql.DialectSQLite
	}
	return &sqlNodeTelemetryStore{
		db:      handle,
		dialect: dialect,
		max:     maxItems,
		ttl:     ttl,
	}
}

func (s *sqlNodeTelemetryStore) Save(sample nodeTelemetrySample, now func() time.Time) error {
	if s == nil || s.db == nil {
		return nil
	}
	if now == nil {
		now = time.Now
	}
	normalized := cloneNodeTelemetrySample(sample)
	if normalized.receivedAt.IsZero() {
		normalized.receivedAt = now().UTC()
	}
	if strings.TrimSpace(normalized.capturedAt) == "" {
		normalized.capturedAt = normalized.receivedAt.UTC().Format(time.RFC3339)
	}
	nodesJSON, err := json.Marshal(normalized.nodes)
	if err != nil {
		return err
	}

	ctx := context.Background()
	_, err = s.db.ExecContext(
		ctx,
		s.bind(`INSERT INTO node_telemetry_samples (
			id, agent_id, source, captured_at, received_at, nodes_json
		) VALUES (?, ?, ?, ?, ?, ?)`),
		nodeTelemetrySampleID(normalized),
		normalized.agentID,
		normalized.source,
		normalized.capturedAt,
		normalized.receivedAt.UTC().Format(time.RFC3339),
		string(nodesJSON),
	)
	if err != nil {
		return err
	}
	return s.prune(ctx, now().UTC())
}

func (s *sqlNodeTelemetryStore) Recent(now time.Time) []nodeTelemetrySample {
	if s == nil || s.db == nil {
		return nil
	}
	ctx := context.Background()
	_ = s.prune(ctx, now.UTC())

	rows, err := s.db.QueryContext(
		ctx,
		s.bind(`SELECT agent_id, source, captured_at, received_at, nodes_json
		   FROM node_telemetry_samples
		  ORDER BY received_at DESC, id DESC
		  LIMIT ?`),
		s.max,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()

	out := make([]nodeTelemetrySample, 0, s.max)
	for rows.Next() {
		sample, err := scanNodeTelemetrySample(rows)
		if err != nil {
			return nil
		}
		out = append(out, sample)
	}
	if err := rows.Err(); err != nil {
		return nil
	}
	return out
}

func (s *sqlNodeTelemetryStore) prune(ctx context.Context, now time.Time) error {
	if s == nil || s.db == nil {
		return nil
	}
	if s.ttl > 0 {
		cutoff := now.Add(-s.ttl).UTC().Format(time.RFC3339)
		if _, err := s.db.ExecContext(
			ctx,
			s.bind(`DELETE FROM node_telemetry_samples WHERE received_at < ?`),
			cutoff,
		); err != nil {
			return err
		}
	}
	if s.max <= 0 {
		return nil
	}
	_, err := s.db.ExecContext(
		ctx,
		s.bind(`DELETE FROM node_telemetry_samples
		  WHERE id NOT IN (
				SELECT id
				  FROM node_telemetry_samples
				 ORDER BY received_at DESC, id DESC
				 LIMIT ?
		  )`),
		s.max,
	)
	return err
}

func (s *sqlNodeTelemetryStore) bind(query string) string {
	return s.dialect.Bind(query)
}

type nodeTelemetrySampleScanner interface {
	Scan(dest ...any) error
}

func scanNodeTelemetrySample(scanner nodeTelemetrySampleScanner) (nodeTelemetrySample, error) {
	var (
		sample       nodeTelemetrySample
		receivedRaw  string
		nodesJSONRaw string
	)
	if err := scanner.Scan(
		&sample.agentID,
		&sample.source,
		&sample.capturedAt,
		&receivedRaw,
		&nodesJSONRaw,
	); err != nil {
		return nodeTelemetrySample{}, err
	}
	receivedAt, err := time.Parse(time.RFC3339, receivedRaw)
	if err != nil {
		return nodeTelemetrySample{}, err
	}
	var nodes []model.NodeTelemetryItem
	if err := json.Unmarshal([]byte(nodesJSONRaw), &nodes); err != nil {
		return nodeTelemetrySample{}, err
	}
	sample.receivedAt = receivedAt
	sample.nodes = nodes
	return cloneNodeTelemetrySample(sample), nil
}

func nodeTelemetrySampleID(sample nodeTelemetrySample) string {
	h := sha256.Sum256([]byte(fmt.Sprintf(
		"%s|%s|%s|%d",
		sample.agentID,
		sample.source,
		sample.receivedAt.UTC().Format(time.RFC3339Nano),
		len(sample.nodes),
	)))
	return "ntel-" + hex.EncodeToString(h[:])[:16]
}
