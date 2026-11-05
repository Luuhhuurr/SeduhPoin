package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"seduhpoin/internal/calc"
	"seduhpoin/internal/models"
)

// ErrVoucherHabis dikembalikan bila kuota voucher sudah habis.
var ErrVoucherHabis = errors.New("kuota voucher sudah habis")

// ErrTransaksiTidakDitemukan dikembalikan saat transaksi tidak ada.
var ErrTransaksiTidakDitemukan = errors.New("transaksi tidak ditemukan")

// ErrTransaksiSudahBatal dikembalikan saat transaksi sudah berstatus BATAL.
var ErrTransaksiSudahBatal = errors.New("transaksi sudah dibatalkan")

// ErrPoinSudahTerpakai dikembalikan bila poin hasil transaksi tersebut sudah
// dipakai pada transaksi lain, sehingga pembatalan ditolak (BR-14).
var ErrPoinSudahTerpakai = errors.New("poin yang diperoleh dari transaksi ini sudah digunakan, pembatalan ditolak")

// DataSimpanTransaksi berisi semua data yang perlu disimpan untuk satu transaksi.
type DataSimpanTransaksi struct {
	CabangID          int64
	MemberID          *int64
	PenggunaID        *int64 // kasir yang memproses penjualan (nil untuk data lama)
	KasirNama         string // snapshot nama kasir saat transaksi disimpan
	Tanggal           time.Time
	Tingkat           string
	Pengali           float64
	Subtotal          int64
	TotalSetelahPromo int64
	PoinDitukar       int64
	TotalBayar        int64
	Items             []calc.Item
	Promos            []models.TransaksiPromo
	AlokasiPoin       []calc.Alokasi
	PoinDiperoleh     int64
	KedaluarsaPoin    time.Time
	VoucherKode       string
}

// SimpanTransaksi menyimpan transaksi beserta seluruh efeknya dalam satu sql.Tx.
//
// Isi transaksi database: header transaksi, item, rincian promo, penggunaan
// voucher, penukaran poin (FIFO), serta lot poin baru dari perolehan.
func (s *Store) SimpanTransaksi(ctx context.Context, d DataSimpanTransaksi) (int64, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return 0, err
	}
	// Rollback otomatis bila terjadi error sebelum Commit.
	defer tx.Rollback()

	kodeSementara := fmt.Sprintf("TMP-%d", time.Now().UnixNano())
	res, err := tx.ExecContext(ctx, `
		INSERT INTO transaksi (kode, cabang_id, member_id, pengguna_id, kasir_nama,
		                       tanggal_transaksi, tingkat_saat_transaksi, pengali_saat_transaksi,
		                       subtotal, total_setelah_promo, poin_ditukar,
		                       total_bayar, voucher_kode, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'SELESAI', ?)`,
		kodeSementara, d.CabangID, d.MemberID, d.PenggunaID, d.KasirNama, d.Tanggal,
		d.Tingkat, d.Pengali, d.Subtotal, d.TotalSetelahPromo, d.PoinDitukar,
		d.TotalBayar, d.VoucherKode, d.Tanggal)
	if err != nil {
		return 0, err
	}
	transaksiID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	// Kode transaksi final menggunakan id transaksi agar selalu unik.
	kode := fmt.Sprintf("TRX-%s-%04d", d.Tanggal.Format("20060102"), transaksiID)
	if _, err := tx.ExecContext(ctx, `UPDATE transaksi SET kode = ? WHERE id = ?`, kode, transaksiID); err != nil {
		return 0, err
	}

	for _, item := range d.Items {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO transaksi_item (transaksi_id, produk_id, nama_produk, kategori,
			                            harga, jumlah, subtotal, persen_diskon, potongan)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			transaksiID, item.ProdukID, item.Nama, item.Kategori, item.Harga,
			item.Jumlah, item.Subtotal(), item.PersenDiskon,
			item.PotonganB2G1+item.PotonganDiskon); err != nil {
			return 0, err
		}
	}

	for _, promo := range d.Promos {
		var promoID any
		if promo.PromoID != nil {
			promoID = *promo.PromoID
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO transaksi_promo (transaksi_id, promo_id, nama_promo, jenis, nilai_potongan)
			VALUES (?, ?, ?, ?, ?)`,
			transaksiID, promoID, promo.NamaPromo, promo.Jenis, promo.NilaiPotongan); err != nil {
			return 0, err
		}
	}

	// Pemakaian voucher: kuota dicek dan dinaikkan dalam satu pernyataan.
	if d.VoucherKode != "" {
		hasil, err := tx.ExecContext(ctx,
			`UPDATE voucher SET terpakai = terpakai + 1 WHERE kode = ? AND terpakai < kuota`,
			d.VoucherKode)
		if err != nil {
			return 0, err
		}
		terpengaruh, err := hasil.RowsAffected()
		if err != nil {
			return 0, err
		}
		if terpengaruh == 0 {
			return 0, ErrVoucherHabis
		}
	}

	// Penukaran poin: FIFO terhadap lot yang dikunci FOR UPDATE.
	if d.MemberID != nil && d.PoinDitukar > 0 {
		lots, err := s.AmbilLotAktifTx(ctx, tx, *d.MemberID, d.Tanggal)
		if err != nil {
			return 0, err
		}
		alokasi := calc.AlokasiFIFO(lots, d.PoinDitukar, d.Tanggal)
		var totalDialokasikan int64
		for _, a := range alokasi {
			totalDialokasikan += a.Jumlah
		}
		if totalDialokasikan != d.PoinDitukar {
			return 0, fmt.Errorf("saldo poin tidak mencukupi untuk ditukar (%d tersedia)", totalDialokasikan)
		}
		for _, a := range alokasi {
			if err := s.KurangiLotTx(ctx, tx, a.LotID, a.Jumlah); err != nil {
				return 0, err
			}
			lotID := a.LotID
			if _, err := s.SimpanMutasiTx(ctx, tx, models.MutasiPoin{
				MemberID:    *d.MemberID,
				Tipe:        models.MutasiPenukaran,
				Jumlah:      a.Jumlah,
				LotID:       &lotID,
				TransaksiID: &transaksiID,
				Keterangan:  "Penukaran poin pada transaksi " + kode,
				CreatedAt:   d.Tanggal,
			}); err != nil {
				return 0, err
			}
		}
	}

	// Perolehan poin: membuat lot baru dengan masa berlaku 6 bulan.
	if d.MemberID != nil && d.PoinDiperoleh > 0 {
		lotID, err := s.SimpanLotTx(ctx, tx, models.PoinLot{
			MemberID:          *d.MemberID,
			TransaksiID:       &transaksiID,
			JumlahAwal:        d.PoinDiperoleh,
			JumlahSisa:        d.PoinDiperoleh,
			TanggalDapat:      d.Tanggal,
			TanggalKedaluarsa: d.KedaluarsaPoin,
			Status:            models.LotAktif,
		})
		if err != nil {
			return 0, err
		}
		if _, err := s.SimpanMutasiTx(ctx, tx, models.MutasiPoin{
			MemberID:    *d.MemberID,
			Tipe:        models.MutasiPerolehan,
			Jumlah:      d.PoinDiperoleh,
			LotID:       &lotID,
			TransaksiID: &transaksiID,
			Keterangan:  "Perolehan poin dari transaksi " + kode,
			CreatedAt:   d.Tanggal,
		}); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return transaksiID, nil
}

// BatalkanTransaksi membatalkan satu transaksi beserta seluruh efeknya
// dalam satu sql.Tx (BR-14).
//
// Akibat pembatalan:
//   - transaksi berstatus BATAL (seluruh transaksi, bukan parsial),
//   - poin yang diperoleh ditarik (lot -> DITARIK, mutasi TARIK),
//   - poin yang ditukar dikembalikan ke lot asalnya (mutasi KEMBALI),
//   - kuota voucher dikembalikan satu,
//   - data transaksi TIDAK dihapus, biaya promo keluar dari laporan karena
//     laporan hanya menghitung transaksi SELESAI.
//
// Pembatalan DITOLAK bila poin yang diperoleh dari transaksi tersebut sudah
// dipakai pada transaksi lain.
func (s *Store) BatalkanTransaksi(ctx context.Context, id int64) error {
	tx, err := s.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Kunci baris transaksi agar tidak ada pembatalan ganda.
	var kode, status string
	var memberID sql.NullInt64
	var voucherKode sql.NullString
	err = tx.QueryRowContext(ctx, `
		SELECT kode, member_id, voucher_kode, status FROM transaksi WHERE id = ? FOR UPDATE`, id).
		Scan(&kode, &memberID, &voucherKode, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrTransaksiTidakDitemukan
	}
	if err != nil {
		return err
	}
	if status != models.StatusSelesai {
		return ErrTransaksiSudahBatal
	}

	now := time.Now()
	if memberID.Valid {
		// 1. Periksa poin perolehan transaksi ini: bila sudah terpakai, batalkan.
		rows, err := tx.QueryContext(ctx, `
			SELECT id, jumlah_awal, jumlah_sisa FROM poin_lot
			WHERE transaksi_id = ? FOR UPDATE`, id)
		if err != nil {
			return err
		}
		type lotInfo struct {
			ID         int64
			JumlahAwal int64
			JumlahSisa int64
		}
		var lots []lotInfo
		for rows.Next() {
			var l lotInfo
			if err := rows.Scan(&l.ID, &l.JumlahAwal, &l.JumlahSisa); err != nil {
				rows.Close()
				return err
			}
			lots = append(lots, l)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for _, l := range lots {
			if l.JumlahSisa < l.JumlahAwal {
				return ErrPoinSudahTerpakai
			}
		}

		// 2. Tarik poin perolehan transaksi ini.
		for _, l := range lots {
			if _, err := tx.ExecContext(ctx, `
				UPDATE poin_lot SET status = 'DITARIK', jumlah_sisa = 0 WHERE id = ?`, l.ID); err != nil {
				return err
			}
			lotID := l.ID
			if _, err := s.SimpanMutasiTx(ctx, tx, models.MutasiPoin{
				MemberID:    memberID.Int64,
				Tipe:        models.MutasiTarik,
				Jumlah:      l.JumlahAwal,
				LotID:       &lotID,
				TransaksiID: &id,
				Keterangan:  "Penarikan poin karena pembatalan transaksi " + kode,
				CreatedAt:   now,
			}); err != nil {
				return err
			}
		}

		// 3. Kembalikan poin yang sempat ditukar pada transaksi ini ke lot asal.
		rows, err = tx.QueryContext(ctx, `
			SELECT lot_id, jumlah FROM mutasi_poin
			WHERE transaksi_id = ? AND tipe = 'PENUKARAN' AND lot_id IS NOT NULL
			FOR UPDATE`, id)
		if err != nil {
			return err
		}
		type kembalian struct {
			LotID  int64
			Jumlah int64
		}
		var daftarKembalian []kembalian
		for rows.Next() {
			var k kembalian
			if err := rows.Scan(&k.LotID, &k.Jumlah); err != nil {
				rows.Close()
				return err
			}
			daftarKembalian = append(daftarKembalian, k)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for _, k := range daftarKembalian {
			if _, err := tx.ExecContext(ctx, `
				UPDATE poin_lot SET jumlah_sisa = jumlah_sisa + ?, status = 'AKTIF' WHERE id = ?`,
				k.Jumlah, k.LotID); err != nil {
				return err
			}
			lotID := k.LotID
			if _, err := s.SimpanMutasiTx(ctx, tx, models.MutasiPoin{
				MemberID:    memberID.Int64,
				Tipe:        models.MutasiKembali,
				Jumlah:      k.Jumlah,
				LotID:       &lotID,
				TransaksiID: &id,
				Keterangan:  "Pengembalian poin karena pembatalan transaksi " + kode,
				CreatedAt:   now,
			}); err != nil {
				return err
			}
		}
	}

	// 4. Kembalikan kuota voucher.
	if voucherKode.Valid && voucherKode.String != "" {
		if _, err := tx.ExecContext(ctx, `
			UPDATE voucher SET terpakai = GREATEST(terpakai - 1, 0) WHERE kode = ?`,
			voucherKode.String); err != nil {
			return err
		}
	}

	// 5. Tandai transaksi sebagai BATAL (data tidak dihapus).
	if _, err := tx.ExecContext(ctx, `
		UPDATE transaksi SET status = 'BATAL', batal_at = ? WHERE id = ?`, now, id); err != nil {
		return err
	}

	return tx.Commit()
}

// AmbilTransaksi mengambil header transaksi beserta item dan rincian promonya.
func (s *Store) AmbilTransaksi(ctx context.Context, id int64) (models.Transaksi, error) {
	var t models.Transaksi
	var memberID sql.NullInt64
	var memberNama, memberNoHP sql.NullString
	var penggunaID sql.NullInt64
	var kasirNama sql.NullString
	var voucherKode sql.NullString
	var tanggal, createdAt time.Time
	var batalAt sql.NullTime

	err := s.DB.QueryRowContext(ctx, `
		SELECT t.id, t.kode, t.cabang_id, c.nama, t.member_id, m.nama, m.no_hp,
		       t.pengguna_id, COALESCE(t.kasir_nama, u.nama, '-'),
		       t.tanggal_transaksi, t.tingkat_saat_transaksi, t.pengali_saat_transaksi,
		       t.subtotal, t.total_setelah_promo, t.poin_ditukar, t.total_bayar,
		       t.voucher_kode, t.status, t.created_at, t.batal_at
		FROM transaksi t
		JOIN cabang c ON c.id = t.cabang_id
		LEFT JOIN member m ON m.id = t.member_id
		LEFT JOIN pengguna u ON u.id = t.pengguna_id
		WHERE t.id = ?`, id).
		Scan(&t.ID, &t.Kode, &t.CabangID, &t.CabangNama, &memberID, &memberNama, &memberNoHP,
			&penggunaID, &kasirNama,
			&tanggal, &t.TingkatSaatTransaksi, &t.PengaliSaatTransaksi,
			&t.Subtotal, &t.TotalSetelahPromo, &t.PoinDitukar, &t.TotalBayar,
			&voucherKode, &t.Status, &createdAt, &batalAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return t, ErrTransaksiTidakDitemukan
		}
		return t, err
	}
	t.TanggalTransaksi = tanggal
	t.CreatedAt = createdAt
	if voucherKode.Valid {
		t.VoucherKode = voucherKode.String
	}
	if penggunaID.Valid {
		id := penggunaID.Int64
		t.PenggunaID = &id
	}
	if kasirNama.Valid {
		t.KasirNama = kasirNama.String
	}
	if memberID.Valid {
		id := memberID.Int64
		t.MemberID = &id
	}
	if memberNama.Valid {
		t.MemberNama = memberNama.String
	}
	if memberNoHP.Valid {
		t.MemberNoHP = memberNoHP.String
	}
	if batalAt.Valid {
		waktu := batalAt.Time
		t.BatalAt = &waktu
	}

	items, err := s.DaftarItemTransaksi(ctx, id)
	if err != nil {
		return t, err
	}
	t.Items = items

	promos, err := s.DaftarPromoTransaksi(ctx, id)
	if err != nil {
		return t, err
	}
	t.Promos = promos
	return t, nil
}

// DaftarItemTransaksi mengambil rincian item sebuah transaksi.
func (s *Store) DaftarItemTransaksi(ctx context.Context, transaksiID int64) ([]models.TransaksiItem, error) {
	rows, err := s.Query(ctx, `
		SELECT id, transaksi_id, produk_id, nama_produk, kategori, harga, jumlah,
		       subtotal, persen_diskon, potongan
		FROM transaksi_item WHERE transaksi_id = ? ORDER BY id`, transaksiID)
	if err != nil {
		return nil, err
	}
	return ScanRows(rows, func(r *Rows) (models.TransaksiItem, error) {
		var i models.TransaksiItem
		var produkID sql.NullInt64
		err := r.Scan(&i.ID, &i.TransaksiID, &produkID, &i.NamaProduk, &i.Kategori, &i.Harga,
			&i.Jumlah, &i.Subtotal, &i.PersenDiskon, &i.Potongan)
		if produkID.Valid {
			id := produkID.Int64
			i.ProdukID = &id
		}
		return i, err
	})
}

// DaftarPromoTransaksi mengambil rincian potongan promo sebuah transaksi.
func (s *Store) DaftarPromoTransaksi(ctx context.Context, transaksiID int64) ([]models.TransaksiPromo, error) {
	rows, err := s.Query(ctx, `
		SELECT id, transaksi_id, promo_id, nama_promo, jenis, nilai_potongan
		FROM transaksi_promo WHERE transaksi_id = ? ORDER BY id`, transaksiID)
	if err != nil {
		return nil, err
	}
	return ScanRows(rows, func(r *Rows) (models.TransaksiPromo, error) {
		var p models.TransaksiPromo
		var promoID sql.NullInt64
		err := r.Scan(&p.ID, &p.TransaksiID, &promoID, &p.NamaPromo, &p.Jenis, &p.NilaiPotongan)
		if promoID.Valid {
			id := promoID.Int64
			p.PromoID = &id
		}
		return p, err
	})
}

// DaftarTransaksi memfilter transaksi berdasarkan tanggal dan cabang.
func (s *Store) DaftarTransaksi(ctx context.Context, dari, sampai *time.Time, cabangID int64) ([]models.Transaksi, error) {
	query := `
		SELECT t.id, t.kode, t.tanggal_transaksi, t.total_bayar, t.status,
		       c.nama, COALESCE(m.nama, '-'), COALESCE(m.no_hp, '-')
		FROM transaksi t
		JOIN cabang c ON c.id = t.cabang_id
		LEFT JOIN member m ON m.id = t.member_id
		WHERE 1 = 1`
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
	query += ` ORDER BY t.tanggal_transaksi DESC, t.id DESC`

	rows, err := s.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return ScanRows(rows, func(r *Rows) (models.Transaksi, error) {
		var t models.Transaksi
		var tanggal time.Time
		err := r.Scan(&t.ID, &t.Kode, &tanggal, &t.TotalBayar, &t.Status,
			&t.CabangNama, &t.MemberNama, &t.MemberNoHP)
		t.TanggalTransaksi = tanggal
		return t, err
	})
}

// DaftarTransaksiMember mengembalikan transaksi milik satu member saja.
//
// Dipakai portal member. Bedanya dengan DaftarTransaksi (milik kasir) adalah
// query ini dikunci member_id, jadi member hanya bisa melihat transaksinya
// sendiri.
func (s *Store) DaftarTransaksiMember(ctx context.Context, memberID int64) ([]models.Transaksi, error) {
	rows, err := s.Query(ctx, `
		SELECT t.id, t.kode, t.tanggal_transaksi, t.total_bayar, t.status,
		       c.nama, COALESCE(m.nama, '-'), COALESCE(m.no_hp, '-')
		FROM transaksi t
		JOIN cabang c ON c.id = t.cabang_id
		LEFT JOIN member m ON m.id = t.member_id
		WHERE t.member_id = ?
		ORDER BY t.tanggal_transaksi DESC, t.id DESC`, memberID)
	if err != nil {
		return nil, err
	}
	return ScanRows(rows, func(r *Rows) (models.Transaksi, error) {
		var t models.Transaksi
		var tanggal time.Time
		err := r.Scan(&t.ID, &t.Kode, &tanggal, &t.TotalBayar, &t.Status,
			&t.CabangNama, &t.MemberNama, &t.MemberNoHP)
		t.TanggalTransaksi = tanggal
		return t, err
	})
}

// AmbilTransaksiMilikMember mengambil satu transaksi milik member tertentu.
//
// Bila transaksi itu bukan miliknya, ErrTransaksiTidakDitemukan dikembalikan
// — sama seperti transaksi yang memang tidak ada. Dengan begitu member tidak
// bisa menebak keberadaan transaksi orang lain hanya dengan mengubah angka
// pada alamat URL.
func (s *Store) AmbilTransaksiMilikMember(ctx context.Context, memberID, transaksiID int64) (models.Transaksi, error) {
	var ada int
	err := s.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM transaksi WHERE id = ? AND member_id = ?`, transaksiID, memberID).Scan(&ada)
	if err != nil {
		return models.Transaksi{}, err
	}
	if ada == 0 {
		return models.Transaksi{}, ErrTransaksiTidakDitemukan
	}
	return s.AmbilTransaksi(ctx, transaksiID)
}
