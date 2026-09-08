package wishlist

import (
	"backend/internal/shared/apperror"
	"backend/internal/shared/timeutil"
	"context"
	"math"
	"strings"
)

// SavingsReader dideklarasikan di sini, di sisi yang memakai. Feature saving
// tidak pernah tahu bahwa wishlist memakainya, dan test wishlist cukup pakai
// struct palsu yang memenuhi interface ini.
type SavingsReader interface {
	AverageMonthlySavable(ctx context.Context, months int) (avg int, sampleMonths int, err error)
}

type WalletsReader interface {
	IsWalletExist(ctx context.Context, id string) error
}

type CategoryInfo struct {
	ID   string
	Type string
}

type CategoriesReader interface {
	GetCategoryInfo(ctx context.Context, id string) (CategoryInfo, error)
}

// affordabilityMonths adalah jumlah bulan lengkap yang dipakai untuk menghitung
// rata-rata kemampuan menabung.
const affordabilityMonths = 3

var allowedPriorities = []string{"low", "medium", "high"}
var allowedStatuses = []string{"planned", "saving", "purchased", "cancelled"}

type WishlistServicer interface {
	CreateItem(ctx context.Context, req CreateItemRequest) (ItemView, error)
	GetAllItems(ctx context.Context, filter ListFilter) ([]ItemView, int, error)
	GetItemByID(ctx context.Context, id string) (ItemView, error)
	PatchItem(ctx context.Context, id string, req PatchItemRequest) (ItemView, error)
	DeleteItem(ctx context.Context, id string) (Item, error)
	RestoreItem(ctx context.Context, id string) (Item, error)
	Allocate(ctx context.Context, id string, amount int) (ItemView, error)
	Purchase(ctx context.Context, id string, req PurchaseRequest) (ItemView, error)
	Summary(ctx context.Context) (SummaryTotals, error)
}

type WishlistService struct {
	repo     WishlistRepositorer
	savings  SavingsReader
	wallet   WalletsReader
	category CategoriesReader
}

func NewWishlistService(repo WishlistRepositorer, savings SavingsReader, wallet WalletsReader, category CategoriesReader) *WishlistService {
	return &WishlistService{repo: repo, savings: savings, wallet: wallet, category: category}
}

func (s *WishlistService) CreateItem(ctx context.Context, req CreateItemRequest) (ItemView, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return ItemView{}, apperror.ValidationError{Field: "name", Message: "name tidak boleh kosong"}
	}
	if req.EstimatedPrice <= 0 {
		return ItemView{}, apperror.ValidationError{Field: "estimated_price", Message: "estimated_price harus lebih besar dari 0"}
	}

	priority := strings.ToLower(strings.TrimSpace(req.Priority))
	if priority == "" {
		priority = "medium"
	}
	if !contains(allowedPriorities, priority) {
		return ItemView{}, apperror.ValidationError{Field: "priority", Message: "priority harus low, medium, atau high"}
	}

	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status == "" {
		status = "planned"
	}
	if !contains(allowedStatuses, status) {
		return ItemView{}, apperror.ValidationError{Field: "status", Message: "status tidak dikenal"}
	}

	item, err := s.repo.CreateItem(ctx, CreateItemParams{
		Name:           name,
		EstimatedPrice: req.EstimatedPrice,
		Priority:       priority,
		TargetDate:     req.TargetDate,
		ProductURL:     req.ProductURL,
		Note:           strings.TrimSpace(req.Note),
		Status:         status,
	})
	if err != nil {
		return ItemView{}, err
	}

	return s.withAffordability(ctx, item)
}

func (s *WishlistService) GetAllItems(ctx context.Context, filter ListFilter) ([]ItemView, int, error) {
	items, total, err := s.repo.GetAllItems(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	// Rata-rata kemampuan menabung cukup dihitung sekali untuk seluruh daftar.
	avg, sample, err := s.savings.AverageMonthlySavable(ctx, affordabilityMonths)
	if err != nil {
		return nil, 0, err
	}

	views := make([]ItemView, 0, len(items))
	for _, item := range items {
		views = append(views, ItemView{Item: item, Affordability: affordabilityFor(item, avg, sample)})
	}

	return views, total, nil
}

func (s *WishlistService) GetItemByID(ctx context.Context, id string) (ItemView, error) {
	item, err := s.repo.GetItemByID(ctx, id)
	if err != nil {
		return ItemView{}, err
	}
	return s.withAffordability(ctx, item)
}

func (s *WishlistService) PatchItem(ctx context.Context, id string, req PatchItemRequest) (ItemView, error) {
	existing, err := s.repo.GetItemByID(ctx, id)
	if err != nil {
		return ItemView{}, err
	}

	// Business rule 12: yang sudah dibeli tidak boleh diubah harganya.
	// Restore dulu ke planned kalau memang mau dikoreksi.
	if existing.Status == "purchased" && req.EstimatedPrice != nil {
		return ItemView{}, apperror.UnprocessableError{
			Message: "item yang sudah dibeli tidak bisa diubah harganya, ubah statusnya dulu ke planned",
		}
	}

	param := PatchItemParams{
		Note:            req.Note,
		SortOrder:       req.SortOrder,
		TargetDate:      req.TargetDate,
		ClearTargetDate: req.ClearTargetDate,
		ProductURL:      req.ProductURL,
		ClearProductURL: req.ClearProductURL,
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return ItemView{}, apperror.ValidationError{Field: "name", Message: "name tidak boleh kosong"}
		}
		param.Name = &name
	}

	if req.EstimatedPrice != nil {
		if *req.EstimatedPrice <= 0 {
			return ItemView{}, apperror.ValidationError{Field: "estimated_price", Message: "estimated_price harus lebih besar dari 0"}
		}
		// Business rule 11 dilihat dari sisi sebaliknya: harga baru tidak boleh
		// lebih kecil dari uang yang sudah terlanjur dialokasikan.
		if *req.EstimatedPrice < existing.SavedAmount {
			return ItemView{}, apperror.ValidationError{
				Field:   "estimated_price",
				Message: "estimated_price tidak boleh lebih kecil dari saved_amount yang sudah terkumpul",
			}
		}
		param.EstimatedPrice = req.EstimatedPrice
	}

	if req.Priority != nil {
		priority := strings.ToLower(strings.TrimSpace(*req.Priority))
		if !contains(allowedPriorities, priority) {
			return ItemView{}, apperror.ValidationError{Field: "priority", Message: "priority harus low, medium, atau high"}
		}
		param.Priority = &priority
	}

	if req.Status != nil {
		status := strings.ToLower(strings.TrimSpace(*req.Status))
		if !contains(allowedStatuses, status) {
			return ItemView{}, apperror.ValidationError{Field: "status", Message: "status tidak dikenal"}
		}
		// Menandai purchased tanpa transaksi tidak diizinkan, harus lewat
		// endpoint /purchase supaya transaksinya ikut tercatat.
		if status == "purchased" && existing.Status != "purchased" {
			return ItemView{}, apperror.UnprocessableError{
				Message: "untuk menandai sudah dibeli, pakai endpoint /wishlist/{id}/purchase",
			}
		}
		param.Status = &status
	}

	item, err := s.repo.PatchItem(ctx, id, param)
	if err != nil {
		return ItemView{}, err
	}

	return s.withAffordability(ctx, item)
}

func (s *WishlistService) DeleteItem(ctx context.Context, id string) (Item, error) {
	return s.repo.DeleteItem(ctx, id)
}

func (s *WishlistService) RestoreItem(ctx context.Context, id string) (Item, error) {
	return s.repo.RestoreItem(ctx, id)
}

// Allocate menambah uang yang disisihkan untuk item ini.
func (s *WishlistService) Allocate(ctx context.Context, id string, amount int) (ItemView, error) {
	if amount <= 0 {
		return ItemView{}, apperror.ValidationError{Field: "amount", Message: "amount harus lebih besar dari 0"}
	}

	existing, err := s.repo.GetItemByID(ctx, id)
	if err != nil {
		return ItemView{}, err
	}

	// Business rule 12
	if existing.Status == "purchased" {
		return ItemView{}, apperror.UnprocessableError{
			Message: "item yang sudah dibeli tidak bisa dialokasi tambahan",
		}
	}
	if existing.Status == "cancelled" {
		return ItemView{}, apperror.UnprocessableError{
			Message: "item yang sudah dibatalkan tidak bisa dialokasi",
		}
	}

	item, err := s.repo.Allocate(ctx, id, amount)
	if err != nil {
		return ItemView{}, err
	}

	return s.withAffordability(ctx, item)
}

// Purchase mencatat pembelian item sekaligus membuat transaksi expense-nya
// dalam satu database transaction.
func (s *WishlistService) Purchase(ctx context.Context, id string, req PurchaseRequest) (ItemView, error) {
	existing, err := s.repo.GetItemByID(ctx, id)
	if err != nil {
		return ItemView{}, err
	}

	if existing.Status == "purchased" {
		return ItemView{}, apperror.UnprocessableError{Message: "item ini sudah ditandai dibeli"}
	}

	if req.ActualPrice <= 0 {
		return ItemView{}, apperror.ValidationError{Field: "actual_price", Message: "actual_price harus lebih besar dari 0"}
	}

	// Wallet dan kategori dicek lewat service masing-masing, jadi yang sudah
	// di-soft-delete otomatis ditolak.
	if err := s.wallet.IsWalletExist(ctx, req.WalletID); err != nil {
		return ItemView{}, err
	}

	cat, err := s.category.GetCategoryInfo(ctx, req.CategoryID)
	if err != nil {
		return ItemView{}, err
	}
	if cat.Type != "expense" {
		return ItemView{}, apperror.ValidationError{
			Field:   "category_id",
			Message: "pembelian wishlist harus memakai kategori bertipe expense",
		}
	}

	occurredAt := req.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = timeutil.Now()
	}
	if occurredAt.After(timeutil.EndOfDay(timeutil.Now().AddDate(0, 0, 1))) {
		return ItemView{}, apperror.ValidationError{Field: "occurred_at", Message: "occurred_at tidak boleh lebih dari 1 hari di masa depan"}
	}

	note := strings.TrimSpace(req.Note)
	if note == "" {
		note = "Pembelian wishlist: " + existing.Name
	}

	item, err := s.repo.Purchase(ctx, id, PurchaseParams{
		WalletID:    req.WalletID,
		CategoryID:  req.CategoryID,
		ActualPrice: req.ActualPrice,
		OccurredAt:  occurredAt,
		Note:        note,
	})
	if err != nil {
		return ItemView{}, err
	}

	return s.withAffordability(ctx, item)
}

func (s *WishlistService) Summary(ctx context.Context) (SummaryTotals, error) {
	return s.repo.Summary(ctx)
}

func (s *WishlistService) withAffordability(ctx context.Context, item Item) (ItemView, error) {
	avg, sample, err := s.savings.AverageMonthlySavable(ctx, affordabilityMonths)
	if err != nil {
		return ItemView{}, err
	}
	return ItemView{Item: item, Affordability: affordabilityFor(item, avg, sample)}, nil
}

// affordabilityFor menghitung perkiraan kapan item ini terbeli.
//
// Kalau data historisnya belum ada satu bulan penuh, hasilnya nil. Lebih baik
// tidak menampilkan apa-apa daripada menampilkan tebakan.
func affordabilityFor(item Item, avgMonthlySavable int, sampleMonths int) *Affordability {
	if sampleMonths < 1 {
		return nil
	}

	result := &Affordability{AvgMonthlySavable: avgMonthlySavable}

	remaining := item.Remaining()
	if remaining == 0 {
		months := 0
		readyDate := timeutil.StartOfDay(timeutil.Now())
		result.MonthsNeeded = &months
		result.EstimatedReadyDate = &readyDate
		result.OnTrackForTargetDay = !item.TargetDate.Valid || !readyDate.After(item.TargetDate.Time)
		return result
	}

	// Kemampuan menabung nol atau minus berarti tidak ada perkiraan yang jujur
	// bisa diberikan.
	if avgMonthlySavable <= 0 {
		result.OnTrackForTargetDay = false
		return result
	}

	months := int(math.Ceil(float64(remaining) / float64(avgMonthlySavable)))
	readyDate := timeutil.StartOfDay(timeutil.Now()).AddDate(0, months, 0)

	result.MonthsNeeded = &months
	result.EstimatedReadyDate = &readyDate
	result.OnTrackForTargetDay = !item.TargetDate.Valid || !readyDate.After(item.TargetDate.Time)

	return result
}

func contains(list []string, value string) bool {
	for _, v := range list {
		if v == value {
			return true
		}
	}
	return false
}
