package store

import (
	"context"
	"database/sql"
	"time"

	"seduhpoin/internal/calc"
	"seduhpoin/internal/models"
)

// LaporanBiayaPromo menyusun laporan biaya promo (FR-18).
//
// Laporan ini menampilkan per promo: jumlah transaksi yang memakainya, total
// potongan nilai rupiahnya, sisa kuota voucher (khusus jenis voucher), dan
// biaya penukaran poin pada kolom terpisah.
//
// Hanya transaksi berstatus SELESAI yang dihitung, sehingga transaksi yang
// dibatalkan otomatis keluar dari laporan biaya promo (BR-14).
// Biaya penukaran poin (poin x Rp1.000) dilaporkan pada kolom terpisah dari
// total potongan promo.
func (s *Store) LaporanBiayaPromo(ctx context.Context, dari, sampai *time.Time, cabangID int64) (models.LaporanPromo, error) {
	query := `
		SELECT tp.promo_id, tp.nama_promo, tp.jenis, tp.nilai_potongan,
		       t.id, t.poin_ditukar
		FROM transaksi_promo tp
		JOIN transaksi t ON t.id = tp.transaksi_id
		WHERE t.status = 'SELESAI'`
	args := []any{}
	if dari != nil {
		query += ` AND t.tanggal_transaksi >= ?`
		args = append(args, *dari)
	}
	if sampai != nil {
		query += ` AND t.tanggal_transaksi < ?`
		args = append(args, *sampai)
	}
	if cabangID > 0 {
		query += ` AND t.cabang_id = ?`
		args = append(args, cabangID)
	}
	query += ` ORDER BY tp.nama_promo, t.id`

	rows, err := s.Query(ctx, query, args...)
	if err != nil {
		return models.LaporanPromo{}, err
	}
	defer rows.Close()

	// Pengelompokan dilakukan di Go agar biaya tukar poin pada transaksi yang
	// memakai beberapa promo tidak terhitung lebih dari sekali.
	//
	// Catatan: satu transaksi boleh memakai beberapa promo sekaligus
	// (diskon % produk + B2G1 + voucher). Bila penjumlahan dilakukan per baris
	// SQL, biaya tukar poin akan terduplikasi untuk setiap promo pada
	// transaksi tersebut. Karena itu biaya tukar poin dijumlahkan per
	// TRANSAKSI UNIK per promo (map transaksiPerPromo), lalu ditampilkan pada
	// kolom terpisah dari total potongan promo (FR-18).
	type promoKey struct {
		id    int64
		nama  string
		jenis string
	}
	baris := map[promoKey]*models.LaporanPromoBaris{}
	urut := []promoKey{}
	transaksiPerPromo := map[promoKey]map[int64]bool{}
	poinPerTransaksi := map[int64]int64{}

	for rows.Next() {
		var promoID sql.NullInt64
		var namaPromo, jenis string
		var nilaiPotongan int64
		var transaksiID int64
		var poinDitukar int64
		if err := rows.Scan(&promoID, &namaPromo, &jenis, &nilaiPotongan, &transaksiID, &poinDitukar); err != nil {
			return models.LaporanPromo{}, err
		}
		var id int64
		if promoID.Valid {
			id = promoID.Int64
		}
		key := promoKey{id: id, nama: namaPromo, jenis: jenis}
		b, ok := baris[key]
		if !ok {
			b = &models.LaporanPromoBaris{
				PromoID:   id,
				NamaPromo: namaPromo,
				Jenis:     jenis,
			}
			baris[key] = b
			transaksiPerPromo[key] = map[int64]bool{}
			urut = append(urut, key)
		}
		b.TotalPotongan += nilaiPotongan
		if !transaksiPerPromo[key][transaksiID] {
			transaksiPerPromo[key][transaksiID] = true
			b.JumlahTransaksi++
		}
		poinPerTransaksi[transaksiID] = poinDitukar
	}
	if err := rows.Err(); err != nil {
		return models.LaporanPromo{}, err
	}

	// Biaya tukar poin per promo: dijumlahkan per transaksi unik.
	for _, key := range urut {
		b := baris[key]
		var biaya int64
		for transaksiID := range transaksiPerPromo[key] {
			biaya += poinPerTransaksi[transaksiID] * 1000
		}
		b.BiayaTukarPoin = biaya
	}

	// Kuota voucher per promo (voucher ditautkan ke promo induknya).
	rowsKuota, err := s.Query(ctx, `SELECT promo_id, COALESCE(SUM(kuota), 0), COALESCE(SUM(terpakai), 0) FROM voucher GROUP BY promo_id`)
	if err != nil {
		return models.LaporanPromo{}, err
	}
	defer rowsKuota.Close()
	kuota := map[int64][2]int{}
	for rowsKuota.Next() {
		var promoID int64
		var k, t int
		if err := rowsKuota.Scan(&promoID, &k, &t); err != nil {
			return models.LaporanPromo{}, err
		}
		kuota[promoID] = [2]int{k, t}
	}
	if err := rowsKuota.Err(); err != nil {
		return models.LaporanPromo{}, err
	}

	var hasil models.LaporanPromo
	for _, key := range urut {
		b := baris[key]
		if k, ok := kuota[b.PromoID]; ok {
			b.KuotaVoucher = k[0]
			b.TerpakaiVoucher = k[1]
			b.SisaKuota = k[0] - k[1]
		}
		hasil.Baris = append(hasil.Baris, *b)
		hasil.TotalPotongan += b.TotalPotongan
		hasil.TotalBiayaTukarPoin += b.BiayaTukarPoin
	}
	hasil.TotalBiayaKeseluruhan = hasil.TotalPotongan + hasil.TotalBiayaTukarPoin

	// Jumlah transaksi unik pada seluruh periode.
	uniq := map[int64]bool{}
	for _, ids := range transaksiPerPromo {
		for id := range ids {
			uniq[id] = true
		}
	}
	hasil.JumlahTransaksi = len(uniq)

	return hasil, nil
}

// LaporanPoinBerlaku menyusun laporan poin berlaku (FR-19).
//
// Laporan ini menampilkan total poin yang masih bisa ditukar beserta nilai
// rupiahnya (1 poin = Rp1.000), dikelompokkan per bulan kedaluwarsa.
//
// Poin dihitung on-the-fly: lot berstatus AKTIF, sisa positif, dan belum
// melewati tanggal kedaluwarsa.
func (s *Store) LaporanPoinBerlaku(ctx context.Context, hariIni time.Time) (models.LaporanPoin, error) {
	rows, err := s.Query(ctx, `
		SELECT DATE_FORMAT(tanggal_kadaluarsa, '%Y-%m') AS bulan,
		       COUNT(*) AS jumlah_lot,
		       COUNT(DISTINCT member_id) AS jumlah_member,
		       COALESCE(SUM(jumlah_sisa), 0) AS jumlah_poin
		FROM poin_lot
		WHERE status = 'AKTIF' AND jumlah_sisa > 0 AND tanggal_kadaluarsa >= ?
		GROUP BY bulan
		ORDER BY bulan`, hariIni)
	if err != nil {
		return models.LaporanPoin{}, err
	}
	defer rows.Close()

	var hasil models.LaporanPoin
	for rows.Next() {
		var b models.LaporanPoinBaris
		if err := rows.Scan(&b.Bulan, &b.JumlahLot, &b.JumlahMember, &b.JumlahPoin); err != nil {
			return models.LaporanPoin{}, err
		}
		b.NilaiRupiah = b.JumlahPoin * 1000
		hasil.Baris = append(hasil.Baris, b)
		hasil.TotalPoin += b.JumlahPoin
	}
	if err := rows.Err(); err != nil {
		return models.LaporanPoin{}, err
	}
	hasil.TotalRupiah = hasil.TotalPoin * 1000
	return hasil, nil
}

// LaporanMemberPerTingkat menyusun laporan member per tingkat (FR-20).
//
// Untuk setiap akhir bulan dalam periode, tingkat member dihitung dari belanja
// 3 bulan sebelumnya (transaksi SELESAI). Jumlah member baru mengikuti
// tgl_daftar member pada bulan tersebut.
//
// Penting: tingkat member TIDAK disimpan di tabel member. Laporan ini menghitung
// ulang (recalculate) tingkat untuk setiap akhir bulan, sehingga nilai yang
// ditampilkan sama dengan tingkat yang dipakai sistem saat transaksi berjalan.
func (s *Store) LaporanMemberPerTingkat(ctx context.Context, dari, sampai time.Time) (models.LaporanTingkat, error) {
	member, err := s.DaftarMember(ctx, "")
	if err != nil {
		return models.LaporanTingkat{}, err
	}

	// Transaksi yang dipakai: 3 bulan sebelum awal periode sampai akhir periode.
	mulaiTransaksi := dari.AddDate(0, -3, 0)
	rows, err := s.Query(ctx, `
		SELECT member_id, tanggal_transaksi, total_setelah_promo
		FROM transaksi
		WHERE status = 'SELESAI' AND member_id IS NOT NULL
		  AND tanggal_transaksi >= ? AND tanggal_transaksi < ?`,
		mulaiTransaksi, sampai.AddDate(0, 0, 1))
	if err != nil {
		return models.LaporanTingkat{}, err
	}
	defer rows.Close()

	type transaksiRow struct {
		memberID int64
		tanggal  time.Time
		total    int64
	}
	var list []transaksiRow
	for rows.Next() {
		var t transaksiRow
		if err := rows.Scan(&t.memberID, &t.tanggal, &t.total); err != nil {
			return models.LaporanTingkat{}, err
		}
		list = append(list, t)
	}
	if err := rows.Err(); err != nil {
		return models.LaporanTingkat{}, err
	}

	// Akhir bulan dalam periode (dari awal bulan sampai ke akhir bulan).
	var hasil models.LaporanTingkat
	for awal := awalBulan(dari); !awal.After(akhirBulan(sampai)); awal = awal.AddDate(0, 1, 0) {
		akhir := akhirBulan(awal)
		batas := awal.AddDate(0, -3, 0) // 3 bulan sebelum awal bulan ini

		baris := models.LaporanTingkatBaris{
			Bulan: awal.Format("2006-01"),
		}
		for _, m := range member {
			// Member baru pada bulan ini.
			if !m.TglDaftar.Before(awal) && !m.TglDaftar.After(akhir) {
				baris.MemberBaru++
			}
			// Member yang belum terdaftar pada akhir bulan ini belum dihitung.
			if m.TglDaftar.After(akhir) {
				continue
			}
			var belanja int64
			for _, t := range list {
				if t.memberID != m.ID {
					continue
				}
				if t.tanggal.Before(batas) || t.tanggal.After(akhir) {
					continue
				}
				belanja += t.total
			}
			if belanja >= calc.AmbangGold {
				baris.Gold++
			} else {
				baris.Silver++
			}
		}
		baris.TotalMember = baris.Silver + baris.Gold
		hasil.Baris = append(hasil.Baris, baris)
	}
	return hasil, nil
}

// awalBulan mengembalikan tanggal awal bulan dari sebuah tanggal.
func awalBulan(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}

// akhirBulan mengembalikan tanggal (00:00) hari pertama bulan berikutnya.
func akhirBulan(t time.Time) time.Time {
	return awalBulan(t).AddDate(0, 1, 0).Add(-time.Nanosecond)
}
