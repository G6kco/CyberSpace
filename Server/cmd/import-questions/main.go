// Command import-questions loads the Level 1 question bank from the reference
// MySQL dump into the CyberSpace schema:
//
//	go run ./cmd/import-questions -dump ../Reference/cyber.sql -admin admin@example.edu
//
// Flags are stored only as HMACs under ANSWER_HMAC_KEY; the dump's plaintext
// answers never reach the database. Run it once, after migrations: a second
// run finds the assessment and changes nothing.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/G6kco/CyberSpace/internal/config"
	"github.com/G6kco/CyberSpace/internal/database"
	"github.com/G6kco/CyberSpace/internal/evaluation"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "import failed: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("import-questions", flag.ContinueOnError)
	dumpPath := flags.String("dump", "", "path to the reference mysqldump file")
	adminEmail := flags.String("admin", "", "email of the active admin recorded as author")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *dumpPath == "" || *adminEmail == "" {
		return errors.New("usage: import-questions -dump <cyber.sql> -admin <admin email>")
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	if len(cfg.AnswerKey) < evaluation.MinKeyLength {
		return fmt.Errorf("ANSWER_HMAC_KEY: %w", evaluation.ErrShortKey)
	}

	file, err := os.Open(*dumpPath)
	if err != nil {
		return err
	}
	defer file.Close()

	tables, err := readDump(file)
	if err != nil {
		return fmt.Errorf("read dump: %w", err)
	}
	bank, err := buildBank(tables)
	if err != nil {
		return fmt.Errorf("check question bank: %w", err)
	}

	db, err := database.NewMySQL(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	result, err := importBank(context.Background(), db, []byte(cfg.AnswerKey), *adminEmail, bank)
	if err != nil {
		return err
	}
	fmt.Printf("imported %d questions with %d flags and %d lab images into %s\n",
		result.questions, result.flags, result.images, assessmentTitle)
	return nil
}
