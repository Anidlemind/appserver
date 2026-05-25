package sso

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	authpb "carstats/proto"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type contextKey string

const userIDContextKey contextKey = "user_id"

type Service struct {
	authpb.UnimplementedAuthServer
	db       *pgxpool.Pool
	tokenTTL time.Duration
}

func New(db *pgxpool.Pool, tokenTTL time.Duration) *Service {
	return &Service{
		db:       db,
		tokenTTL: tokenTTL,
	}
}

func (s *Service) Register(ctx context.Context, req *authpb.RegisterRequest) (*authpb.RegisterResponse, error) {
	email := strings.TrimSpace(strings.ToLower(req.GetEmail()))
	if email == "" || req.GetPassword() == "" {
		return nil, status.Error(codes.InvalidArgument, "email and password are required")
	}

	passwordHash, err := hashPassword(req.GetPassword())
	if err != nil {
		return nil, status.Error(codes.Internal, "cannot hash password")
	}

	var userID int64
	err = s.db.QueryRow(ctx, `
		INSERT INTO users (email, password_salt, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id
	`, email, "", passwordHash).Scan(&userID)
	if err != nil {
		return nil, err
	}

	token, err := s.issueToken(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &authpb.RegisterResponse{
		UserId: userID,
		Token:  token,
	}, nil
}

func (s *Service) Login(ctx context.Context, req *authpb.LoginRequest) (*authpb.LoginResponse, error) {
	email := strings.TrimSpace(strings.ToLower(req.GetEmail()))
	if email == "" || req.GetPassword() == "" {
		return nil, status.Error(codes.InvalidArgument, "email and password are required")
	}

	var (
		userID       int64
		passwordSalt string
		passwordHash string
	)

	err := s.db.QueryRow(ctx, `
		SELECT id, password_salt, password_hash
		FROM users
		WHERE email = $1
	`, email).Scan(&userID, &passwordSalt, &passwordHash)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}

	if err := verifyPassword(req.GetPassword(), passwordHash, passwordSalt); err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}

	token, err := s.issueToken(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &authpb.LoginResponse{Token: token}, nil
}

func (s *Service) issueToken(ctx context.Context, userID int64) (string, error) {
	token, err := randomHex(32)
	if err != nil {
		return "", status.Error(codes.Internal, "cannot generate token")
	}

	_, err = s.db.Exec(ctx, `
		INSERT INTO auth_tokens (token, user_id, expires_at)
		VALUES ($1, $2, $3)
	`, token, userID, time.Now().UTC().Add(s.tokenTTL))
	if err != nil {
		return "", status.Error(codes.Internal, "cannot persist token")
	}

	return token, nil
}

func (s *Service) ValidateToken(ctx context.Context, token string) (int64, error) {
	var userID int64

	err := s.db.QueryRow(ctx, `
		SELECT user_id
		FROM auth_tokens
		WHERE token = $1 AND expires_at > NOW()
	`, token).Scan(&userID)
	if err != nil {
		return 0, status.Error(codes.Unauthenticated, "invalid or expired token")
	}

	return userID, nil
}

func UnaryAuthInterceptor(authService *Service) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		if info.FullMethod == "/auth.Auth/Register" || info.FullMethod == "/auth.Auth/Login" {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "authorization metadata is required")
		}

		token, err := extractBearerToken(md)
		if err != nil {
			return nil, err
		}

		userID, err := authService.ValidateToken(ctx, token)
		if err != nil {
			return nil, err
		}

		return handler(context.WithValue(ctx, userIDContextKey, userID), req)
	}
}

func UserIDFromContext(ctx context.Context) (int64, bool) {
	userID, ok := ctx.Value(userIDContextKey).(int64)
	return userID, ok
}

func extractBearerToken(md metadata.MD) (string, error) {
	values := md.Get("authorization")
	if len(values) == 0 {
		return "", status.Error(codes.Unauthenticated, "authorization token is required")
	}

	parts := strings.SplitN(values[0], " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") || strings.TrimSpace(parts[1]) == "" {
		return "", status.Error(codes.Unauthenticated, "authorization header must be Bearer <token>")
	}

	return strings.TrimSpace(parts[1]), nil
}

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func verifyPassword(password, passwordHash, passwordSalt string) error {
	if strings.HasPrefix(passwordHash, "$2") {
		return bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password))
	}

	legacyHash := legacyHashPassword(password, passwordSalt)
	if legacyHash != passwordHash {
		return bcrypt.ErrMismatchedHashAndPassword
	}

	return nil
}

func legacyHashPassword(password, salt string) string {
	sum := sha256SumHex(salt + ":" + password)
	return sum
}

func sha256SumHex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func randomHex(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
