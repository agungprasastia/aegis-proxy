package combo

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aegis-proxy/aegis/internal/database"
	"github.com/aegis-proxy/aegis/internal/models"
)

type Service struct {
	db *database.DB
}

func NewService(db *database.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Create(combo *models.Combo) error {
	if combo == nil {
		return fmt.Errorf("combo is nil")
	}
	combo.Name = strings.TrimSpace(combo.Name)
	if err := s.validate(combo); err != nil {
		return err
	}

	targets, err := json.Marshal(combo.Targets)
	if err != nil {
		return fmt.Errorf("marshal combo targets: %w", err)
	}

	now := time.Now()
	combo.CreatedAt = now
	combo.UpdatedAt = now
	result, err := s.db.Exec(`
		INSERT INTO combos (name, targets, created_at, updated_at)
		VALUES (?, ?, ?, ?)
	`, combo.Name, string(targets), combo.CreatedAt, combo.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create combo: %w", err)
	}

	combo.ID, err = result.LastInsertId()
	if err != nil {
		return fmt.Errorf("read combo id: %w", err)
	}
	return nil
}

func (s *Service) Read(id int64) (*models.Combo, error) {
	combo, err := s.readByQuery("SELECT id, name, targets, created_at, updated_at FROM combos WHERE id = ?", id)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("combo not found")
	}
	if err != nil {
		return nil, fmt.Errorf("read combo: %w", err)
	}
	return combo, nil
}

func (s *Service) Update(combo *models.Combo) error {
	if combo == nil {
		return fmt.Errorf("combo is nil")
	}
	if combo.ID == 0 {
		return fmt.Errorf("combo id is required")
	}
	combo.Name = strings.TrimSpace(combo.Name)
	if err := s.validate(combo); err != nil {
		return err
	}

	targets, err := json.Marshal(combo.Targets)
	if err != nil {
		return fmt.Errorf("marshal combo targets: %w", err)
	}

	combo.UpdatedAt = time.Now()
	result, err := s.db.Exec(`
		UPDATE combos SET name = ?, targets = ?, updated_at = ? WHERE id = ?
	`, combo.Name, string(targets), combo.UpdatedAt, combo.ID)
	if err != nil {
		return fmt.Errorf("update combo: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("combo not found")
	}
	return nil
}

func (s *Service) Delete(id int64) error {
	result, err := s.db.Exec("DELETE FROM combos WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete combo: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("combo not found")
	}
	return nil
}

func (s *Service) List() ([]*models.Combo, error) {
	rows, err := s.db.Query("SELECT id, name, targets, created_at, updated_at FROM combos ORDER BY created_at DESC")
	if err != nil {
		return nil, fmt.Errorf("list combos: %w", err)
	}
	defer rows.Close()

	var combos []*models.Combo
	for rows.Next() {
		combo, err := scanCombo(rows)
		if err != nil {
			return nil, err
		}
		combos = append(combos, combo)
	}
	return combos, rows.Err()
}

func (s *Service) validate(combo *models.Combo) error {
	if combo.Name == "" {
		return fmt.Errorf("combo name is required")
	}
	if len(combo.Targets) == 0 {
		return fmt.Errorf("combo target chain is empty")
	}

	seenTargets := make(map[string]struct{}, len(combo.Targets))
	seenPriorities := make(map[int]struct{}, len(combo.Targets))
	for i := range combo.Targets {
		target := &combo.Targets[i]
		target.Provider = strings.TrimSpace(target.Provider)
		target.Model = strings.TrimSpace(target.Model)
		if target.Provider == "" || target.Model == "" {
			return fmt.Errorf("combo target provider and model are required")
		}
		if target.Priority <= 0 {
			return fmt.Errorf("combo target priority must be positive")
		}
		key := target.Provider + ":" + target.Model
		if _, ok := seenTargets[key]; ok {
			return fmt.Errorf("duplicate combo target: %s", key)
		}
		seenTargets[key] = struct{}{}
		if _, ok := seenPriorities[target.Priority]; ok {
			return fmt.Errorf("duplicate combo target priority: %d", target.Priority)
		}
		seenPriorities[target.Priority] = struct{}{}

		if err := s.validateTarget(combo, *target, map[string]struct{}{combo.Name: {}}); err != nil {
			return err
		}
	}

	sort.Slice(combo.Targets, func(i, j int) bool {
		return combo.Targets[i].Priority < combo.Targets[j].Priority
	})
	return nil
}

func (s *Service) validateTarget(combo *models.Combo, target models.ComboTarget, visiting map[string]struct{}) error {
	if info, ok := models.GetModelInfo(target.Model); ok {
		if info.Provider != target.Provider {
			return fmt.Errorf("target %s provider mismatch: got %s want %s", target.Model, target.Provider, info.Provider)
		}
		return nil
	}

	if target.Provider != "combo" {
		return fmt.Errorf("invalid target: %s:%s", target.Provider, target.Model)
	}
	if target.Model == combo.Name || (combo.ID != 0 && target.Model == combo.Name) {
		return fmt.Errorf("recursive combo target: %s", target.Model)
	}
	if _, ok := visiting[target.Model]; ok {
		return fmt.Errorf("recursive combo target: %s", target.Model)
	}

	referenced, err := s.readByQuery("SELECT id, name, targets, created_at, updated_at FROM combos WHERE name = ?", target.Model)
	if err == sql.ErrNoRows {
		return fmt.Errorf("invalid combo target: %s", target.Model)
	}
	if err != nil {
		return fmt.Errorf("read combo target %s: %w", target.Model, err)
	}
	if combo.ID != 0 && referenced.ID == combo.ID {
		return fmt.Errorf("recursive combo target: %s", target.Model)
	}

	visiting[target.Model] = struct{}{}
	for _, child := range referenced.Targets {
		if err := s.validateTarget(combo, child, visiting); err != nil {
			return err
		}
	}
	delete(visiting, target.Model)
	return nil
}

func (s *Service) readByQuery(query string, args ...any) (*models.Combo, error) {
	return scanCombo(s.db.QueryRow(query, args...))
}

type comboScanner interface {
	Scan(dest ...any) error
}

func scanCombo(scanner comboScanner) (*models.Combo, error) {
	var combo models.Combo
	var targets string
	if err := scanner.Scan(&combo.ID, &combo.Name, &targets, &combo.CreatedAt, &combo.UpdatedAt); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(targets), &combo.Targets); err != nil {
		return nil, fmt.Errorf("unmarshal combo targets: %w", err)
	}
	return &combo, nil
}
