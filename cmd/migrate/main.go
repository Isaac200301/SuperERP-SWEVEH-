// Command migrate aplica las migraciones de db/migrations con el rol sweveh_migrator.
// Uso: migrate up | down | status   (lee MIGRATIONS_DATABASE_URL)
package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"sweveh/db/migrations"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
}

func run() error {
	cmd := "up"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	url := os.Getenv("MIGRATIONS_DATABASE_URL")
	if url == "" {
		return fmt.Errorf("MIGRATIONS_DATABASE_URL es obligatoria (rol sweveh_migrator)")
	}
	conn, err := sql.Open("pgx", url)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	p, err := goose.NewProvider(goose.DialectPostgres, conn, migrations.FS)
	if err != nil {
		return err
	}
	ctx := context.Background()
	switch cmd {
	case "up":
		res, err := p.Up(ctx)
		for _, r := range res {
			fmt.Println(r)
		}
		return err
	case "down":
		r, err := p.Down(ctx)
		if r != nil {
			fmt.Println(r)
		}
		return err
	case "status":
		st, err := p.Status(ctx)
		for _, s := range st {
			fmt.Printf("%-6s %s\n", s.State, s.Source.Path)
		}
		return err
	default:
		return fmt.Errorf("comando desconocido %q (up, down, status)", cmd)
	}
}
