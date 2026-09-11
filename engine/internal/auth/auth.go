package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type contextKey string

const userIDKey contextKey = "user_id"

// Service 处理用户注册、登录与 JWT 鉴权。
type Service struct {
	db     *sql.DB
	secret []byte
}

func NewService(db *sql.DB) *Service {
	return &Service{
		db:     db,
		secret: []byte("smart-assistant-dev-secret-change-in-production"),
	}
}

type registerRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// AuthResult 认证结果（注册或登录成功后返回）。
type AuthResult struct {
	Token    string `json:"token"`
	UserID   string `json:"user_id"`
	Username string `json:"username"`
}

// Register 创建新用户并返回认证结果。
func (s *Service) Register(username, password string) (*AuthResult, error) {
	if username == "" || password == "" {
		return nil, fmt.Errorf("username and password required")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("internal error")
	}

	userID := uuid.New().String()
	_, err = s.db.Exec(
		`INSERT INTO users (id, username, password_hash) VALUES (?, ?, ?)`,
		userID, username, string(hash),
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, fmt.Errorf("username already exists")
		}
		return nil, err
	}

	// 创建默认工作空间
	_, _ = s.db.Exec(
		`INSERT INTO workspaces (id, user_id, name) VALUES (?, ?, ?)`,
		uuid.New().String(), userID, "默认空间",
	)

	token, err := s.generateToken(userID, username)
	if err != nil {
		return nil, fmt.Errorf("token generation failed")
	}

	return &AuthResult{Token: token, UserID: userID, Username: username}, nil
}

// Login 验证用户凭据并返回认证结果。
func (s *Service) Login(username, password string) (*AuthResult, error) {
	var userID, dbUsername, passwordHash string
	err := s.db.QueryRow(
		`SELECT id, username, password_hash FROM users WHERE username = ?`, username,
	).Scan(&userID, &dbUsername, &passwordHash)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("invalid credentials")
	} else if err != nil {
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	// 确保用户有默认工作空间（兼容已有用户或 DB 重置场景）
	var wsCount int
	if err := s.db.QueryRow(`SELECT COUNT(1) FROM workspaces WHERE user_id = ?`, userID).Scan(&wsCount); err == nil && wsCount == 0 {
		_, _ = s.db.Exec(
			`INSERT INTO workspaces (id, user_id, name) VALUES (?, ?, ?)`,
			uuid.New().String(), userID, "默认空间",
		)
	}

	token, err := s.generateToken(userID, dbUsername)
	if err != nil {
		return nil, fmt.Errorf("token generation failed")
	}

	return &AuthResult{Token: token, UserID: userID, Username: dbUsername}, nil
}

func (s *Service) HandleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	result, err := s.Register(req.Username, req.Password)
	if err != nil {
		code := http.StatusInternalServerError
		if strings.Contains(err.Error(), "required") {
			code = http.StatusBadRequest
		} else if strings.Contains(err.Error(), "already exists") {
			code = http.StatusConflict
		}
		http.Error(w, err.Error(), code)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Service) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	result, err := s.Login(req.Username, req.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Service) generateToken(userID, username string) (string, error) {
	claims := jwt.MapClaims{
		"user_id":  userID,
		"username": username,
		"exp":      time.Now().Add(72 * time.Hour).Unix(),
		"iat":      time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

// Middleware JWT 鉴权中间件。
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if header == "" {
			http.Error(w, "authorization required", http.StatusUnauthorized)
			return
		}

		tokenStr := strings.TrimPrefix(header, "Bearer ")
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			return s.secret, nil
		})
		if err != nil || !token.Valid {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			http.Error(w, "invalid token claims", http.StatusUnauthorized)
			return
		}

		userID, _ := claims["user_id"].(string)
		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// UserIDFromContext 从请求上下文中提取用户 ID。
func UserIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(userIDKey).(string)
	return id
}