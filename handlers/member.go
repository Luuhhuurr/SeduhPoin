package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"seduhpoin/internal/auth"
	"seduhpoin/internal/calc"
	"seduhpoin/internal/format"
	"seduhpoin/internal/models"
)

// memberRow adalah data member beserta tingkat dan total belanja 3 bulan.
//
// Tingkat TIDAK disimpan sebagai kolom di tabel member; ia dihitung ulang
// setiap kali ditampilkan dari transaksi 3 bulan terakhir.
type memberRow struct {
	Member  models.Member
	Tingkat string
	Belanja int64
}

// registerMember mendaftarkan rute CRUD member.
//
// Admin mengelola data member (termasuk-given kata sandi awal dan reset
// kata sandi); kasir hanya membaca daftar dan detail member supaya bisa
// mengenali pelanggan saatmelayani transaksi.
func (a *App) registerMember(mux *http.ServeMux) {
	mux.HandleFunc("GET /member", a.akses(peranStaff, a.daftarMember))
	mux.HandleFunc("GET /member/{id}", a.akses(peranStaff, a.detailMember))
	mux.HandleFunc("GET /member/tambah", a.akses(peranAdmin, a.formMember))
	mux.HandleFunc("POST /member/tambah", a.akses(peranAdmin, a.simpanMember))
	mux.HandleFunc("GET /member/{id}/edit", a.akses(peranAdmin, a.formMember))
	mux.HandleFunc("POST /member/{id}/edit", a.akses(peranAdmin, a.simpanMember))
	mux.HandleFunc("POST /member/{id}/hapus", a.akses(peranAdmin, a.hapusMember))
	mux.HandleFunc("POST /member/{id}/sandi", a.akses(peranAdmin, a.resetSandiMember))
}

// daftarMember menampilkan daftar member dengan tingkat dan belanja 3 bulan.
//
// Satu query agregat menghitung belanja seluruh member sekaligus (GROUP BY),
// sehingga tidak ada query per baris daftar.
func (a *App) daftarMember(w http.ResponseWriter, r *http.Request) {
	keyword := strings.TrimSpace(r.URL.Query().Get("q"))

	daftar, err := a.Store.DaftarMember(r.Context(), keyword)
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal memuat daftar member", err.Error())
		return
	}
	// Batas bawah rentang 3 bulan; ambang Gold Rp1.500.000 (lihat calc.TingkatMember).
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

	a.render(w, r, "member/list.html", View{
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
	// Nomor HP dapat diisi dari halaman transaksi (?no_hp=...): admin mengetik
	// nomor HP di form transaksi, lalu diarahkan ke sini untuk mendaftar.
	if raw := r.URL.Query().Get("no_hp"); raw != "" && mode == "tambah" {
		m.NoHP = raw
	}

	a.render(w, r, "member/form.html", View{
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
//
// Nomor HP adalah identitas member: unik dan hanya boleh berisi angka
// (validasi againAngka). Pencarian member pada form transaksi memakai nomor HP
// ini; hanya member terdaftar yang memperoleh dan menukar poin.
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

	// Validasi server-side nomor HP: wajib diisi, hanya angka, dan unik.
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

	// ID == 0 berarti data baru (tambah); selain itu berarti ubah.
	if m.ID == 0 {
		id, err := a.Store.SimpanMember(r.Context(), m)
		if err != nil {
			pesanError(w, http.StatusInternalServerError, "Gagal menyimpan member", err.Error())
			return
		}
		// Kata sandi awal (opsional): kalau diisi, member langsung punya
		// akun login portal. Kalau dikosongkan, member belum bisa login
		// sampai admin mengaturnya dari halaman detail member.
		if sandi := strings.TrimSpace(r.FormValue("sandi")); sandi != "" {
			if !auth.PasswordValid(sandi) {
				redirect(w, r, "/member/"+strconv.FormatInt(id, 10),
					"Member tersimpan, tetapi kata sandi minimal "+itoa(auth.PanjangMinimumSandi)+" karakter.")
				return
			}
			hash, err := auth.HashPassword(sandi)
			if err != nil {
				pesanError(w, http.StatusInternalServerError, "Gagal memproses kata sandi", err.Error())
				return
			}
			if err := a.Store.SetPasswordMember(r.Context(), id, hash); err != nil {
				pesanError(w, http.StatusInternalServerError, "Gagal menyimpan kata sandi member", err.Error())
				return
			}
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
//
// Tingkat dihitung on-the-fly dari total belanja 3 bulan terakhir atas
// transaksi berstatus SELESAI (transaksi BATAL tidak dihitung).
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
	a.render(w, r, "member/detail.html", View{
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
//
// Member yang dihapus tidak hilang dari riwayat: transaksi lamanya tetap
// menyimpan salinan nama dan nomor HP.
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

// resetSandiMember memasang atau mengganti kata sandi login seorang member.
//
// Kasir/admin boleh mengatur ulang sandi member (misalnya member lupa
// sandinya). Member yang sedang login ikut kehilangan sesi aktifnya karena
// kata sandinya berubah.
func (a *App) resetSandiMember(w http.ResponseWriter, r *http.Request) {
	id, err := idDariPath(r)
	if err != nil {
		pesanError(w, http.StatusBadRequest, "ID member tidak valid", err.Error())
		return
	}
	if err := r.ParseForm(); err != nil {
		pesanError(w, http.StatusBadRequest, "Data form tidak valid", err.Error())
		return
	}
	sandi := strings.TrimSpace(r.FormValue("sandi"))
	if !auth.PasswordValid(sandi) {
		redirect(w, r, "/member/"+strconv.FormatInt(id, 10),
			"Kata sandi minimal "+itoa(auth.PanjangMinimumSandi)+" karakter.")
		return
	}
	hash, err := auth.HashPassword(sandi)
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal memproses kata sandi", err.Error())
		return
	}
	if err := a.Store.SetPasswordMember(r.Context(), id, hash); err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal menyimpan kata sandi member", err.Error())
		return
	}
	redirectSukses(w, r, "/member/"+strconv.FormatInt(id, 10), "Kata sandi member berhasil diganti.")
}

// hanyaAngka memeriksa apakah seluruh karakter string berupa angka.
//
// Dipakai untuk memvalidasi nomor HP member (hanya angka, tanpa spasi,
// tanda hubung, atau awalan +62).
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
