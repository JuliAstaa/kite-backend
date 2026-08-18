package category

import "context"

type CategoryServicer interface {
	CreateCategory(ctx context.Context, requestBody *CreateCategoryRequest) (Category, error)
	GetAllCategories(ctx context.Context, limit int, offset int) ([]Category, int, error)
	PatchCategory(ctx context.Context, id string, requestBody *PatchCategoryRequest) (Category, error)
	DeleteCategory(ctx context.Context, id string) (Category, error)
	GetCategoryByID(ctx context.Context, id string) (Category, error)
	RestoreCategory(ctx context.Context, id string) (Category, error)
}

type CategoryService struct {
	repo CategoryRepositorer
}

func NewCategoryService(repo CategoryRepositorer) *CategoryService {
	return &CategoryService{repo: repo}
}

func (s *CategoryService) CreateCategory(ctx context.Context, requestBody *CreateCategoryRequest) (Category, error) {
	return s.repo.CreateCategory(ctx, requestBody.Name, requestBody.Type, requestBody.Color, requestBody.Icon)
}

func (s *CategoryService) GetAllCategories(ctx context.Context, limit int, offset int) ([]Category, int, error) {
	return s.repo.GetAllCategories(ctx, limit, offset)
}

func (s *CategoryService) PatchCategory(ctx context.Context, id string, requestBody *PatchCategoryRequest) (Category, error) {
	return s.repo.PatchCategory(ctx, id, requestBody.Name, requestBody.Type, requestBody.Color, requestBody.Icon)
}

func (s *CategoryService) DeleteCategory(ctx context.Context, id string) (Category, error) {
	return s.repo.DeleteCategory(ctx, id)
}

func (s *CategoryService) GetCategoryByID(ctx context.Context, id string) (Category, error) {
	return s.repo.GetCategoryByID(ctx, id)
}
func (s *CategoryService) RestoreCategory(ctx context.Context, id string) (Category, error) {
	return s.repo.RestoreCategory(ctx, id)
}
