package store

import (
	"context"
	"database/sql"

	"seduhpoin/internal/models"
)

// DaftarProduk mengembalikan produk aktif, opsional disaring dengan kata kunci.
func (s *Store) DaftarProduk(ctx context.Context, keyword, kategori string) ([]models.Produk, error) {
	query := `SELECT id, kode, nama, kategori, harga, deleted_at FROM produk WHERE deleted_at IS NULL`
	args := []any{}
	if keyword != "" {
		query += ` AND (kode LIKE ? OR nama LIKE ?)`
		like := "%" + keyword + "%"
		args = append(args, like, like)
	}
	if kategori != "" {
		query += ` AND kategori = ?`
		args = append(args, kategori)
	}
	query += ` ORDER BY kategori, nama`

	rows, err := s.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return ScanRows(rows, func(r *Rows) (models.Produk, error) {
		var p models.Produk
		var deleted sql.NullTime
		err := r.Scan(&p.ID, &p.Kode, &p.Nama, &p.Kategori, &p.Harga, &deleted)
		if deleted.Valid {
			t := deleted.Time
			p.DeletedAt = &t
		}
		return p, err
	})
}

// AmbilProduk mengambil satu produk berdasarkan id.
func (s *Store) AmbilProduk(ctx context.Context, id int64) (models.Produk, error) {
	var p models.Produk
	var deleted sql.NullTime
	err := s.DB.QueryRowContext(ctx,
		`SELECT id, kode, nama, kategori, harga, deleted_at FROM produk WHERE id = ?`, id).
		Scan(&p.ID, &p.Kode, &p.Nama, &p.Kategori, &p.Harga, &deleted)
	if deleted.Valid {
		t := deleted.Time
		p.DeletedAt = &t
	}
	return p, err
}

// SimpanProduk menyimpan produk baru dan mengembalikan id-nya.
func (s *Store) SimpanProduk(ctx context.Context, p models.Produk) (int64, error) {
	res, err := s.Exec(ctx,
		`INSERT INTO produk (kode, nama, kategori, harga) VALUES (?, ?, ?, ?)`,
		p.Kode, p.Nama, p.Kategori, p.Harga)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UbahProduk memperbarui data produk.
func (s *Store) UbahProduk(ctx context.Context, p models.Produk) error {
	_, err := s.Exec(ctx,
		`UPDATE produk SET kode = ?, nama = ?, kategori = ?, harga = ? WHERE id = ?`,
		p.Kode, p.Nama, p.Kategori, p.Harga, p.ID)
	return err
}

// HapusProduk melakukan soft delete (data tidak dihapus dari tabel).
func (s *Store) HapusProduk(ctx context.Context, id int64) error {
	_, err := s.Exec(ctx, `UPDATE produk SET deleted_at = NOW() WHERE id = ?`, id)
	return err
}

// KodeProdukDipakai memeriksa apakah kode produk sudah dipakai.
func (s *Store) KodeProdukDipakai(ctx context.Context, kode string, kecualiID int64) (bool, error) {
	var n int
	err := s.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM produk WHERE kode = ? AND id <> ?`, kode, kecualiID).Scan(&n)
	return n > 0, err
}

// DaftarKategori mengembalikan daftar kategori produk yang unik.
func (s *Store) DaftarKategori(ctx context.Context) ([]string, error) {
	rows, err := s.Query(ctx,
		`SELECT DISTINCT kategori FROM produk WHERE deleted_at IS NULL ORDER BY kategori`)
	if err != nil {
		return nil, err
	}
	return ScanRows(rows, func(r *Rows) (string, error) {
		var k string
		err := r.Scan(&k)
		return k, err
	})
}
