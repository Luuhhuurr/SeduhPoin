package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"seduhpoin/internal/models"
)

// ErrPenggunaTidakDitemukan dikembalikan saat username tidak terdaftar.
var ErrPenggunaTidakDitemukan = errors.New("pengguna dengan username tersebut tidak ditemukan")

// ErrAkunMemberTidakDitemukan dikembalikan saat nomor HP tidak punya akun
// login (member yang belum diberi kata sandi oleh kasir).
var ErrAkunMemberTidakDitemukan = errors.New("member dengan nomor HP tersebut belum memiliki akun login")

// ErrSesiTidakValid dikembalikan saat sesi tidak ditemukan, sudah kedaluwarsa,
// atau pemiliknya sudah tidak aktif.
var ErrSesiTidakValid = errors.New("sesi tidak valid")

// SesiAktif adalah hasil pembacaan satu sesi beserta identitas pemiliknya.
//
// Hanya salah satu dari Pengguna atau Member yang terisi, tergantung pada
// Sesi.Jenis: PENGGUNA untuk staf, MEMBER untuk pelanggan.
type SesiAktif struct {
	Sesi     models.Sesi
	Pengguna models.Pengguna
	Member   models.Member
}

// ---------------------------------------------------------------------
// Akun staf (tabel pengguna)
// ---------------------------------------------------------------------

// AmbilPenggunaByUsername mengambil satu akun staf berdasarkan username.
//
// Hanya akun aktif yang dikembalikan: akun yang dinonaktifkan admin tidak
// boleh bisa login lagi.
func (s *Store) AmbilPenggunaByUsername(ctx context.Context, username string) (models.Pengguna, error) {
	var p models.Pengguna
	var created time.Time
	var lastLogin sql.NullTime
	err := s.DB.QueryRowContext(ctx, `
		SELECT id, username, nama, password_hash, role, aktif,
		       harus_ganti_sandi, created_at, last_login_at
		FROM pengguna
		WHERE username = ? AND aktif = 1`, username).
		Scan(&p.ID, &p.Username, &p.Nama, &p.PasswordHash, &p.Role, &p.Aktif,
			&p.HarusGantiSandi, &created, &lastLogin)
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrPenggunaTidakDitemukan
	}
	if err != nil {
		return p, err
	}
	p.CreatedAt = created
	if lastLogin.Valid {
		t := lastLogin.Time
		p.LastLoginAt = &t
	}
	return p, nil
}

// AmbilPenggunaByID mengambil satu akun staf berdasarkan id.
//
// Tidak memfilter aktif, supaya akun yang sudah dinonaktifkan tetap bisa
// ditampilkan di daftar.
func (s *Store) AmbilPenggunaByID(ctx context.Context, id int64) (models.Pengguna, error) {
	var p models.Pengguna
	var created time.Time
	var lastLogin sql.NullTime
	err := s.DB.QueryRowContext(ctx, `
		SELECT id, username, nama, password_hash, role, aktif,
		       harus_ganti_sandi, created_at, last_login_at
		FROM pengguna
		WHERE id = ?`, id).
		Scan(&p.ID, &p.Username, &p.Nama, &p.PasswordHash, &p.Role, &p.Aktif,
			&p.HarusGantiSandi, &created, &lastLogin)
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrPenggunaTidakDitemukan
	}
	if err != nil {
		return p, err
	}
	p.CreatedAt = created
	if lastLogin.Valid {
		t := lastLogin.Time
		p.LastLoginAt = &t
	}
	return p, nil
}

// DaftarPengguna mengembalikan seluruh akun staf, urut dari admin lalu nama.
func (s *Store) DaftarPengguna(ctx context.Context) ([]models.Pengguna, error) {
	rows, err := s.Query(ctx, `
		SELECT id, username, nama, password_hash, role, aktif,
		       harus_ganti_sandi, created_at, last_login_at
		FROM pengguna
		ORDER BY role ASC, nama`)
	if err != nil {
		return nil, err
	}
	return ScanRows(rows, func(r *Rows) (models.Pengguna, error) {
		var p models.Pengguna
		var created time.Time
		var lastLogin sql.NullTime
		err := r.Scan(&p.ID, &p.Username, &p.Nama, &p.PasswordHash, &p.Role, &p.Aktif,
			&p.HarusGantiSandi, &created, &lastLogin)
		p.CreatedAt = created
		if lastLogin.Valid {
			t := lastLogin.Time
			p.LastLoginAt = &t
		}
		return p, err
	})
}

// SimpanPengguna menyimpan akun staf baru dan mengembalikan id-nya.
//
// username UNIQUE di database; pengecekan ganda tetap dilakukan di handler
// supaya pesannya ramah.
func (s *Store) SimpanPengguna(ctx context.Context, p models.Pengguna) (int64, error) {
	res, err := s.Exec(ctx, `
		INSERT INTO pengguna (username, nama, password_hash, role, aktif,
		                      harus_ganti_sandi, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		p.Username, p.Nama, p.PasswordHash, p.Role, p.Aktif,
		p.HarusGantiSandi, p.CreatedAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UbahPengguna memperbarui data akun staf (nama, role, dan status aktif).
//
// Kata sandi tidak ikut diubah di sini; itu memakai SetPasswordPengguna
// supaya setiap perubahan sandi tercatat sebagai satu tindakan tersendiri.
func (s *Store) UbahPengguna(ctx context.Context, p models.Pengguna) error {
	_, err := s.Exec(ctx, `
		UPDATE pengguna
		SET nama = ?, role = ?, aktif = ?
		WHERE id = ?`, p.Nama, p.Role, p.Aktif, p.ID)
	return err
}

// SetPasswordPengguna mengganti kata sandi akun staf.
//
// harusGanti dipakai untuk memaksa pengguna mengganti sandi awal pada
// login berikutnya.
func (s *Store) SetPasswordPengguna(ctx context.Context, id int64, hash string, harusGanti bool) error {
	_, err := s.Exec(ctx, `
		UPDATE pengguna
		SET password_hash = ?, harus_ganti_sandi = ?
		WHERE id = ?`, hash, harusGanti, id)
	return err
}

// SetAktifPengguna mengaktifkan atau menonaktifkan akun staf.
func (s *Store) SetAktifPengguna(ctx context.Context, id int64, aktif bool) error {
	_, err := s.Exec(ctx, `UPDATE pengguna SET aktif = ? WHERE id = ?`, aktif, id)
	return err
}

// UsernameDipakai memeriksa apakah username sudah dipakai akun lain.
//
// kecualiID mengecualikan akun yang sedang diubah.
func (s *Store) UsernameDipakai(ctx context.Context, username string, kecualiID int64) (bool, error) {
	var n int
	err := s.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pengguna WHERE username = ? AND id <> ?`, username, kecualiID).Scan(&n)
	return n > 0, err
}

// CatatLoginPengguna menyimpan waktu login terakhir sebuah akun staf.
func (s *Store) CatatLoginPengguna(ctx context.Context, id int64, waktu time.Time) error {
	_, err := s.Exec(ctx, `UPDATE pengguna SET last_login_at = ? WHERE id = ?`, waktu, id)
	return err
}

// ---------------------------------------------------------------------
// Akun login member
// ---------------------------------------------------------------------

// CariMemberAkunByNoHP mencari member terdaftar berdasarkan nomor HP dan
// mengembalikan hash kata sandinya.
//
// ErrAkunMemberTidakDitemukan dikembalikan bila nomor HP tidak terdaftar
// ATAU member tersebut belum diberi kata sandi. Handler login memakai
// error ini untuk menampilkan pesan yang sama dengan "sandi salah",
// sehingga tidak bisa dipakai menebak nomor HP mana yang punya akun.
func (s *Store) CariMemberAkunByNoHP(ctx context.Context, noHP string) (models.Member, string, error) {
	var m models.Member
	var hash sql.NullString
	var deleted sql.NullTime
	err := s.DB.QueryRowContext(ctx, `
		SELECT id, no_hp, nama, tgl_daftar, deleted_at, password_hash
		FROM member
		WHERE no_hp = ? AND deleted_at IS NULL`, noHP).
		Scan(&m.ID, &m.NoHP, &m.Nama, &m.TglDaftar, &deleted, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		return m, "", ErrAkunMemberTidakDitemukan
	}
	if err != nil {
		return m, "", err
	}
	if deleted.Valid {
		t := deleted.Time
		m.DeletedAt = &t
	}
	if !hash.Valid {
		return m, "", ErrAkunMemberTidakDitemukan
	}
	m.PunyaAkun = true
	return m, hash.String, nil
}

// SetPasswordMember memasang atau mengganti kata sandi member.
func (s *Store) SetPasswordMember(ctx context.Context, memberID int64, hash string) error {
	_, err := s.Exec(ctx, `UPDATE member SET password_hash = ? WHERE id = ?`, hash, memberID)
	return err
}

// HashMember mengembalikan hash kata sandi seorang member.
//
// Dipakai saat member mengganti kata sandinya sendiri: sandi lama harus
// dicocokkan lebih dulu. Bila member belum punya akun, ErrAkunMemberTidakDitemukan.
func (s *Store) HashMember(ctx context.Context, memberID int64) (string, error) {
	var hash sql.NullString
	err := s.DB.QueryRowContext(ctx,
		`SELECT password_hash FROM member WHERE id = ? AND deleted_at IS NULL`, memberID).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrAkunMemberTidakDitemukan
	}
	if err != nil {
		return "", err
	}
	if !hash.Valid {
		return "", ErrAkunMemberTidakDitemukan
	}
	return hash.String, nil
}

// HashPengguna mengembalikan hash kata sandi sebuah akun staf.
func (s *Store) HashPengguna(ctx context.Context, penggunaID int64) (string, error) {
	var hash string
	err := s.DB.QueryRowContext(ctx,
		`SELECT password_hash FROM pengguna WHERE id = ?`, penggunaID).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrPenggunaTidakDitemukan
	}
	if err != nil {
		return "", err
	}
	return hash, nil
}

// MemberPunyaAkun memeriksa apakah member sudah punya kata sandi login.
func (s *Store) MemberPunyaAkun(ctx context.Context, memberID int64) (bool, error) {
	var n int
	err := s.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM member WHERE id = ? AND password_hash IS NOT NULL`, memberID).Scan(&n)
	return n > 0, err
}

// ---------------------------------------------------------------------
// Sesi login
// ---------------------------------------------------------------------

// BuatSesi menyimpan satu sesi login baru.
//
// Yang disimpan adalah hash SHA-256 dari token cookie (TokenHash), bukan
// token mentah. Sesi berakhir otomatis karena kolom kedaluwarsa_at.
func (s *Store) BuatSesi(ctx context.Context, sesi models.Sesi) error {
	_, err := s.Exec(ctx, `
		INSERT INTO sesi (token_hash, jenis, pengguna_id, member_id, peran,
		                  dibuat_at, kedaluwarsa_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		sesi.TokenHash, sesi.Jenis, sesi.PenggunaID, sesi.MemberID, sesi.Peran,
		sesi.DibuatAt, sesi.KedaluwarsaAt)
	return err
}

// AmbilSesi mencari sesi yang masih berlaku berdasarkan hash token.
//
// Sesi dianggap tidak berlaku bila: sudah melewati kedaluwarsa_at, akun
// stafnya sudah dinonaktifkan, atau member-nya sudah dihapus. Ketiganya
// menghasilkan ErrSesiTidakValid.
func (s *Store) AmbilSesi(ctx context.Context, hashToken string, now time.Time) (SesiAktif, error) {
	var hasil SesiAktif
	var penggunaID sql.NullInt64
	var memberID sql.NullInt64
	var aktif sql.NullBool
	var harusGanti sql.NullBool
	var deleted sql.NullTime
	// Kolom hasil JOIN bisa NULL: sesi MEMBER tidak punya baris pengguna,
	// dan sebaliknya. Karena itu semua kolom itu dipindai lewat
	// sql.NullString agar tidak terjadi error scan.
	var pNama, pRole, mNoHP, mNama sql.NullString

	err := s.DB.QueryRowContext(ctx, `
		SELECT s.token_hash, s.jenis, s.pengguna_id, s.member_id, s.peran,
		       s.dibuat_at, s.kedaluwarsa_at,
		       p.nama, p.role, p.aktif, p.harus_ganti_sandi,
		       m.no_hp, m.nama, m.deleted_at
		FROM sesi s
		LEFT JOIN pengguna p ON p.id = s.pengguna_id
		LEFT JOIN member   m ON m.id = s.member_id
		WHERE s.token_hash = ? AND s.kedaluwarsa_at > ?`, hashToken, now).
		Scan(&hasil.Sesi.TokenHash, &hasil.Sesi.Jenis, &penggunaID, &memberID,
			&hasil.Sesi.Peran, &hasil.Sesi.DibuatAt, &hasil.Sesi.KedaluwarsaAt,
			&pNama, &pRole, &aktif, &harusGanti,
			&mNoHP, &mNama, &deleted)
	if errors.Is(err, sql.ErrNoRows) {
		return hasil, ErrSesiTidakValid
	}
	if err != nil {
		return hasil, err
	}

	hasil.Pengguna.Nama = pNama.String
	hasil.Pengguna.Role = pRole.String
	hasil.Pengguna.HarusGantiSandi = harusGanti.Valid && harusGanti.Bool
	hasil.Member.NoHP = mNoHP.String
	hasil.Member.Nama = mNama.String

	if penggunaID.Valid {
		hasil.Sesi.PenggunaID = &penggunaID.Int64
		hasil.Pengguna.ID = penggunaID.Int64
	}
	if memberID.Valid {
		hasil.Sesi.MemberID = &memberID.Int64
		hasil.Member.ID = memberID.Int64
	}
	if deleted.Valid {
		t := deleted.Time
		hasil.Member.DeletedAt = &t
	}

	// Akun staf yang dinonaktifkan atau member yang dihapus tidak boleh
	// memakai sesi lamanya.
	if hasil.Sesi.Jenis == models.JenisPengguna && (!aktif.Valid || !aktif.Bool) {
		return hasil, ErrSesiTidakValid
	}
	if hasil.Sesi.Jenis == models.JenisMember && hasil.Member.DeletedAt != nil {
		return hasil, ErrSesiTidakValid
	}
	return hasil, nil
}

// HapusSesi menghapus satu sesi, sehingga tokennya langsung tidak berlaku.
//
// Inilah yang membuat logout bersifat instan, bukan sekadar menghapus
// cookie di browser.
func (s *Store) HapusSesi(ctx context.Context, hashToken string) error {
	_, err := s.Exec(ctx, `DELETE FROM sesi WHERE token_hash = ?`, hashToken)
	return err
}

// HapusSemuaSesiPengguna mencabut semua sesi milik satu akun staf.
//
// Dipanggil ketika admin menonaktifkan akun atau mengganti kata sandinya.
func (s *Store) HapusSemuaSesiPengguna(ctx context.Context, penggunaID int64) error {
	_, err := s.Exec(ctx, `DELETE FROM sesi WHERE pengguna_id = ?`, penggunaID)
	return err
}

// HapusSemuaSesiMember mencabut semua sesi milik satu member.
//
// Dipanggil ketika kata sandi member diganti, supaya sesi yang dibuat dengan
// kata sandi lama langsung tidak berlaku.
func (s *Store) HapusSemuaSesiMember(ctx context.Context, memberID int64) error {
	_, err := s.Exec(ctx, `DELETE FROM sesi WHERE member_id = ?`, memberID)
	return err
}

// BersihkanSesiKedaluwarsa menghapus sesi yang sudah lewat masa berlaku.
//
// Dipanggil secara lazy (setiap kali ada login berhasil), sehingga tidak
// perlu cron atau penjadwal tambahan.
func (s *Store) BersihkanSesiKedaluwarsa(ctx context.Context, now time.Time) error {
	_, err := s.Exec(ctx, `DELETE FROM sesi WHERE kedaluwarsa_at <= ?`, now)
	return err
}
