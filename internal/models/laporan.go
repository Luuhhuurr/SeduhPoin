package models

// LaporanPromoBaris adalah satu baris laporan biaya promo (FR-18).
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
type LaporanPromo struct {
	Baris                 []LaporanPromoBaris
	JumlahTransaksi       int
	TotalPotongan         int64
	TotalBiayaTukarPoin   int64
	TotalBiayaKeseluruhan int64
}

// LaporanPoinBaris adalah rincian poin aktif per bulan kedaluwarsa (FR-19).
type LaporanPoinBaris struct {
	Bulan        string
	Label        string
	JumlahPoin   int64
	NilaiRupiah  int64
	JumlahLot    int
	JumlahMember int
}

// LaporanPoin adalah hasil laporan poin berlaku.
type LaporanPoin struct {
	Baris       []LaporanPoinBaris
	TotalPoin   int64
	TotalRupiah int64
}

// LaporanTingkatBaris adalah rekap member per tingkat dalam satu bulan (FR-20).
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
