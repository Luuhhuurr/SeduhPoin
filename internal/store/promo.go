package store

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	"seduhpoin/internal/models"
)

// ErrPromoSudahDipakai dikembalikan saat promo yang sudah pernah dipakai
// ingin dihapus. Promo tersebut hanya boleh dinonaktifkan.
var ErrPromoSudahDipakai = errors.New("promo sudah pernah dipakai pada transaksi, sehingga hanya dapat dinonaktifkan (tidak dapat dihapus)")

// DaftarPromo mengembalikan daftar promo (belum dihapus) beserta info voucher.
//
// Promo disimpan dengan soft delete (deleted_at), sehingga promo yang pernah
// dipakai tetap dapat dirujuk oleh transaksi lama.
func (s *Store) DaftarPromo(ctx context.Context, keyword, jenis string) ([]models.Promo, error) {
	query := `SELECT p.id, p.kode, p.nama, p.jenis, COALESCE(p.nilai_persen,0),
	                 COALESCE(p.nilai_nominal,0), COALESCE(p.min_belanja,0),
	                 p.tgl_mulai, p.tgl_selesai, p.jam_mulai, p.jam_selesai,
	                 COALESCE(p.cabang_ids,''), p.aktif, p.deleted_at
	          FROM promo p WHERE p.deleted_at IS NULL`
	args := []any{}
	if keyword != "" {
		query += ` AND (p.kode LIKE ? OR p.nama LIKE ?)`
		like := "%" + keyword + "%"
		args = append(args, like, like)
	}
	if jenis != "" {
		query += ` AND p.jenis = ?`
		args = append(args, jenis)
	}
	query += ` ORDER BY p.aktif DESC, p.kode`

	rows, err := s.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	daftar, err := ScanRows(rows, func(r *Rows) (models.Promo, error) {
		return scanPromo(r)
	})
	if err != nil {
		return nil, err
	}
	for i := range daftar {
		if daftar[i].Jenis == models.JenisVoucher {
			if v, err := s.AmbilVoucherPromo(ctx, daftar[i].ID); err == nil {
				daftar[i].KuotaVoucher = v.Kuota
				daftar[i].Terpakai = v.Terpakai
			}
		}
	}
	return daftar, nil
}

// scanPromo memindai satu baris promo.
func scanPromo(r *Rows) (models.Promo, error) {
	var p models.Promo
	var jamMulai, jamSelesai sql.NullString
	var cabangIDs string
	var deleted sql.NullTime
	err := r.Scan(&p.ID, &p.Kode, &p.Nama, &p.Jenis, &p.NilaiPersen, &p.NilaiNominal,
		&p.MinBelanja, &p.TglMulai, &p.TglSelesai, &jamMulai, &jamSelesai,
		&cabangIDs, &p.Aktif, &deleted)
	if err != nil {
		return p, err
	}
	if jamMulai.Valid {
		p.JamMulai = jamMulai.String
	}
	if jamSelesai.Valid {
		p.JamSelesai = jamSelesai.String
	}
	if deleted.Valid {
		t := deleted.Time
		p.DeletedAt = &t
	}
	p.CabangIDs = parseCabangIDs(cabangIDs)
	return p, nil
}

// parseCabangIDs mengubah CSV id cabang menjadi slice int64.
//
// Cabang tujuan promo disimpan sebagai CSV pada kolom VARCHAR agar tidak perlu
// tabel pivot. Kosong berarti promo berlaku untuk semua cabang.
func parseCabangIDs(raw string) []int64 {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var hasil []int64
	for _, bagian := range strings.Split(raw, ",") {
		bagian = strings.TrimSpace(bagian)
		if bagian == "" {
			continue
		}
		if id, err := strconv.ParseInt(bagian, 10, 64); err == nil {
			hasil = append(hasil, id)
		}
	}
	return hasil
}

// formatCabangIDs mengubah slice id cabang menjadi CSV.
func formatCabangIDs(ids []int64) string {
	if len(ids) == 0 {
		return ""
	}
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, strconv.FormatInt(id, 10))
	}
	return strings.Join(parts, ",")
}

// AmbilPromo mengambil satu promo beserta aturan detailnya.
//
// Aturan detail tiap jenis promo (BR-09):
//   - DISKON_PRODUK         -> daftar produk + persen diskon (tabel promo_produk)
//   - POTONGAN_MIN_BELANJA  -> nilai nominal + minimal belanja (kolom promo)
//   - B2G1                  -> daftar kategori (tabel promo_kategori)
//   - VOUCHER               -> kode + kuota (tabel voucher)
func (s *Store) AmbilPromo(ctx context.Context, id int64) (models.Promo, error) {
	var p models.Promo
	var jamMulai, jamSelesai sql.NullString
	var cabangIDs string
	var deleted sql.NullTime
	err := s.DB.QueryRowContext(ctx, `
		SELECT id, kode, nama, jenis, COALESCE(nilai_persen,0), COALESCE(nilai_nominal,0),
		       COALESCE(min_belanja,0), tgl_mulai, tgl_selesai, jam_mulai, jam_selesai,
		       COALESCE(cabang_ids,''), aktif, deleted_at
		FROM promo WHERE id = ?`, id).
		Scan(&p.ID, &p.Kode, &p.Nama, &p.Jenis, &p.NilaiPersen, &p.NilaiNominal,
			&p.MinBelanja, &p.TglMulai, &p.TglSelesai, &jamMulai, &jamSelesai,
			&cabangIDs, &p.Aktif, &deleted)
	if err != nil {
		return p, err
	}
	if jamMulai.Valid {
		p.JamMulai = jamMulai.String
	}
	if jamSelesai.Valid {
		p.JamSelesai = jamSelesai.String
	}
	if deleted.Valid {
		t := deleted.Time
		p.DeletedAt = &t
	}
	p.CabangIDs = parseCabangIDs(cabangIDs)

	// Aturan diskon produk
	rows, err := s.Query(ctx, `
		SELECT pp.produk_id, pr.nama, pp.persen
		FROM promo_produk pp JOIN produk pr ON pr.id = pp.produk_id
		WHERE pp.promo_id = ? ORDER BY pr.nama`, id)
	if err != nil {
		return p, err
	}
	p.DiskonProduk, err = ScanRows(rows, func(r *Rows) (models.PromoDiskonProduk, error) {
		var d models.PromoDiskonProduk
		err := r.Scan(&d.ProdukID, &d.Nama, &d.Persen)
		return d, err
	})
	if err != nil {
		return p, err
	}

	// Kategori B2G1
	rows, err = s.Query(ctx, `SELECT kategori FROM promo_kategori WHERE promo_id = ? ORDER BY kategori`, id)
	if err != nil {
		return p, err
	}
	p.KategoriB2G1, err = ScanRows(rows, func(r *Rows) (string, error) {
		var k string
		err := r.Scan(&k)
		return k, err
	})
	if err != nil {
		return p, err
	}

	// Voucher (jika ada)
	if v, err := s.AmbilVoucherPromo(ctx, id); err == nil {
		p.KuotaVoucher = v.Kuota
		p.Terpakai = v.Terpakai
	}
	return p, nil
}

// AmbilVoucherPromo mengambil data voucher yang menempel pada sebuah promo.
func (s *Store) AmbilVoucherPromo(ctx context.Context, promoID int64) (models.Voucher, error) {
	var v models.Voucher
	err := s.DB.QueryRowContext(ctx,
		`SELECT kode, promo_id, kuota, terpakai FROM voucher WHERE promo_id = ?`, promoID).
		Scan(&v.Kode, &v.PromoID, &v.Kuota, &v.Terpakai)
	return v, err
}

// AmbilVoucherByKode mengambil voucher berdasarkan kode voucher.
func (s *Store) AmbilVoucherByKode(ctx context.Context, kode string) (models.Voucher, error) {
	var v models.Voucher
	err := s.DB.QueryRowContext(ctx,
		`SELECT kode, promo_id, kuota, terpakai FROM voucher WHERE kode = ?`, kode).
		Scan(&v.Kode, &v.PromoID, &v.Kuota, &v.Terpakai)
	return v, err
}

// DaftarVoucher mengembalikan seluruh voucher beserta nama promo.
func (s *Store) DaftarVoucher(ctx context.Context) ([]models.Voucher, error) {
	rows, err := s.Query(ctx, `
		SELECT v.kode, v.promo_id, v.kuota, v.terpakai, p.nama
		FROM voucher v JOIN promo p ON p.id = v.promo_id
		WHERE p.deleted_at IS NULL ORDER BY v.kode`)
	if err != nil {
		return nil, err
	}
	return ScanRows(rows, func(r *Rows) (models.Voucher, error) {
		var v models.Voucher
		var namaPromo string
		err := r.Scan(&v.Kode, &v.PromoID, &v.Kuota, &v.Terpakai, &namaPromo)
		return v, err
	})
}

// SimpanPromo menyimpan promo baru beserta aturan detailnya dalam satu transaksi.
func (s *Store) SimpanPromo(ctx context.Context, p models.Promo, kodeVoucher string, kuota int) (int64, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `
		INSERT INTO promo (kode, nama, jenis, nilai_persen, nilai_nominal, min_belanja,
		                   tgl_mulai, tgl_selesai, jam_mulai, jam_selesai, cabang_ids, aktif)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.Kode, p.Nama, p.Jenis, nullableInt(p.NilaiPersen), nullableNominal(p.NilaiNominal),
		nullableNominal(p.MinBelanja), p.TglMulai, p.TglSelesai,
		nullableJam(p.JamMulai), nullableJam(p.JamSelesai),
		formatCabangIDs(p.CabangIDs), p.Aktif)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if err := simpanDetailPromo(ctx, tx, id, p, kodeVoucher, kuota); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

// UbahPromo memperbarui promo beserta aturan detailnya dalam satu transaksi.
func (s *Store) UbahPromo(ctx context.Context, p models.Promo, kodeVoucher string, kuota int) error {
	tx, err := s.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		UPDATE promo SET kode = ?, nama = ?, jenis = ?, nilai_persen = ?, nilai_nominal = ?,
		       min_belanja = ?, tgl_mulai = ?, tgl_selesai = ?, jam_mulai = ?, jam_selesai = ?,
		       cabang_ids = ?, aktif = ?
		WHERE id = ?`,
		p.Kode, p.Nama, p.Jenis, nullableInt(p.NilaiPersen), nullableNominal(p.NilaiNominal),
		nullableNominal(p.MinBelanja), p.TglMulai, p.TglSelesai,
		nullableJam(p.JamMulai), nullableJam(p.JamSelesai),
		formatCabangIDs(p.CabangIDs), p.Aktif, p.ID)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM promo_produk WHERE promo_id = ?`, p.ID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM promo_kategori WHERE promo_id = ?`, p.ID); err != nil {
		return err
	}
	if err := simpanDetailPromo(ctx, tx, p.ID, p, kodeVoucher, kuota); err != nil {
		return err
	}
	return tx.Commit()
}

// simpanDetailPromo menyimpan promo_produk, promo_kategori, dan voucher.
func simpanDetailPromo(ctx context.Context, tx *sql.Tx, promoID int64, p models.Promo, kodeVoucher string, kuota int) error {
	for _, d := range p.DiskonProduk {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO promo_produk (promo_id, produk_id, persen) VALUES (?, ?, ?)`,
			promoID, d.ProdukID, d.Persen)
		if err != nil {
			return err
		}
	}
	for _, k := range p.KategoriB2G1 {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO promo_kategori (promo_id, kategori) VALUES (?, ?)`,
			promoID, k)
		if err != nil {
			return err
		}
	}
	if p.Jenis == models.JenisVoucher && kodeVoucher != "" {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO voucher (kode, promo_id, kuota, terpakai) VALUES (?, ?, ?, 0)
			 ON DUPLICATE KEY UPDATE kuota = VALUES(kuota)`,
			kodeVoucher, promoID, kuota)
		if err != nil {
			return err
		}
	}
	return nil
}

// SetAktifPromo mengaktifkan atau menonaktifkan promo.
func (s *Store) SetAktifPromo(ctx context.Context, id int64, aktif bool) error {
	_, err := s.Exec(ctx, `UPDATE promo SET aktif = ? WHERE id = ?`, aktif, id)
	return err
}

// HapusPromo melakukan soft delete. Promo yang sudah pernah dipakai akan ditolak.
//
// Aturan: promo yang sudah pernah dipakai pada transaksi HANYA BOLEH
// dinonaktifkan, tidak boleh dihapus — supaya laporan biaya promo (FR-18)
// masih dapat menunjuk promo lama.
func (s *Store) HapusPromo(ctx context.Context, id int64) error {
	dipakai, err := s.PromoSudahDipakai(ctx, id)
	if err != nil {
		return err
	}
	if dipakai {
		return ErrPromoSudahDipakai
	}
	_, err = s.Exec(ctx, `UPDATE promo SET deleted_at = NOW() WHERE id = ?`, id)
	return err
}

// PromoSudahDipakai memeriksa apakah promo pernah dipakai pada transaksi.
func (s *Store) PromoSudahDipakai(ctx context.Context, id int64) (bool, error) {
	var n int
	err := s.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM transaksi_promo WHERE promo_id = ?`, id).Scan(&n)
	return n > 0, err
}

// PromoAktif mengembalikan seluruh promo yang berlaku pada waktu dan cabang tertentu.
//
// Promo yang sudah dinonaktifkan (aktif = 0), sudah kedaluwarsa, di luar
// jam berlaku, atau tidak berlaku untuk cabang tersebut akan dibuang di sini
// (method BerlakuPada pada model Promo).
func (s *Store) PromoAktif(ctx context.Context, cabangID int64, waktu time.Time) ([]models.Promo, error) {
	daftar, err := s.DaftarPromo(ctx, "", "")
	if err != nil {
		return nil, err
	}
	var hasil []models.Promo
	for _, p := range daftar {
		if !p.BerlakuPada(waktu, cabangID) {
			continue
		}
		detail, err := s.AmbilPromo(ctx, p.ID)
		if err != nil {
			return nil, err
		}
		hasil = append(hasil, detail)
	}
	return hasil, nil
}

// UbahKuotaVoucher memperbarui kuota voucher (tidak boleh lebih kecil dari terpakai).
//
// Syarat terpakai <= kuota dituliskan pada WHERE sehingga kuota voucher tidak
// pernah diturunkan di bawah jumlah yang sudah terpakai.
func (s *Store) UbahKuotaVoucher(ctx context.Context, kode string, kuota int) error {
	_, err := s.Exec(ctx,
		`UPDATE voucher SET kuota = ? WHERE kode = ? AND terpakai <= ?`, kuota, kode, kuota)
	return err
}

// KodePromoDipakai memeriksa apakah kode promo sudah dipakai.
func (s *Store) KodePromoDipakai(ctx context.Context, kode string, kecualiID int64) (bool, error) {
	var n int
	err := s.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM promo WHERE kode = ? AND id <> ?`, kode, kecualiID).Scan(&n)
	return n > 0, err
}

// KodeVoucherDipakai memeriksa apakah kode voucher sudah dipakai.
func (s *Store) KodeVoucherDipakai(ctx context.Context, kode string, kecualiPromoID int64) (bool, error) {
	var n int
	err := s.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM voucher WHERE kode = ? AND promo_id <> ?`, kode, kecualiPromoID).Scan(&n)
	return n > 0, err
}

// Helper untuk menulis nilai kosong sebagai NULL.
func nullableInt(v int) any {
	if v <= 0 {
		return nil
	}
	return v
}

func nullableNominal(v int64) any {
	if v <= 0 {
		return nil
	}
	return v
}

func nullableJam(v string) any {
	if v == "" {
		return nil
	}
	return v
}
