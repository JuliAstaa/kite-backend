package wallet

import "context"

type WalletServicer interface {
	CreateWallet(ctx context.Context, reqBody *CreateWalletRequest) (Wallet, error)
	GetAllWallets(ctx context.Context, limit int, offset int) ([]Wallet, int, error)
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

func (s *WalletService) GetAllWallets(ctx context.Context, limit int, offet int) ([]Wallet, int, error) {
	return s.repo.GetAllWallets(ctx, limit, offet)
}
