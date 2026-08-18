package category

import (
	"backend/internal/shared/apperror"
	"backend/internal/shared/testutil"
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"
)

var categoryRepoTest *CategoryRepository

func TestMain(m *testing.M) {
	db := testutil.ConnectTestDB("../../../.env")

	categoryRepoTest = NewCategoryRepository(db)

	code := m.Run()
	os.Exit(code)
}

func resetTable(t *testing.T) {
	if _, err := categoryRepoTest.db.Exec("TRUNCATE categories CASCADE"); err != nil {
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

			category, err := categoryRepoTest.CreateCategory(ctx, tt.catName, tt.catType, tt.color, tt.icon)
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
			categoryRepoTest.CreateCategory(ctx, fmt.Sprintf("category %d", i), "expense", "#ffffff", "icon.jpeg")
		}

		categories, total, err := categoryRepoTest.GetAllCategories(ctx, 2, 0)

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
			categoryRepoTest.CreateCategory(ctx, fmt.Sprintf("category %d", i), "expense", "#ffffff", "icon.jpeg")
		}

		categories, _, err := categoryRepoTest.GetAllCategories(ctx, 10, 0)
		if err != nil {
			t.Errorf("error: %v", err)
		}

		_, err = categoryRepoTest.DeleteCategory(ctx, categories[0].ID)

		if err != nil {
			t.Errorf("error: %v", err)
		}

		categories, total, err := categoryRepoTest.GetAllCategories(ctx, 10, 0)
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
		categories, total, err := categoryRepoTest.GetAllCategories(ctx, 10, 0)
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

func TestPatchCategory(t *testing.T) {

	tests := []struct {
		name     string
		catName  string
		catType  string
		catColor string
		catIcon  string
	}{
		{"patches name", "kategori baru", "expense", "#ff44ff", "iniicon"},
		{"patches type", "kategori 1", "income", "#ff44ff", "iniicon"},
		{"patches color", "kategori 1", "expense", "#ff1133", "iniicon"},
		{"patches icon", "kategori 1", "expense", "#ff44ff", "iniiconyangbaru"},
		{"patches all field", "kategori baru", "income", "#aa44ff", "iniicon"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetTable(t)
			ctx := context.Background()
			categoryRepoTest.CreateCategory(ctx, "kategori 1", "expense", "#ff44ff", "iniicon")
			categories, _, _ := categoryRepoTest.GetAllCategories(ctx, 10, 0)
			timeBeforeUpdated := categories[0].UpdatedAt

			updatedCategory, err := categoryRepoTest.PatchCategory(ctx, categories[0].ID, &tt.catName, &tt.catType, &tt.catColor, &tt.catIcon)

			if err != nil {
				t.Fatalf("error: %v", err)
			}

			if updatedCategory.Name != tt.catName {
				t.Errorf("name %s, mau %s", updatedCategory.Name, tt.name)
			}

			if updatedCategory.Type != tt.catType {
				t.Errorf("type %s, mau %s", updatedCategory.Type, tt.catType)
			}

			if updatedCategory.Color != tt.catColor {
				t.Errorf("color %s, mau %s", updatedCategory.Color, tt.catColor)
			}

			if updatedCategory.Icon != tt.catIcon {
				t.Errorf("icon %s, mau %s", updatedCategory.Icon, tt.catIcon)
			}

			if time.Time.Equal(timeBeforeUpdated, updatedCategory.UpdatedAt) {
				t.Error("updated_at tidak berubah, harusnya berubah 1 detik")
			}

		})
	}

	t.Run("error category name sudah ada", func(t *testing.T) {
		resetTable(t)
		ctx := context.Background()

		categoryRepoTest.CreateCategory(ctx, "makan", "expense", "#ffffff", "iniicon")
		categoryRepoTest.CreateCategory(ctx, "minum", "expense", "#ffffff", "iniicon")

		categories, _, _ := categoryRepoTest.GetAllCategories(ctx, 10, 0)

		newName := "minum"

		_, err := categoryRepoTest.PatchCategory(ctx, categories[0].ID, &newName, nil, nil, nil)

		if !errors.Is(err, apperror.ErrCategoryAlreadyExists) {
			t.Errorf("tidak ada error, mau error constraint already have")

		}

	})

	t.Run("error constraint type selain 'expense' dan 'income'", func(t *testing.T) {
		resetTable(t)
		ctx := context.Background()
		categoryRepoTest.CreateCategory(ctx, "makan", "expense", "#ffffff", "iniicon")
		categories, _, _ := categoryRepoTest.GetAllCategories(ctx, 10, 0)

		newType := "lainnya"

		_, err := categoryRepoTest.PatchCategory(ctx, categories[0].ID, nil, &newType, nil, nil)

		if err == nil {
			t.Errorf("tidak ada error, mau error constraint")
		}
	})
	t.Run("not found", func(t *testing.T) {
		resetTable(t)
		ctx := context.Background()

		_, err := categoryRepoTest.PatchCategory(ctx, "e9c9017d-3287-4711-8535-9e8faa7143da", nil, nil, nil, nil)

		var nf apperror.NotFoundError
		if !errors.As(err, &nf) {
			t.Errorf("mau NotFoundError, dapat %v", err)
		}
	})
}

func TestDeleteCategory(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		resetTable(t)
		ctx := context.Background()
		categoryRepoTest.CreateCategory(ctx, "makan", "expense", "#ffffff", "iniicon")
		categories, _, _ := categoryRepoTest.GetAllCategories(ctx, 10, 0)

		category, err := categoryRepoTest.DeleteCategory(ctx, categories[0].ID)
		if err != nil {
			t.Errorf("error: %v", err)
		}
		categories, total, _ := categoryRepoTest.GetAllCategories(ctx, 10, 0)

		if !category.DeletedAt.Valid {
			t.Errorf("deleted at masih null, mau isi")
		}

		if total != 0 {
			t.Error("total tidak nol, mau nol")
		}

		if len(categories) != 0 {
			t.Errorf("panjang categories tidak nol, harusnya 0")
		}
	})

	t.Run("not found", func(t *testing.T) {
		resetTable(t)
		ctx := context.Background()

		_, err := categoryRepoTest.DeleteCategory(ctx, "e9c9017d-3287-4711-8535-9e8faa7143da")

		var nf apperror.NotFoundError
		if !errors.As(err, &nf) {
			t.Errorf("mau NotFoundError, dapat %v", err)
		}
	})
}

func TestGetCategoryByIDRepository(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		resetTable(t)
		ctx := context.Background()
		categoryRepoTest.CreateCategory(ctx, "makan", "expense", "#ffffff", "iniicon")
		categories, _, _ := categoryRepoTest.GetAllCategories(ctx, 10, 0)

		category, err := categoryRepoTest.GetCategoryByID(ctx, categories[0].ID)

		if err != nil {
			t.Fatalf("error: %v", err)
		}

		if category.ID != categories[0].ID {
			t.Errorf("ID %s, mau %s", category.ID, categories[0].ID)
		}

		if category.Name != "makan" {
			t.Errorf("name %s, mau makan", category.Name)
		}
	})

	t.Run("not found", func(t *testing.T) {
		resetTable(t)
		ctx := context.Background()

		_, err := categoryRepoTest.GetCategoryByID(ctx, "e9c9017d-3287-4711-8535-9e8faa7143da")

		var nf apperror.NotFoundError
		if !errors.As(err, &nf) {
			t.Errorf("mau NotFoundError, dapat %v", err)
		}
	})

}
func TestRestoreCategoryRepository(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		resetTable(t)
		ctx := context.Background()
		categoryRepoTest.CreateCategory(ctx, "makan", "expense", "#ffffff", "iniicon")
		categories, _, _ := categoryRepoTest.GetAllCategories(ctx, 10, 0)

		deletedCatgory, err := categoryRepoTest.DeleteCategory(ctx, categories[0].ID)
		category, err := categoryRepoTest.RestoreCategory(ctx, deletedCatgory.ID)

		if err != nil {
			t.Fatalf("error: %v", err)
		}

		if category.ID != categories[0].ID {
			t.Errorf("ID %s, mau %s", category.ID, categories[0].ID)
		}

		if category.Name != "makan" {
			t.Errorf("name %s, mau makan", category.Name)
		}
	})

	t.Run("not found", func(t *testing.T) {
		resetTable(t)
		ctx := context.Background()

		_, err := categoryRepoTest.RestoreCategory(ctx, "e9c9017d-3287-4711-8535-9e8faa7143da")

		var nf apperror.NotFoundError
		if !errors.As(err, &nf) {
			t.Errorf("mau NotFoundError, dapat %v", err)
		}
	})

}
