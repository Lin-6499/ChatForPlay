package main

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type store struct{ *sql.DB }

type user struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Admin    bool   `json:"admin"`
	Online   bool   `json:"online"`
}

type message struct {
	ID             int64  `json:"id"`
	ConversationID int64  `json:"conversation_id"`
	SenderID       int64  `json:"sender_id"`
	SenderName     string `json:"sender_name"`
	ClientID       string `json:"client_id"`
	Kind           string `json:"kind"`
	Content        string `json:"content"`
	FileID         string `json:"file_id,omitempty"`
	FileName       string `json:"file_name,omitempty"`
	FileSize       int64  `json:"file_size,omitempty"`
	FileMIME       string `json:"file_mime,omitempty"`
	CreatedAt      int64  `json:"created_at"`
}

const messageSelect = `SELECT m.id,m.conversation_id,m.sender_id,u.nickname,m.client_id,m.kind,m.content,
	COALESCE(m.file_id,''),COALESCE(f.name,''),COALESCE(f.size,0),COALESCE(f.mime,''),m.created_at
	FROM messages m JOIN users u ON u.id=m.sender_id LEFT JOIN files f ON f.id=m.file_id`

type scanner interface{ Scan(...any) error }

func scanMessage(row scanner) (message, error) {
	var m message
	err := row.Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.SenderName, &m.ClientID, &m.Kind,
		&m.Content, &m.FileID, &m.FileName, &m.FileSize, &m.FileMIME, &m.CreatedAt)
	return m, err
}

func openStore(path string) (*store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	for _, pragma := range []string{"PRAGMA journal_mode=WAL", "PRAGMA busy_timeout=10000", "PRAGMA foreign_keys=ON", "PRAGMA synchronous=FULL"} {
		if _, err = db.Exec(pragma); err != nil {
			db.Close()
			return nil, err
		}
	}
	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS users (
	  id INTEGER PRIMARY KEY AUTOINCREMENT, username TEXT NOT NULL UNIQUE COLLATE NOCASE,
	  nickname TEXT NOT NULL, password TEXT NOT NULL, admin INTEGER NOT NULL DEFAULT 0,
	  created_at INTEGER NOT NULL
	);
	CREATE TABLE IF NOT EXISTS sessions (
	  token TEXT PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES users(id), expires_at INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS session_user ON sessions(user_id);
	CREATE TABLE IF NOT EXISTS invites (
	  token TEXT PRIMARY KEY, created_by INTEGER NOT NULL REFERENCES users(id),
	  remaining INTEGER NOT NULL, expires_at INTEGER NOT NULL
	);
	CREATE TABLE IF NOT EXISTS conversations (
	  id INTEGER PRIMARY KEY AUTOINCREMENT, kind TEXT NOT NULL, name TEXT NOT NULL,
	  owner_id INTEGER NOT NULL REFERENCES users(id), direct_key TEXT UNIQUE,
	  created_at INTEGER NOT NULL
	);
	CREATE TABLE IF NOT EXISTS members (
	  conversation_id INTEGER NOT NULL REFERENCES conversations(id), user_id INTEGER NOT NULL REFERENCES users(id),
	  last_read INTEGER NOT NULL DEFAULT 0, PRIMARY KEY(conversation_id,user_id)
	);
	CREATE TABLE IF NOT EXISTS files (
	  id TEXT PRIMARY KEY, conversation_id INTEGER NOT NULL REFERENCES conversations(id),
	  uploaded_by INTEGER NOT NULL REFERENCES users(id), name TEXT NOT NULL,
	  size INTEGER NOT NULL, mime TEXT NOT NULL, created_at INTEGER NOT NULL
	);
	CREATE TABLE IF NOT EXISTS messages (
	  id INTEGER PRIMARY KEY AUTOINCREMENT, conversation_id INTEGER NOT NULL REFERENCES conversations(id),
	  sender_id INTEGER NOT NULL REFERENCES users(id), client_id TEXT NOT NULL,
	  kind TEXT NOT NULL, content TEXT NOT NULL, file_id TEXT REFERENCES files(id), created_at INTEGER NOT NULL,
	  UNIQUE(sender_id,client_id)
	);
	CREATE INDEX IF NOT EXISTS messages_conversation ON messages(conversation_id,id);
	CREATE INDEX IF NOT EXISTS members_user ON members(user_id,conversation_id);
	PRAGMA user_version=1;
	`)
	if err != nil {
		db.Close()
		return nil, err
	}
	_, err = db.Exec("DELETE FROM sessions WHERE expires_at < ?", time.Now().UnixMilli())
	if err != nil {
		db.Close()
		return nil, err
	}
	return &store{db}, nil
}

func (s *store) isMember(cid, uid int64) bool {
	var n int
	return s.QueryRow("SELECT 1 FROM members WHERE conversation_id=? AND user_id=?", cid, uid).Scan(&n) == nil
}

func (s *store) memberIDs(cid int64) []int64 {
	rows, err := s.Query("SELECT user_id FROM members WHERE conversation_id=?", cid)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

func (s *store) getMessage(id int64) (message, error) {
	return scanMessage(s.QueryRow(messageSelect+" WHERE m.id=?", id))
}

func directKey(a, b int64) string {
	if a > b {
		a, b = b, a
	}
	return fmt.Sprintf("%d:%d", a, b)
}
