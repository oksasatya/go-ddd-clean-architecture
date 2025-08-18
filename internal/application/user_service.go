package application

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"

	"github.com/oksasatya/go-ddd-clean-architecture/internal/domain/entity"
	repo "github.com/oksasatya/go-ddd-clean-architecture/internal/domain/repository"
	"github.com/oksasatya/go-ddd-clean-architecture/pkg/helpers"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserNotFound       = errors.New("user not found")
)

type Service struct {
	Repo      repo.UserRepository
	JWT       *helpers.JWTManager
	GCS       *storage.Client
	GCSBucket string
	Redis     *redis.Client
	Logger    *logrus.Logger
}

type TokenPair struct {
	AccessToken        string
	AccessTokenExpiry  time.Time
	RefreshToken       string
	RefreshTokenExpiry time.Time
}

func sessionKey(userID string) string {
	return "user:session:" + userID
}

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func NewService(repo repo.UserRepository, jwt *helpers.JWTManager, gcs *storage.Client, gcsBucket string, rdb *redis.Client, logger *logrus.Logger) *Service {
	return &Service{Repo: repo, JWT: jwt, GCS: gcs, GCSBucket: gcsBucket, Redis: rdb, Logger: logger}
}

type LoginResponse struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
}

func (s *Service) Login(ctx context.Context, email, password string) (*LoginResponse, TokenPair, error) {
	u, err := s.Repo.GetByEmail(email)
	if err != nil || u == nil {
		return nil, TokenPair{}, ErrInvalidCredentials
	}
	if !helpers.CompareHashAndPassword(u.Password, password) {
		return nil, TokenPair{}, ErrInvalidCredentials
	}

	access, aexp, err := s.JWT.GenerateAccessToken(u.ID)
	if err != nil {
		if s.Logger != nil {
			s.Logger.WithError(err).WithField("user_id", u.ID).Error("generate access token failed")
		}
		return nil, TokenPair{}, err
	}
	refresh, rexp, err := s.JWT.GenerateRefreshToken(u.ID)
	if err != nil {
		if s.Logger != nil {
			s.Logger.WithError(err).WithField("user_id", u.ID).Error("generate refresh token failed")
		}
		return nil, TokenPair{}, err
	}

	if s.Redis != nil {
		fields := map[string]any{
			"user_id":    u.ID,
			"email":      u.Email,
			"name":       u.Name,
			"avatar_url": u.AvatarURL,
			"logged_in":  true,
			"created_at": nowRFC3339(),
		}
		key := sessionKey(u.ID)
		pipe := s.Redis.Pipeline()
		pipe.HSet(ctx, key, fields)
		pipe.Expire(ctx, key, 24*time.Hour)
		if _, rErr := pipe.Exec(ctx); rErr != nil && s.Logger != nil {
			s.Logger.WithError(rErr).WithField("key", key).Warn("redis pipeline failed")
		}
	}

	resp := &LoginResponse{UserID: u.ID, Email: u.Email, Name: u.Name}
	return resp, TokenPair{
		AccessToken:        access,
		AccessTokenExpiry:  aexp,
		RefreshToken:       refresh,
		RefreshTokenExpiry: rexp,
	}, nil
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (TokenPair, string, error) {
	claims, err := s.JWT.ParseRefreshToken(refreshToken)
	if err != nil {
		return TokenPair{}, "", ErrInvalidCredentials
	}
	u, err := s.Repo.GetByID(claims.UserID)
	if err != nil || u == nil {
		return TokenPair{}, "", ErrInvalidCredentials
	}
	access, aexp, err := s.JWT.GenerateAccessToken(u.ID)
	if err != nil {
		return TokenPair{}, "", err
	}
	refresh, rexp, err := s.JWT.GenerateRefreshToken(u.ID)
	if err != nil {
		return TokenPair{}, "", err
	}
	return TokenPair{
		AccessToken:        access,
		AccessTokenExpiry:  aexp,
		RefreshToken:       refresh,
		RefreshTokenExpiry: rexp,
	}, u.ID, nil
}

func (s *Service) GetProfile(userID string) (*entity.User, error) {
	u, err := s.Repo.GetByID(userID)
	if err != nil || u == nil {
		return nil, ErrUserNotFound
	}
	return u, nil
}

type UpdateProfileInput struct {
	Name      string
	AvatarURL string
}

// UpdateProfile with ctx, RFC3339 timestamps, and TTL preservation
func (s *Service) UpdateProfile(ctx context.Context, userID string, in UpdateProfileInput) (*entity.User, error) {
	u, err := s.Repo.GetByID(userID)
	if err != nil || u == nil {
		return nil, ErrUserNotFound
	}
	if in.Name != "" {
		u.Name = in.Name
	}
	if in.AvatarURL != "" {
		u.AvatarURL = in.AvatarURL
	}
	if err := s.Repo.Update(u); err != nil {
		return nil, err
	}

	if s.Redis != nil {
		key := sessionKey(u.ID)
		pipe := s.Redis.Pipeline()
		pipe.HSet(ctx, key, map[string]any{
			"name":       u.Name,
			"avatar_url": u.AvatarURL,
			"updated_at": nowRFC3339(),
		})

		if ttl, tErr := s.Redis.TTL(ctx, key).Result(); tErr == nil && ttl > 0 {
			pipe.Expire(ctx, key, ttl)
		}
		if _, pErr := pipe.Exec(ctx); pErr != nil && s.Logger != nil {
			s.Logger.WithError(pErr).WithField("key", key).Warn("redis pipeline failed")
		}
	}
	return u, nil
}

// UploadAvatar demonstrates uploading an avatar to GCS from a reader and updating profile
func (s *Service) UploadAvatar(ctx context.Context, userID string, r io.Reader, filename, contentType string) (string, error) {
	u, err := s.Repo.GetByID(userID)
	if err != nil || u == nil {
		return "", ErrUserNotFound
	}
	url, err := s.uploadImageToGCS(ctx, userID, r, filename, contentType)
	if err != nil {
		return "", err
	}
	u.AvatarURL = url
	if err := s.Repo.Update(u); err != nil {
		return "", err
	}
	// cache meta in redis (optional)
	if s.Redis != nil {
		key := "user:session:" + u.ID
		s.Redis.HSet(ctx, key, map[string]any{
			"avatar_url": u.AvatarURL,
			"updated_at": nowRFC3339(),
		})
	}
	return url, nil
}

func (s *Service) uploadImageToGCS(ctx context.Context, userID string, r io.Reader, filename, contentType string) (string, error) {
	if s.GCS == nil || s.GCSBucket == "" {
		return "", errors.New("gcs not configured")
	}
	id := uuid.NewString()
	ext := strings.ToLower(filepath.Ext(filename))
	objectPath := filepath.ToSlash(filepath.Join("avatars", userID, id+ext))
	return helpers.UploadImageToGCS(ctx, s.GCS, s.GCSBucket, objectPath, contentType, r)
}
