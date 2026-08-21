package wallet

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

var walletRepoTest *WalletRepository

func TestMain(m *testing.M) {
	db := testutil.ConnectTestDB("../../../.env")
	walletRepoTest = NewWalletRepository(db)
	code := m.Run()
	os.Exit(code)
}

func resetTable(t *testing.T) {
	if _, err := walletRepoTest.db.Exec(`TRUNCATE wallets CASCADE`); err != nil {
		t.Error(err)
	}
}

func TestCreateWalletRepo(t *testing.T) {
	tests := []struct {
		name                string
		walletName          string
		walletType          string
		initialBalance      int
		color               string
		icon                string
		isExcludedFromTotal bool
		wantError           bool
	}{
		{"success", "BCA", "ewallet", 50000, "#ffffff", "sebuahicon", false, false},
		{"error constraint", "BCA", "apanyak", 50000, "#ffffff", "sebuahicon", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetTable(t)
			ctx := context.Background()

			wallet, err := walletRepoTest.CreateWallet(ctx, tt.walletName, tt.walletType, tt.initialBalance, tt.color, tt.icon, tt.isExcludedFromTotal)
			if tt.wantError {
				if err == nil {
					t.Errorf("tidak ada error, mau error constraint")
				}
				return
			}

			if err != nil {
				t.Fatalf("error: %v", err)
			}
			if wallet.ID == "" {
				t.Error("ID kosong, harusnya ke generate")
			}

			if wallet.Name != tt.walletName {
				t.Errorf("nama %s, mau %s", wallet.Name, tt.walletName)
			}

			if wallet.Type != tt.walletType {
				t.Errorf("type %s, mau %s", wallet.Type, tt.walletType)
			}

			if wallet.InitialBalance != tt.initialBalance {
				t.Errorf("InitialBalance %d, mau %d", wallet.InitialBalance, tt.initialBalance)
			}

			if wallet.Icon != tt.icon {
				t.Errorf("icon %s, mau %s", wallet.Icon, tt.icon)
			}

			if wallet.Color != tt.color {
				t.Errorf("color %s, mau %s", wallet.Color, tt.color)
			}

			if wallet.IsExcludedFromTotal != false {
				t.Errorf("is_excluded_from_total %v, mau false", wallet.IsExcludedFromTotal)
			}

			if wallet.SortOrder != 0 {
				t.Error("sort_order bukan 0, harusnya secara default 0")
			}

			if wallet.CreatedAt.IsZero() {
				t.Error("created_at kosong, harusnya keisi now()")
			}

			if wallet.UpdatedAt.IsZero() {
				t.Error("updated_at kosong, harusnya keisi now()")
			}

			if wallet.DeletedAt.Valid {
				t.Error("deleted_at tidak null, harusnya kosong")
			}

		})
	}

	t.Run("wallet already exist", func(t *testing.T) {
		resetTable(t)
		ctx := context.Background()

		walletRepoTest.CreateWallet(ctx, "BCA", "bank", 50000, "#fffffff", "iniicon", false)

		_, err := walletRepoTest.CreateWallet(ctx, "BCA", "bank", 50000, "#fffffff", "iniicon", false)
		var alreadyExistErr apperror.AlreadyExistsErr
		if !errors.As(err, &alreadyExistErr) {
			t.Error("tidak ada error, mau error already exist")
		}

	})
}

func TestGetAllWallets(t *testing.T) {
	t.Run("pagination motong dengan benar", func(t *testing.T) {
		resetTable(t)
		ctx := context.Background()
		for i := 0; i < 5; i++ {
			walletRepoTest.CreateWallet(ctx, fmt.Sprintf("wallet ke-%d", i), "bank", 40000, "#ffffff", "inicon", false)
		}

		wallets, total, err := walletRepoTest.GetAllWallets(ctx, 3, 0)

		if err != nil {
			t.Fatalf("error: %v", err)
		}

		if len(wallets) != 3 {
			t.Errorf("jumlah wallet %d, mau 3", len(wallets))
		}

		if total != 5 {
			t.Errorf("total wallet %d, mau 5", total)
		}
	})

	t.Run("soft-deleted tidak muncul", func(t *testing.T) {
		resetTable(t)
		ctx := context.Background()

		for i := 0; i < 5; i++ {
			walletRepoTest.CreateWallet(ctx, fmt.Sprintf("wallet ke-%d", i), "bank", 40000, "#ffffff", "inicon", false)
		}

		wallets, _, err := walletRepoTest.GetAllWallets(ctx, 10, 0)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		_, err = walletRepoTest.DeleteWallet(ctx, wallets[0].ID)
		if err != nil {
			t.Errorf("error: %v", err)
		}

		wallets, total, err := walletRepoTest.GetAllWallets(ctx, 10, 0)
		if err != nil {
			t.Errorf("error: %v", err)
		}

		if total != 4 {
			t.Errorf("total %d, mau 4", total)
		}

		if len(wallets) != 4 {
			t.Errorf("len wallet %d, mau 4", len(wallets))
		}

	})

	t.Run("kosong", func(t *testing.T) {
		resetTable(t)
		ctx := context.Background()
		wallets, total, err := walletRepoTest.GetAllWallets(ctx, 10, 0)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		if total != 0 {
			t.Errorf("total %d, mau 0", total)
		}

		if len(wallets) != 0 {
			t.Errorf("len wallet %d, mau 0", len(wallets))
		}
	})

}

func TestPatchWallet(t *testing.T) {
	tests := []struct {
		testName            string
		walletName          string
		walletType          string
		initialBalancae     int
		color               string
		icon                string
		isExcludedFromTotal bool
	}{
		{"patches name", "wallet baru", "bank", 50000, "#ffffff", "iniicon", false},
		{"patches type", "wallet", "ewallet", 50000, "#ffffff", "iniicon", false},
		{"patches initial balance", "wallet", "bank", 40000, "#ffffff", "iniicon", false},
		{"patches color", "wallet", "bank", 50000, "#ff23ff", "iniicon", false},
		{"patches icon", "wallet", "bank", 50000, "#ffffff", "iconbaru", false},
		{"patches is excluded from total", "wallet", "bank", 50000, "#ffffff", "iniicon", true},
		{"patches all field", "wallet baru", "cash", 100000, "#12ffff", "iconbarucihuy", true},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			resetTable(t)
			ctx := context.Background()
			walletRepoTest.CreateWallet(ctx, "wallet", "bank", 50000, "#ffffff", "iniicon", false)
			wallets, _, _ := walletRepoTest.GetAllWallets(ctx, 10, 0)
			timeBeforeUpdated := wallets[0].UpdatedAt

			updatedWallet, err := walletRepoTest.PatchWallet(ctx, wallets[0].ID, &tt.walletName, &tt.walletType, &tt.initialBalancae, &tt.color, &tt.icon, &tt.isExcludedFromTotal)
			if err != nil {
				t.Fatalf("error: %v", err)
			}

			if updatedWallet.Name != tt.walletName {
				t.Errorf("name %s, mau %s", updatedWallet.Name, tt.walletName)
			}

			if updatedWallet.Type != tt.walletType {
				t.Errorf("type %s, mau %s", updatedWallet.Type, tt.walletType)
			}

			if updatedWallet.InitialBalance != tt.initialBalancae {
				t.Errorf("InitialBalance %d, mau %d", updatedWallet.InitialBalance, tt.initialBalancae)
			}

			if updatedWallet.Color != tt.color {
				t.Errorf("color %s, mau %s", updatedWallet.Color, tt.color)
			}

			if updatedWallet.Icon != tt.icon {
				t.Errorf("icon %s, mau %s", updatedWallet.Icon, tt.icon)
			}

			if updatedWallet.IsExcludedFromTotal != tt.isExcludedFromTotal {
				t.Errorf("IsExcludedFromTotal %v, mau %v", updatedWallet.IsExcludedFromTotal, tt.isExcludedFromTotal)
			}

			if time.Time.Equal(timeBeforeUpdated, updatedWallet.UpdatedAt) {
				t.Error("updated_at tidak berubah, harusnya berubah")
			}
		})
	}

	t.Run("patch - already exist", func(t *testing.T) {
		resetTable(t)
		ctx := context.Background()
		walletRepoTest.CreateWallet(ctx, "wallet", "bank", 50000, "#ffffff", "iniicon", false)
		walletRepoTest.CreateWallet(ctx, "cash", "cash", 50000, "#ffffff", "iniicon", false)

		wallets, _, _ := walletRepoTest.GetAllWallets(ctx, 10, 0)

		newName := "cash"
		newType := "cash"

		_, err := walletRepoTest.PatchWallet(ctx, wallets[0].ID, &newName, &newType, nil, nil, nil, nil)
		var alreadyExistErr apperror.AlreadyExistsErr
		if !errors.As(err, &alreadyExistErr) {
			t.Errorf("mau error AlreadyExistsErr, dapat %v", err)
		}
	})

	t.Run("path - not found", func(t *testing.T) {
		resetTable(t)
		ctx := context.Background()
		_, err := walletRepoTest.PatchWallet(ctx, "e9c9017d-3287-4711-8535-9e8faa7143da", nil, nil, nil, nil, nil, nil)

		var nf apperror.NotFoundError
		if !errors.As(err, &nf) {
			t.Errorf("mau error NotFoundErr, dapat %v", err)
		}
	})
}

func TestDeleteWallet(t *testing.T) {
	t.Run("delete - success", func(t *testing.T) {
		resetTable(t)
		ctx := context.Background()
		walletRepoTest.CreateWallet(ctx, "wallet", "bank", 50000, "#ffffff", "iniicon", false)
		wallets, _, _ := walletRepoTest.GetAllWallets(ctx, 10, 0)

		wallet, err := walletRepoTest.DeleteWallet(ctx, wallets[0].ID)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		wallets, total, _ := walletRepoTest.GetAllWallets(ctx, 10, 0)
		if !wallet.DeletedAt.Valid {
			t.Errorf("deleted_at masih null, mau now()")
		}

		if total != 0 {
			t.Error("total tidak 0, mau 0")
		}

		if len(wallets) != 0 {
			t.Error("len wallet tidak 0, mau 0")
		}
	})
	t.Run("delete - not found", func(t *testing.T) {
		resetTable(t)
		ctx := context.Background()
		_, err := walletRepoTest.DeleteWallet(ctx, "e9c9017d-3287-4711-8535-9e8faa7143da")

		var nf apperror.NotFoundError
		if !errors.As(err, &nf) {
			t.Errorf("mau error NotFoundErr, dapat %v", err)
		}
	})
}

func TestGetWalletByIDRepository(t *testing.T) {
	t.Run("get wallet by id - success", func(t *testing.T) {
		resetTable(t)
		ctx := context.Background()
		walletRepoTest.CreateWallet(ctx, "wallet", "bank", 50000, "#ffffff", "iniicon", false)
		wallets, _, _ := walletRepoTest.GetAllWallets(ctx, 10, 0)

		wallet, err := walletRepoTest.GetWalletByID(ctx, wallets[0].ID)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		if wallet.ID != wallets[0].ID {
			t.Errorf("ID %s, mau %s", wallet.ID, wallets[0].ID)
		}

		if wallet.Name != wallets[0].Name {
			t.Errorf("Name %s, mau %s", wallet.Name, wallets[0].Name)

		}
	})

	t.Run("wallet get by id - not found", func(t *testing.T) {
		resetTable(t)
		ctx := context.Background()
		_, err := walletRepoTest.GetWalletByID(ctx, "e9c9017d-3287-4711-8535-9e8faa7143da")

		var nf apperror.NotFoundError
		if !errors.As(err, &nf) {
			t.Errorf("mau error NotFoundErr, dapat %v", err)
		}
	})
}

func TestRestoreWalletRepository(t *testing.T) {
	t.Run("restore wallet - success", func(t *testing.T) {
		resetTable(t)
		ctx := context.Background()
		walletRepoTest.CreateWallet(ctx, "wallet", "bank", 50000, "#ffffff", "iniicon", false)
		wallets, _, _ := walletRepoTest.GetAllWallets(ctx, 10, 0)

		deletedWallet, err := walletRepoTest.DeleteWallet(ctx, wallets[0].ID)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		if !deletedWallet.DeletedAt.Valid {
			t.Error("deleted_at masih null, mau now()")
		}

		wallet, err := walletRepoTest.RestoreWallet(ctx, deletedWallet.ID)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		if wallet.DeletedAt.Valid {
			t.Error("deleted_at not null, mau null")
		}

	})

	t.Run("wallet get by id - not found", func(t *testing.T) {
		resetTable(t)
		ctx := context.Background()
		_, err := walletRepoTest.RestoreWallet(ctx, "e9c9017d-3287-4711-8535-9e8faa7143da")

		var nf apperror.NotFoundError
		if !errors.As(err, &nf) {
			t.Errorf("mau error NotFoundErr, dapat %v", err)
		}
	})
}
