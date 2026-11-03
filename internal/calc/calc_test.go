package calc

import (
	"testing"
	"time"

	"seduhpoin/internal/models"
)

// TestContohAngkaSubbab22 menguji contoh perhitungan wajib pada spesifikasi:
//
//	Subtotal Rp145.000
//	- B2G1 Rp20.000
//	- diskon produk Rp6.000
//	- potongan minimal belanja Rp15.000
//	= total setelah promo Rp104.000
//	Tukar 40 poin  -> total bayar Rp64.000
//	Poin diperoleh = floor(104.000 / 10.000) x 1.5 (Gold) = 15 poin
func TestContohAngkaSubbab22(t *testing.T) {
	items := []Item{
		{ProdukID: 1, Nama: "Kopi Susu", Kategori: "Kopi", Harga: 20000, Jumlah: 3},
		{ProdukID: 2, Nama: "Roti Bakar", Kategori: "Makanan", Harga: 20000, Jumlah: 3},
		{ProdukID: 3, Nama: "Air Mineral", Kategori: "Minuman", Harga: 25000, Jumlah: 1},
	}

	promos := []PromoAturan{
		{
			PromoID:      10,
			Nama:         "Diskon Kopi 10%",
			Jenis:        models.JenisDiskonProduk,
			DiskonProduk: map[int64]int{1: 10},
		},
		{
			PromoID:  11,
			Nama:     "Beli 2 Gratis 1 Makanan",
			Jenis:    models.JenisB2G1,
			Kategori: []string{"Makanan"},
		},
		{
			PromoID:      12,
			Nama:         "Potongan Rp15.000 min belanja Rp100.000",
			Jenis:        models.JenisPotonganMinBelanja,
			MinBelanja:   100000,
			NilaiNominal: 15000,
		},
	}

	// Member Gold dengan saldo 60 poin, menukar 40 poin.
	hasil, err := Hitung(items, promos, nil, 40, 60, PengaliGold)
	if err != nil {
		t.Fatalf("Hitung menghasilkan error: %v", err)
	}

	if hasil.Subtotal != 145000 {
		t.Errorf("Subtotal = %d, ingin 145000", hasil.Subtotal)
	}
	if hasil.PotonganProduk != 26000 {
		t.Errorf("Potongan produk = %d, ingin 26000 (B2G1 20000 + diskon 6000)", hasil.PotonganProduk)
	}
	if hasil.TotalSetelahPromoProduk != 119000 {
		t.Errorf("Total setelah promo produk = %d, ingin 119000", hasil.TotalSetelahPromoProduk)
	}
	if hasil.PotonganLain != 15000 {
		t.Errorf("Potongan minimal belanja = %d, ingin 15000", hasil.PotonganLain)
	}
	if hasil.TotalSetelahPromo != 104000 {
		t.Errorf("Total setelah promo = %d, ingin 104000", hasil.TotalSetelahPromo)
	}
	if hasil.BatasTukarPoin != 52 {
		t.Errorf("Batas tukar poin = %d, ingin 52", hasil.BatasTukarPoin)
	}
	if hasil.TotalBayar != 64000 {
		t.Errorf("Total bayar = %d, ingin 64000", hasil.TotalBayar)
	}
	if hasil.PoinDiperoleh != 15 {
		t.Errorf("Poin diperoleh = %d, ingin 15", hasil.PoinDiperoleh)
	}

	// Rincian per promo harus memuat ketiga jenis potongan.
	perPromo := map[string]int64{}
	for _, r := range hasil.Rincian {
		perPromo[r.NamaPromo] = r.NilaiPotongan
	}
	if perPromo["Diskon Kopi 10%"] != 6000 {
		t.Errorf("Potongan diskon = %d, ingin 6000", perPromo["Diskon Kopi 10%"])
	}
	if perPromo["Beli 2 Gratis 1 Makanan"] != 20000 {
		t.Errorf("Potongan B2G1 = %d, ingin 20000", perPromo["Beli 2 Gratis 1 Makanan"])
	}
	if perPromo["Potongan Rp15.000 min belanja Rp100.000"] != 15000 {
		t.Errorf("Potongan min belanja = %d, ingin 15000", perPromo["Potongan Rp15.000 min belanja Rp100.000"])
	}
}

// TestContohAngkaTanpaMember memastikan non-member tetap bisa memakai promo
// namun tidak memperoleh maupun menukar poin.
func TestContohAngkaTanpaMember(t *testing.T) {
	items := []Item{
		{ProdukID: 1, Nama: "Kopi Susu", Kategori: "Kopi", Harga: 20000, Jumlah: 3},
	}
	promos := []PromoAturan{
		{PromoID: 10, Nama: "Diskon Kopi 10%", Jenis: models.JenisDiskonProduk, DiskonProduk: map[int64]int{1: 10}},
	}
	hasil, err := Hitung(items, promos, nil, 0, 0, PengaliSilver)
	if err != nil {
		t.Fatalf("Hitung menghasilkan error: %v", err)
	}
	if hasil.TotalSetelahPromo != 54000 {
		t.Errorf("Total setelah promo = %d, ingin 54000", hasil.TotalSetelahPromo)
	}
	if hasil.PoinDiperoleh != 5 {
		t.Errorf("Poin diperoleh = %d, ingin 5", hasil.PoinDiperoleh)
	}
}

// TestVoucherTidakDigabungDenganPotonganMinBelanja memverifikasi BR-11:
// bila voucher dipakai, promo potongan minimal belanja tidak boleh ikut terpotong.
func TestVoucherTidakDigabungDenganPotonganMinBelanja(t *testing.T) {
	items := []Item{
		{ProdukID: 1, Nama: "Kopi Susu", Kategori: "Kopi", Harga: 50000, Jumlah: 3},
	}
	promos := []PromoAturan{
		{PromoID: 12, Nama: "Potongan 15rb min 100rb", Jenis: models.JenisPotonganMinBelanja, MinBelanja: 100000, NilaiNominal: 15000},
	}
	voucher := &PromoAturan{PromoID: 13, Nama: "Voucher 10rb", Jenis: models.JenisVoucher, NilaiNominal: 10000}

	hasil, err := Hitung(items, promos, voucher, 0, 0, PengaliSilver)
	if err != nil {
		t.Fatalf("Hitung menghasilkan error: %v", err)
	}
	// 150.000 - voucher 10.000 = 140.000 (potongan 15.000 min belanja TIDAK dipakai)
	if hasil.TotalSetelahPromo != 140000 {
		t.Errorf("Total setelah promo = %d, ingin 140000", hasil.TotalSetelahPromo)
	}
	if hasil.PotonganLain != 10000 {
		t.Errorf("Potongan voucher = %d, ingin 10000", hasil.PotonganLain)
	}
}

// TestPoinMelebihiBatas memverifikasi penolakan penukaran poin di atas batas 50%.
func TestPoinMelebihiBatas(t *testing.T) {
	items := []Item{
		{ProdukID: 1, Nama: "Kopi Susu", Kategori: "Kopi", Harga: 50000, Jumlah: 3},
	}
	_, err := Hitung(items, nil, nil, 100, 1000, PengaliSilver)
	if err != ErrPoinMelebihiBatas {
		t.Fatalf("err = %v, ingin ErrPoinMelebihiBatas", err)
	}
}

// TestFIFO DuaLot menguji penukaran poin memakai 2 lot: lot dengan tanggal
// kadaluarsa paling dekat dipakai lebih dulu, lalu lot berikutnya.
func TestFIFODuaLot(t *testing.T) {
	hariIni := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	lots := []Lot{
		{ID: 1, JumlahSisa: 10, TanggalDapat: time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC), TanggalKedaluarsa: time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC), Status: models.LotAktif},
		{ID: 2, JumlahSisa: 5, TanggalDapat: time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC), TanggalKedaluarsa: time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC), Status: models.LotAktif},
	}

	urut := UrutFIFO(lots)
	if urut[0].ID != 1 || urut[1].ID != 2 {
		t.Fatalf("Urutan FIFO salah: [%d %d], ingin [1 2]", urut[0].ID, urut[1].ID)
	}

	alokasi := AlokasiFIFO(lots, 12, hariIni)
	if len(alokasi) != 2 {
		t.Fatalf("Jumlah alokasi = %d, ingin 2", len(alokasi))
	}
	if alokasi[0].LotID != 1 || alokasi[0].Jumlah != 10 {
		t.Errorf("Alokasi pertama = lot %d jumlah %d, ingin lot 1 jumlah 10", alokasi[0].LotID, alokasi[0].Jumlah)
	}
	if alokasi[1].LotID != 2 || alokasi[1].Jumlah != 2 {
		t.Errorf("Alokasi kedua = lot %d jumlah %d, ingin lot 2 jumlah 2", alokasi[1].LotID, alokasi[1].Jumlah)
	}

	// Lot yang sudah kedaluwarsa dilewati.
	lots[0].TanggalKedaluarsa = time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC)
	alokasi = AlokasiFIFO(lots, 3, hariIni)
	if len(alokasi) != 1 || alokasi[0].LotID != 2 || alokasi[0].Jumlah != 3 {
		t.Errorf("Lot kedaluwarsa tidak dilewati: %+v", alokasi)
	}
}

// TestFIFOSamaKedaluarsa memastikan lot dengan tanggal kadaluarsa sama diurutkan
// dari yang lebih dulu diperoleh.
func TestFIFOSamaKedaluarsa(t *testing.T) {
	hariIni := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	lots := []Lot{
		{ID: 7, JumlahSisa: 4, TanggalDapat: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), TanggalKedaluarsa: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), Status: models.LotAktif},
		{ID: 8, JumlahSisa: 4, TanggalDapat: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), TanggalKedaluarsa: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), Status: models.LotAktif},
	}
	alokasi := AlokasiFIFO(lots, 6, hariIni)
	if len(alokasi) != 2 || alokasi[0].LotID != 8 {
		t.Errorf("Alokasi = %+v, ingin lot 8 lebih dulu", alokasi)
	}
}

// TestTingkatGoldBoundary menguji batas tingkat Gold: Rp1.499.999 masih Silver,
// Rp1.500.000 menjadi Gold.
func TestTingkatGoldBoundary(t *testing.T) {
	if got := TingkatMember(1499999); got != models.TingkatSilver {
		t.Errorf("TingkatMember(1.499.999) = %s, ingin SILVER", got)
	}
	if got := TingkatMember(1500000); got != models.TingkatGold {
		t.Errorf("TingkatMember(1.500.000) = %s, ingin GOLD", got)
	}
	if got := PengaliTingkat(models.TingkatSilver); got != PengaliSilver {
		t.Errorf("Pengali Silver = %v, ingin 1.0", got)
	}
	if got := PengaliTingkat(models.TingkatGold); got != PengaliGold {
		t.Errorf("Pengali Gold = %v, ingin 1.5", got)
	}
}

// TestPoinDiperoleh menguji dasar poin dan pengali tingkat.
func TestPoinDiperoleh(t *testing.T) {
	kasus := []struct {
		total   int64
		pengali float64
		ingin   int64
	}{
		{104000, PengaliGold, 15},
		{104000, PengaliSilver, 10},
		{104999, PengaliSilver, 10},
		{105000, PengaliSilver, 10},
		{19999, PengaliSilver, 1},
		{9999, PengaliSilver, 0},
		{0, PengaliGold, 0},
	}
	for _, k := range kasus {
		got := PoinDiperoleh(k.total, k.pengali)
		if got != k.ingin {
			t.Errorf("PoinDiperoleh(%d, %v) = %d, ingin %d", k.total, k.pengali, got, k.ingin)
		}
	}
}

// TestBatasTukarPoin menguji batas tukar: maksimal 50% total setelah promo.
func TestBatasTukarPoin(t *testing.T) {
	kasus := []struct {
		saldo int64
		total int64
		ingin int64
	}{
		{1000, 104000, 52},
		{30, 104000, 30},
		{0, 104000, 0},
		{100, 10000, 5},
		{100, 1000, 0},
	}
	for _, k := range kasus {
		got := BatasTukarPoin(k.saldo, k.total)
		if got != k.ingin {
			t.Errorf("BatasTukarPoin(%d, %d) = %d, ingin %d", k.saldo, k.total, got, k.ingin)
		}
	}
}

// TestTanggalKedaluarsa menguji masa berlaku poin 6 bulan.
func TestTanggalKedaluarsa(t *testing.T) {
	masuk := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	ingin := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	if got := TanggalKedaluarsa(masuk); !got.Equal(ingin) {
		t.Errorf("TanggalKedaluarsa = %v, ingin %v", got, ingin)
	}

	// 31 Agustus + 6 bulan -> 28 Februari (tahun berikutnya).
	masuk = time.Date(2026, 8, 31, 10, 0, 0, 0, time.UTC)
	ingin = time.Date(2027, 2, 28, 0, 0, 0, 0, time.UTC)
	if got := TanggalKedaluarsa(masuk); !got.Equal(ingin) {
		t.Errorf("TanggalKedaluarsa = %v, ingin %v", got, ingin)
	}
}

// TestB2G1 GabungKategori memastikan unit dalam kategori yang sama digabung
// meskipun baris item berbeda.
func TestB2G1GabungKategori(t *testing.T) {
	items := []Item{
		{ProdukID: 1, Nama: "Roti", Kategori: "Makanan", Harga: 15000, Jumlah: 2},
		{ProdukID: 2, Nama: "Kroissant", Kategori: "Makanan", Harga: 20000, Jumlah: 1},
	}
	hasil := HitungB2G1(items, []string{"Makanan"})
	// 3 unit dalam kategori Makanan -> 1 unit termurah (Rp15.000) gratis.
	if hasil[0].PotonganB2G1 != 15000 {
		t.Errorf("Potongan B2G1 = %d, ingin 15000", hasil[0].PotonganB2G1)
	}
	if hasil[0].UnitGratis != 1 {
		t.Errorf("Unit gratis = %d, ingin 1", hasil[0].UnitGratis)
	}
}
