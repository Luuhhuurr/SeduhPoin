package handlers

import (
	"net/http"

	"seduhpoin/internal/calc"
	"seduhpoin/internal/format"
	"seduhpoin/internal/models"
)

// registerPortal mendaftarkan rute portal member.
//
// Portal bersifat read-only: member boleh melihat saldo poin, riwayat
// transaksinya sendiri, dan promo yang sedang berjalan, tetapi TIDAK boleh
// membuat transaksi atau menukar poin sendiri. Semua aksi itu tetap lewat
// kasir di cabang.
func (a *App) registerPortal(mux *http.ServeMux) {
	mux.HandleFunc("GET /portal", a.akses(peranMember, a.portalDashboard))
	mux.HandleFunc("GET /portal/transaksi", a.akses(peranMember, a.portalTransaksi))
	mux.HandleFunc("GET /portal/transaksi/{id}", a.akses(peranMember, a.portalDetailTransaksi))
	mux.HandleFunc("GET /portal/promo", a.akses(peranMember, a.portalPromo))
}

// portalDashboard menampilkan ringkasan poin dan aktivitas member.
//
// Identitas member diambil dari sesi, bukan dari parameter URL, sehingga
// member tidak bisa mengintip data member lain lewat crafting URL.
func (a *App) portalDashboard(w http.ResponseWriter, r *http.Request) {
	id := identitasDari(r)
	hariIni := format.HariIni()

	saldo, err := a.Store.SaldoPoinAktif(r.Context(), id.MemberID, hariIni)
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal memuat saldo poin", err.Error())
		return
	}
	belanja, err := a.Store.Belanja3Bulan(r.Context(), id.MemberID, calc.BatasTier(hariIni))
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal memuat data belanja", err.Error())
		return
	}
	lots, err := a.Store.AmbilLotMember(r.Context(), id.MemberID)
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal memuat rincian poin", err.Error())
		return
	}
	// Status efektif lot dihitung on-the-fly, sama seperti halaman poin staf:
	// lot yang tanggal kedaluarsanya lewat dianggap kedaluwarsa tanpa perlu
	// kolom status khusus di database.
	barisLots := make([]barisLot, 0, len(lots))
	for _, l := range lots {
		b := barisLot{PoinLot: l}
		switch {
		case l.Status == models.LotDitarik:
			b.StatusEfektif = "DITARIK"
		case l.TanggalKedaluarsa.Before(hariIni):
			b.StatusEfektif = "KEDALUWARSA"
		default:
			b.StatusEfektif = "AKTIF"
			b.Aktif = true
		}
		barisLots = append(barisLots, b)
	}
	mutasi, err := a.Store.DaftarMutasiMember(r.Context(), id.MemberID)
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal memuat riwayat poin", err.Error())
		return
	}
	transaksi, err := a.Store.DaftarTransaksiMember(r.Context(), id.MemberID)
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal memuat riwayat transaksi", err.Error())
		return
	}
	promo, err := a.Store.PromoAktifSemuaCabang(r.Context(), format.WIB())
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal memuat daftar promo", err.Error())
		return
	}

	// Mutasi ditampilkan 10 terakhir saja; mutasi lengkap ada di halaman poin.
	if len(mutasi) > 10 {
		mutasi = mutasi[:10]
	}

	a.render(w, r, "portal/dashboard.html", View{
		Title: "Portal Member",
		Menu:  "portal",
		Error: ambilError(r),
		Data: map[string]any{
			"Identitas":  id,
			"Saldo":      saldo,
			"NilaiPoin":  saldo * calc.NilaiTukarPoin,
			"Tingkat":    calc.TingkatMember(belanja),
			"Belanja":    belanja,
			"Lots":       barisLots,
			"Mutasi":     mutasi,
			"Transaksi":  transaksi,
			"JumlahTxn":  len(transaksi),
			"Promo":      promo,
			"BatasTier":  calc.BatasTier(hariIni),
			"NilaiTukar": calc.NilaiTukarPoin,
		},
	})
}

// portalTransaksi menampilkan riwayat transaksi milik member yang login.
func (a *App) portalTransaksi(w http.ResponseWriter, r *http.Request) {
	id := identitasDari(r)

	daftar, err := a.Store.DaftarTransaksiMember(r.Context(), id.MemberID)
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal memuat riwayat transaksi", err.Error())
		return
	}

	a.render(w, r, "portal/transaksi.html", View{
		Title: "Riwayat Transaksi Saya",
		Menu:  "portal-transaksi",
		Error: ambilError(r),
		Data: map[string]any{
			"Daftar": daftar,
		},
	})
}

// portalDetailTransaksi menampilkan rincian satu transaksi milik member.
//
// Query dikunci member_id (lihat store.AmbilTransaksiMilikMember), sehingga
// transaksi milik orang lain diperlakukan sebagai "tidak ditemukan".
func (a *App) portalDetailTransaksi(w http.ResponseWriter, r *http.Request) {
	id := identitasDari(r)

	transaksiID, err := idDariPath(r)
	if err != nil {
		pesanError(w, http.StatusBadRequest, "ID transaksi tidak valid", err.Error())
		return
	}
	trx, err := a.Store.AmbilTransaksiMilikMember(r.Context(), id.MemberID, transaksiID)
	if err != nil {
		pesanError(w, http.StatusNotFound, "Transaksi tidak ditemukan", "Transaksi tersebut bukan milik akun Anda.")
		return
	}

	a.render(w, r, "portal/transaksi-detail.html", View{
		Title: "Detail Transaksi",
		Menu:  "portal-transaksi",
		Error: ambilError(r),
		Data: map[string]any{
			"Transaksi": trx,
		},
	})
}

// portalPromo menampilkan promo dan voucher yang sedang berjalan.
func (a *App) portalPromo(w http.ResponseWriter, r *http.Request) {
	promo, err := a.Store.PromoAktifSemuaCabang(r.Context(), format.WIB())
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal memuat daftar promo", err.Error())
		return
	}
	voucher, err := a.Store.DaftarVoucher(r.Context())
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal memuat daftar voucher", err.Error())
		return
	}

	a.render(w, r, "portal/promo.html", View{
		Title: "Promo & Voucher",
		Menu:  "portal-promo",
		Error: ambilError(r),
		Data: map[string]any{
			"Promo":   promo,
			"Voucher": voucher,
		},
	})
}
