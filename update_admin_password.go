//go:build ignore

package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

func main() {
	db, err := sql.Open("sqlite", "./data/primetime.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// New password: PrimeTime2024
	passwordHash := "$2a$10$AWqtkl.ip5.bVDJyvIQYXOi3TiVQJjhUlcMzFR5nANOPmG3Yg.vM6"

	result, err := db.Exec("UPDATE auth_users SET password_hash = ? WHERE username = ?", passwordHash, "admin")
	if err != nil {
		log.Fatal(err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		log.Fatal(err)
	}

	if rows == 0 {
		fmt.Println("No admin user found. Creating one...")
		_, err = db.Exec(`
			INSERT INTO auth_users (id, username, password_hash, is_admin, created_at)
			VALUES ('admin', 'admin', ?, 1, strftime('%s', 'now'))
		`, passwordHash)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Admin user created!")
	} else {
		fmt.Printf("Updated %d row(s)\n", rows)
	}

	fmt.Println("========================================")
	fmt.Println("Admin password has been reset!")
	fmt.Println("Username: admin")
	fmt.Println("Password: PrimeTime2024")
	fmt.Println("========================================")
}
