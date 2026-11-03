package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"seduhpoin/internal/calc"
	"seduhpoin/internal/format"
	"seduhpoin/internal/models"
)

// memberRow adalah data member beserta tingkat dan total belanja 3 bulan.
type memberRow struct {
	Member  models.Member
	Tingkat string
	Belanja int64
}

// registerMember mendaftarkan rute CRUD member.
func (a *App) registerMember(mux *http.ServeMux) {
	mux.HandleFunc("GET /member", a.withDB(a.daftarMember))
	mux.HandleFunc("GET /member/tambah", a.withDB(a.formMember))
	mux.HandleFunc("POST /member/tambah", a.withDB(a.simpanMember))
	mux.HandleFunc("GET /member/{id}", a.withDB(a.detailMember))
	mux.HandleFunc("GET /member/{id}/edit", a.withDB(a.formMember))
	mux.HandleFunc("POST /member/{id}/edit", a.withDB(a.simpanMember))
	mux.HandleFunc("POST /member/{id}/hapus", a.withDB(a.hapusMember))
}

// daftarMember menampilkan daftar member dengan tingkat dan belanja 3 bulan.
func (a *App) daftarMember(w http.ResponseWriter, r *http.Request) {
	keyword := strings.TrimSpace(r.URL.Query().Get("q"))

	daftar, err := a.Store.DaftarMember(r.Context(), keyword)
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal memuat daftar member", err.Error())
		return
	}
	batas := calc.BatasTier(format.HariIni())
	belanja, err := a.Store.Belanja3BulanSemuaMember(r.Context(), batas)
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal memuat data belanja member", err.Error())
		return
	}

	baris := make([]memberRow, 0, len(daftar))
	for _, m := range daftar {
		total := belanja[m.ID]
		baris = append(baris, memberRow{
			Member:  m,
			Tingkat: calc.TingkatMember(total),
			Belanja: total,
		})
	}

	a.render(w, "member/list.html", View{
		Title: "Data Member",
		Menu:  "member",
		Error: ambilError(r),
		Data: map[string]any{
			"Daftar":  baris,
			"Keyword": keyword,
		},
	})
}

// formMember menampilkan form tambah / ubah member.
func (a *App) formMember(w http.ResponseWriter, r *http.Request) {
	var m models.Member
	mode := "tambah"

	if raw := r.PathValue("id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			pesanError(w, http.StatusBadRequest, "ID member tidak valid", raw)
			return
		}
		m, err = a.Store.AmbilMember(r.Context(), id)
		if err != nil {
			pesanError(w, http.StatusNotFound, "Member tidak ditemukan", err.Error())
			return
		}
		mode = "edit"
	} else {
		m.TglDaftar = format.HariIni()
	}
	// Nomor HP dapat diisi dari halaman transaksi (?no_hp=...).
	if raw := r.URL.Query().Get("no_hp"); raw != "" && mode == "tambah" {
		m.NoHP = raw
	}

	a.render(w, "member/form.html", View{
		Title: "Form Member",
		Menu:  "member",
		Error: ambilError(r),
		Data: map[string]any{
			"Member": m,
			"Mode":   mode,
		},
	})
}

// simpanMember memproses penyimpanan member (tambah maupun ubah).
func (a *App) simpanMember(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		pesanError(w, http.StatusBadRequest, "Data form tidak valid", err.Error())
		return
	}

	m := models.Member{
		NoHP: strings.TrimSpace(r.FormValue("no_hp")),
		Nama: strings.TrimSpace(r.FormValue("nama")),
	}
	tglDaftar, err := parseTanggalOpsional(r.FormValue("tgl_daftar"))
	if err != nil {
		redirect(w, r, "/member/tambah", "Tanggal daftar tidak valid (format YYYY-MM-DD).")
		return
	}
	if tglDaftar.IsZero() {
		tglDaftar = format.HariIni()
	}
	m.TglDaftar = tglDaftar

	if raw := r.PathValue("id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			pesanError(w, http.StatusBadRequest, "ID member tidak valid", raw)
			return
		}
		m.ID = id
	}

	if m.NoHP == "" {
		redirect(w, r, "/member/tambah", "Nomor HP wajib diisi.")
		return
	}
	if !hanyaAngka(m.NoHP) {
		redirect(w, r, "/member/tambah", "Nomor HP hanya boleh berisi angka.")
		return
	}
	if m.Nama == "" {
		redirect(w, r, "/member/tambah", "Nama member wajib diisi.")
		return
	}
	dipakai, err := a.Store.NoHPDipakai(r.Context(), m.NoHP, m.ID)
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal memeriksa nomor HP", err.Error())
		return
	}
	if dipakai {
		redirect(w, r, "/member/tambah", "Nomor HP sudah terdaftar. Gunakan nomor lain.")
		return
	}

	if m.ID == 0 {
		id, err := a.Store.SimpanMember(r.Context(), m)
		if err != nil {
			pesanError(w, http.StatusInternalServerError, "Gagal menyimpan member", err.Error())
			return
		}
		redirect(w, r, "/member/"+strconv.FormatInt(id, 10), "")
		return
	}
	if err := a.Store.UbahMember(r.Context(), m); err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal mengubah member", err.Error())
		return
	}
	redirect(w, r, "/member/"+strconv.FormatInt(m.ID, 10), "")
}

// detailMember menampilkan rincian satu member.
func (a *App) detailMember(w http.ResponseWriter, r *http.Request) {
	id, err := idDariPath(r)
	if err != nil {
		pesanError(w, http.StatusBadRequest, "ID member tidak valid", err.Error())
		return
	}
	m, err := a.Store.AmbilMember(r.Context(), id)
	if err != nil {
		pesanError(w, http.StatusNotFound, "Member tidak ditemukan", err.Error())
		return
	}
	total, err := a.Store.Belanja3Bulan(r.Context(), id, calc.BatasTier(format.HariIni()))
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal memuat data belanja member", err.Error())
		return
	}
	a.render(w, "member/detail.html", View{
		Title: "Detail Member",
		Menu:  "member",
		Data: map[string]any{
			"Member":  m,
			"Tingkat": calc.TingkatMember(total),
			"Belanja": total,
		},
	})
}

// hapusMember melakukan soft delete member.
func (a *App) hapusMember(w http.ResponseWriter, r *http.Request) {
	id, err := idDariPath(r)
	if err != nil {
		pesanError(w, http.StatusBadRequest, "ID member tidak valid", err.Error())
		return
	}
	if err := a.Store.HapusMember(r.Context(), id); err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal menghapus member", err.Error())
		return
	}
	redirect(w, r, "/member", "")
}

// hanyaAngka memeriksa apakah seluruh karakter string berupa angka.
func hanyaAngka(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
