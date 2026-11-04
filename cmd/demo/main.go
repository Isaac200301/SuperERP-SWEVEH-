// Command demo carga el taller ficticio de la demo, Servitaller El Pistón (HU-97, HU-98, ADR-0031).
//
//	go run ./cmd/demo seed    # lo carga; si el dueño ya tiene un taller activo, no hace nada
//	go run ./cmd/demo reset   # archiva la demo anterior y lo carga de nuevo (entre dos demos)
//
// Lee DATABASE_URL (rol sweveh_api), MIGRATIONS_DATABASE_URL (rol sweveh_migrator), STORAGE_DIR y los celulares de la
// demo (DEMO_CELULAR_DUENO, DEMO_CELULAR_MECANICO, DEMO_CELULAR_RECEPCION, DEMO_CELULAR_B, DEMO_CELULAR_D y, opcionales,
// DEMO_CELULAR_DIEGO, DEMO_CELULAR_MARTA, DEMO_CELULAR_CARLOS). Sin ellos usa números de prueba. Nunca en producción.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"sweveh/internal/demo"
	"sweveh/internal/platform/db"
	"sweveh/internal/platform/storage"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "demo:", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 2 || (os.Args[1] != "seed" && os.Args[1] != "reset") {
		return errors.New("uso: demo seed | demo reset")
	}
	if env := os.Getenv("APP_ENV"); env == "production" {
		return errors.New("el taller ficticio no se carga en producción")
	}
	apiURL, bootURL := os.Getenv("DATABASE_URL"), os.Getenv("MIGRATIONS_DATABASE_URL")
	if apiURL == "" || bootURL == "" {
		return errors.New("DATABASE_URL (sweveh_api) y MIGRATIONS_DATABASE_URL (sweveh_migrator) son obligatorias")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	api, err := db.NewPool(ctx, db.PoolConfig{URL: apiURL, MaxConns: 4})
	if err != nil {
		return fmt.Errorf("base (api): %w", err)
	}
	defer api.Close()
	boot, err := db.NewPool(ctx, db.PoolConfig{URL: bootURL, MaxConns: 2})
	if err != nil {
		return fmt.Errorf("base (migrator): %w", err)
	}
	defer boot.Close()
	dir := os.Getenv("STORAGE_DIR")
	if dir == "" {
		dir = ".data/fotos"
	}
	st, err := storage.NewLocal(dir)
	if err != nil {
		return err
	}
	phones := demo.DefaultPhones
	for env, dst := range map[string]*string{"DEMO_CELULAR_DUENO": &phones.Owner, "DEMO_CELULAR_MECANICO": &phones.Mechanic,
		"DEMO_CELULAR_RECEPCION": &phones.Reception, "DEMO_CELULAR_B": &phones.B, "DEMO_CELULAR_D": &phones.D,
		"DEMO_CELULAR_DIEGO": &phones.Diego, "DEMO_CELULAR_MARTA": &phones.Marta, "DEMO_CELULAR_CARLOS": &phones.Carlos} {
		if v := os.Getenv(env); v != "" {
			*dst = v
		}
	}
	start := time.Now()
	res, err := demo.Seed(ctx, demo.Deps{API: api, Bootstrap: boot, Storage: st, Phones: phones}, os.Args[1] == "reset")
	if err != nil {
		return err
	}
	for _, id := range res.Archived {
		fmt.Println("Demo anterior archivada:", id)
	}
	fmt.Printf("%s listo (%s) en %.1f s. Entra con el celular %s (dueño), %s (mecánico) o %s (recepción).\n",
		demo.Name, res.TallerID, time.Since(start).Seconds(), phones.Owner, phones.Mechanic, phones.Reception)
	return nil
}
