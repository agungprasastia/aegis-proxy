package combo

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/aegis-proxy/aegis/internal/database"
	"github.com/aegis-proxy/aegis/internal/models"
)

func TestCreateReadUpdateDeleteCombo(t *testing.T) {
	db := openComboTestDB(t, filepath.Join(t.TempDir(), "combos.db"))
	service := NewService(db)

	combo := &models.Combo{
		Name: "standard-fallback",
		Targets: []models.ComboTarget{
			{Provider: models.ProviderCodeBuddy, Model: "gpt-5.2", Priority: 2},
			{Provider: models.ProviderKiro, Model: "claude-sonnet-4", Priority: 1},
		},
	}
	if err := service.Create(combo); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if combo.ID == 0 {
		t.Fatalf("Create() did not assign id")
	}
	if combo.Targets[0].Priority != 1 || combo.Targets[1].Priority != 2 {
		t.Fatalf("targets not ordered by priority: %#v", combo.Targets)
	}

	read, err := service.Read(combo.ID)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if read.Name != combo.Name || len(read.Targets) != 2 {
		t.Fatalf("Read() = %#v", read)
	}

	combo.Targets = []models.ComboTarget{{Provider: models.ProviderKiro, Model: "deepseek-3.2", Priority: 1}}
	if err := service.Update(combo); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	updated, err := service.Read(combo.ID)
	if err != nil {
		t.Fatalf("Read() updated error = %v", err)
	}
	if got := updated.Targets[0].Model; got != "deepseek-3.2" {
		t.Fatalf("updated target = %q", got)
	}

	list, err := service.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List() length = %d", len(list))
	}

	if err := service.Delete(combo.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := service.Read(combo.ID); err == nil {
		t.Fatalf("Read() deleted combo expected error")
	}
}

func TestValidationRejectsLoopsAndDuplicates(t *testing.T) {
	db := openComboTestDB(t, filepath.Join(t.TempDir(), "combos.db"))
	service := NewService(db)

	base := &models.Combo{
		Name:    "base",
		Targets: []models.ComboTarget{{Provider: models.ProviderKiro, Model: "claude-sonnet-4", Priority: 1}},
	}
	if err := service.Create(base); err != nil {
		t.Fatalf("Create(base) error = %v", err)
	}

	cases := []struct {
		name    string
		combo   *models.Combo
		wantErr string
	}{
		{
			name:    "empty chain",
			combo:   &models.Combo{Name: "empty"},
			wantErr: "empty",
		},
		{
			name: "duplicate target",
			combo: &models.Combo{Name: "dupe-target", Targets: []models.ComboTarget{
				{Provider: models.ProviderKiro, Model: "claude-sonnet-4", Priority: 1},
				{Provider: models.ProviderKiro, Model: "claude-sonnet-4", Priority: 2},
			}},
			wantErr: "duplicate combo target",
		},
		{
			name: "duplicate priority",
			combo: &models.Combo{Name: "dupe-priority", Targets: []models.ComboTarget{
				{Provider: models.ProviderKiro, Model: "claude-sonnet-4", Priority: 1},
				{Provider: models.ProviderCodeBuddy, Model: "gpt-5.2", Priority: 1},
			}},
			wantErr: "duplicate combo target priority",
		},
		{
			name: "invalid target",
			combo: &models.Combo{Name: "invalid", Targets: []models.ComboTarget{
				{Provider: models.ProviderKiro, Model: "gpt-5.2", Priority: 1},
			}},
			wantErr: "provider mismatch",
		},
		{
			name: "direct recursion",
			combo: &models.Combo{Name: "self", Targets: []models.ComboTarget{
				{Provider: "combo", Model: "self", Priority: 1},
			}},
			wantErr: "recursive",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := service.Create(tc.combo)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Create() error = %v, want containing %q", err, tc.wantErr)
			}
		})
	}

	referencing := &models.Combo{
		Name:    "references-base",
		Targets: []models.ComboTarget{{Provider: "combo", Model: "base", Priority: 1}},
	}
	if err := service.Create(referencing); err != nil {
		t.Fatalf("Create(referencing) error = %v", err)
	}
	base.Targets = []models.ComboTarget{{Provider: "combo", Model: "references-base", Priority: 1}}
	if err := service.Update(base); err == nil || !strings.Contains(err.Error(), "recursive") {
		t.Fatalf("Update(base) error = %v, want recursive", err)
	}
}

func TestCombosSurviveRestart(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "combos.db")
	db := openComboTestDB(t, dbPath)
	service := NewService(db)

	created := &models.Combo{
		Name: "restart-safe",
		Targets: []models.ComboTarget{
			{Provider: models.ProviderKiro, Model: "claude-sonnet-4", Priority: 1},
			{Provider: models.ProviderCodeBuddy, Model: "gpt-5.2", Priority: 2},
			{Provider: models.ProviderWavespeed, Model: "wavespeed-gpt-5", Priority: 3},
		},
	}
	if err := service.Create(created); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	reopened, err := database.InitDB(dbPath)
	if err != nil {
		t.Fatalf("reopen db: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	restartedService := NewService(reopened)

	read, err := restartedService.Read(created.ID)
	if err != nil {
		t.Fatalf("Read() after restart error = %v", err)
	}
	if read.Name != created.Name || len(read.Targets) != 3 || read.Targets[2].Model != "wavespeed-gpt-5" {
		t.Fatalf("persisted combo = %#v", read)
	}
}

func openComboTestDB(t *testing.T, dbPath string) *database.DB {
	t.Helper()
	db, err := database.InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
