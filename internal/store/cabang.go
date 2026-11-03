package store

import (
	"context"
	"database/sql"

	"seduhpoin/internal/models"
)

// DaftarCabang mengembalikan seluruh cabang yang belum dihapus.
func (s *Store) DaftarCabang(ctx context.Context, keyword string) ([]models.Cabang, error) {
	query := `SELECT id, kode, nama, alamat, telepon, deleted_at FROM cabang WHERE deleted_at IS NULL`
	args := []any{}
	if keyword != "" {
		query += ` AND (kode LIKE ? OR nama LIKE ?)`
		like := "%" + keyword + "%"
		args = append(args, like, like)
	}
	query += ` ORDER BY kode`

	rows, err := s.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return ScanRows(rows, func(r *Rows) (models.Cabang, error) {
		var c models.Cabang
		var deleted sql.NullTime
		err := r.Scan(&c.ID, &c.Kode, &c.Nama, &c.Alamat, &c.Telepon, &deleted)
		if deleted.Valid {
			t := deleted.Time
			c.DeletedAt = &t
		}
		return c, err
	})
}

// AmbilCabang mengambil satu cabang berdasarkan id.
func (s *Store) AmbilCabang(ctx context.Context, id int64) (models.Cabang, error) {
	var c models.Cabang
	var deleted sql.NullTime
	err := s.DB.QueryRowContext(ctx,
		`SELECT id, kode, nama, alamat, telepon, deleted_at FROM cabang WHERE id = ?`, id).
		Scan(&c.ID, &c.Kode, &c.Nama, &c.Alamat, &c.Telepon, &deleted)
	if deleted.Valid {
		t := deleted.Time
		c.DeletedAt = &t
	}
	return c, err
}

// SimpanCabang menyimpan cabang baru dan mengembalikan id-nya.
func (s *Store) SimpanCabang(ctx context.Context, c models.Cabang) (int64, error) {
	res, err := s.Exec(ctx,
		`INSERT INTO cabang (kode, nama, alamat, telepon) VALUES (?, ?, ?, ?)`,
		c.Kode, c.Nama, c.Alamat, c.Telepon)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UbahCabang memperbarui data cabang.
func (s *Store) UbahCabang(ctx context.Context, c models.Cabang) error {
	_, err := s.Exec(ctx,
		`UPDATE cabang SET kode = ?, nama = ?, alamat = ?, telepon = ? WHERE id = ?`,
		c.Kode, c.Nama, c.Alamat, c.Telepon, c.ID)
	return err
}

// HapusCabang melakukan soft delete (data tidak dihapus dari tabel).
func (s *Store) HapusCabang(ctx context.Context, id int64) error {
	_, err := s.Exec(ctx, `UPDATE cabang SET deleted_at = NOW() WHERE id = ?`, id)
	return err
}

// KodeCabangDipakai memeriksa apakah kode cabang sudah dipakai.
func (s *Store) KodeCabangDipakai(ctx context.Context, kode string, kecualiID int64) (bool, error) {
	var n int
	err := s.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM cabang WHERE kode = ? AND id <> ?`, kode, kecualiID).Scan(&n)
	return n > 0, err
}
