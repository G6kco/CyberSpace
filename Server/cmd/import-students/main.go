package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/G6kco/CyberSpace/internal/config"
	"github.com/G6kco/CyberSpace/internal/database"
	"github.com/G6kco/CyberSpace/internal/students"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "import failed: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("import-students", flag.ContinueOnError)
	xlsxPath := flags.String("xlsx", "", "path to the students xlsx file")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *xlsxPath == "" {
		return errors.New("usage: import-students -xlsx <students.xlsx>")
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	records, err := students.ParseExcel(*xlsxPath)
	if err != nil {
		return fmt.Errorf("parse xlsx: %w", err)
	}

	db, err := database.NewMySQL(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	result, err := students.Import(context.Background(), db, records)
	if err != nil {
		return err
	}
	fmt.Printf("imported %d students (%d skipped, %d total rows)\n",
		result.Imported, result.Skipped, len(records))
	return nil
}
