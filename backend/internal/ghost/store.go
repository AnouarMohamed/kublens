package ghost

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	storesql "kubelens-backend/internal/db"
	"kubelens-backend/internal/model"
)

const DefaultSimulationStoreLimit = 200

type SimulationStore struct {
	db       *sql.DB
	dialect  storesql.Dialect
	maxItems int
}

func NewSimulationStore(db *sql.DB, maxItems int, dialects ...storesql.Dialect) *SimulationStore {
	if db == nil {
		return nil
	}
	if maxItems <= 0 {
		maxItems = DefaultSimulationStoreLimit
	}
	return &SimulationStore{
		db:       db,
		dialect:  firstDialect(dialects),
		maxItems: maxItems,
	}
}

func (s *SimulationStore) Save(record model.GhostSimulationRecord) model.GhostSimulationRecord {
	if s == nil {
		return record
	}

	normalized := cloneRecord(record)
	normalized.ID = strings.TrimSpace(normalized.ID)
	if normalized.ID == "" {
		normalized.ID = strings.TrimSpace(normalized.Result.ID)
	}
	if strings.TrimSpace(normalized.CreatedAt) == "" {
		normalized.CreatedAt = strings.TrimSpace(normalized.Result.GeneratedAt)
	}
	if strings.TrimSpace(normalized.CreatedAt) == "" {
		normalized.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if strings.TrimSpace(normalized.TopologyHash) == "" {
		normalized.TopologyHash = strings.TrimSpace(normalized.Result.TopologyHash)
	}

	requestJSON, err := json.Marshal(normalized.Request)
	if err != nil {
		return record
	}
	resultJSON, err := json.Marshal(normalized.Result)
	if err != nil {
		return record
	}

	ctx := context.Background()
	result, err := s.db.ExecContext(
		ctx,
		s.bind(`UPDATE ghost_simulations
		    SET created_at = ?, action = ?, node_name = ?, topology_hash = ?, request_json = ?, result_json = ?
		  WHERE id = ?`),
		normalized.CreatedAt,
		normalized.Request.Action,
		normalized.Request.NodeName,
		normalized.TopologyHash,
		string(requestJSON),
		string(resultJSON),
		normalized.ID,
	)
	if err != nil {
		return record
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return record
	}
	if affected == 0 {
		if _, err := s.db.ExecContext(
			ctx,
			s.bind(`INSERT INTO ghost_simulations (
				id, created_at, action, node_name, topology_hash, request_json, result_json
			) VALUES (?, ?, ?, ?, ?, ?, ?)`),
			normalized.ID,
			normalized.CreatedAt,
			normalized.Request.Action,
			normalized.Request.NodeName,
			normalized.TopologyHash,
			string(requestJSON),
			string(resultJSON),
		); err != nil {
			return record
		}
	}
	_ = s.trim(ctx)
	return cloneRecord(normalized)
}

func (s *SimulationStore) List(limit int) []model.GhostSimulationRecord {
	if s == nil {
		return nil
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > s.maxItems {
		limit = s.maxItems
	}

	rows, err := s.db.QueryContext(
		context.Background(),
		s.bind(`SELECT id, created_at, topology_hash, request_json, result_json
		   FROM ghost_simulations
		  ORDER BY created_at DESC, id DESC
		  LIMIT ?`),
		limit,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()

	out := make([]model.GhostSimulationRecord, 0, limit)
	for rows.Next() {
		record, err := scanSimulationRecord(rows)
		if err != nil {
			return nil
		}
		out = append(out, record)
	}
	if err := rows.Err(); err != nil {
		return nil
	}
	return out
}

func (s *SimulationStore) Get(id string) (model.GhostSimulationRecord, bool) {
	if s == nil {
		return model.GhostSimulationRecord{}, false
	}
	needle := strings.TrimSpace(id)
	if needle == "" {
		return model.GhostSimulationRecord{}, false
	}

	row := s.db.QueryRowContext(
		context.Background(),
		s.bind(`SELECT id, created_at, topology_hash, request_json, result_json
		   FROM ghost_simulations
		  WHERE id = ?`),
		needle,
	)
	record, err := scanSimulationRecord(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.GhostSimulationRecord{}, false
	}
	if err != nil {
		return model.GhostSimulationRecord{}, false
	}
	return record, true
}

func (s *SimulationStore) trim(ctx context.Context) error {
	if s.maxItems <= 0 {
		return nil
	}
	_, err := s.db.ExecContext(
		ctx,
		s.bind(`DELETE FROM ghost_simulations
		  WHERE id NOT IN (
				SELECT id
				  FROM ghost_simulations
				 ORDER BY created_at DESC, id DESC
				 LIMIT ?
		  )`),
		s.maxItems,
	)
	return err
}

func (s *SimulationStore) bind(query string) string {
	return s.dialect.Bind(query)
}

func firstDialect(dialects []storesql.Dialect) storesql.Dialect {
	if len(dialects) > 0 && dialects[0] != "" {
		return dialects[0]
	}
	return storesql.DialectSQLite
}

type simulationRecordScanner interface {
	Scan(dest ...any) error
}

func scanSimulationRecord(scanner simulationRecordScanner) (model.GhostSimulationRecord, error) {
	var (
		record      model.GhostSimulationRecord
		requestJSON string
		resultJSON  string
	)
	if err := scanner.Scan(
		&record.ID,
		&record.CreatedAt,
		&record.TopologyHash,
		&requestJSON,
		&resultJSON,
	); err != nil {
		return model.GhostSimulationRecord{}, err
	}
	if err := json.Unmarshal([]byte(requestJSON), &record.Request); err != nil {
		return model.GhostSimulationRecord{}, err
	}
	if err := json.Unmarshal([]byte(resultJSON), &record.Result); err != nil {
		return model.GhostSimulationRecord{}, err
	}
	return cloneRecord(record), nil
}

func cloneRecord(record model.GhostSimulationRecord) model.GhostSimulationRecord {
	out := record
	out.Result.Limitations = append([]string(nil), record.Result.Limitations...)
	out.Result.Verdict.Recommendations = append([]string(nil), record.Result.Verdict.Recommendations...)
	out.Result.Frames = append([]model.GhostTimelineFrame(nil), record.Result.Frames...)
	for index := range out.Result.Frames {
		out.Result.Frames[index].Nodes = append([]model.GhostFrameNode(nil), record.Result.Frames[index].Nodes...)
		out.Result.Frames[index].Pods = append([]model.GhostFramePod(nil), record.Result.Frames[index].Pods...)
		out.Result.Frames[index].Events = append([]model.GhostTimelineEvent(nil), record.Result.Frames[index].Events...)
	}
	return out
}
