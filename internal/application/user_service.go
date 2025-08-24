package application

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
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
	ErrEmailNotVerified   = errors.New("email not verified")
)

type Service struct {
	Repo         repo.UserRepository
	JWT          *helpers.JWTManager
	GCS          *storage.Client
	GCSBucket    string
	Redis        *redis.Client
	Logger       *logrus.Logger
	ES           *elasticsearch.Client
	ESUsersIndex string
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

func NewService(repo repo.UserRepository, jwt *helpers.JWTManager, gcs *storage.Client, gcsBucket string, rdb *redis.Client, logger *logrus.Logger, es *elasticsearch.Client, esUsersIndex string) *Service {
	return &Service{
		Repo:         repo,
		JWT:          jwt,
		GCS:          gcs,
		GCSBucket:    gcsBucket,
		Redis:        rdb,
		Logger:       logger,
		ES:           es,
		ESUsersIndex: esUsersIndex,
	}
}

type LoginResponse struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
}

// Authenticate validates email/password and returns the user without issuing tokens.
func (s *Service) Authenticate(ctx context.Context, email, password string) (*entity.User, error) {
	u, err := s.Repo.GetByEmail(email)
	if err != nil || u == nil {
		return nil, ErrInvalidCredentials
	}
	if !helpers.CompareHashAndPassword(u.Password, password) {
		return nil, ErrInvalidCredentials
	}
	// Do not block on email verification here; email verification is triggered after login via protected endpoint.
	return u, nil
}

// IssueTokens generates access/refresh tokens and records a session in Redis.
func (s *Service) IssueTokens(ctx context.Context, u *entity.User) (TokenPair, error) {
	sid := uuid.NewString()
	access, aexp, err := s.JWT.GenerateAccessToken(u.ID, sid)
	if err != nil {
		if s.Logger != nil {
			s.Logger.WithError(err).WithField("user_id", u.ID).Error("generate access token failed")
		}
		return TokenPair{}, err
	}
	refresh, rexp, err := s.JWT.GenerateRefreshToken(u.ID, sid)
	if err != nil {
		if s.Logger != nil {
			s.Logger.WithError(err).WithField("user_id", u.ID).Error("generate refresh token failed")
		}
		return TokenPair{}, err
	}

	if s.Redis != nil {
		fields := map[string]any{
			"user_id":    u.ID,
			"email":      u.Email,
			"name":       u.Name,
			"avatar_url": u.AvatarURL,
			"sid":        sid,
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

	return TokenPair{AccessToken: access, AccessTokenExpiry: aexp, RefreshToken: refresh, RefreshTokenExpiry: rexp}, nil
}

func (s *Service) Login(ctx context.Context, email, password string) (*LoginResponse, TokenPair, error) {
	u, err := s.Authenticate(ctx, email, password)
	if err != nil {
		return nil, TokenPair{}, err
	}
	pair, err := s.IssueTokens(ctx, u)
	if err != nil {
		return nil, TokenPair{}, err
	}
	resp := &LoginResponse{UserID: u.ID, Email: u.Email, Name: u.Name}
	return resp, pair, nil
}

// GetUserByEmail New helper to get user by email without password check (used by OTP confirm flow)
func (s *Service) GetUserByEmail(ctx context.Context, email string) (*entity.User, error) {
	u, err := s.Repo.GetByEmail(email)
	if err != nil || u == nil {
		return nil, ErrUserNotFound
	}
	return u, nil
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
	// Validate current session id matches the token's sid
	if s.Redis != nil {
		key := sessionKey(u.ID)
		data, rErr := s.Redis.HGetAll(ctx, key).Result()
		if rErr != nil || len(data) == 0 || data["sid"] != claims.SessionID {
			return TokenPair{}, "", ErrInvalidCredentials
		}
	}
	// Rotate session id and tokens
	sid := uuid.NewString()
	access, aexp, err := s.JWT.GenerateAccessToken(u.ID, sid)
	if err != nil {
		return TokenPair{}, "", err
	}
	refresh, rexp, err := s.JWT.GenerateRefreshToken(u.ID, sid)
	if err != nil {
		return TokenPair{}, "", err
	}
	if s.Redis != nil {
		key := sessionKey(u.ID)
		pipe := s.Redis.Pipeline()
		pipe.HSet(ctx, key, map[string]any{
			"sid":        sid,
			"updated_at": nowRFC3339(),
		})
		pipe.Expire(ctx, key, 24*time.Hour)
		_, _ = pipe.Exec(ctx)
	}
	return TokenPair{AccessToken: access, AccessTokenExpiry: aexp, RefreshToken: refresh, RefreshTokenExpiry: rexp}, u.ID, nil
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

	// Index latest profile to Elasticsearch
	_ = s.indexUser(ctx, u)
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
	// Re-index
	_ = s.indexUser(ctx, u)
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

func (s *Service) indexUser(ctx context.Context, u *entity.User) error {
	if s.ES == nil || s.ESUsersIndex == "" {
		return nil
	}
	doc := map[string]any{
		"id":         u.ID,
		"email":      u.Email,
		"name":       u.Name,
		"avatar_url": u.AvatarURL,
		"created_at": u.CreatedAt.Format(time.RFC3339Nano),
		"updated_at": u.UpdatedAt.Format(time.RFC3339Nano),
	}
	b, _ := json.Marshal(doc)
	req := esapi.IndexRequest{Index: s.ESUsersIndex, DocumentID: u.ID, Body: strings.NewReader(string(b)), Refresh: "false"}
	c, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	res, err := req.Do(c, s.ES)
	if err != nil {
		if s.Logger != nil {
			s.Logger.WithError(err).WithField("user_id", u.ID).Warn("es index failed")
		}
		return err
	}
	defer func() { _ = res.Body.Close() }()
	if res.IsError() && s.Logger != nil {
		s.Logger.WithField("status", res.Status()).WithField("user_id", u.ID).Warn("es index response error")
	}
	return nil
}

// SearchUsers performs a simple multi_match search on email and name.
func (s *Service) SearchUsers(ctx context.Context, q string, size int) ([]map[string]any, error) {
	if s.ES == nil || s.ESUsersIndex == "" {
		return []map[string]any{}, nil
	}
	if size <= 0 || size > 50 {
		size = 10
	}
	query := map[string]any{
		"query": map[string]any{
			"multi_match": map[string]any{
				"query":  q,
				"fields": []string{"email^2", "name"},
			},
		},
		"size": size,
	}
	b, _ := json.Marshal(query)

	c, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	res, err := s.ES.Search(s.ES.Search.WithContext(c), s.ES.Search.WithIndex(s.ESUsersIndex), s.ES.Search.WithBody(strings.NewReader(string(b))))

	if err != nil {
		return nil, err
	}

	defer func() {
		_ = res.Body.Close()
	}()

	var parsed struct {
		Hits struct {
			Hits []struct {
				ID     string         `json:"_id"`
				Source map[string]any `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(res.Body).Decode(&parsed); err != nil {
		return nil, err
	}

	out := make([]map[string]any, 0, len(parsed.Hits.Hits))

	for _, h := range parsed.Hits.Hits {
		out = append(out, h.Source)
	}

	return out, nil
}
