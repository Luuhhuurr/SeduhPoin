package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"seduhpoin/internal/models"
)

// ErrMemberTidakDitemukan dikembalikan saat nomor HP tidak terdaftar.
var ErrMemberTidakDitemukan = errors.New("member dengan nomor HP tersebut tidak ditemukan")

// DaftarMember mengembalikan seluruh member aktif.
func (s *Store) DaftarMember(ctx context.Context, keyword string) ([]models.Member, error) {
	query := `SELECT id, no_hp, nama, tgl_daftar, deleted_at FROM member WHERE deleted_at IS NULL`
	args := []any{}
	if keyword != "" {
		query += ` AND (no_hp LIKE ? OR nama LIKE ?)`
		like := "%" + keyword + "%"
		args = append(args, like, like)
	}
	query += ` ORDER BY nama`

	rows, err := s.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return ScanRows(rows, func(r *Rows) (models.Member, error) {
		var m models.Member
		var deleted sql.NullTime
		err := r.Scan(&m.ID, &m.NoHP, &m.Nama, &m.TglDaftar, &deleted)
		if deleted.Valid {
			t := deleted.Time
			m.DeletedAt = &t
		}
		return m, err
	})
}

// AmbilMember mengambil satu member berdasarkan id.
func (s *Store) AmbilMember(ctx context.Context, id int64) (models.Member, error) {
	var m models.Member
	var deleted sql.NullTime
	err := s.DB.QueryRowContext(ctx,
		`SELECT id, no_hp, nama, tgl_daftar, deleted_at FROM member WHERE id = ?`, id).
		Scan(&m.ID, &m.NoHP, &m.Nama, &m.TglDaftar, &deleted)
	if deleted.Valid {
		t := deleted.Time
		m.DeletedAt = &t
	}
	return m, err
}

// CariMemberByNoHP mencari member terdaftar berdasarkan nomor HP (dipakai di form transaksi).
func (s *Store) CariMemberByNoHP(ctx context.Context, noHP string) (models.Member, error) {
	var m models.Member
	var deleted sql.NullTime
	err := s.DB.QueryRowContext(ctx,
		`SELECT id, no_hp, nama, tgl_daftar, deleted_at FROM member
		 WHERE no_hp = ? AND deleted_at IS NULL`, noHP).
		Scan(&m.ID, &m.NoHP, &m.Nama, &m.TglDaftar, &deleted)
	if errors.Is(err, sql.ErrNoRows) {
		return m, ErrMemberTidakDitemukan
	}
	return m, err
}

// SimpanMember menyimpan member baru dan mengembalikan id-nya.
func (s *Store) SimpanMember(ctx context.Context, m models.Member) (int64, error) {
	res, err := s.Exec(ctx,
		`INSERT INTO member (no_hp, nama, tgl_daftar) VALUES (?, ?, ?)`,
		m.NoHP, m.Nama, m.TglDaftar)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UbahMember memperbarui data member.
func (s *Store) UbahMember(ctx context.Context, m models.Member) error {
	_, err := s.Exec(ctx,
		`UPDATE member SET no_hp = ?, nama = ?, tgl_daftar = ? WHERE id = ?`,
		m.NoHP, m.Nama, m.TglDaftar, m.ID)
	return err
}

// HapusMember melakukan soft delete (data tidak dihapus dari tabel).
func (s *Store) HapusMember(ctx context.Context, id int64) error {
	_, err := s.Exec(ctx, `UPDATE member SET deleted_at = NOW() WHERE id = ?`, id)
	return err
}

// NoHPDipakai memeriksa apakah nomor HP sudah dipakai member lain.
func (s *Store) NoHPDipakai(ctx context.Context, noHP string, kecualiID int64) (bool, error) {
	var n int
	err := s.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM member WHERE no_hp = ? AND id <> ?`, noHP, kecualiID).Scan(&n)
	return n > 0, err
}

// TglDaftarDefault mengembalikan tanggal hari ini sebagai default pendaftaran member.
func TglDaftarDefault() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

// Belanja3Bulan menjumlahkan nilai transaksi selesai milik member dalam rentang tertentu.
// Nilai ini menjadi dasar perhitungan tingkat member (Silver/Gold).
func (s *Store) Belanja3Bulan(ctx context.Context, memberID int64, batas time.Time) (int64, error) {
	var total int64
	err := s.DB.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(total_setelah_promo), 0) FROM transaksi
		WHERE member_id = ? AND status = 'SELESAI' AND tanggal_transaksi >= ?`,
		memberID, batas).Scan(&total)
	return total, err
}

// Belanja3BulanSemuaMember menghitung belanja 3 bulan terakhir untuk seluruh member
// dan mengembalikan peta memberID -> total belanja.
func (s *Store) Belanja3BulanSemuaMember(ctx context.Context, batas time.Time) (map[int64]int64, error) {
	rows, err := s.Query(ctx, `
		SELECT member_id, COALESCE(SUM(total_setelah_promo), 0) FROM transaksi
		WHERE status = 'SELESAI' AND tanggal_transaksi >= ? AND member_id IS NOT NULL
		GROUP BY member_id`, batas)
	if err != nil {
		return nil, err
	}
	hasil, err := ScanRows(rows, func(r *Rows) (struct {
		ID    int64
		Total int64
	}, error) {
		var x struct {
			ID    int64
			Total int64
		}
		err := r.Scan(&x.ID, &x.Total)
		return x, err
	})
	if err != nil {
		return nil, err
	}
	peta := make(map[int64]int64, len(hasil))
	for _, x := range hasil {
		peta[x.ID] = x.Total
	}
	return peta, nil
}
