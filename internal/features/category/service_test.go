package category

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"
)

type FakeCategoryRepository struct {
	CreateCategoryFunc   func(ctx context.Context, name string, catType string, color string, icon string) (Category, error)
	GetAllCategoriesFunc func(ctx context.Context, limit int, offset int) ([]Category, int, error)
	PatchCategoryFunc    func(ctx context.Context, id string, name *string, catType *string, color *string, icon *string) (Category, error)
	DeleteCategoryFunc   func(ctx context.Context, id string) (Category, error)
	GetCategoryByIDFunc  func(ctx context.Context, id string) (Category, error)
	RestoreCategoryFunc  func(ctx context.Context, id string) (Category, error)
}

func (f *FakeCategoryRepository) CreateCategory(ctx context.Context, name string, catType string, color string, icon string) (Category, error) {
	return f.CreateCategoryFunc(ctx, name, catType, color, icon)
}

func (f *FakeCategoryRepository) GetAllCategories(ctx context.Context, limit int, offset int) ([]Category, int, error) {
	return f.GetAllCategoriesFunc(ctx, limit, offset)
}

func (f *FakeCategoryRepository) PatchCategory(ctx context.Context, id string, name *string, catType *string, color *string, icon *string) (Category, error) {
	return f.PatchCategoryFunc(ctx, id, name, catType, color, icon)
}

func (f *FakeCategoryRepository) DeleteCategory(ctx context.Context, id string) (Category, error) {
	return f.DeleteCategoryFunc(ctx, id)
}

func (f *FakeCategoryRepository) GetCategoryByID(ctx context.Context, id string) (Category, error) {
	return f.GetCategoryByIDFunc(ctx, id)
}

func (f *FakeCategoryRepository) RestoreCategory(ctx context.Context, id string) (Category, error) {
	return f.RestoreCategoryFunc(ctx, id)
}

func TestCreateCategoryService(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctx := context.Background()
		fakeRepo := &FakeCategoryRepository{
			CreateCategoryFunc: func(ctx context.Context, name, catType, color, icon string) (Category, error) {
				return Category{Name: name, Type: catType, Color: color, Icon: icon}, nil
			},
		}

		service := NewCategoryService(fakeRepo)

		category, err := service.CreateCategory(ctx, &CreateCategoryRequest{
			Name:  "kos",
			Type:  "expense",
			Color: "#ff44ff",
			Icon:  "inisebuahicon",
		})

		if err != nil {
			t.Errorf("tidak ekpektasi dapat error, tapi dapat %v", err)
		}

		if category.Name != "kos" {
			t.Errorf("name %s, mau %s", category.Name, "kos")
		}

		if category.Type != "expense" {
			t.Errorf("type %s, mau %s", category.Type, "expense")
		}
		if category.Color != "#ff44ff" {
			t.Errorf("color %s, mau %s", category.Color, "#ff44ff")
		}
		if category.Icon != "inisebuahicon" {
			t.Errorf("icon %s, mau %s", category.Icon, "inisebuahicon")
		}

	})

	t.Run("nerusin error", func(t *testing.T) {
		ctx := context.Background()
		fakeRepo := &FakeCategoryRepository{
			CreateCategoryFunc: func(ctx context.Context, name, catType, color, icon string) (Category, error) {
				return Category{}, errors.New("error DB")
			},
		}

		service := NewCategoryService(fakeRepo)

		category, err := service.CreateCategory(ctx, &CreateCategoryRequest{
			Name:  "kos",
			Type:  "expense",
			Color: "#ff44ff",
			Icon:  "inisebuahicon",
		})

		if err == nil {
			t.Error("harusnya error DB, tetapi tidak dapat")
		}

		if err.Error() != "error DB" {
			t.Errorf("error %q, mau %q", err.Error(), "error DB")
		}

		if category != (Category{}) {
			t.Error("harusnya tidak return data, tetapi malah return data")
		}
	})
}

func TestGetAllCategoryService(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctx := context.Background()
		fakeRepo := &FakeCategoryRepository{
			GetAllCategoriesFunc: func(ctx context.Context, limit, offset int) ([]Category, int, error) {
				return []Category{{Name: "ini ketogori"}}, 1, nil
			},
		}

		serviceTest := NewCategoryService(fakeRepo)

		categories, total, err := serviceTest.GetAllCategories(ctx, 10, 0)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		if total != 1 {
			t.Errorf("total %d, mau 1", total)
		}

		if len(categories) != 1 {
			t.Errorf("len categories %d, mau 1", len(categories))
		}
	})

	t.Run("forward error", func(t *testing.T) {
		ctx := context.Background()
		fakeRepo := &FakeCategoryRepository{
			GetAllCategoriesFunc: func(ctx context.Context, limit, offset int) ([]Category, int, error) {
				return nil, 0, errors.New("error DB")
			},
		}

		serviceTest := NewCategoryService(fakeRepo)

		_, _, err := serviceTest.GetAllCategories(ctx, 10, 0)
		if err == nil {
			t.Error("harusnya error DB, tetapi tidak dapat")
		}

		if err.Error() != "error DB" {
			t.Errorf("error %q, mau %q", err.Error(), "error DB")
		}
	})
}

func TestPatchCategoryService(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctx := context.Background()
		fakeRepo := &FakeCategoryRepository{
			PatchCategoryFunc: func(ctx context.Context, id string, name, catType, color, icon *string) (Category, error) {
				return Category{Name: "makan", Type: "expense"}, nil
			},
		}

		serviceTest := NewCategoryService(fakeRepo)

		newName := "makan"
		newType := "expense"
		newColor := "#123456"
		newIcon := "iniicon"

		category, err := serviceTest.PatchCategory(ctx, "inisebuahid", &PatchCategoryRequest{
			Name:  &newName,
			Type:  &newType,
			Color: &newColor,
			Icon:  &newIcon,
		})

		if err != nil {
			t.Fatalf("error: %v", err)
		}

		if category.Name != newName {
			t.Errorf("name %s, mau %s", category.Name, newName)
		}

		if category.Type != newType {
			t.Errorf("Type %s, mau %s", category.Type, newType)
		}

	})

	t.Run("forward error", func(t *testing.T) {
		ctx := context.Background()
		fakeRepo := &FakeCategoryRepository{
			PatchCategoryFunc: func(ctx context.Context, id string, name, catType, color, icon *string) (Category, error) {
				return Category{}, errors.New("error DB")
			},
		}

		serviceTest := NewCategoryService(fakeRepo)

		newName := "makan"
		newType := "expense"
		newColor := "#123456"
		newIcon := "iniicon"

		category, err := serviceTest.PatchCategory(ctx, "inisebuahid", &PatchCategoryRequest{
			Name:  &newName,
			Type:  &newType,
			Color: &newColor,
			Icon:  &newIcon,
		})

		if err == nil {
			t.Error("harusnya error DB, tetapi tidak dapat")
		}

		if err.Error() != "error DB" {
			t.Errorf("error %q, mau %q", err.Error(), "error DB")
		}

		if category != (Category{}) {
			t.Error("harusnya tidak return data, tetapi malah return data")
		}
	})
}

func TestDeleteCategoryService(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctx := context.Background()
		fakeRepo := &FakeCategoryRepository{
			DeleteCategoryFunc: func(ctx context.Context, id string) (Category, error) {
				return Category{Name: "makan", DeletedAt: sql.NullTime{Time: time.Now(), Valid: true}}, nil
			},
		}

		serviceTest := NewCategoryService(fakeRepo)

		category, err := serviceTest.DeleteCategory(ctx, "inisebuahid")
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		if category.Name != "makan" {
			t.Errorf("name %s, mau %s", category.Name, "makan")
		}

		if !category.DeletedAt.Valid {
			t.Errorf("deleted_at null, mau tidak null")
		}
	})

	t.Run("forward error", func(t *testing.T) {
		ctx := context.Background()
		fakeRepo := &FakeCategoryRepository{
			DeleteCategoryFunc: func(ctx context.Context, id string) (Category, error) {
				return Category{}, errors.New("error DB")
			},
		}

		serviceTest := NewCategoryService(fakeRepo)

		_, err := serviceTest.DeleteCategory(ctx, "inisebuahid")
		if err == nil {
			t.Error("harusnya error DB, tetapi tidak dapat")
		}

		if err.Error() != "error DB" {
			t.Errorf("error %q, mau %q", err.Error(), "error DB")
		}
	})

}

func TestGetCategoryByIDService(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctx := context.Background()
		fakeRepo := &FakeCategoryRepository{
			GetCategoryByIDFunc: func(ctx context.Context, id string) (Category, error) {
				return Category{Name: "makan"}, nil
			},
		}
		serviceTest := NewCategoryService(fakeRepo)

		category, err := serviceTest.GetCategoryByID(ctx, "inisebuahid")
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		if category.Name != "makan" {
			t.Errorf("name %s, mau makan", category.Name)
		}
	})

	t.Run("forward error from db", func(t *testing.T) {
		ctx := context.Background()
		fakeRepo := &FakeCategoryRepository{
			GetCategoryByIDFunc: func(ctx context.Context, id string) (Category, error) {
				return Category{}, errors.New("DB error")
			},
		}
		serviceTest := NewCategoryService(fakeRepo)

		_, err := serviceTest.GetCategoryByID(ctx, "inisebuahid")

		if err == nil {
			t.Error("harusnya DB error, tetapi tidak dapat")
		}

		if err.Error() != "DB error" {
			t.Errorf("error %q, mau %q", err.Error(), "DB error")
		}
	})

}

func TestRestoreCategoryService(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctx := context.Background()
		fakeRepo := &FakeCategoryRepository{
			RestoreCategoryFunc: func(ctx context.Context, id string) (Category, error) {
				return Category{Name: "makan"}, nil
			},
		}
		serviceTest := NewCategoryService(fakeRepo)

		category, err := serviceTest.RestoreCategory(ctx, "inisebuahid")
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		if category.Name != "makan" {
			t.Errorf("name %s, mau makan", category.Name)
		}
	})

	t.Run("forward error from db", func(t *testing.T) {
		ctx := context.Background()
		fakeRepo := &FakeCategoryRepository{
			RestoreCategoryFunc: func(ctx context.Context, id string) (Category, error) {
				return Category{}, errors.New("DB error")
			},
		}
		serviceTest := NewCategoryService(fakeRepo)

		_, err := serviceTest.RestoreCategory(ctx, "inisebuahid")

		if err == nil {
			t.Error("harusnya DB error, tetapi tidak dapat")
		}

		if err.Error() != "DB error" {
			t.Errorf("error %q, mau %q", err.Error(), "DB error")
		}
	})

}
