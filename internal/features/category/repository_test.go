package category

import (
	"backend/internal/shared/testutil"
	"context"
	"fmt"
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

func TestGetAllCategories(t *testing.T) {
	t.Run("pagination motong data dengan benar", func(t *testing.T) {
		resetTable(t)
		ctx := context.Background()
		for i := 0; i < 5; i++ {
			categoryRepo.CreateCategory(ctx, fmt.Sprintf("category %d", i), "expense", "#ffffff", "icon.jpeg")
		}

		categories, total, err := categoryRepo.GetAllCategories(ctx, 2, 0)

		if err != nil {
			t.Error(err)
		}

		if len(categories) != 2 {
			t.Errorf("jumlah kategori %d, mau 2", len(categories))
		}

		if total != 5 {
			t.Errorf("total data %d, mau 5", total)
		}
	})

	t.Run("soft-deleted tidak muncul", func(t *testing.T) {
		resetTable(t)
		ctx := context.Background()

		for i := 0; i < 3; i++ {
			categoryRepo.CreateCategory(ctx, fmt.Sprintf("category %d", i), "expense", "#ffffff", "icon.jpeg")
		}

		categories, _, err := categoryRepo.GetAllCategories(ctx, 10, 0)
		if err != nil {
			t.Errorf("error: %v", err)
		}

		_, err = categoryRepo.DeleteCategory(ctx, categories[0].ID)

		if err != nil {
			t.Errorf("error: %v", err)
		}

		categories, total, err := categoryRepo.GetAllCategories(ctx, 10, 0)
		if err != nil {
			t.Errorf("error: %v", err)
		}

		if total != 2 {
			t.Errorf("total %d, mau 2", total)
		}

		if len(categories) != 2 {
			t.Errorf("panjang kategori %d, mau 0", len(categories))
		}

	})

	t.Run("kosong", func(t *testing.T) {
		resetTable(t)
		ctx := context.Background()
		categories, total, err := categoryRepo.GetAllCategories(ctx, 10, 0)
		if err != nil {
			t.Errorf("error: %v", err)
		}
		if len(categories) != 0 {
			t.Errorf("panjang kategori %d, mau 0", len(categories))
		}

		if total != 0 {
			t.Errorf("total %d, mau 0", total)
		}
	})
}
