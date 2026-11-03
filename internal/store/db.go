// Package store berisi akses basis data (query SQL mentah, tanpa ORM).
package store

import (
	"context"
	"database/sql"
	"fmt"
)

// Store membungkus *sql.DB agar handler cukup memakai satu objek akses data.
type Store struct {
	DB *sql.DB
}

// New membuat Store baru dari koneksi database.
func New(db *sql.DB) *Store {
	return &Store{DB: db}
}

// Ping memeriksa koneksi ke database.
func (s *Store) Ping(ctx context.Context) error {
	return s.DB.PingContext(ctx)
}

// Begin membuka transaksi database (sql.Tx) baru.
func (s *Store) Begin(ctx context.Context) (*sql.Tx, error) {
	return s.DB.BeginTx(ctx, nil)
}

// Rows adalah pembungkus *sql.Rows agar defer Close mudah dan aman.
type Rows struct {
	*sql.Rows
}

// Query menjalankan query dan mengembalikan Rows.
func (s *Store) Query(ctx context.Context, query string, args ...any) (*Rows, error) {
	r, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query gagal: %w", err)
	}
	return &Rows{Rows: r}, nil
}

// QueryTx menjalankan query di dalam transaksi.
func (s *Store) QueryTx(ctx context.Context, tx *sql.Tx, query string, args ...any) (*Rows, error) {
	r, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query gagal: %w", err)
	}
	return &Rows{Rows: r}, nil
}

// Exec menjalankan perintah yang mengubah data.
func (s *Store) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return s.DB.ExecContext(ctx, query, args...)
}

// ExecTx menjalankan perintah yang mengubah data di dalam transaksi.
func (s *Store) ExecTx(ctx context.Context, tx *sql.Tx, query string, args ...any) (sql.Result, error) {
	return tx.ExecContext(ctx, query, args...)
}

// QueryRowBaru menjalankan query satu baris di dalam transaksi.
func (s *Store) QueryRowBaru(ctx context.Context, tx *sql.Tx, query string, args ...any) *sql.Row {
	return tx.QueryRowContext(ctx, query, args...)
}

// ScanRows adalah helper generik untuk memindai seluruh baris hasil query
// menjadi slice. Fungsi scan harus mengisi satu baris ke variabel output.
func ScanRows[T any](rows *Rows, scan func(*Rows) (T, error)) ([]T, error) {
	defer rows.Close()
	var hasil []T
	for rows.Next() {
		item, err := scan(rows)
		if err != nil {
			return nil, err
		}
		hasil = append(hasil, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return hasil, nil
}
