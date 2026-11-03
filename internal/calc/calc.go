// Package calc berisi seluruh perhitungan aturan bisnis SeduhPoin.
//
// Semua fungsi di package ini bersifat pure (tidak menyentuh database),
// sehingga mudah diuji secara unit.
package calc

import (
	"errors"
	"math"
	"sort"
	"time"

	"seduhpoin/internal/models"
)

// Konstanta aturan bisnis inti.
const (
	// PoinPerRupiah: 1 poin untuk setiap kelipatan Rp10.000.
	PoinPerRupiah int64 = 10000
	// PengaliSilver: pengali poin untuk member tingkat Silver.
	PengaliSilver = 1.0
	// PengaliGold: pengali poin untuk member tingkat Gold.
	PengaliGold = 1.5
	// NilaiTukarPoin: 1 poin ditukar menjadi Rp1.000.
	NilaiTukarPoin int64 = 1000
	// BatasTukarPersen: maksimal 50% dari total setelah promo.
	BatasTukarPersen int64 = 50
	// AmbangGold: belanja Rp1.500.000 dalam 3 bulan terakhir menjadi Gold.
	AmbangGold int64 = 1500000
	// MasaBerlakuPoinBulan: masa berlaku poin 6 bulan.
	MasaBerlakuPoinBulan = 6
	// RentangTierBulan: rentang perhitungan tingkat, 3 bulan terakhir.
	RentangTierBulan = 3
)

// Item adalah satu baris item penjualan beserta hasil potongannya.
type Item struct {
	ProdukID int64
	Nama     string
	Kategori string
	Harga    int64
	Jumlah   int64

	// UnitGratis: jumlah unit yang/gratis karena promo B2G1.
	UnitGratis int64
	// PersenDiskon: diskon persen produk yang berlaku.
	PersenDiskon int
	// PotonganB2G1: nilai rupiah unit gratis (B2G1).
	PotonganB2G1 int64
	// PotonganDiskon: nilai rupiah diskon persen produk.
	PotonganDiskon int64
}

// Subtotal mengembalikan nilai bruto item (harga x jumlah).
func (i Item) Subtotal() int64 { return i.Harga * i.Jumlah }

// Total mengembalikan nilai item setelah seluruh promo produk.
func (i Item) Total() int64 { return i.Subtotal() - i.PotonganB2G1 - i.PotonganDiskon }

// Subtotal menjumlahkan nilai bruto seluruh item.
func Subtotal(items []Item) int64 {
	var total int64
	for _, it := range items {
		total += it.Subtotal()
	}
	return total
}

// TotalSetelahPromoProduk menjumlahkan nilai seluruh item setelah promo produk.
func TotalSetelahPromoProduk(items []Item) int64 {
	var total int64
	for _, it := range items {
		total += it.Total()
	}
	return total
}

// HitungB2G1 menerapkan promo beli 2 gratis 1 untuk kategori yang terdaftar.
//
// Dalam setiap kategori, setiap 3 unit yang dibeli memberi 1 unit termurah gratis.
// Unit digabung dalam kategori (baris item dengan produk berbeda dalam kategori
// yang sama ikut dihitung bersama).
func HitungB2G1(items []Item, kategoriList []string) []Item {
	hasil := make([]Item, len(items))
	copy(hasil, items)

	for _, kategori := range kategoriList {
		// Kumpulkan seluruh unit pada kategori ini beserta indeks item-nya.
		type unit struct {
			index int
			harga int64
		}
		var units []unit
		for i := range hasil {
			if hasil[i].Kategori != kategori {
				continue
			}
			// Unit yang sudah mendapat diskon gratis promo lain tidak dihitung lagi.
			sisa := hasil[i].Jumlah - hasil[i].UnitGratis
			for j := int64(0); j < sisa; j++ {
				units = append(units, unit{index: i, harga: hasil[i].Harga})
			}
		}
		if len(units) < 3 {
			continue
		}
		// Urutkan dari harga termurah; baris item sama diurutkan berdasar indeks.
		for i := 0; i < len(units); i++ {
			for j := i + 1; j < len(units); j++ {
				if units[j].harga < units[i].harga ||
					(units[j].harga == units[i].harga && units[j].index < units[i].index) {
					units[i], units[j] = units[j], units[i]
				}
			}
		}
		gratis := len(units) / 3
		for k := 0; k < gratis; k++ {
			idx := units[k].index
			hasil[idx].UnitGratis++
			hasil[idx].PotonganB2G1 += units[k].harga
		}
	}
	return hasil
}

// HitungDiskonProduk menerapkan diskon persen pada produk tertentu.
//
// Diskon hanya berlaku pada unit yang dibayar (unit gratis B2G1 tidak didiskon).
// Bila satu produk mendapat lebih dari satu promo diskon, yang dipakai adalah
// persentase terbesar.
func HitungDiskonProduk(items []Item, aturan map[int64]int) []Item {
	hasil := make([]Item, len(items))
	copy(hasil, items)

	for i := range hasil {
		persen := aturan[hasil[i].ProdukID]
		if persen <= 0 {
			continue
		}
		if persen > 100 {
			persen = 100
		}
		hasil[i].PersenDiskon = persen
		dibayar := hasil[i].Jumlah - hasil[i].UnitGratis
		if dibayar <= 0 {
			continue
		}
		potonganPerUnit := hasil[i].Harga * int64(persen) / 100
		hasil[i].PotonganDiskon = potonganPerUnit * dibayar
	}
	return hasil
}

// HitungPotonganMinBelanja menghitung potongan nominal dengan syarat minimal belanja.
// Bila belanja belum mencapai minimal, potongan bernilai 0.
func HitungPotonganMinBelanja(totalBelanja, minBelanja, nilaiPotongan int64) int64 {
	if totalBelanja < minBelanja || nilaiPotongan <= 0 {
		return 0
	}
	if nilaiPotongan > totalBelanja {
		return totalBelanja
	}
	return nilaiPotongan
}

// TerapkanVoucher memotong nilai voucher dari total (maksimal 1 voucher per transaksi).
func TerapkanVoucher(totalBelanja, nilaiVoucher int64) int64 {
	if nilaiVoucher <= 0 {
		return 0
	}
	if nilaiVoucher > totalBelanja {
		return totalBelanja
	}
	return nilaiVoucher
}

// PoinDiperoleh menghitung poin yang diperoleh dari total setelah promo.
//
// Dasar poin = floor(total / 10.000), lalu dikalikan pengali tingkat member
// dan dibulatkan ke bawah.
func PoinDiperoleh(totalSetelahPromo int64, pengali float64) int64 {
	if totalSetelahPromo <= 0 {
		return 0
	}
	dasar := totalSetelahPromo / PoinPerRupiah
	basisPersen := int64(math.Round(pengali * 100))
	return dasar * basisPersen / 100
}

// BatasTukarPoin menghitung maksimal poin yang boleh ditukar.
//
// Batas = min(saldo poin aktif, floor(50% x total setelah promo / Rp1.000)).
func BatasTukarPoin(saldoPoinAktif, totalSetelahPromo int64) int64 {
	if saldoPoinAktif <= 0 || totalSetelahPromo <= 0 {
		return 0
	}
	maksimal := totalSetelahPromo * BatasTukarPersen / 100 / NilaiTukarPoin
	if maksimal > saldoPoinAktif {
		return saldoPoinAktif
	}
	return maksimal
}

// TotalBayar menghitung total pembayaran setelah penukaran poin.
func TotalBayar(totalSetelahPromo, poinTukar int64) int64 {
	total := totalSetelahPromo - poinTukar*NilaiTukarPoin
	if total < 0 {
		return 0
	}
	return total
}

// TingkatMember menentukan tingkat member berdasarkan belanja 3 bulan terakhir.
func TingkatMember(totalBelanja3Bulan int64) string {
	if totalBelanja3Bulan >= AmbangGold {
		return "GOLD"
	}
	return "SILVER"
}

// PengaliTingkat mengembalikan pengali poin sesuai tingkat member.
func PengaliTingkat(tingkat string) float64 {
	if tingkat == "GOLD" {
		return PengaliGold
	}
	return PengaliSilver
}

// Lot adalah batch poin member dengan masa berlaku (dipakai untuk FIFO).
type Lot struct {
	ID                int64
	JumlahSisa        int64
	TanggalDapat      time.Time
	TanggalKedaluarsa time.Time
	Status            string
}

// AktifLot memeriksa apakah lot masih bisa dipakai pada tanggal tertentu.
func AktifLot(lot Lot, hariIni time.Time) bool {
	if lot.Status != "" && lot.Status != "AKTIF" {
		return false
	}
	if lot.JumlahSisa <= 0 {
		return false
	}
	// Poin yang sudah lewat tanggal kadaluarsa diabaikan.
	return !lot.TanggalKedaluarsa.Before(hariIni)
}

// UrutFIFO mengurutkan lot dari yang paling dekat kadaluarsa.
// Bila tanggal kadaluarsa sama, yang lebih dulu diperoleh didahulukan.
func UrutFIFO(lots []Lot) []Lot {
	hasil := make([]Lot, len(lots))
	copy(hasil, lots)
	for i := 0; i < len(hasil); i++ {
		for j := i + 1; j < len(hasil); j++ {
			a, b := hasil[i], hasil[j]
			lebihDulu := b.TanggalKedaluarsa.Before(a.TanggalKedaluarsa) ||
				(b.TanggalKedaluarsa.Equal(a.TanggalKedaluarsa) && b.TanggalDapat.Before(a.TanggalDapat))
			if lebihDulu {
				hasil[i], hasil[j] = hasil[j], hasil[i]
			}
		}
	}
	return hasil
}

// Alokasi menunjukkan jumlah poin yang diambil dari sebuah lot.
type Alokasi struct {
	LotID  int64
	Jumlah int64
}

// AlokasiFIFO membagi penukaran poin ke beberapa lot sesuai urutan FIFO.
// Lot yang sudah kedaluwarsa atau berstatus DITARIK dilewati.
func AlokasiFIFO(lots []Lot, jumlahPoin int64, hariIni time.Time) []Alokasi {
	var hasil []Alokasi
	sisa := jumlahPoin
	for _, lot := range UrutFIFO(lots) {
		if sisa <= 0 {
			break
		}
		if !AktifLot(lot, hariIni) {
			continue
		}
		ambil := lot.JumlahSisa
		if ambil > sisa {
			ambil = sisa
		}
		hasil = append(hasil, Alokasi{LotID: lot.ID, Jumlah: ambil})
		sisa -= ambil
	}
	return hasil
}

// TanggalKedaluarsa menghitung tanggal kedaluarsa lot poin (6 bulan setelah diperoleh).
// Lot masih berlaku sampai akhir hari pada tanggal tersebut.
func TanggalKedaluarsa(tanggalDapat time.Time) time.Time {
	tahun, bulan, hari := tanggalDapat.Date()
	bulanTarget := int(bulan) + MasaBerlakuPoinBulan
	tahunTarget := tahun
	for bulanTarget > 12 {
		bulanTarget -= 12
		tahunTarget++
	}
	hariMaks := hariDalamBulan(tahunTarget, time.Month(bulanTarget))
	if hari > hariMaks {
		hari = hariMaks
	}
	return time.Date(tahunTarget, time.Month(bulanTarget), hari, 0, 0, 0, 0, tanggalDapat.Location())
}

// hariDalamBulan mengembalikan jumlah hari dalam satu bulan.
func hariDalamBulan(tahun int, bulan time.Month) int {
	return time.Date(tahun, bulan+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// BatasTier mengembalikan batas bawah perhitungan tingkat (3 bulan terakhir).
func BatasTier(hariIni time.Time) time.Time {
	return hariIni.AddDate(0, -RentangTierBulan, 0)
}

// PromoAturan adalah aturan promo yang sudah dipastikan berlaku pada saat transaksi.
type PromoAturan struct {
	PromoID int64
	Nama    string
	Jenis   string
	// DiskonProduk memetakan produkID -> persentase diskon (jenis DISKON_PRODUK).
	DiskonProduk map[int64]int
	// Kategori berisi kategori yang promot (jenis B2G1).
	Kategori     []string
	NilaiNominal int64
	MinBelanja   int64
}

// RincianPromo adalah rincian potongan per promo pada sebuah transaksi.
type RincianPromo struct {
	PromoID       int64
	NamaPromo     string
	Jenis         string
	NilaiPotongan int64
}

// Hasil adalah hasil perhitungan satu transaksi.
type Hasil struct {
	Items                   []Item
	Rincian                 []RincianPromo
	Subtotal                int64
	TotalSetelahPromoProduk int64
	PotonganProduk          int64
	PotonganLain            int64
	TotalSetelahPromo       int64
	PoinDiperoleh           int64
	BatasTukarPoin          int64
	PoinTukar               int64
	TotalBayar              int64
}

// ErrPoinMelebihiBatas dikembalikan bila poin yang ditukar melebihi batas.
var ErrPoinMelebihiBatas = errors.New("jumlah poin yang ditukar melebihi batas tukar")

// Hitung menjalankan seluruh perhitungan transaksi secara berurutan.
//
// Urutan (WAJIB):
//  1. subtotal = Σ harga × jumlah
//  2. promo produk: B2G1 dulu, lalu diskon % produk
//  3. salah satu: voucher ATAU potongan minimal belanja
//  4. totalSetelahPromo (minimal Rp0) = dasar poin & batas tukar
//  5. tukar poin -> totalBayar
//  6. poin diperoleh dari total setelah promo
//
// Parameter:
//   - items        : item penjualan (harga dan jumlah sudah terisi)
//   - promos       : promo yang berlaku (diskon produk, B2G1, potongan min belanja)
//   - voucher      : promo voucher yang dipakai, atau nil bila tidak ada voucher
//   - poinTukar    : jumlah poin yang ingin ditukar
//   - saldoPoin    : saldo poin aktif member
//   - pengali      : pengali poin sesuai tingkat member
func Hitung(items []Item, promos []PromoAturan, voucher *PromoAturan, poinTukar, saldoPoin int64, pengali float64) (Hasil, error) {
	hasil := Hasil{Items: items, Subtotal: Subtotal(items)}

	// 2a. Promo beli 2 gratis 1 (diterapkan per promo agar rincian akurat)
	for _, promo := range promos {
		if promo.Jenis != models.JenisB2G1 {
			continue
		}
		sebelum := totalB2G1(hasil.Items)
		hasil.Items = HitungB2G1(hasil.Items, promo.Kategori)
		sesudah := totalB2G1(hasil.Items)
		if sesudah > sebelum {
			hasil.Rincian = append(hasil.Rincian, RincianPromo{
				PromoID:       promo.PromoID,
				NamaPromo:     promo.Nama,
				Jenis:         promo.Jenis,
				NilaiPotongan: sesudah - sebelum,
			})
		}
	}

	// 2b. Diskon % produk (produk dengan persentase terbesar yang berlaku)
	type aturanDiskon struct {
		persen  int
		promoID int64
		nama    string
	}
	rules := map[int64]aturanDiskon{}
	for _, promo := range promos {
		if promo.Jenis != models.JenisDiskonProduk {
			continue
		}
		for produkID, persen := range promo.DiskonProduk {
			if persen <= 0 {
				continue
			}
			ada, ok := rules[produkID]
			if !ok || persen > ada.persen {
				rules[produkID] = aturanDiskon{persen: persen, promoID: promo.PromoID, nama: promo.Nama}
			}
		}
	}
	petaAturan := map[int64]int{}
	for produkID, aturan := range rules {
		petaAturan[produkID] = aturan.persen
	}
	hasil.Items = HitungDiskonProduk(hasil.Items, petaAturan)

	// Rincian diskon per promo
	perPromo := map[int64]int64{}
	namaPromo := map[int64]string{}
	for _, item := range hasil.Items {
		aturan, ok := rules[item.ProdukID]
		if !ok || item.PotonganDiskon <= 0 {
			continue
		}
		perPromo[aturan.promoID] += item.PotonganDiskon
		namaPromo[aturan.promoID] = aturan.nama
	}
	for promoID, nilai := range perPromo {
		hasil.Rincian = append(hasil.Rincian, RincianPromo{
			PromoID:       promoID,
			NamaPromo:     namaPromo[promoID],
			Jenis:         models.JenisDiskonProduk,
			NilaiPotongan: nilai,
		})
	}

	for _, item := range hasil.Items {
		hasil.PotonganProduk += item.PotonganB2G1 + item.PotonganDiskon
	}
	hasil.TotalSetelahPromoProduk = TotalSetelahPromoProduk(hasil.Items)

	// 3. Voucher ATAU potongan minimal belanja (tidak boleh digabung)
	if voucher != nil {
		potongan := TerapkanVoucher(hasil.TotalSetelahPromoProduk, voucher.NilaiNominal)
		hasil.Rincian = append(hasil.Rincian, RincianPromo{
			PromoID:       voucher.PromoID,
			NamaPromo:     voucher.Nama,
			Jenis:         voucher.Jenis,
			NilaiPotongan: potongan,
		})
		hasil.PotonganLain = potongan
	} else {
		var terpilih *PromoAturan
		var nilaiPotongan int64
		for i := range promos {
			promo := promos[i]
			if promo.Jenis != models.JenisPotonganMinBelanja {
				continue
			}
			nilai := HitungPotonganMinBelanja(hasil.TotalSetelahPromoProduk, promo.MinBelanja, promo.NilaiNominal)
			if nilai > nilaiPotongan {
				nilaiPotongan = nilai
				terpilih = &promo
			}
		}
		if terpilih != nil {
			hasil.Rincian = append(hasil.Rincian, RincianPromo{
				PromoID:       terpilih.PromoID,
				NamaPromo:     terpilih.Nama,
				Jenis:         terpilih.Jenis,
				NilaiPotongan: nilaiPotongan,
			})
			hasil.PotonganLain = nilaiPotongan
		}
	}

	// 4. Total setelah promo (dasar poin dan batas tukar)
	hasil.TotalSetelahPromo = hasil.TotalSetelahPromoProduk - hasil.PotonganLain
	if hasil.TotalSetelahPromo < 0 {
		hasil.TotalSetelahPromo = 0
	}

	// 5. Tukar poin
	if poinTukar < 0 {
		return hasil, errors.New("jumlah poin yang ditukar tidak boleh negatif")
	}
	hasil.BatasTukarPoin = BatasTukarPoin(saldoPoin, hasil.TotalSetelahPromo)
	if poinTukar > hasil.BatasTukarPoin {
		return hasil, ErrPoinMelebihiBatas
	}
	hasil.PoinTukar = poinTukar
	hasil.TotalBayar = TotalBayar(hasil.TotalSetelahPromo, poinTukar)

	// 6. Poin yang diperoleh
	hasil.PoinDiperoleh = PoinDiperoleh(hasil.TotalSetelahPromo, pengali)

	sort.Slice(hasil.Rincian, func(i, j int) bool {
		return hasil.Rincian[i].NamaPromo < hasil.Rincian[j].NamaPromo
	})
	return hasil, nil
}

// totalB2G1 menjumlahkan seluruh potongan B2G1 pada item.
func totalB2G1(items []Item) int64 {
	var total int64
	for _, it := range items {
		total += it.PotonganB2G1
	}
	return total
}
