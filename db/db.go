// Package database is the package that contains the database files
package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Page struct {
	ID        int
	Title     string
	Body      []byte
	CreatedOn time.Time
	EntryType string
	EntryDate sql.NullTime
}

// defining the DB variable as sql.DB
var DB *sql.DB

// InitDB that will be exported to main
func InitDB() {
	var err error

	exe, err := os.Executable()
	if err != nil {
		log.Fatalf("Failed to resolve executable path: %v", err)
	}

	dbPath := filepath.Join(filepath.Dir(exe), "db_wiki.db")
	DB, err = sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
}

// GetUserPassword to be exproted to main
func GetUserPassword(username string) (string, error) {
	var password string
	err := DB.QueryRow("SELECT password from users Where username = ?", username).Scan(&password)
	return password, err
}

func GetPage(title string) (*Page, error) {
	var p Page

	err := DB.QueryRow(`
		SELECT id, title, body, entry_type, entry_date, created_at 
		FROM pages
		WHERE title = ?
	`, title).Scan(
		&p.ID,
		&p.Title,
		&p.Body,
		&p.EntryType,
		&p.EntryDate,
		&p.CreatedOn,
	)
	if err != nil {
		return nil, err
	}

	return &p, nil
}

// save page function
func SavePage(title string, body []byte) error {
	var id int
	err := DB.QueryRow("SELECT id FROM pages WHERE title = ?", title).Scan(&id)

	if err == sql.ErrNoRows {
		// Title doesn't exist, INSERT new page
		_, err = DB.Exec("INSERT INTO pages (title, body, created_at) VALUES (?, ?, ?)", title, body, time.Now())
		return err
	} else if err != nil {
		return err
	}

	// Title exists, UPDATE existing page body
	_, err = DB.Exec("UPDATE pages SET body = ? WHERE title = ?", body, title)
	return err
}

// create page
func CreatePage(title string, body []byte, entryType string, entryDate *time.Time) error {
	var nullDate sql.NullTime
	if entryDate != nil {
		nullDate = sql.NullTime{Time: *entryDate, Valid: true}
	}
	_, err := DB.Exec(
		"INSERT into PAGES (title, body, entry_type, entry_date, created_at) VALUES(?,?,?,?,?)",
		title, body, entryType, nullDate, time.Now(),
	)
	return err
}

func GetTitles() ([]Page, error) {
	rows, err := DB.Query("SELECT title, entry_type, entry_date FROM pages ORDER BY title")
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var pages []Page
	for rows.Next() {
		var p Page
		if err := rows.Scan(&p.Title, &p.EntryType, &p.EntryDate); err != nil {
			return nil, fmt.Errorf("row scan failed: %w", err)
		}
		pages = append(pages, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}
	return pages, nil
}
