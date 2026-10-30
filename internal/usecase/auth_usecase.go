package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"jaga-warung-go/internal/domain"
	"jaga-warung-go/pkg/auth"
)

// AuthUsecase implements domain.AuthUseCase. It wires pkg/auth (crypto) and
// repository (data) with no echo dependency. The delivery/http layer is the
// only place that parses request bodies and sets HTTP status codes.
type authUsecase struct {
	accountRepo domain.AccountRepository
	refreshRepo domain.RefreshTokenRepository
	jwtSecret   []byte
	accessTTL   time.Duration
	refreshTTL  time.Duration
}

func NewAuthUsecase(acc domain.AccountRepository, rt domain.RefreshTokenRepository, jwtSecret string, accessTTL, refreshTTL time.Duration) domain.AuthUseCase {
	return &authUsecase{
		accountRepo: acc,
		refreshRepo: rt,
		jwtSecret:   []byte(jwtSecret),
		accessTTL:   accessTTL,
		refreshTTL:  refreshTTL,
	}
}

func (uc *authUsecase) Login(ctx context.Context, phone, password string) (*domain.AuthTokens, error) {

	acc, err := uc.accountRepo.FindByPhone(ctx, phone)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, errors.New("invalid credentials")
		}
		return nil, err
	}
	if err := auth.CheckPassword(password, acc.PasswordHash); err != nil {
		return nil, errors.New("invalid credentials")
	}
	return issueTokens(ctx, uc, acc)
}

func (uc *authUsecase) Refresh(ctx context.Context, rawToken string) (*domain.AuthTokens, error) {
	hash := auth.HashRefresh(rawToken)
	rt, err := uc.refreshRepo.FindByHash(ctx, hash)
	if err != nil || rt == nil {
		return nil, errors.New("invalid refresh token")
	}
	if time.Now().After(rt.ExpiresAt) {
		return nil, errors.New("refresh token expired")
	}

	// Rotate: revoke the used token before issuing a new one.
	if err := uc.refreshRepo.Revoke(ctx, hash); err != nil {
		return nil, errors.New("invalid refresh token")
	}

	// Re-fetch account by ID for the new token pair.
	acc, err := uc.accountRepo.GetByID(ctx, rt.AccountID)
	if err != nil {
		return nil, errors.New("invalid refresh token")
	}
	return issueTokens(ctx, uc, acc)
}

func (uc *authUsecase) Logout(ctx context.Context, rawToken string) error {
	hash := auth.HashRefresh(rawToken)
	if err := uc.refreshRepo.Revoke(ctx, hash); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			// Idempotent: already revoked or unknown — still a success.
			return nil
		}
		return err
	}
	return nil
}

func issueTokens(ctx context.Context, uc *authUsecase, acc *domain.Account) (*domain.AuthTokens, error) {
	accessTTLSec := int64(uc.accessTTL.Seconds())
	access, err := auth.SignAccess(uc.jwtSecret, acc, accessTTLSec)
	if err != nil {
		return nil, err
	}
	raw, hash := auth.IssueRefresh()
	rt := domain.RefreshToken{
		ID:        uuid.New(),
		AccountID: acc.ID,
		TokenHash: hash,
		ExpiresAt: time.Now().Add(uc.refreshTTL),
		CreatedAt: time.Now(),
	}
	if err := uc.refreshRepo.Create(ctx, &rt); err != nil {
		return nil, err
	}
	return &domain.AuthTokens{
		AccessToken:  access,
		RefreshToken: raw,
		ExpiresIn:    accessTTLSec,
	}, nil
}
