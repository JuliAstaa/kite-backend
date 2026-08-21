package wallet

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"
)

type FakeWalletRepository struct {
	CreateWalletFunc  func(ctx context.Context, name string, walletType string, initialBalancae int, color string, icon string, IsExcludedFromTotal bool) (Wallet, error)
	GetAllWalletsFunc func(ctx context.Context, limit int, offset int) ([]Wallet, int, error)
	PatchWalletFunc   func(ctx context.Context, id string, name *string, walletType *string, initialBalancae *int, color *string, icon *string, IsExcludedFromTotal *bool) (Wallet, error)
	GetWalletByIDFunc func(ctx context.Context, id string) (Wallet, error)
	DeleteWalletFunc  func(ctx context.Context, id string) (Wallet, error)
	RestoreWalletFunc func(ctx context.Context, id string) (Wallet, error)
}

func (f *FakeWalletRepository) CreateWallet(ctx context.Context, name string, walletType string, initialBalancae int, color string, icon string, IsExcludedFromTotal bool) (Wallet, error) {
	return f.CreateWalletFunc(ctx, name, walletType, initialBalancae, color, icon, IsExcludedFromTotal)
}

func (f *FakeWalletRepository) GetAllWallets(ctx context.Context, limit int, offset int) ([]Wallet, int, error) {
	return f.GetAllWalletsFunc(ctx, limit, offset)
}

func (f *FakeWalletRepository) PatchWallet(ctx context.Context, id string, name *string, walletType *string, initialBalancae *int, color *string, icon *string, IsExcludedFromTotal *bool) (Wallet, error) {
	return f.PatchWalletFunc(ctx, id, name, walletType, initialBalancae, color, icon, IsExcludedFromTotal)
}

func (f *FakeWalletRepository) GetWalletByID(ctx context.Context, id string) (Wallet, error) {
	return f.GetWalletByIDFunc(ctx, id)
}

func (f *FakeWalletRepository) DeleteWallet(ctx context.Context, id string) (Wallet, error) {
	return f.DeleteWalletFunc(ctx, id)
}

func (f *FakeWalletRepository) RestoreWallet(ctx context.Context, id string) (Wallet, error) {
	return f.RestoreWalletFunc(ctx, id)
}

func TestCreateWalletService(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctx := context.Background()
		fakeRepo := &FakeWalletRepository{
			CreateWalletFunc: func(ctx context.Context, name, walletType string, initialBalancae int, color, icon string, IsExcludedFromTotal bool) (Wallet, error) {
				return Wallet{Name: name, Type: walletType, InitialBalance: initialBalancae, Color: color, Icon: icon, IsExcludedFromTotal: IsExcludedFromTotal}, nil
			},
		}

		service := NewWalletService(fakeRepo)

		wallet, err := service.CreateWallet(ctx, &CreateWalletRequest{
			Name:                "wallet",
			Type:                "bank",
			InitialBalance:      50000,
			Color:               "#ffffff",
			Icon:                "iniicon",
			IsExcludedFromTotal: false,
		})

		if err != nil {
			t.Fatalf("tidak ekspektasi dapat error, tapi dapat %v", err)
		}

		if wallet.Name != "wallet" {
			t.Errorf("name %s, mau wallet", wallet.Name)
		}

		if wallet.Type != "bank" {
			t.Errorf("type %s, mau bank", wallet.Type)
		}

		if wallet.InitialBalance != 50000 {
			t.Errorf("initial balance %d, mau 50000", wallet.InitialBalance)
		}

		if wallet.Color != "#ffffff" {
			t.Errorf("color %s, mau #ffffff", wallet.Color)
		}

		if wallet.Icon != "iniicon" {
			t.Errorf("icon %s, mau iniicon", wallet.Icon)
		}

		if wallet.IsExcludedFromTotal != false {
			t.Errorf("is excluded from total %v, mau false", wallet.IsExcludedFromTotal)
		}
	})

	t.Run("service create - forward error from db", func(t *testing.T) {
		ctx := context.Background()
		fakeRepo := &FakeWalletRepository{
			CreateWalletFunc: func(ctx context.Context, name, walletType string, initialBalancae int, color, icon string, IsExcludedFromTotal bool) (Wallet, error) {
				return Wallet{}, errors.New("error DB")
			},
		}

		service := NewWalletService(fakeRepo)

		wallet, err := service.CreateWallet(ctx, &CreateWalletRequest{
			Name:                "wallet",
			Type:                "bank",
			InitialBalance:      50000,
			Color:               "#ffffff",
			Icon:                "iniicon",
			IsExcludedFromTotal: false,
		})

		if err == nil {
			t.Fatal("harusnya ada error DB, tapi tidak dapat")
		}

		if err.Error() != "error DB" {
			t.Errorf("error %q, mau %q", err.Error(), "error DB")
		}

		if wallet != (Wallet{}) {
			t.Error("harusnya repo tidak return data, tapi malah return data")
		}
	})
}

func TestGetAllWalletsService(t *testing.T) {
	t.Run("get all wallet - success", func(t *testing.T) {
		ctx := context.Background()
		fakeRepo := &FakeWalletRepository{
			GetAllWalletsFunc: func(ctx context.Context, limit, offset int) ([]Wallet, int, error) {
				return []Wallet{{Name: "wallet"}}, 1, nil
			},
		}

		servieTest := NewWalletService(fakeRepo)

		wallets, total, err := servieTest.GetAllWallets(ctx, 10, 0)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		if total != 1 {
			t.Errorf("total %d, mau 1", total)
		}

		if len(wallets) != 1 {
			t.Errorf("len wallets %d, mau 1", len(wallets))
		}

	})

	t.Run("get all wallet - forward error from repo", func(t *testing.T) {
		ctx := context.Background()
		fakeRepo := &FakeWalletRepository{
			GetAllWalletsFunc: func(ctx context.Context, limit, offset int) ([]Wallet, int, error) {
				return nil, 0, errors.New("error DB")
			},
		}

		servieTest := NewWalletService(fakeRepo)

		_, _, err := servieTest.GetAllWallets(ctx, 10, 0)

		if err == nil {
			t.Error("harusnya error DB, tetapi tidak dapat")
		}

		if err.Error() != "error DB" {
			t.Errorf("error %q, mau %q", err.Error(), "error DB")
		}
	})
}

func TestPatchWalletService(t *testing.T) {
	t.Run("patch wallet - success", func(t *testing.T) {
		ctx := context.Background()
		fakeRepo := &FakeWalletRepository{
			PatchWalletFunc: func(ctx context.Context, id string, name, walletType *string, initialBalancae *int, color, icon *string, IsExcludedFromTotal *bool) (Wallet, error) {
				return Wallet{Name: "wallet", Type: "bank"}, nil
			},
		}

		serviceTest := NewWalletService(fakeRepo)

		newName := "wallet"
		newType := "bank"

		wallet, err := serviceTest.PatchWallet(ctx, "inisebuahid", &PatchWalletRequest{
			Name: &newName,
			Type: &newType,
		})

		if err != nil {
			t.Fatalf("error: %v", err)
		}

		if wallet.Name != newName {
			t.Errorf("name %s, mau %s", wallet.Name, newName)
		}

		if wallet.Type != newType {
			t.Errorf("Type %s, mau %s", wallet.Type, newType)
		}
	})

	t.Run("patch wallet - forward error from repo", func(t *testing.T) {
		ctx := context.Background()
		fakeRepo := &FakeWalletRepository{
			PatchWalletFunc: func(ctx context.Context, id string, name, walletType *string, initialBalancae *int, color, icon *string, IsExcludedFromTotal *bool) (Wallet, error) {
				return Wallet{}, errors.New("error DB")
			},
		}

		serviceTest := NewWalletService(fakeRepo)

		newName := "wallet"
		newType := "bank"

		_, err := serviceTest.PatchWallet(ctx, "inisebuahid", &PatchWalletRequest{
			Name: &newName,
			Type: &newType,
		})

		if err == nil {
			t.Error("harusnya error DB, tetapi tidak dapat")
		}

		if err.Error() != "error DB" {
			t.Errorf("error %q, mau %q", err.Error(), "error DB")
		}
	})
}

func TestDeleteWalletService(t *testing.T) {
	t.Run("delete wallet - success", func(t *testing.T) {
		ctx := context.Background()
		fakeRepo := &FakeWalletRepository{
			DeleteWalletFunc: func(ctx context.Context, id string) (Wallet, error) {
				return Wallet{Name: "wallet", DeletedAt: sql.NullTime{Time: time.Now(), Valid: true}}, nil
			},
		}

		serviceTest := NewWalletService(fakeRepo)

		wallet, err := serviceTest.DeleteWallet(ctx, "iniidkatanya")
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		if wallet.Name != "wallet" {
			t.Errorf("name %s, mau wallet", wallet.Name)
		}
		if !wallet.DeletedAt.Valid {
			t.Errorf("deleted_at null, mau now()")
		}
	})

	t.Run("delete wallet - forward error from repo", func(t *testing.T) {
		ctx := context.Background()
		fakeRepo := &FakeWalletRepository{
			DeleteWalletFunc: func(ctx context.Context, id string) (Wallet, error) {
				return Wallet{}, errors.New("error DB")
			},
		}

		serviceTest := NewWalletService(fakeRepo)

		_, err := serviceTest.DeleteWallet(ctx, "iniidkatanya")

		if err == nil {
			t.Error("harusnya error DB, tetapi tidak dapat")
		}

		if err.Error() != "error DB" {
			t.Errorf("error %q, mau %q", err.Error(), "error DB")
		}
	})
}

func TestRestoreWalletService(t *testing.T) {
	t.Run("restore wallet - success", func(t *testing.T) {
		ctx := context.Background()
		fakeRepo := &FakeWalletRepository{
			RestoreWalletFunc: func(ctx context.Context, id string) (Wallet, error) {
				return Wallet{Name: "wallet"}, nil
			},
		}

		serviceTest := NewWalletService(fakeRepo)

		wallet, err := serviceTest.RestoreWallet(ctx, "iniidkatanya")
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		if wallet.Name != "wallet" {
			t.Errorf("name %s, mau wallet", wallet.Name)
		}
	})

	t.Run("restore wallet - forward error from db", func(t *testing.T) {
		ctx := context.Background()
		fakeRepo := &FakeWalletRepository{
			RestoreWalletFunc: func(ctx context.Context, id string) (Wallet, error) {
				return Wallet{}, errors.New("error DB")
			},
		}

		serviceTest := NewWalletService(fakeRepo)

		_, err := serviceTest.RestoreWallet(ctx, "iniidkatanya")
		if err == nil {
			t.Error("harusnya DB error, tetapi tidak dapat")
		}

		if err.Error() != "error DB" {
			t.Errorf("error %q, mau %q", err.Error(), "error DB")
		}
	})

}
