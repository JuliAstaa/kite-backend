package wallet

import "context"

type WalletServicer interface {
	CreateWallet(ctx context.Context, reqBody *CreateWalletRequest) (Wallet, error)
}

type WalletService struct {
	repo WalletRepositorer
}

func NewWalletService(repo WalletRepositorer) *WalletService {
	return &WalletService{repo: repo}
}

func (s *WalletService) CreateWallet(ctx context.Context, reqBody *CreateWalletRequest) (Wallet, error) {
	if reqBody.InitialBalance == nil {
		initialBalance := 0
		reqBody.InitialBalance = &initialBalance
	}

	if reqBody.IsExcludedFromTotal == nil {
		isExcludedFromTotal := false
		reqBody.IsExcludedFromTotal = &isExcludedFromTotal
	}
	return s.repo.CreateWallet(ctx, reqBody.Name, reqBody.Type, reqBody.InitialBalance, reqBody.Color, reqBody.Icon, reqBody.IsExcludedFromTotal)
}
