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
	return s.repo.CreateWallet(ctx, reqBody.Name, reqBody.Type, reqBody.InitialBalance, reqBody.Color, reqBody.Icon, reqBody.IsExcludedFromTotal)
}
