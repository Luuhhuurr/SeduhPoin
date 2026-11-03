package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"seduhpoin/internal/calc"
	"seduhpoin/internal/models"
)

// ErrPoinTidakCukup dikembalikan bila lot poin tidak memiliki sisa cukup.
var ErrPoinTidakCukup = errors.New("saldo poin pada lot tidak mencukupi")

// SaldoPoinAktif menghitung saldo poin aktif member.
//
// Poin yang sudah melewati tanggal kedaluwarsa diabaikan (dihitung on-the-fly),
// begitu juga lot yang berstatus DITARIK.
func (s *Store) SaldoPoinAktif(ctx context.Context, memberID int64, hariIni time.Time) (int64, error) {
	var saldo int64
	err := s.DB.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(jumlah_sisa), 0) FROM poin_lot
		WHERE member_id = ? AND status = 'AKTIF' AND jumlah_sisa > 0
		  AND tanggal_kadaluarsa >= ?`, memberID, hariIni).Scan(&saldo)
	return saldo, err
}

// SaldoPoinAktifSemuaMember menghitung saldo poin aktif seluruh member
// dalam satu query dan mengembalikan peta memberID -> saldo.
func (s *Store) SaldoPoinAktifSemuaMember(ctx context.Context, hariIni time.Time) (map[int64]int64, error) {
	rows, err := s.Query(ctx, `
		SELECT member_id, COALESCE(SUM(jumlah_sisa), 0) FROM poin_lot
		WHERE status = 'AKTIF' AND jumlah_sisa > 0 AND tanggal_kadaluarsa >= ?
		GROUP BY member_id`, hariIni)
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

// AmbilLotAktifTx mengambil lot poin aktif member dengan penguncian FOR UPDATE.
//
// Penguncian memastikan poin tidak dipakai dua kali oleh transaksi bersamaan.
func (s *Store) AmbilLotAktifTx(ctx context.Context, tx *sql.Tx, memberID int64, hariIni time.Time) ([]calc.Lot, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, jumlah_sisa, tanggal_dapat, tanggal_kadaluarsa, status
		FROM poin_lot
		WHERE member_id = ? AND status = 'AKTIF' AND jumlah_sisa > 0
		  AND tanggal_kadaluarsa >= ?
		ORDER BY tanggal_kadaluarsa ASC, tanggal_dapat ASC
		FOR UPDATE`, memberID, hariIni)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lots []calc.Lot
	for rows.Next() {
		var l calc.Lot
		var status string
		if err := rows.Scan(&l.ID, &l.JumlahSisa, &l.TanggalDapat, &l.TanggalKedaluarsa, &status); err != nil {
			return nil, err
		}
		l.Status = status
		lots = append(lots, l)
	}
	return lots, rows.Err()
}

// AmbilLotMember mengambil seluruh lot poin milik member (untuk halaman poin).
func (s *Store) AmbilLotMember(ctx context.Context, memberID int64) ([]models.PoinLot, error) {
	rows, err := s.Query(ctx, `
		SELECT id, member_id, transaksi_id, jumlah_awal, jumlah_sisa,
		       tanggal_dapat, tanggal_kadaluarsa, status
		FROM poin_lot WHERE member_id = ? ORDER BY tanggal_kadaluarsa ASC, tanggal_dapat ASC`, memberID)
	if err != nil {
		return nil, err
	}
	return ScanRows(rows, func(r *Rows) (models.PoinLot, error) {
		var l models.PoinLot
		var transaksiID sql.NullInt64
		err := r.Scan(&l.ID, &l.MemberID, &transaksiID, &l.JumlahAwal, &l.JumlahSisa,
			&l.TanggalDapat, &l.TanggalKedaluarsa, &l.Status)
		if transaksiID.Valid {
			id := transaksiID.Int64
			l.TransaksiID = &id
		}
		return l, err
	})
}

// SimpanLotTx membuat lot poin baru hasil perolehan.
func (s *Store) SimpanLotTx(ctx context.Context, tx *sql.Tx, l models.PoinLot) (int64, error) {
	var transaksiID any
	if l.TransaksiID != nil {
		transaksiID = *l.TransaksiID
	}
	res, err := tx.ExecContext(ctx, `
		INSERT INTO poin_lot (member_id, transaksi_id, jumlah_awal, jumlah_sisa,
		                      tanggal_dapat, tanggal_kadaluarsa, status)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		l.MemberID, transaksiID, l.JumlahAwal, l.JumlahSisa,
		l.TanggalDapat, l.TanggalKedaluarsa, l.Status)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// KurangiLotTx mengurangi sisa poin pada sebuah lot.
func (s *Store) KurangiLotTx(ctx context.Context, tx *sql.Tx, lotID int64, jumlah int64) error {
	res, err := tx.ExecContext(ctx,
		`UPDATE poin_lot SET jumlah_sisa = jumlah_sisa - ? WHERE id = ? AND jumlah_sisa >= ?`,
		jumlah, lotID, jumlah)
	if err != nil {
		return err
	}
	terpengaruh, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if terpengaruh == 0 {
		return ErrPoinTidakCukup
	}
	return nil
}

// KembalikanLotTx mengembalikan jumlah poin ke sebuah lot (saat pembatalan).
func (s *Store) KembalikanLotTx(ctx context.Context, tx *sql.Tx, lotID int64, jumlah int64) error {
	_, err := tx.ExecContext(ctx,
		`UPDATE poin_lot SET jumlah_sisa = jumlah_sisa + ?, status = 'AKTIF' WHERE id = ?`,
		jumlah, lotID)
	return err
}

// TarikLotTx menarik lot poin (saat pembatalan transaksi).
func (s *Store) TarikLotTx(ctx context.Context, tx *sql.Tx, lotID int64) error {
	_, err := tx.ExecContext(ctx,
		`UPDATE poin_lot SET status = 'DITARIK', jumlah_sisa = 0 WHERE id = ?`, lotID)
	return err
}

// SimpanMutasiTx mencatat mutasi poin member.
func (s *Store) SimpanMutasiTx(ctx context.Context, tx *sql.Tx, m models.MutasiPoin) (int64, error) {
	var lotID, transaksiID any
	if m.LotID != nil {
		lotID = *m.LotID
	}
	if m.TransaksiID != nil {
		transaksiID = *m.TransaksiID
	}
	res, err := tx.ExecContext(ctx, `
		INSERT INTO mutasi_poin (member_id, tipe, jumlah, lot_id, transaksi_id, keterangan, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		m.MemberID, m.Tipe, m.Jumlah, lotID, transaksiID, m.Keterangan, m.CreatedAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// DaftarMutasiMember mengambil riwayat mutasi poin member.
func (s *Store) DaftarMutasiMember(ctx context.Context, memberID int64) ([]models.MutasiPoin, error) {
	rows, err := s.Query(ctx, `
		SELECT id, member_id, tipe, jumlah, lot_id, transaksi_id, keterangan, created_at
		FROM mutasi_poin WHERE member_id = ? ORDER BY created_at DESC, id DESC`, memberID)
	if err != nil {
		return nil, err
	}
	return ScanRows(rows, func(r *Rows) (models.MutasiPoin, error) {
		var m models.MutasiPoin
		var lotID, transaksiID sql.NullInt64
		err := r.Scan(&m.ID, &m.MemberID, &m.Tipe, &m.Jumlah, &lotID, &transaksiID, &m.Keterangan, &m.CreatedAt)
		if lotID.Valid {
			id := lotID.Int64
			m.LotID = &id
		}
		if transaksiID.Valid {
			id := transaksiID.Int64
			m.TransaksiID = &id
		}
		return m, err
	})
}
