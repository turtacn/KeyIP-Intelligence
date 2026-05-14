package main

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	dsn := "postgres://keyip:keyip_dev@keyip-postgres:5432/keyip_dev?sslmode=disable"
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		fmt.Printf("ERROR open: %v\n", err)
		return
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		fmt.Printf("ERROR ping: %v\n", err)
		return
	}
	fmt.Println("Connected to postgres!")

	var (
		id, email, username, displayName string
		pwd                              sql.NullString
		status, avatarURL, locale, timezone string
		lastLoginIP                      sql.NullString
		loginCount, failedLoginCount     int
		mfaEnabled                       bool
		mfaSecret                        sql.NullString
	)
	var lastLoginAt, lockedUntil, emailVerifiedAt, createdAt, updatedAt, deletedAt sql.NullTime
	var pref, meta []byte

	row := db.QueryRow("SELECT * FROM users WHERE email = $1 AND deleted_at IS NULL", "test@test.com")
	err = row.Scan(&id, &email, &username, &displayName, &pwd, &status, &avatarURL, &locale, &timezone,
		&lastLoginAt, &lastLoginIP, &loginCount, &failedLoginCount, &lockedUntil,
		&emailVerifiedAt, &mfaEnabled, &mfaSecret, &pref, &meta, &createdAt, &updatedAt, &deletedAt)
	
	if err != nil {
		fmt.Printf("ERROR scan: %v\n", err)
		return
	}

	fmt.Printf("User found: email=%s status=%s pwd=%s\n", email, status, pwd.String)

	if !pwd.Valid {
		fmt.Println("ERROR: password_hash is NULL!")
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(pwd.String), []byte("123456"))
	fmt.Printf("bcrypt compare: err=%v\n", err)
}
