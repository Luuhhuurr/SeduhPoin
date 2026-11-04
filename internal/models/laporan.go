package models

// LaporanPromoBaris adalah satu baris laporan biaya promo (FR-18).
//
// Aturan pelaporan penting: TotalPotongan (total diskon promo) dan
// BiayaTukarPoin (poin yang ditukar x Rp1.000) berada pada kolom TERPISAH,
// supaya biaya tukar poin tidak tercampur dengan diskon promo dan tidak
// terhitung dua kali.
//
// Sisanya: KuotaVoucher/TerpakaiVoucher/SisaKuota berlaku untuk jenis VOUCHER,
// yang kuotanya dilacak terpisah dari biaya potongan.
type LaporanPromoBaris struct {
	PromoID         int64
	NamaPromo       string
	Jenis           string
	JumlahTransaksi int
	TotalPotongan   int64
	KuotaVoucher    int
	TerpakaiVoucher int
	SisaKuota       int
	BiayaTukarPoin  int64
}

// LaporanPromo adalah hasil laporan biaya promo.
//
// Hanya transaksi berstatus SELESAI yang dihitung, sehingga biaya promo dari
// transaksi yang dibatalkan otomatis keluar dari laporan.
type LaporanPromo struct {
	Baris                 []LaporanPromoBaris
	JumlahTransaksi       int
	TotalPotongan         int64
	TotalBiayaTukarPoin   int64
	TotalBiayaKeseluruhan int64
}

// LaporanPoinBaris adalah rincian poin aktif per bulan kedaluwarsa (FR-19).
//
// Poin dihitung on-the-fly: lot yang sudah lewat tanggal kedaluwarsa atau
// berstatus DITARIK tidak dihitung sebagai poin berlaku.
type LaporanPoinBaris struct {
	Bulan        string
	Label        string
	JumlahPoin   int64
	NilaiRupiah  int64
	JumlahLot    int
	JumlahMember int
}

// LaporanPoin adalah hasil laporan poin berlaku.
//
// TotalPoin adalah saldo poin aktif seluruh member; TotalRupiah adalah nilai
// tukarnya (1 poin = Rp1.000).
type LaporanPoin struct {
	Baris       []LaporanPoinBaris
	TotalPoin   int64
	TotalRupiah int64
}

// LaporanTingkatBaris adalah rekap member per tingkat dalam satu bulan (FR-20).
//
// Silver dan Gold dihitung pada AKHIR bulan (snapshot kondisi member saat
// tanggal tersebut), sedangkan MemberBaru adalah jumlah member yang
// terdaftar pada bulan itu.
type LaporanTingkatBaris struct {
	Bulan       string
	Label       string
	Silver      int
	Gold        int
	TotalMember int
	MemberBaru  int
}

// LaporanTingkat adalah hasil laporan member per tingkat.
type LaporanTingkat struct {
	Baris []LaporanTingkatBaris
}
