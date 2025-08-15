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

	entity "boilerplate-go-pgsql/internal/domain/entity"
	repo "boilerplate-go-pgsql/internal/domain/repository"
	"boilerplate-go-pgsql/pkg/helpers"
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

func NewService(repo repo.UserRepository, jwt *helpers.JWTManager, gcs *storage.Client, gcsBucket string, rdb *redis.Client, logger *logrus.Logger) *Service {
	return &Service{Repo: repo, JWT: jwt, GCS: gcs, GCSBucket: gcsBucket, Redis: rdb, Logger: logger}
}

type LoginResponse struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
}

func (s *Service) Login(email, password string) (*LoginResponse, string, time.Time, string, time.Time, error) {
	u, err := s.Repo.GetByEmail(email)
	if err != nil {
		return nil, "", time.Time{}, "", time.Time{}, ErrInvalidCredentials
	}
	if !helpers.CompareHashAndPassword(u.Password, password) {
		return nil, "", time.Time{}, "", time.Time{}, ErrInvalidCredentials
	}
	access, aexp, err := s.JWT.GenerateAccessToken(u.ID)
	if err != nil {
		return nil, "", time.Time{}, "", time.Time{}, err
	}
	refresh, rexp, err := s.JWT.GenerateRefreshToken(u.ID)
	if err != nil {
		return nil, "", time.Time{}, "", time.Time{}, err
	}
	return &LoginResponse{UserID: u.ID, Email: u.Email, Name: u.Name}, access, aexp, refresh, rexp, nil
}

func (s *Service) Refresh(refreshToken string) (string, time.Time, string, time.Time, string, error) {
	claims, err := s.JWT.ParseRefreshToken(refreshToken)
	if err != nil {
		return "", time.Time{}, "", time.Time{}, "", ErrInvalidCredentials
	}
	// ensure user still exists
	u, err := s.Repo.GetByID(claims.UserID)
	if err != nil || u == nil {
		return "", time.Time{}, "", time.Time{}, "", ErrInvalidCredentials
	}
	access, aexp, err := s.JWT.GenerateAccessToken(u.ID)
	if err != nil {
		return "", time.Time{}, "", time.Time{}, "", err
	}
	refresh, rexp, err := s.JWT.GenerateRefreshToken(u.ID)
	if err != nil {
		return "", time.Time{}, "", time.Time{}, "", err
	}
	return access, aexp, refresh, rexp, u.ID, nil
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

func (s *Service) UpdateProfile(userID string, in UpdateProfileInput) (*entity.User, error) {
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
	meta := map[string]any{"user_id": userID, "avatar_url": url, "updated_at": time.Now()}
	_ = helpers.RedisSetJSON(ctx, s.Redis, "user:avatar:"+userID, meta, 24*time.Hour)
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
