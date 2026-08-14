package category

import (
	"backend/internal/shared/testutil"
	"context"
	"os"
	"testing"
)

var categoryRepo *CategoryRepository

func TestMain(m *testing.M) {
	db := testutil.ConnectTestDB("../../../.env")

	categoryRepo = NewCategoryRepository(db)

	code := m.Run()
	os.Exit(code)
}

func resetTable(t *testing.T) {
	if _, err := categoryRepo.db.Exec("TRUNCATE categories CASCADE"); err != nil {
		t.Error(err)
	}
}

func TestCreateCategoryRepo(t *testing.T) {
	tests := []struct {
		name      string
		catName   string
		catType   string
		color     string
		icon      string
		wantError bool
	}{
		{"success", "Makan", "expense", "#FFD93D", "circle", false},
		{"error constraint", "Makan", "lainnya", "#FFD93D", "circle", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetTable(t)
			ctx := context.Background()

			category, err := categoryRepo.CreateCategory(ctx, tt.catName, tt.catType, tt.color, tt.icon)
			if tt.wantError {
				if err == nil {
					t.Errorf("tidak ada error, mau error constraint")
				}

				return
			}
			if err != nil {
				t.Fatalf("error: %v", err)
			}

			if category.ID == "" {
				t.Error("ID kosong, harusnya ke generate")
			}

			if category.Name != tt.catName {
				t.Errorf("nama %s, mau %s", category.Name, tt.catName)
			}

			if category.Type != tt.catType {
				t.Errorf("type %s, mau %s", category.Type, tt.catType)
			}

			if category.Icon != tt.icon {
				t.Errorf("icon %s, mau %s", category.Icon, tt.icon)
			}

			if category.Color != tt.color {
				t.Errorf("color %s, mau %s", category.Color, tt.color)
			}

			if category.IsDefault != false {
				t.Errorf("is_default %v, mau false", category.IsDefault)
			}

			if category.CreatedAt.IsZero() {
				t.Error("created_at kosong, harusnya keisi now()")
			}

			if category.UpdatedAt.IsZero() {
				t.Error("updated_at kosong, harusnya keisi now()")
			}

			if category.DeletedAt.Valid {
				t.Error("deleted_at tidak null, harusnya kosong")
			}
		})
	}
}
