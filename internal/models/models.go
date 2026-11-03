// Package models berisi struktur data domain aplikasi SeduhPoin.
package models

import "time"

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
type Produk struct {
	ID        int64
	Kode      string
	Nama      string
	Kategori  string
	Harga     int64
	DeletedAt *time.Time
}

// Member adalah pelanggan terdaftar pemilik loyalty.
type Member struct {
	ID        int64
	NoHP      string
	Nama      string
	TglDaftar time.Time
	DeletedAt *time.Time
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

// Voucher adalah kode promo berkuota (jenis VOUCHER).
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
type Transaksi struct {
	ID                   int64
	Kode                 string
	CabangID             int64
	CabangNama           string
	MemberID             *int64
	MemberNama           string
	MemberNoHP           string
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
