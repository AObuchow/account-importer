package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

var (
	outputToFile  = flag.Bool("output", false, "Write SQL to a .sql file")
	userIDFlag    = flag.String("user_id", "", "Specify a user_id directly")
	accountIDFlag = flag.String("account_id", "", "Specify an account_id to look up user_id")
)

// Entry point of the script
func main() {
	flag.Parse()

	if (*userIDFlag != "" && *accountIDFlag != "") || (len(flag.Args()) > 0 && (*userIDFlag != "" || *accountIDFlag != "")) {
		log.Fatal("Provide either --user_id or --account_id, not both. Or pass a positional argument (assumed to be user_id by default).")
	}

	var userID string
	var err error

	switch {
	case *userIDFlag != "":
		userID = *userIDFlag
		if !userExists(userID) {
			log.Fatalf("Could not find user_id %s in the database.", userID)
		}
	case *accountIDFlag != "":
		userID, err = getUserIDFromAccountID(*accountIDFlag)
		if err != nil {
			log.Fatalf("Could not find user_id associated with account_id %s: %v", *accountIDFlag, err)
		}
	case len(flag.Args()) == 1:
		userID = flag.Args()[0]
		if !userExists(userID) {
			log.Fatalf("Could not find user_id %s in the database.", userID)
		}
	default:
		log.Fatal("Usage: go run main.go [--output=true] [--user_id=<id> | --account_id=<id>] <user_id>")
	}

	db := connectDB()
	defer db.Close()

	queries := map[string]string{
		"accounts":         "SELECT * FROM accounts WHERE user_id = $1",
		"users":            "SELECT * FROM users WHERE id = $1",
		"app_auth_tokens":  "SELECT * FROM app_auth_tokens WHERE user_id = $1",
		"user_identities":  "SELECT * FROM user_identities WHERE user_id = $1",
		"user_preferences": "SELECT * FROM user_preferences WHERE user_id = $1",
	}

	var output strings.Builder
	sortedKeys := make([]string, 0, len(queries))
	for k := range queries {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	for _, table := range sortedKeys {
		dump, err := generateInsertStatements(db, queries[table], table, userID)
		if err != nil {
			log.Printf("Warning: Skipping table %s due to error: %v", table, err)
			continue
		}
		output.WriteString(fmt.Sprintf("-- Insert for %s\n", table))
		output.WriteString(dump)
		output.WriteString("\n")
	}

	fmt.Print(output.String())

	if *outputToFile {
		filename := fmt.Sprintf("user_%s_dump.sql", userID)
		err := os.WriteFile(filename, []byte(output.String()), 0644)
		if err != nil {
			log.Fatalf("Failed to write to file: %v", err)
		}
		fmt.Printf("Wrote SQL output to %s\n", filename)
	}
}

// Retrieves user_id from an account_id by querying the DB
func getUserIDFromAccountID(accountID string) (string, error) {
	db := connectDB()
	defer db.Close()

	var userID string
	err := db.QueryRow("SELECT user_id FROM accounts WHERE id = $1", accountID).Scan(&userID)
	if err != nil {
		return "", err
	}
	return userID, nil
}

// Verifies if the user_id exists in the users table
func userExists(userID string) bool {
	db := connectDB()
	defer db.Close()

	var id string
	err := db.QueryRow("SELECT id FROM users WHERE id = $1", userID).Scan(&id)
	if err != nil {
		fmt.Println(err)
	}
	return err == nil
}

// Connects to Postgres DB using env vars
// Connects to Postgres DB using env vars
func connectDB() *sql.DB {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		host := os.Getenv("DB_HOST")
		port := os.Getenv("DB_PORT")
		user := os.Getenv("DB_USER")
		pass := os.Getenv("DB_PASS")
		dbname := os.Getenv("DB_NAME")
		sslmode := os.Getenv("DB_SSLMODE")
		if sslmode == "" {
			sslmode = "disable"
		}
		if host == "" || port == "" || user == "" || dbname == "" {
			log.Fatal("Missing required DB environment variables. Either set DATABASE_URL or DB_HOST, DB_PORT, DB_USER, DB_NAME (and optionally DB_PASS and DB_SSLMODE)")
		}

		u := &url.URL{
			Scheme: "postgres",
			User:   url.UserPassword(user, pass),
			Host:   fmt.Sprintf("%s:%s", host, port),
			Path:   dbname,
			RawQuery: url.Values{
				"sslmode":                       []string{sslmode},
				"default_transaction_read_only": []string{"true"},
			}.Encode(),
		}
		dbURL = u.String()
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}

	return db
}

// Generates INSERT statements for a table given the user_id parameter
func generateInsertStatements(db *sql.DB, query string, table string, param string) (string, error) {
	rows, err := db.Query(query, param)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return "", err
	}

	var out strings.Builder
	for rows.Next() {
		rawResult := make([]interface{}, len(cols))
		dest := make([]interface{}, len(cols))
		for i := range rawResult {
			dest[i] = &rawResult[i]
		}

		if err := rows.Scan(dest...); err != nil {
			return "", err
		}

		values := make([]string, len(cols))
		for i, raw := range rawResult {
			switch val := raw.(type) {
			case nil:
				values[i] = "NULL"
			case bool:
				values[i] = fmt.Sprintf("%t", val)
			case []byte:
				values[i] = fmt.Sprintf("'%s'", escapeSingleQuotes(string(val)))
			case time.Time:
				values[i] = fmt.Sprintf("'%s'", val.UTC().Format("2006-01-02T15:04:05Z"))
			default:
				values[i] = fmt.Sprintf("'%v'", val)
			}
		}

		out.WriteString(fmt.Sprintf("INSERT INTO \"%s\" (%s) VALUES (%s);\n", table, strings.Join(cols, ", "), strings.Join(values, ", ")))
	}

	return out.String(), nil
}

// Escapes single quotes in string values for SQL safety
func escapeSingleQuotes(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
