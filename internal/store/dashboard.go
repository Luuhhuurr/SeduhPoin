package store

import (
	"context"
	"time"

	"seduhpoin/internal/models"
)

// Dashboard berisi ringkasan angka untuk halaman depan.
type Dashboard struct {
	JumlahCabang           int
	JumlahProduk           int
	JumlahMember           int
	JumlahTransaksiHariIni int
	OmzetHariIni           int64
	JumlahGold             int
	JumlahSilver           int
	TransaksiTerakhir      []models.Transaksi
}

// AmbilDashboard gathers ringkasan untuk halaman dashboard.
func (s *Store) AmbilDashboard(ctx context.Context, hariIni time.Time) (Dashboard, error) {
	var d Dashboard
	mulai := hariIni
	akhir := hariIni.Add(24 * time.Hour)

	hitung := func(query string, args ...any) (int, error) {
		var n int
		err := s.DB.QueryRowContext(ctx, query, args...).Scan(&n)
		return n, err
	}

	var err error
	if d.JumlahCabang, err = hitung(`SELECT COUNT(*) FROM cabang WHERE deleted_at IS NULL`); err != nil {
		return d, err
	}
	if d.JumlahProduk, err = hitung(`SELECT COUNT(*) FROM produk WHERE deleted_at IS NULL`); err != nil {
		return d, err
	}
	if d.JumlahMember, err = hitung(`SELECT COUNT(*) FROM member WHERE deleted_at IS NULL`); err != nil {
		return d, err
	}
	if d.JumlahTransaksiHariIni, err = hitung(
		`SELECT COUNT(*) FROM transaksi WHERE status='SELESAI' AND tanggal_transaksi >= ? AND tanggal_transaksi < ?`,
		mulai, akhir); err != nil {
		return d, err
	}
	if err := s.DB.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(total_bayar),0) FROM transaksi WHERE status='SELESAI' AND tanggal_transaksi >= ? AND tanggal_transaksi < ?`,
		mulai, akhir).Scan(&d.OmzetHariIni); err != nil {
		return d, err
	}

	// Tingkat member dihitung on-the-fly dari belanja 3 bulan terakhir.
	batas := hariIni.AddDate(0, -3, 0)
	if d.JumlahGold, err = hitung(`
		SELECT COUNT(*) FROM member m
		WHERE m.deleted_at IS NULL AND (
			SELECT COALESCE(SUM(t.total_setelah_promo),0) FROM transaksi t
			WHERE t.member_id = m.id AND t.status = 'SELESAI' AND t.tanggal_transaksi >= ?
		) >= 1500000`, batas); err != nil {
		return d, err
	}
	if d.JumlahSilver, err = hitung(`
		SELECT COUNT(*) FROM member m
		WHERE m.deleted_at IS NULL AND (
			SELECT COALESCE(SUM(t.total_setelah_promo),0) FROM transaksi t
			WHERE t.member_id = m.id AND t.status = 'SELESAI' AND t.tanggal_transaksi >= ?
		) < 1500000`, batas); err != nil {
		return d, err
	}

	// 5 transaksi terakhir
	rows, err := s.Query(ctx, `
		SELECT t.id, t.kode, t.tanggal_transaksi, t.total_bayar, t.status,
		       c.nama, COALESCE(m.nama, '-')
		FROM transaksi t
		JOIN cabang c ON c.id = t.cabang_id
		LEFT JOIN member m ON m.id = t.member_id
		ORDER BY t.tanggal_transaksi DESC, t.id DESC
		LIMIT 5`)
	if err != nil {
		return d, err
	}
	d.TransaksiTerakhir, err = ScanRows(rows, func(r *Rows) (models.Transaksi, error) {
		var t models.Transaksi
		var tanggal time.Time
		err := r.Scan(&t.ID, &t.Kode, &tanggal, &t.TotalBayar, &t.Status, &t.CabangNama, &t.MemberNama)
		t.TanggalTransaksi = tanggal
		return t, err
	})
	if err != nil {
		return d, err
	}
	return d, nil
}
