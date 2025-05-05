package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	
	_ "github.com/lib/pq"
)

// Table-specific SQL queries to dump rows related to a user_id/account_id.
var queries = map[string]string{
	"accounts":          "SELECT * FROM accounts WHERE id = $1",
	"users":             "SELECT * FROM users WHERE id = (SELECT user_id FROM accounts WHERE id = $1)",
	"app_auth_tokens":   "SELECT * FROM app_auth_tokens WHERE user_id = (SELECT user_id FROM accounts WHERE id = $1)",
	"user_identities":   "SELECT * FROM user_identities WHERE user_id = (SELECT user_id FROM accounts WHERE id = $1)",
	"user_preferences":  "SELECT * FROM user_preferences WHERE user_id = (SELECT user_id FROM accounts WHERE id = $1)",
}

func main() {
	flagOutput := flag.Bool("output", false, "Write output to .sql file named after user_id")
	flag.Parse()

	if flag.NArg() != 1 {
		log.Fatalf("Usage: %s [--output=true] <account_id>", os.Args[0])
	}
	accountID := flag.Arg(0)

	db := setupDB()
	defer db.Close()

	userID := lookupUserID(db, accountID)

	var file *os.File
	if *flagOutput {
		filename := fmt.Sprintf("user_%s_dump.sql", userID)
		var err error
		file, err = os.Create(filename)
		if err != nil {
			log.Fatalf("Failed to create output file: %v", err)
		}
		defer file.Close()
	}

	// Generate and print INSERT statements for each table
	for table, query := range queries {
		runDump(db, query, table, accountID, file)
	}
}

// setupDB reads DB connection info from env and returns a ready *sql.DB
func setupDB() *sql.DB {
	// Use DATABASE_URL if present
	if url := os.Getenv("DATABASE_URL"); url != "" {
		db, err := sql.Open("postgres", url)
		if err != nil {
			log.Fatalf("Failed to connect using DATABASE_URL: %v", err)
		}
		return db
	}

	// Otherwise fall back to individual env vars
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	pass := os.Getenv("DB_PASS")
	name := os.Getenv("DB_NAME")
	ssl := os.Getenv("DB_SSLMODE")

	if host == "" || port == "" || user == "" || name == "" {
		log.Fatal("Missing required DB environment variables. Either set DATABASE_URL or DB_HOST, DB_PORT, DB_USER, DB_NAME (and optionally DB_PASSWORD)")
	}

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s", host, port, user, pass, name, ssl)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to DB: %v", err)
	}
	return db
}

// lookupUserID extracts the user_id from the accounts table for a given account_id
func lookupUserID(db *sql.DB, accountID string) string {
	var userID string
	err := db.QueryRow("SELECT user_id FROM accounts WHERE id = $1", accountID).Scan(&userID)
	if err != nil {
		log.Fatalf("Failed to look up user_id: %v", err)
	}
	return userID
}

// runDump executes a query and prints INSERT statements for each row found
func runDump(db *sql.DB, query, table, param string, file *os.File) {
	rows, err := db.Query(query, param)
	if err != nil {
		log.Fatalf("Query for table %s failed: %v", table, err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		log.Fatalf("Failed to get columns: %v", err)
	}

	// Loop through each row and print formatted INSERT
	for rows.Next() {
		raw := make([]interface{}, len(cols))
		dest := make([]interface{}, len(cols))
		for i := range raw {
			dest[i] = &raw[i]
		}
		err := rows.Scan(dest...)
		if err != nil {
			log.Fatalf("Scan failed: %v", err)
		}

		values := make([]string, len(cols))
		for i, val := range raw {
			switch v := val.(type) {
			case nil:
				values[i] = "NULL"
			case bool:
				values[i] = fmt.Sprintf("%t", v)
			case []byte:
				values[i] = fmt.Sprintf("'%s'", escapeSingleQuotes(string(v)))
			default:
				values[i] = fmt.Sprintf("'%v'", v)
			}
		}

		stmt := fmt.Sprintf("-- Insert for %s\nINSERT INTO \"%s\" (%s) VALUES (%s);\n\n", table, table, strings.Join(cols, ", "), strings.Join(values, ", "))
		fmt.Print(stmt)
		if file != nil {
			file.WriteString(stmt)
		}
	}
}

// escapeSingleQuotes escapes single quotes for SQL compatibility
func escapeSingleQuotes(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}