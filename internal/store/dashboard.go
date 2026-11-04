package store

// Catatan arsitektur (pola yang dipakai di seluruh package store):
//   - SELECT         : s.Query / s.DB.QueryRowContext
//   - INSERT/UPDATE  : s.Exec
//   - ScanRows       : pembantu memindai seluruh baris ke slice
//
// Tulis-manyok (transaksi & pembatalan) selalu dibungkus s.Begin (sql.Tx)
// agar bersifat atomik.

import (
	"context"
	"time"

	"seduhpoin/internal/models"
)

// Dashboard berisi ringkasan angka untuk halaman depan.
//
// Semua angka dihitung ulang dari tabel (bukan kolom tersimpan), sehingga
// selalu sesuai dengan data terkini.
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
//
// Aturan perhitungan ringkasan:
//   - Jumlah cabang/produk/member: hanya yang belum di-soft delete (deleted_at IS NULL).
//   - Transaksi hari ini & omzet: hanya status SELESAI (transaksi BATAL dikeluarkan).
//   - Jumlah Gold/Silver: dihitung on-the-fly dari belanja 3 bulan terakhir.
func (s *Store) AmbilDashboard(ctx context.Context, hariIni time.Time) (Dashboard, error) {
	var d Dashboard
	// Rentang "hari ini" dibatasi [mulai, akhir) agar aman dari data antar hari.
	mulai := hariIni
	akhir := hariIni.Add(24 * time.Hour)

	// Pembantu untuk query COUNT(*) yang hanya butuh satu angka.
	hitung := func(query string, args ...any) (int, error) {
		var n int
		err := s.DB.QueryRowContext(ctx, query, args...).Scan(&n)
		return n, err
	}

	var err error
	// Master data: baris yang di-soft delete tidak dihitung.
	if d.JumlahCabang, err = hitung(`SELECT COUNT(*) FROM cabang WHERE deleted_at IS NULL`); err != nil {
		return d, err
	}
	if d.JumlahProduk, err = hitung(`SELECT COUNT(*) FROM produk WHERE deleted_at IS NULL`); err != nil {
		return d, err
	}
	if d.JumlahMember, err = hitung(`SELECT COUNT(*) FROM member WHERE deleted_at IS NULL`); err != nil {
		return d, err
	}
	// Transaksi hari ini: hanya yang berstatus SELESAI.
	if d.JumlahTransaksiHariIni, err = hitung(
		`SELECT COUNT(*) FROM transaksi WHERE status='SELESAI' AND tanggal_transaksi >= ? AND tanggal_transaksi < ?`,
		mulai, akhir); err != nil {
		return d, err
	}
	// Omzet memakai total_bayar (sudah termasuk penukaran poin).
	if err := s.DB.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(total_bayar),0) FROM transaksi WHERE status='SELESAI' AND tanggal_transaksi >= ? AND tanggal_transaksi < ?`,
		mulai, akhir).Scan(&d.OmzetHariIni); err != nil {
		return d, err
	}

	// Tingkat member dihitung on-the-fly dari belanja 3 bulan terakhir.
	// Basic: total_setelah_promo (bukan total_bayar) dan ambang Gold Rp1.500.000.
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

	// 5 transaksi terakhir (termasuk yang BATAL agar kasir melihat riwayatnya).
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
