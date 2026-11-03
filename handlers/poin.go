package handlers

import (
	"net/http"

	"seduhpoin/internal/calc"
	"seduhpoin/internal/format"
	"seduhpoin/internal/models"
)

// barisPoin adalah satu baris daftar poin member.
type barisPoin struct {
	Member      models.Member
	Saldo       int64
	NilaiRupiah int64
	Tingkat     string
	Pengali     float64
}

// barisLot adalah satu lot poin beserta status efektifnya.
//
// Status efektif dihitung on-the-fly: lot yang tanggal kadaluarsanya sudah
// lewat dianggap kedaluwarsa tanpa perlu kolom status khusus.
type barisLot struct {
	models.PoinLot
	StatusEfektif string
	Aktif         bool
}

// registerPoin mendaftarkan rute halaman poin member.
func (a *App) registerPoin(mux *http.ServeMux) {
	mux.HandleFunc("GET /poin", a.withDB(a.daftarPoin))
	mux.HandleFunc("GET /poin/{id}", a.withDB(a.detailPoin))
}

// daftarPoin menampilkan daftar seluruh member beserta saldo poin aktifnya.
func (a *App) daftarPoin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	hariIni := format.HariIni()

	member, err := a.Store.DaftarMember(ctx, "")
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal memuat daftar member", err.Error())
		return
	}
	saldo, err := a.Store.SaldoPoinAktifSemuaMember(ctx, hariIni)
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal menghitung saldo poin", err.Error())
		return
	}
	belanja, err := a.Store.Belanja3BulanSemuaMember(ctx, calc.BatasTier(hariIni))
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal menghitung tingkat member", err.Error())
		return
	}

	var baris []barisPoin
	for _, m := range member {
		tingkat := calc.TingkatMember(belanja[m.ID])
		baris = append(baris, barisPoin{
			Member:      m,
			Saldo:       saldo[m.ID],
			NilaiRupiah: saldo[m.ID] * calc.NilaiTukarPoin,
			Tingkat:     tingkat,
			Pengali:     calc.PengaliTingkat(tingkat),
		})
	}

	a.render(w, "poin/list.html", View{
		Title: "Poin Member",
		Menu:  "poin",
		Data: map[string]any{
			"Baris":   baris,
			"HariIni": hariIni,
		},
	})
}

// detailPoin menampilkan rincian poin satu member: saldo aktif, tingkat,
// daftar lot poin, dan riwayat mutasi poin.
func (a *App) detailPoin(w http.ResponseWriter, r *http.Request) {
	id, err := idDariPath(r)
	if err != nil {
		pesanError(w, http.StatusBadRequest, "ID member tidak valid", err.Error())
		return
	}
	ctx := r.Context()
	hariIni := format.HariIni()

	member, err := a.Store.AmbilMember(ctx, id)
	if err != nil {
		pesanError(w, http.StatusNotFound, "Member tidak ditemukan", err.Error())
		return
	}

	saldo, err := a.Store.SaldoPoinAktif(ctx, member.ID, hariIni)
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal menghitung saldo poin", err.Error())
		return
	}
	belanja, err := a.Store.Belanja3Bulan(ctx, member.ID, calc.BatasTier(hariIni))
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal menghitung tingkat member", err.Error())
		return
	}
	tingkat := calc.TingkatMember(belanja)

	lots, err := a.Store.AmbilLotMember(ctx, member.ID)
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal memuat lot poin", err.Error())
		return
	}
	var barisLots []barisLot
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

	mutasi, err := a.Store.DaftarMutasiMember(ctx, member.ID)
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal memuat mutasi poin", err.Error())
		return
	}

	a.render(w, "poin/detail.html", View{
		Title: "Poin " + member.Nama,
		Menu:  "poin",
		Data: map[string]any{
			"Member":      member,
			"Saldo":       saldo,
			"NilaiRupiah": saldo * calc.NilaiTukarPoin,
			"Tingkat":     tingkat,
			"Pengali":     calc.PengaliTingkat(tingkat),
			"Belanja":     belanja,
			"Lots":        barisLots,
			"Mutasi":      mutasi,
			"HariIni":     hariIni,
		},
	})
}
