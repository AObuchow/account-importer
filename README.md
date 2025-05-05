# User Data Dumper

This utility generates PostgreSQL `INSERT` statements for a given Timehop `user_id` or `account_id`, allowing you to recreate a user's related database records locally. It supports exporting to `.sql` files and runs in read-only mode for safety.

---

## 🔧 Environment Setup

The script uses environment variables to connect to your Postgres database. You can define them in a `.env` file or export them directly.

### `.env` Example

```env
DB_HOST=localhost
DB_PORT=5432
DB_USER=timehop
DB_PASS=timehop
DB_NAME=development
DB_SSLMODE=disable
```

You can then run the program using:

```bash
env $(cat .env | xargs) go run main.go <user_id>
```

---

## 🚀 Usage

```bash
go run main.go [flags] <user_id>
```

If you don't use any flags, the positional argument is treated as a `user_id`.

---

## Flags

| Flag            | Description                                                           |
| --------------- | --------------------------------------------------------------------- |
| `--user_id`     | Explicitly specify a `user_id` (takes precedence over positional arg) |
| `--account_id`  | Specify an `account_id` (used to look up the `user_id`)               |
| `--output=true` | Write output to a `.sql` file named `user_<user_id>_dump.sql`         |

️⚠️ You must not provide both `--user_id` and `--account_id` at the same time.

---

## 💡 Examples

### Using a positional `user_id`:

```bash
env $(cat .env | xargs) go run main.go 12345678
```

### Using `--account_id` to look up the user:

```bash
env $(cat .env | xargs) go run main.go --account_id=36832707
```

### Outputting to a `.sql` file:

```bash
env $(cat .env | xargs) go run main.go  --output=true --user_id=12345678
```
