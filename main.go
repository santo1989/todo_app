package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

const (
	statusTodo       = "todo"
	statusInProgress = "in-progress"
	statusDone       = "done"
)

var allowedStatuses = map[string]struct{}{
	statusTodo:       {},
	statusInProgress: {},
	statusDone:       {},
}

type note struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Status    string `json:"status"`
	NoteDate  string `json:"noteDate"`
	SortOrder int    `json:"sortOrder"`
}

type user struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

type createNoteRequest struct {
	Title    string `json:"title"`
	Content  string `json:"content"`
	Status   string `json:"status"`
	NoteDate string `json:"noteDate"`
}

type updateNoteRequest struct {
	Title    *string `json:"title"`
	Content  *string `json:"content"`
	Status   *string `json:"status"`
	NoteDate *string `json:"noteDate"`
}

type reorderItem struct {
	ID        int64  `json:"id"`
	Status    string `json:"status"`
	SortOrder int    `json:"sortOrder"`
}

type reorderRequest struct {
	Items []reorderItem `json:"items"`
}

type authRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

const sessionCookieName = "daily_notes_session"

var errUnauthorized = errors.New("unauthorized")

func main() {
	dbPath := getEnv("DB_PATH", "daily_notes.db")
	port := getEnv("PORT", "8080")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := initializeDB(db); err != nil {
		log.Fatal(err)
	}

	r := gin.Default()
	r.LoadHTMLGlob("templates/*")
	r.Static("/assets", "Assets")

	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})

	api := r.Group("/api")
	auth := api.Group("/auth")

	auth.GET("/me", func(c *gin.Context) {
		userID, username, err := userFromRequest(db, c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"authenticated": false})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"authenticated": true,
			"user":          user{ID: userID, Username: username},
		})
	})

	auth.POST("/register", func(c *gin.Context) {
		var req authRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
			return
		}

		username := strings.TrimSpace(req.Username)
		if err := validateUsername(username); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := validatePassword(req.Password); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		hashBytes, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not secure password"})
			return
		}

		res, err := db.Exec("INSERT INTO users (username, password_hash) VALUES (?, ?)", username, string(hashBytes))
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "unique") {
				c.JSON(http.StatusConflict, gin.H{"error": "username already exists"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create user"})
			return
		}

		userID, err := res.LastInsertId()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create session"})
			return
		}

		if err := issueSession(db, c, userID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create session"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"user": user{ID: userID, Username: username}})
	})

	auth.POST("/login", func(c *gin.Context) {
		var req authRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
			return
		}

		username := strings.TrimSpace(req.Username)
		if username == "" || req.Password == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "username and password are required"})
			return
		}

		var userID int64
		var passwordHash string
		err := db.QueryRow("SELECT id, password_hash FROM users WHERE username = ?", username).Scan(&userID, &passwordHash)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not authenticate"})
			return
		}

		if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)) != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}

		if err := issueSession(db, c, userID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create session"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"user": user{ID: userID, Username: username}})
	})

	auth.POST("/logout", func(c *gin.Context) {
		token, err := c.Cookie(sessionCookieName)
		if err == nil && token != "" {
			_, _ = db.Exec("DELETE FROM sessions WHERE token = ?", token)
		}
		clearSessionCookie(c)
		c.Status(http.StatusNoContent)
	})

	notes := api.Group("/notes")
	notes.Use(authMiddleware(db))

	notes.GET("", func(c *gin.Context) {
		userID := currentUserID(c)
		date := c.DefaultQuery("date", time.Now().Format("2006-01-02"))
		if !isValidDate(date) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "date must use YYYY-MM-DD"})
			return
		}

		notes, err := listNotes(db, userID, date)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load notes"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"date": date, "notes": notes})
	})

	notes.POST("", func(c *gin.Context) {
		userID := currentUserID(c)
		var req createNoteRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
			return
		}

		if req.Status == "" {
			req.Status = statusTodo
		}
		if req.NoteDate == "" {
			req.NoteDate = time.Now().Format("2006-01-02")
		}
		if err := validateTitle(req.Title); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if !isValidStatus(req.Status) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
			return
		}
		if !isValidDate(req.NoteDate) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "noteDate must use YYYY-MM-DD"})
			return
		}

		id, err := createNote(db, userID, req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create note"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"id": id})
	})

	notes.PATCH("/:id", func(c *gin.Context) {
		userID := currentUserID(c)
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}

		var req updateNoteRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
			return
		}

		if req.Title != nil {
			if err := validateTitle(*req.Title); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}
		if req.Status != nil && !isValidStatus(*req.Status) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
			return
		}
		if req.NoteDate != nil && !isValidDate(*req.NoteDate) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "noteDate must use YYYY-MM-DD"})
			return
		}

		if err := updateNote(db, userID, id, req); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				c.JSON(http.StatusNotFound, gin.H{"error": "note not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update note"})
			return
		}

		c.Status(http.StatusNoContent)
	})

	notes.PATCH("/reorder", func(c *gin.Context) {
		userID := currentUserID(c)
		var req reorderRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
			return
		}
		if len(req.Items) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "at least one reorder item is required"})
			return
		}

		for _, item := range req.Items {
			if item.ID < 1 || item.SortOrder < 0 || !isValidStatus(item.Status) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid reorder items"})
				return
			}
		}

		if err := reorderNotes(db, userID, req.Items); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				c.JSON(http.StatusNotFound, gin.H{"error": "one or more notes were not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not reorder notes"})
			return
		}

		c.Status(http.StatusNoContent)
	})

	notes.DELETE("/:id", func(c *gin.Context) {
		userID := currentUserID(c)
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}

		res, err := db.Exec("DELETE FROM notes WHERE id = ? AND user_id = ?", id, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not delete note"})
			return
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "note not found"})
			return
		}

		c.Status(http.StatusNoContent)
	})

	if err := r.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}

func initializeDB(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS sessions (
			token TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL,
			expires_at DATETIME NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(user_id) REFERENCES users(id)
		);

		CREATE TABLE IF NOT EXISTS notes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL DEFAULT 1,
			title TEXT NOT NULL,
			content TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL CHECK(status IN ('todo','in-progress','done')),
			note_date TEXT NOT NULL,
			sort_order INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(user_id) REFERENCES users(id)
		);

		CREATE INDEX IF NOT EXISTS idx_notes_user_date_status_sort ON notes(user_id, note_date, status, sort_order);
		CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);
	`)
	if err != nil {
		return err
	}

	legacyUserID, err := ensureLegacyUser(db)
	if err != nil {
		return err
	}

	if err := ensureColumnExists(db, "notes", "user_id", fmt.Sprintf("INTEGER NOT NULL DEFAULT %d", legacyUserID)); err != nil {
		return err
	}

	_, err = db.Exec("UPDATE notes SET user_id = ? WHERE user_id IS NULL OR user_id = 0", legacyUserID)
	return err
}

func listNotes(db *sql.DB, userID int64, date string) ([]note, error) {
	rows, err := db.Query(`
		SELECT id, title, content, status, note_date, sort_order
		FROM notes
		WHERE user_id = ? AND note_date = ?
		ORDER BY CASE status
			WHEN 'todo' THEN 1
			WHEN 'in-progress' THEN 2
			WHEN 'done' THEN 3
		END, sort_order ASC, id ASC
	`, userID, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]note, 0)
	for rows.Next() {
		var n note
		if err := rows.Scan(&n.ID, &n.Title, &n.Content, &n.Status, &n.NoteDate, &n.SortOrder); err != nil {
			return nil, err
		}
		result = append(result, n)
	}

	return result, rows.Err()
}

func createNote(db *sql.DB, userID int64, req createNoteRequest) (int64, error) {
	var nextOrder int
	err := db.QueryRow(
		"SELECT COALESCE(MAX(sort_order), -1) + 1 FROM notes WHERE user_id = ? AND note_date = ? AND status = ?",
		userID,
		req.NoteDate,
		req.Status,
	).Scan(&nextOrder)
	if err != nil {
		return 0, err
	}

	res, err := db.Exec(
		"INSERT INTO notes (user_id, title, content, status, note_date, sort_order) VALUES (?, ?, ?, ?, ?, ?)",
		userID,
		req.Title,
		req.Content,
		req.Status,
		req.NoteDate,
		nextOrder,
	)
	if err != nil {
		return 0, err
	}

	return res.LastInsertId()
}

func updateNote(db *sql.DB, userID int64, id int64, req updateNoteRequest) error {
	var n note
	err := db.QueryRow(
		"SELECT id, title, content, status, note_date, sort_order FROM notes WHERE id = ? AND user_id = ?",
		id,
		userID,
	).Scan(&n.ID, &n.Title, &n.Content, &n.Status, &n.NoteDate, &n.SortOrder)
	if err != nil {
		return err
	}

	if req.Title != nil {
		n.Title = *req.Title
	}
	if req.Content != nil {
		n.Content = *req.Content
	}
	if req.Status != nil {
		n.Status = *req.Status
	}
	if req.NoteDate != nil {
		n.NoteDate = *req.NoteDate
	}

	_, err = db.Exec(
		"UPDATE notes SET title = ?, content = ?, status = ?, note_date = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND user_id = ?",
		n.Title,
		n.Content,
		n.Status,
		n.NoteDate,
		n.ID,
		userID,
	)
	return err
}

func reorderNotes(db *sql.DB, userID int64, items []reorderItem) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare("UPDATE notes SET status = ?, sort_order = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND user_id = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, item := range items {
		res, err := stmt.Exec(item.Status, item.SortOrder, item.ID, userID)
		if err != nil {
			return err
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			return sql.ErrNoRows
		}
	}

	return tx.Commit()
}

func authMiddleware(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _, err := userFromRequest(db, c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			c.Abort()
			return
		}

		c.Set("userID", userID)
		c.Next()
	}
}

func currentUserID(c *gin.Context) int64 {
	v, ok := c.Get("userID")
	if !ok {
		return 0
	}
	userID, _ := v.(int64)
	return userID
}

func userFromRequest(db *sql.DB, c *gin.Context) (int64, string, error) {
	token, err := c.Cookie(sessionCookieName)
	if err != nil || token == "" {
		return 0, "", errUnauthorized
	}

	var userID int64
	var username string
	err = db.QueryRow(`
		SELECT users.id, users.username
		FROM sessions
		JOIN users ON users.id = sessions.user_id
		WHERE sessions.token = ? AND sessions.expires_at > CURRENT_TIMESTAMP
	`, token).Scan(&userID, &username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, "", errUnauthorized
		}
		return 0, "", err
	}

	return userID, username, nil
}

func issueSession(db *sql.DB, c *gin.Context, userID int64) error {
	token, err := generateToken(32)
	if err != nil {
		return err
	}

	expiresAt := time.Now().Add(7 * 24 * time.Hour).UTC().Format("2006-01-02 15:04:05")
	if _, err := db.Exec("INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)", token, userID, expiresAt); err != nil {
		return err
	}

	maxAgeSeconds := int((7 * 24 * time.Hour).Seconds())
	c.SetCookie(sessionCookieName, token, maxAgeSeconds, "/", "", false, true)
	return nil
}

func clearSessionCookie(c *gin.Context) {
	c.SetCookie(sessionCookieName, "", -1, "/", "", false, true)
}

func generateToken(byteLen int) (string, error) {
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func ensureLegacyUser(db *sql.DB) (int64, error) {
	res, err := db.Exec("INSERT OR IGNORE INTO users (username, password_hash) VALUES ('legacy_user', 'legacy_user')")
	if err != nil {
		return 0, err
	}

	id, err := res.LastInsertId()
	if err == nil && id > 0 {
		return id, nil
	}

	var legacyID int64
	err = db.QueryRow("SELECT id FROM users WHERE username = 'legacy_user'").Scan(&legacyID)
	if err != nil {
		return 0, err
	}
	return legacyID, nil
}

func ensureColumnExists(db *sql.DB, tableName string, columnName string, columnDefinition string) error {
	query := fmt.Sprintf("PRAGMA table_info(%s)", tableName)
	rows, err := db.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()

	var (
		cid       int
		name      string
		typeName  string
		notNull   int
		defaultV  interface{}
		primaryK  int
	)

	for rows.Next() {
		if err := rows.Scan(&cid, &name, &typeName, &notNull, &defaultV, &primaryK); err != nil {
			return err
		}
		if name == columnName {
			return nil
		}
	}

	if err := rows.Err(); err != nil {
		return err
	}

	alterQuery := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", tableName, columnName, columnDefinition)
	_, err = db.Exec(alterQuery)
	return err
}

func validateUsername(username string) error {
	if len(username) < 3 || len(username) > 40 {
		return errors.New("username must be 3 to 40 characters")
	}
	for _, r := range username {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return errors.New("username can only contain letters, numbers, and underscore")
	}
	return nil
}

func validatePassword(password string) error {
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	if len(password) > 128 {
		return errors.New("password is too long")
	}
	return nil
}

func getEnv(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func validateTitle(title string) error {
	if len(title) == 0 {
		return errors.New("title is required")
	}
	if len(title) > 120 {
		return errors.New("title must be 120 characters or less")
	}
	return nil
}

func isValidStatus(status string) bool {
	_, ok := allowedStatuses[status]
	return ok
}

func isValidDate(date string) bool {
	_, err := time.Parse("2006-01-02", date)
	return err == nil
}
