package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// PBKDF2-HMAC-SHA256 implemented with the standard library for a dependency-light
// demonstration project. Iteration count is intentionally high; never store plain passwords.
const passwordIterations = 600000

func pbkdf2SHA256(password, salt []byte, iterations, keyLen int) []byte {
	hLen := sha256.Size
	blocks := (keyLen + hLen - 1) / hLen
	out := make([]byte, 0, blocks*hLen)
	for block := 1; block <= blocks; block++ {
		h := hmac.New(sha256.New, password)
		h.Write(salt)
		h.Write([]byte{byte(block >> 24), byte(block >> 16), byte(block >> 8), byte(block)})
		u := h.Sum(nil)
		t := make([]byte, len(u))
		copy(t, u)
		for i := 1; i < iterations; i++ {
			h = hmac.New(sha256.New, password)
			h.Write(u)
			u = h.Sum(nil)
			for j := range t {
				t[j] ^= u[j]
			}
		}
		out = append(out, t...)
	}
	return out[:keyLen]
}
func hashPassword(password string) (string, error) {
	if len(password) < 10 || len(password) > 200 {
		return "", errors.New("password must have between 10 and 200 characters")
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := pbkdf2SHA256([]byte(password), salt, passwordIterations, 32)
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s", passwordIterations, hex.EncodeToString(salt), hex.EncodeToString(key)), nil
}
func verifyPassword(password, encoded string) bool {
	pieces := strings.Split(encoded, "$")
	if len(pieces) != 4 || pieces[0] != "pbkdf2-sha256" {
		return false
	}
	var iterations int
	if _, err := fmt.Sscanf(pieces[1], "%d", &iterations); err != nil || iterations < 10000 || iterations > 1000000 {
		return false
	}
	salt, e := hex.DecodeString(pieces[2])
	if e != nil || len(salt) != 16 {
		return false
	}
	expect, e := hex.DecodeString(pieces[3])
	if e != nil || len(expect) != 32 {
		return false
	}
	actual := pbkdf2SHA256([]byte(password), salt, iterations, 32)
	return subtle.ConstantTimeCompare(expect, actual) == 1
}

type account struct {
	ID                       int64
	StudentID                int64
	Username, FullName, Role string
	Active                   bool
}
type session struct {
	UserID  int64
	CSRF    string
	Expires time.Time
}
type loginAttempt struct {
	Failures    int
	WindowStart time.Time
}
type App struct {
	store    *Store
	mu       sync.Mutex
	sessions map[string]session
	attempts map[string]loginAttempt
}

func newApp(s *Store) *App {
	return &App{store: s, sessions: map[string]session{}, attempts: map[string]loginAttempt{}}
}
func secureToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func (a *App) currentUser(r *http.Request) (account, string, bool) {
	c, e := r.Cookie("student_session")
	if e != nil {
		return account{}, "", false
	}
	a.mu.Lock()
	ses, ok := a.sessions[c.Value]
	if ok && time.Now().After(ses.Expires) {
		delete(a.sessions, c.Value)
		ok = false
	}
	a.mu.Unlock()
	if !ok {
		return account{}, "", false
	}
	var acc account
	var active int
	err := a.store.DB.QueryRow(`SELECT user_id,username,full_name,role,is_active,COALESCE(student_id,0) FROM users WHERE user_id=?`, ses.UserID).Scan(&acc.ID, &acc.Username, &acc.FullName, &acc.Role, &active, &acc.StudentID)
	if err != nil || active != 1 || (acc.Role != "Admin" && acc.Role != "Student") || (acc.Role == "Student" && acc.StudentID == 0) {
		return account{}, "", false
	}
	acc.Active = true
	return acc, ses.CSRF, true
}
func (a *App) clearSession(w http.ResponseWriter, r *http.Request) {
	if c, e := r.Cookie("student_session"); e == nil {
		a.mu.Lock()
		delete(a.sessions, c.Value)
		a.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: "student_session", Value: "", Path: "/", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode, MaxAge: -1})
}
func (a *App) revokeUserSessions(userID int64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for key, s := range a.sessions {
		if s.UserID == userID {
			delete(a.sessions, key)
		}
	}
}
func (a *App) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Credential verification and session creation are atomic relative to resets.
	a.store.writeMu.Lock()
	defer a.store.writeMu.Unlock()
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &in); err != nil {
		apiError(w, 400, err.Error())
		return
	}
	in.Username = strings.TrimSpace(in.Username)
	if len(in.Username) == 0 || len(in.Username) > 80 || len(in.Password) > 200 {
		apiError(w, 400, "invalid credentials")
		return
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	now := time.Now()
	a.mu.Lock()
	attempt := a.attempts[host]
	if now.Sub(attempt.WindowStart) > 10*time.Minute {
		attempt = loginAttempt{WindowStart: now}
	}
	if attempt.Failures >= 10 {
		a.mu.Unlock()
		apiError(w, 429, "too many attempts; please wait 10 minutes")
		return
	}
	// Reserve the attempt before hashing so concurrent requests cannot bypass the limit.
	attempt.Failures++
	a.attempts[host] = attempt
	for key, item := range a.attempts {
		if now.Sub(item.WindowStart) > 10*time.Minute {
			delete(a.attempts, key)
		}
	}
	for key, item := range a.sessions {
		if now.After(item.Expires) {
			delete(a.sessions, key)
		}
	}
	a.mu.Unlock()
	var id int64
	var hash string
	var active int
	err = a.store.DB.QueryRow(`SELECT user_id,password_hash,is_active FROM users WHERE username=? COLLATE NOCASE`, in.Username).Scan(&id, &hash, &active)
	// Equal-cost fake hashing reduces username enumeration timing differences.
	if err == sql.ErrNoRows {
		hash = "pbkdf2-sha256$600000$00112233445566778899aabbccddeeff$00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
	}
	valid := verifyPassword(in.Password, hash) && err == nil && active == 1
	if !valid {
		apiError(w, 401, "incorrect username or password")
		return
	}
	a.mu.Lock()
	delete(a.attempts, host)
	a.mu.Unlock()
	token, e := secureToken()
	if e != nil {
		apiError(w, 500, "unable to establish a session")
		return
	}
	csrf, e := secureToken()
	if e != nil {
		apiError(w, 500, "unable to establish a session")
		return
	}
	a.clearSession(w, r)
	a.mu.Lock()
	a.sessions[token] = session{UserID: id, CSRF: csrf, Expires: time.Now().Add(8 * time.Hour)}
	a.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "student_session", Value: token, Path: "/", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode, Expires: time.Now().Add(8 * time.Hour)})
	writeJSON(w, 200, map[string]string{"status": "ok"})
}
func sameOrigin(r *http.Request) bool {
	if r.Method == "GET" || r.Method == "HEAD" || r.Method == "OPTIONS" {
		return true
	}
	// Browser fetch includes Origin on cross-origin requests; reject mismatched values.
	origin := r.Header.Get("Origin")
	if origin == "" {
		return r.Header.Get("Sec-Fetch-Site") != "cross-site"
	}
	return origin == "http://"+r.Host || origin == "https://"+r.Host
}
func (a *App) protect(next http.HandlerFunc, adminOnly bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			a.store.writeMu.Lock()
			defer a.store.writeMu.Unlock()
		}
		acc, csrf, ok := a.currentUser(r)
		if !ok {
			apiError(w, 401, "please log in")
			return
		}
		if adminOnly && acc.Role != "Admin" {
			apiError(w, 403, "admin access required")
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" && subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(csrf)) != 1 {
			apiError(w, 403, "invalid request token")
			return
		}
		next(w, r)
	}
}
func (a *App) handleMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		apiError(w, 405, "Method not allowed.")
		return
	}
	acc, csrf, ok := a.currentUser(r)
	if !ok {
		apiError(w, 401, "please log in")
		return
	}
	writeJSON(w, 200, map[string]any{"user": acc, "csrf_token": csrf})
}
func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		apiError(w, 405, "Method not allowed.")
		return
	}
	a.clearSession(w, r)
	writeJSON(w, 200, map[string]string{"status": "logged out"})
}
