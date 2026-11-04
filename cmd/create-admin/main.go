package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"adelson-backend/internal/config"
	"adelson-backend/internal/store"

	"golang.org/x/crypto/bcrypt"
)

const demoAgentEmail = "agente@adelson.example"

func main() {
	email := flag.String("email", os.Getenv("ADMIN_EMAIL"), "correo del administrador")
	password := flag.String("password", os.Getenv("ADMIN_PASSWORD"), "contraseña inicial")
	firstName := flag.String("nombre", valueOrDefault("ADMIN_NOMBRE", "Marcelo"), "nombre")
	lastName := flag.String("apellido", valueOrDefault("ADMIN_APELLIDO", "Benato"), "apellido")
	dni := flag.String("dni", os.Getenv("ADMIN_DNI"), "DNI o identificador interno")
	reassignDemo := flag.Bool("reassign-demo", true, "reasignar inmuebles y consultas del agente demo")
	deactivateDemo := flag.Bool("deactivate-demo", true, "desactivar el agente demo")
	flag.Parse()

	if strings.TrimSpace(*email) == "" || strings.TrimSpace(*password) == "" {
		fail("ADMIN_EMAIL y ADMIN_PASSWORD son obligatorios")
	}
	if utf8.RuneCountInString(*dni) > 20 {
		fail("el DNI no puede superar los 20 caracteres")
	}
	cfg, err := config.Load()
	if err != nil {
		fail("config: %v", err)
	}
	db, err := store.Open(cfg.DatabaseURL)
	if err != nil {
		fail("open database: %v", err)
	}
	defer db.Close()
	if db.DB == nil {
		fail("DATABASE_URL no está configurada")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
	if err != nil {
		fail("hash password: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		fail("begin: %v", err)
	}
	defer tx.Rollback()

	var adminID string
	err = tx.QueryRowContext(ctx, `SELECT id FROM personas WHERE lower(correo_electronico) = lower($1)`, *email).Scan(&adminID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		adminDNI := *dni
		if adminDNI == "" {
			adminDNI = fmt.Sprintf("ADMIN-%d", time.Now().UnixMilli())
		}
		err = tx.QueryRowContext(ctx, `INSERT INTO personas (nombre, apellido, dni, correo_electronico, contrasena_hash) VALUES ($1, $2, $3, $4, $5) RETURNING id`, *firstName, *lastName, adminDNI, *email, string(hash)).Scan(&adminID)
		if err != nil {
			fail("crear persona: %v", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO usuarios (id, rol, estado_operativo) VALUES ($1, 'Administrador', 'Activo')`, adminID); err != nil {
			fail("crear usuario: %v", err)
		}
		fmt.Printf("Administrador creado: %s (%s)\n", *email, adminID)
	case err != nil:
		fail("buscar administrador: %v", err)
	default:
		if _, err := tx.ExecContext(ctx, `UPDATE personas SET nombre = $2, apellido = $3, dni = COALESCE(NULLIF($4, ''), dni), contrasena_hash = $5, deleted_at = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = $1`, adminID, *firstName, *lastName, *dni, string(hash)); err != nil {
			fail("actualizar persona: %v", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO usuarios (id, rol, estado_operativo) VALUES ($1, 'Administrador', 'Activo') ON CONFLICT (id) DO UPDATE SET rol = 'Administrador', estado_operativo = 'Activo'`, adminID); err != nil {
			fail("actualizar usuario: %v", err)
		}
		fmt.Printf("Administrador actualizado: %s (%s)\n", *email, adminID)
	}

	if *reassignDemo {
		if _, err := tx.ExecContext(ctx, `UPDATE inmuebles SET agente_id = $1, updated_at = CURRENT_TIMESTAMP WHERE agente_id = (SELECT id FROM personas WHERE correo_electronico = $2)`, adminID, demoAgentEmail); err != nil {
			fail("reasignar inmuebles: %v", err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE consultas SET agente_asignado_id = $1 WHERE agente_asignado_id IS NULL`, adminID); err != nil {
			fail("reasignar consultas: %v", err)
		}
	}
	if *deactivateDemo {
		if _, err := tx.ExecContext(ctx, `UPDATE usuarios SET estado_operativo = 'Inactivo' WHERE id = (SELECT id FROM personas WHERE correo_electronico = $1)`, demoAgentEmail); err != nil {
			fail("desactivar agente demo: %v", err)
		}
	}

	if err := tx.Commit(); err != nil {
		fail("commit: %v", err)
	}
	fmt.Println("Listo. Inmuebles demo conservados y reasignados.")
}

func valueOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func fail(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}
