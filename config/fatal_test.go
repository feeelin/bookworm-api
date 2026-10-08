package config_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/feeelin/bookworm-api/config"
)

// Both failure paths in config end in log.Fatal, which exits the process. The
// tests below re-run this binary as a child so the exit can be observed:
// the child runs the case named by crashCaseEnv and the parent asserts on it.
const crashCaseEnv = "BOOKWORM_CONFIG_CRASH_CASE"

func TestMain(m *testing.M) {
	switch os.Getenv(crashCaseEnv) {
	case "connect":
		// A directory cannot be opened as a SQLite database file.
		if err := os.Mkdir("database.db", 0o755); err != nil {
			os.Exit(2)
		}
		config.ConnectDatabase()
		os.Exit(0)
	case "migrate":
		db := newBrokenDB()
		config.RunMigrations(db)
		os.Exit(0)
	}

	os.Exit(m.Run())
}

func runCrashCase(t *testing.T, name string) (int, string) {
	t.Helper()

	cmd := exec.Command(os.Args[0], "-test.run=TestMain")
	cmd.Env = append(os.Environ(), crashCaseEnv+"="+name)
	cmd.Dir = t.TempDir()

	output, err := cmd.CombinedOutput()

	exitErr, ok := err.(*exec.ExitError)
	switch {
	case err == nil:
		return 0, string(output)
	case ok:
		return exitErr.ExitCode(), string(output)
	default:
		t.Fatalf("run child process: %v", err)
		return 0, ""
	}
}

func TestConnectDatabaseFailureExits(t *testing.T) {
	code, output := runCrashCase(t, "connect")

	if code == 0 {
		t.Fatalf("child exited successfully, want a non-zero code (output: %s)", output)
	}
	if !strings.Contains(output, "Database connection failed.") {
		t.Errorf("output = %q, want it to mention the connection failure", output)
	}
}

func TestRunMigrationsFailureExits(t *testing.T) {
	code, output := runCrashCase(t, "migrate")

	if code == 0 {
		t.Fatalf("child exited successfully, want a non-zero code (output: %s)", output)
	}
	if !strings.Contains(output, "Migration failed.") {
		t.Errorf("output = %q, want it to mention the migration failure", output)
	}
}

// newBrokenDB opens a database and closes its connection pool, so every
// statement through it fails. It runs inside the child process, where the
// testing.T-based helpers are not available.
func newBrokenDB() *gorm.DB {
	db, err := gorm.Open(sqlite.Open("broken.db"), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		os.Exit(2)
	}

	sqlDB, err := db.DB()
	if err != nil {
		os.Exit(2)
	}
	if err := sqlDB.Close(); err != nil {
		os.Exit(2)
	}

	return db
}
