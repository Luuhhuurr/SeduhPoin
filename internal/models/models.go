// Package models berisi struktur data domain aplikasi SeduhPoin.
//
// Struct di sini adalah struct biasa — bukan objek ORM. Tidak ada tag atau
// aturan mapping otomatis: pemetaan kolom database ke field struct dilakukan
// manual pada query di package store.
package models

import "time"

// Peran pengguna untuk kontrol akses berbasis peran (RBAC).
//
// Peran ADMIN dan KASIR dimiliki akun pada tabel pengguna (staf),
// sedangkan peran MEMBER adalah pemilik tabel member.
const (
	RoleAdmin  = "ADMIN"
	RoleKasir  = "KASIR"
	RoleMember = "MEMBER"
)

// Pengguna adalah akun staf aplikasi (admin atau kasir).
//
// PasswordHash berisi hasil bcrypt; kata sandi asli tidak pernah disimpan.
// HarusGantiSandi dipakai untuk memaksa pengguna mengganti kata sandi
// awal (misalnya akun demo admin) pada login pertama.
type Pengguna struct {
	ID              int64
	Username        string
	Nama            string
	PasswordHash    string
	Role            string
	Aktif           bool
	HarusGantiSandi bool
	CreatedAt       time.Time
	LastLoginAt     *time.Time
}

// Jenis pemilik sesi.
const (
	JenisPengguna = "PENGGUNA"
	JenisMember   = "MEMBER"
)

// Sesi adalah satu sesi login aktif.
//
// Yang disimpan di database adalah hash SHA-256 dari token yang dikirim ke
// browser, bukan token itu sendiri. Dengan begitu bocornya isi tabel sesi
// tidak langsung berarti ada orang yang bisa memakai cookie tersebut.
//
// Logout menghapus baris sesi, sehingga token langsung tidak berlaku.
type Sesi struct {
	TokenHash     string
	Jenis         string // JenisPengguna atau JenisMember
	PenggunaID    *int64
	MemberID      *int64
	Peran         string // peran saat login (disalin agar tetap terbaca)
	DibuatAt      time.Time
	KedaluwarsaAt time.Time
}

// Cabang adalah titik penjualan (tidak ada soft delete pada level kode:
// kolom deleted_at sudah tersedia di tabel cabang).
type Cabang struct {
	ID        int64
	Kode      string
	Nama      string
	Alamat    string
	Telepon   string
	DeletedAt *time.Time
}

// Produk adalah item yang bisa dijual.
//
// Harga disimpan sebagai int64 (rupiah penuh, tanpa pecahan) agar tidak ada
// galat pembulatan floating point pada perhitungan promo dan total.
type Produk struct {
	ID        int64
	Kode      string
	Nama      string
	Kategori  string
	Harga     int64
	DeletedAt *time.Time
}

// Member adalah pelanggan terdaftar pemilik loyalty.
//
// PERHATIAN: struct ini sengaja tidak memiliki field Tingkat. Tingkat member
// (Silver/Gold) dihitung on-the-fly dari total belanja 3 bulan terakhir
// (lihat internal/calc.TingkatMember) supaya selalu akurat tanpa perlu
// job pemeliharaan data. Yang disimpan pada tabel transaksi adalah tingkat
// yang berlaku saat transaksi itu dibuat.
type Member struct {
	ID        int64
	NoHP      string
	Nama      string
	TglDaftar time.Time
	DeletedAt *time.Time

	// PunyaAkun menandai member yang sudah punya kata sandi sehingga bisa
	// login ke portal member. PasswordHash sendiri tidak ikut di struct
	// ini supaya tidak terbawa ke halaman yang tidak membutuhkannya.
	PunyaAkun bool
}

// Jenis promo (BR-09).
const (
	JenisDiskonProduk       = "DISKON_PRODUK"
	JenisPotonganMinBelanja = "POTONGAN_MIN_BELANJA"
	JenisB2G1               = "B2G1"
	JenisVoucher            = "VOUCHER"
)

// PromoDiskonProduk adalah aturan diskon % untuk satu produk dalam promo.
type PromoDiskonProduk struct {
	ProdukID int64
	Nama     string
	Persen   int
}

// Promo adalah promotion rule. CabangID kosong berarti promo berlaku untuk semua cabang.
//
// DeletedAt != nil berarti promo sudah dihapus (soft delete). Promo yang pernah
// dipakai tidak boleh dihapus, hanya dinonaktifkan lewat field Aktif.
type Promo struct {
	ID           int64
	Kode         string
	Nama         string
	Jenis        string
	NilaiPersen  int
	NilaiNominal int64
	MinBelanja   int64
	TglMulai     time.Time
	TglSelesai   time.Time
	JamMulai     string
	JamSelesai   string
	CabangIDs    []int64
	Aktif        bool
	DeletedAt    *time.Time
	DiskonProduk []PromoDiskonProduk
	KategoriB2G1 []string
	KuotaVoucher int
	Terpakai     int
	TerpakaiEver bool
}

// BerlakuPada menyatakan apakah promo berlaku pada tanggal/waktu dan cabang tertentu.
//
// Empat syarat dicek berurutan: promo aktif, tanggal dalam masa berlaku,
// jam dalam rentang (bila diisi), dan cabang termasuk daftar tujuan (bila diisi).
// Daftar cabang dan jam yang kosong berarti promo berlaku untuk semua cabang
// dan sepanjang hari.
func (p Promo) BerlakuPada(t time.Time, cabangID int64) bool {
	if !p.Aktif {
		return false
	}
	hari := t.Format("2006-01-02")
	if hari < p.TglMulai.Format("2006-01-02") || hari > p.TglSelesai.Format("2006-01-02") {
		return false
	}
	if p.JamMulai != "" {
		jam := t.Format("15:04:05")
		if jam < p.JamMulai {
			return false
		}
	}
	if p.JamSelesai != "" {
		jam := t.Format("15:04:05")
		if jam > p.JamSelesai {
			return false
		}
	}
	if len(p.CabangIDs) > 0 {
		dikenal := false
		for _, id := range p.CabangIDs {
			if id == cabangID {
				dikenal = true
				break
			}
		}
		if !dikenal {
			return false
		}
	}
	return true
}

// BerlakuPadaTanpaCabang menyatakan apakah promo berlaku pada tanggal/waktu
// tertentu, tanpa mempertimbangkan cabang.
//
// Dipakai portal member: member tidak punya cabang, sehingga promo yang
// hanya berlaku untuk cabang tertentu tetap ditampilkan sebagai informasi.
func (p Promo) BerlakuPadaTanpaCabang(t time.Time) bool {
	if !p.Aktif {
		return false
	}
	hari := t.Format("2006-01-02")
	if hari < p.TglMulai.Format("2006-01-02") || hari > p.TglSelesai.Format("2006-01-02") {
		return false
	}
	if p.JamMulai != "" {
		jam := t.Format("15:04:05")
		if jam < p.JamMulai {
			return false
		}
	}
	if p.JamSelesai != "" {
		jam := t.Format("15:04:05")
		if jam > p.JamSelesai {
			return false
		}
	}
	return true
}

// Voucher adalah kode promo berkuota (jenis VOUCHER).
//
// Kuota tidak boleh pernah lebih kecil dari jumlah terpakai; hal itu
// dijamin oleh satu pernyataan SQL penghitung decrement saat pembatalan.
type Voucher struct {
	Kode     string
	PromoID  int64
	Kuota    int
	Terpakai int
}

// Status transaksi.
const (
	StatusSelesai = "SELESAI"
	StatusBatal   = "BATAL"
)

// Tingkat member.
const (
	TingkatSilver = "SILVER"
	TingkatGold   = "GOLD"
)

// Transaksi adalah header satu penjualan.
//
// TingkatSaatTransaksi dan PengaliSaatTransaksi disimpan di sini karena
// pengali sebuah transaksi memakai tingkat member PADA SAAT transaksi dimulai
// (sebelum transaksi itu ikut dihitung dalam perhitungan tingkat 3 bulan).
// Transaksi yang dibatalkan tetap tersimpan dengan status BATAL — data tidak
// dihapus (BR-14).
type Transaksi struct {
	ID                   int64
	Kode                 string
	CabangID             int64
	CabangNama           string
	MemberID             *int64
	MemberNama           string
	MemberNoHP           string
	PenggunaID           *int64 // kasir yang memproses penjualan (nil untuk data lama)
	KasirNama            string // snapshot nama kasir saat transaksi disimpan
	TanggalTransaksi     time.Time
	TingkatSaatTransaksi string
	PengaliSaatTransaksi float64
	Subtotal             int64
	TotalSetelahPromo    int64
	PoinDitukar          int64
	TotalBayar           int64
	VoucherKode          string
	Status               string
	CreatedAt            time.Time
	BatalAt              *time.Time
	Items                []TransaksiItem
	Promos               []TransaksiPromo
}

// TransaksiItem adalah rincian item penjualan (nama & harga disalin).
//
// NamaProduk, Kategori, dan Harga disalin dari tabel produk saat transaksi
// disimpan. Kalau produk nanti diubah harganya atau di-soft delete, riwayat
// transaksi lama tetap menampilkan nama dan harga yang benar saat penjualan.
type TransaksiItem struct {
	ID           int64
	TransaksiID  int64
	ProdukID     *int64
	NamaProduk   string
	Kategori     string
	Harga        int64
	Jumlah       int64
	Subtotal     int64
	PersenDiskon int
	Potongan     int64
}

// TransaksiPromo adalah rincian potongan per promo pada satu transaksi.
//
// Sama seperti item, NamaPromo disalin saat transaksi disimpan agar laporan
// biaya promo tetap historis walaupun promo diubah atau dinonaktifkan kemudian.
type TransaksiPromo struct {
	ID            int64
	TransaksiID   int64
	PromoID       *int64
	NamaPromo     string
	Jenis         string
	NilaiPotongan int64
}

// Status lot poin.
const (
	LotAktif   = "AKTIF"
	LotDitarik = "DITARIK"
)

// Tipe mutasi poin.
const (
	MutasiPerolehan = "PEROLEHAN"
	MutasiPenukaran = "PENUKARAN"
	MutasiKembali   = "KEMBALI"
	MutasiTarik     = "TARIK"
)

// PoinLot adalah batch poin dengan masa berlaku (FIFO).
//
// Poin tidak disimpan sebagai satu saldo per member, melainkan per batch lot
// sehingga tanggal kedaluwarsa tiap perolehan tetap tercatat dan penukaran
// bisa memakai lot yang paling dekat kadaluarsa (FIFO).
type PoinLot struct {
	ID                int64
	MemberID          int64
	TransaksiID       *int64
	JumlahAwal        int64
	JumlahSisa        int64
	TanggalDapat      time.Time
	TanggalKedaluarsa time.Time
	Status            string
}

// MutasiPoin adalah riwayat perubahan saldo poin member.
//
// Tipe mutasi: PEROLEHAN (poin baru dari transaksi), PENUKARAN (poin dipakai
// menukar diskon), KEMBALI (poin dikembalikan saat pembatalan), dan TARIK
// (poin perolehan ditarik saat pembatalan).
type MutasiPoin struct {
	ID          int64
	MemberID    int64
	Tipe        string
	Jumlah      int64
	LotID       *int64
	TransaksiID *int64
	Keterangan  string
	CreatedAt   time.Time
}
