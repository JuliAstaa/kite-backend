package category

import (
	"context"
	"errors"
	"testing"
)

type FakeCategoryRepository struct {
	CreateCategoryFunc func(ctx context.Context, name string, catType string, color string, icon string) (Category, error)
}

func (f *FakeCategoryRepository) CreateCategory(ctx context.Context, name string, catType string, color string, icon string) (Category, error) {
	return f.CreateCategoryFunc(ctx, name, catType, color, icon)
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
