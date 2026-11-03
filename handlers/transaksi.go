package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"seduhpoin/internal/calc"
	"seduhpoin/internal/format"
	"seduhpoin/internal/models"
	"seduhpoin/internal/store"
)

// formTransaksi berisi input kasir pada form transaksi.
type formTransaksi struct {
	CabangID    int64
	NoHP        string
	KodeVoucher string
	PoinTukar   int64
	ProdukID    []int64
	Jumlah      []int64
}

// barisItem adalah satu baris item pada form (produk + jumlah).
type barisItem struct {
	ProdukID int64
	Jumlah   int64
}

// hasilTransaksi berisi hasil perhitungan transaksi.
type hasilTransaksi struct {
	Items             []calc.Item
	RincianPromo      []models.TransaksiPromo
	Subtotal          int64
	TotalSetelahPromo int64
	Potongan          int64
	PotonganProduk    int64
	SaldoPoin         int64
	BatasTukarPoin    int64
	PoinTukar         int64
	PoinDiperoleh     int64
	TotalBayar        int64
	Tingkat           string
	Pengali           float64
	MemberID          *int64
	MemberNama        string
	VoucherKode       string
}

// registerTransaksi mendaftarkan rute transaksi.
func (a *App) registerTransaksi(mux *http.ServeMux) {
	mux.HandleFunc("GET /transaksi", a.withDB(a.formTransaksiBaru))
	mux.HandleFunc("POST /transaksi/proses", a.withDB(a.prosesTransaksi))
	mux.HandleFunc("GET /transaksi/struk/{id}", a.withDB(a.strukTransaksi))
}

// formTransaksiBaru menampilkan form transaksi kosong.
func (a *App) formTransaksiBaru(w http.ResponseWriter, r *http.Request) {
	a.renderFormTransaksi(w, r, formTransaksi{}, nil, "")
}

// prosesTransaksi menangani tombol "Hitung" dan "Simpan".
func (a *App) prosesTransaksi(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		pesanError(w, http.StatusBadRequest, "Data form tidak valid", err.Error())
		return
	}

	f := formTransaksi{
		CabangID:    parseInt64(r.FormValue("cabang_id"), 0),
		NoHP:        strings.TrimSpace(r.FormValue("no_hp")),
		KodeVoucher: strings.TrimSpace(r.FormValue("kode_voucher")),
		PoinTukar:   parseInt64(r.FormValue("poin_tukar"), 0),
	}
	for _, raw := range r.Form["produk_id"] {
		f.ProdukID = append(f.ProdukID, parseInt64(raw, 0))
	}
	for _, raw := range r.Form["jumlah"] {
		f.Jumlah = append(f.Jumlah, parseInt64(raw, 0))
	}

	hasil, pesan := a.hitungTransaksi(r.Context(), f)
	if pesan != "" {
		a.renderFormTransaksi(w, r, f, hasil, pesan)
		return
	}

	if r.FormValue("aksi") != "simpan" {
		// Tombol "Hitung": tampilkan rincian perhitungan.
		a.renderFormTransaksi(w, r, f, hasil, "")
		return
	}

	// Tombol "Simpan": simpan seluruh data dalam satu sql.Tx.
	data := store.DataSimpanTransaksi{
		CabangID:          f.CabangID,
		MemberID:          hasil.MemberID,
		Tanggal:           format.WIB(),
		Tingkat:           hasil.Tingkat,
		Pengali:           hasil.Pengali,
		Subtotal:          hasil.Subtotal,
		TotalSetelahPromo: hasil.TotalSetelahPromo,
		PoinDitukar:       hasil.PoinTukar,
		TotalBayar:        hasil.TotalBayar,
		Items:             hasil.Items,
		Promos:            hasil.RincianPromo,
		PoinDiperoleh:     hasil.PoinDiperoleh,
		KedaluarsaPoin:    calc.TanggalKedaluarsa(format.WIB()),
		VoucherKode:       hasil.VoucherKode,
	}

	transaksiID, err := a.Store.SimpanTransaksi(r.Context(), data)
	if err != nil {
		if errors.Is(err, store.ErrVoucherHabis) {
			a.renderFormTransaksi(w, r, f, hasil, "Kuota voucher sudah habis, transaksi dibatalkan.")
			return
		}
		pesanError(w, http.StatusInternalServerError, "Gagal menyimpan transaksi", err.Error())
		return
	}
	redirect(w, r, "/transaksi/struk/"+strconv.FormatInt(transaksiID, 10), "")
}

// hitungTransaksi menjalankan seluruh perhitungan transaksi.
//
// Urutan perhitungan (WAJIB):
//  1. subtotal = Σ harga × jumlah
//  2. promo produk: B2G1 dulu, lalu diskon % produk
//  3. salah satu: voucher ATAU potongan minimal belanja
//  4. totalSetelahPromo (minimal Rp0)
//  5. tukar poin -> totalBayar
//  6. poin diperoleh (dihitung setelah transaksi tersimpan)
func (a *App) hitungTransaksi(ctx context.Context, f formTransaksi) (*hasilTransaksi, string) {
	hasil := &hasilTransaksi{Tingkat: models.TingkatSilver, Pengali: calc.PengaliSilver}
	now := format.WIB()
	hariIni := format.HariIni()

	if f.CabangID <= 0 {
		return hasil, "Pilih cabang terlebih dahulu."
	}

	// 1. Item transaksi
	var items []calc.Item
	for i, produkID := range f.ProdukID {
		if produkID <= 0 {
			continue
		}
		jumlah := int64(0)
		if i < len(f.Jumlah) {
			jumlah = f.Jumlah[i]
		}
		if jumlah <= 0 {
			continue
		}
		produk, err := a.Store.AmbilProduk(ctx, produkID)
		if err != nil {
			return hasil, "Produk yang dipilih tidak ditemukan."
		}
		items = append(items, calc.Item{
			ProdukID: produk.ID,
			Nama:     produk.Nama,
			Kategori: produk.Kategori,
			Harga:    produk.Harga,
			Jumlah:   jumlah,
		})
	}
	if len(items) == 0 {
		return hasil, "Tambahkan minimal satu produk ke transaksi."
	}
	hasil.Subtotal = calc.Subtotal(items)

	// Member terdaftar (pengguna poin)
	if f.NoHP != "" {
		member, err := a.Store.CariMemberByNoHP(ctx, f.NoHP)
		if errors.Is(err, store.ErrMemberTidakDitemukan) {
			return hasil, fmt.Sprintf("Nomor HP %s belum terdaftar sebagai member. Daftarkan member terlebih dahulu agar poin dapat digunakan.", f.NoHP)
		}
		if err != nil {
			return hasil, "Gagal mencari member: " + err.Error()
		}
		hasil.MemberID = &member.ID
		hasil.MemberNama = member.Nama

		saldo, err := a.Store.SaldoPoinAktif(ctx, member.ID, hariIni)
		if err != nil {
			return hasil, "Gagal menghitung saldo poin: " + err.Error()
		}
		hasil.SaldoPoin = saldo

		// Tingkat dihitung dari belanja 3 bulan terakhir SEBELUM transaksi ini.
		belanja, err := a.Store.Belanja3Bulan(ctx, member.ID, calc.BatasTier(hariIni))
		if err != nil {
			return hasil, "Gagal menghitung tingkat member: " + err.Error()
		}
		hasil.Tingkat = calc.TingkatMember(belanja)
		hasil.Pengali = calc.PengaliTingkat(hasil.Tingkat)
	}

	// Promo yang berlaku pada cabang dan waktu ini
	promos, err := a.Store.PromoAktif(ctx, f.CabangID, now)
	if err != nil {
		return hasil, "Gagal memuat data promo: " + err.Error()
	}

	// Terjemahkan promo DB menjadi aturan perhitungan (package calc).
	var aturan []calc.PromoAturan
	for _, promo := range promos {
		switch promo.Jenis {
		case models.JenisDiskonProduk:
			diskon := map[int64]int{}
			for _, d := range promo.DiskonProduk {
				if d.Persen > 0 {
					diskon[d.ProdukID] = d.Persen
				}
			}
			aturan = append(aturan, calc.PromoAturan{
				PromoID:      promo.ID,
				Nama:         promo.Nama,
				Jenis:        promo.Jenis,
				DiskonProduk: diskon,
			})
		case models.JenisB2G1:
			aturan = append(aturan, calc.PromoAturan{
				PromoID:  promo.ID,
				Nama:     promo.Nama,
				Jenis:    promo.Jenis,
				Kategori: promo.KategoriB2G1,
			})
		case models.JenisPotonganMinBelanja:
			aturan = append(aturan, calc.PromoAturan{
				PromoID:      promo.ID,
				Nama:         promo.Nama,
				Jenis:        promo.Jenis,
				NilaiNominal: promo.NilaiNominal,
				MinBelanja:   promo.MinBelanja,
			})
		}
	}

	// Voucher: maksimal 1 per transaksi. Bila ada voucher, promo potongan
	// minimal belanja TIDAK boleh digabung (BR-11).
	var aturanVoucher *calc.PromoAturan
	if f.KodeVoucher != "" {
		voucher, err := a.Store.AmbilVoucherByKode(ctx, f.KodeVoucher)
		if err != nil {
			return hasil, fmt.Sprintf("Kode voucher %s tidak dikenal.", f.KodeVoucher)
		}
		if voucher.Terpakai >= voucher.Kuota {
			return hasil, fmt.Sprintf("Kuota voucher %s sudah habis.", voucher.Kode)
		}
		promo, err := a.Store.AmbilPromo(ctx, voucher.PromoID)
		if err != nil {
			return hasil, "Gagal memuat data promo voucher: " + err.Error()
		}
		if !promo.BerlakuPada(now, f.CabangID) {
			return hasil, fmt.Sprintf("Voucher %s tidak berlaku untuk cabang atau waktu ini.", voucher.Kode)
		}
		aturanVoucher = &calc.PromoAturan{
			PromoID:      promo.ID,
			Nama:         promo.Nama,
			Jenis:        models.JenisVoucher,
			NilaiNominal: promo.NilaiNominal,
		}
		hasil.VoucherKode = voucher.Kode
	}

	// Poin hanya boleh ditukar oleh member terdaftar.
	if hasil.MemberID == nil && f.PoinTukar > 0 {
		return hasil, "Poin hanya dapat ditukar oleh member terdaftar."
	}

	// Seluruh perhitungan dijalankan oleh package calc (fungsi yang sama
	// dipakai ulang saat menyimpan transaksi).
	perhitungan, err := calc.Hitung(items, aturan, aturanVoucher, f.PoinTukar, hasil.SaldoPoin, hasil.Pengali)
	if err != nil {
		if errors.Is(err, calc.ErrPoinMelebihiBatas) {
			return hasil, fmt.Sprintf("Poin yang ditukar (%d) melebihi batas tukar (%d poin).",
				f.PoinTukar, perhitungan.BatasTukarPoin)
		}
		return hasil, "Perhitungan gagal: " + err.Error()
	}

	hasil.Items = perhitungan.Items
	hasil.Subtotal = perhitungan.Subtotal
	hasil.PotonganProduk = perhitungan.PotonganProduk
	hasil.Potongan = perhitungan.PotonganProduk + perhitungan.PotonganLain
	hasil.TotalSetelahPromo = perhitungan.TotalSetelahPromo
	hasil.BatasTukarPoin = perhitungan.BatasTukarPoin
	hasil.PoinTukar = perhitungan.PoinTukar
	hasil.TotalBayar = perhitungan.TotalBayar
	for _, r := range perhitungan.Rincian {
		id := r.PromoID
		hasil.RincianPromo = append(hasil.RincianPromo, models.TransaksiPromo{
			PromoID:       &id,
			NamaPromo:     r.NamaPromo,
			Jenis:         r.Jenis,
			NilaiPotongan: r.NilaiPotongan,
		})
	}

	// 6. Poin yang akan diperoleh (hanya untuk member terdaftar)
	if hasil.MemberID != nil {
		hasil.PoinDiperoleh = perhitungan.PoinDiperoleh
	}

	sort.Slice(hasil.RincianPromo, func(i, j int) bool {
		return hasil.RincianPromo[i].NamaPromo < hasil.RincianPromo[j].NamaPromo
	})
	return hasil, ""
}

// renderFormTransaksi menampilkan form transaksi beserta hasil perhitungan.
func (a *App) renderFormTransaksi(w http.ResponseWriter, r *http.Request, f formTransaksi, hasil *hasilTransaksi, pesan string) {
	cabang, err := a.Store.DaftarCabang(r.Context(), "")
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal memuat daftar cabang", err.Error())
		return
	}
	produk, err := a.Store.DaftarProduk(r.Context(), "", "")
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal memuat daftar produk", err.Error())
		return
	}

	// Susun baris item (minimal 8 baris siap diisi).
	jumlahBaris := len(f.ProdukID)
	if jumlahBaris < 8 {
		jumlahBaris = 8
	}
	baris := make([]barisItem, jumlahBaris)
	for i := range baris {
		if i < len(f.ProdukID) {
			jumlah := int64(1)
			if i < len(f.Jumlah) {
				jumlah = f.Jumlah[i]
			}
			baris[i] = barisItem{ProdukID: f.ProdukID[i], Jumlah: jumlah}
		} else {
			baris[i] = barisItem{Jumlah: 1}
		}
	}

	a.render(w, "transaksi/form.html", View{
		Title: "Transaksi Baru",
		Menu:  "transaksi",
		Error: pesan,
		Data: map[string]any{
			"Cabang":  cabang,
			"Produk":  produk,
			"Baris":   baris,
			"Form":    f,
			"Hasil":   hasil,
			"HariIni": format.HariIni(),
		},
	})
}

// strukTransaksi menampilkan struk / bukti pembayaran.
func (a *App) strukTransaksi(w http.ResponseWriter, r *http.Request) {
	id, err := idDariPath(r)
	if err != nil {
		pesanError(w, http.StatusBadRequest, "ID transaksi tidak valid", err.Error())
		return
	}
	transaksi, err := a.Store.AmbilTransaksi(r.Context(), id)
	if err != nil {
		pesanError(w, http.StatusNotFound, "Transaksi tidak ditemukan", err.Error())
		return
	}
	a.render(w, "transaksi/struk.html", View{
		Title: "Struk " + transaksi.Kode,
		Menu:  "transaksi",
		Data: map[string]any{
			"Transaksi": transaksi,
		},
	})
}
