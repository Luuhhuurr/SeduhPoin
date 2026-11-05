package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"seduhpoin/internal/auth"
	"seduhpoin/internal/format"
	"seduhpoin/internal/models"
)

// registerPengguna mendaftarkan rute pengelolaan akun staf.
//
// Seluruh rute di sini memakai peran ADMIN. Kasir memang boleh mengelola
// master data dan member, tetapi TIDAK boleh membuat atau mengubah akun
// staf.
func (a *App) registerPengguna(mux *http.ServeMux) {
	mux.HandleFunc("GET /pengguna", a.akses(peranAdmin, a.daftarPengguna))
	mux.HandleFunc("GET /pengguna/tambah", a.akses(peranAdmin, a.formPengguna))
	mux.HandleFunc("POST /pengguna/tambah", a.akses(peranAdmin, a.simpanPengguna))
	mux.HandleFunc("GET /pengguna/{id}", a.akses(peranAdmin, a.detailPengguna))
	mux.HandleFunc("GET /pengguna/{id}/edit", a.akses(peranAdmin, a.formPengguna))
	mux.HandleFunc("POST /pengguna/{id}/edit", a.akses(peranAdmin, a.simpanPengguna))
	mux.HandleFunc("POST /pengguna/{id}/sandi", a.akses(peranAdmin, a.resetSandiPengguna))
	mux.HandleFunc("POST /pengguna/{id}/status", a.akses(peranAdmin, a.ubahStatusPengguna))
}

// daftarPengguna menampilkan seluruh akun staf.
func (a *App) daftarPengguna(w http.ResponseWriter, r *http.Request) {
	daftar, err := a.Store.DaftarPengguna(r.Context())
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal memuat daftar akun", err.Error())
		return
	}
	a.render(w, r, "pengguna/list.html", View{
		Title: "Kelola Akun Pengguna",
		Menu:  "pengguna",
		Error: ambilError(r),
		Data: map[string]any{
			"Daftar": daftar,
		},
	})
}

// formPengguna menampilkan form tambah / ubah akun staf.
func (a *App) formPengguna(w http.ResponseWriter, r *http.Request) {
	var p models.Pengguna
	p.Role = models.RoleKasir
	mode := "tambah"

	if raw := r.PathValue("id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			pesanError(w, http.StatusBadRequest, "ID pengguna tidak valid", raw)
			return
		}
		p, err = a.Store.AmbilPenggunaByID(r.Context(), id)
		if err != nil {
			pesanError(w, http.StatusNotFound, "Akun tidak ditemukan", err.Error())
			return
		}
		mode = "edit"
	}

	a.render(w, r, "pengguna/form.html", View{
		Title: "Form Akun Pengguna",
		Menu:  "pengguna",
		Error: ambilError(r),
		Data: map[string]any{
			"Pengguna": p,
			"Mode":     mode,
		},
	})
}

// simpanPengguna memproses penyimpanan akun staf (tambah maupun ubah).
//
// Saat menambah, kata sandi wajib diisi. Saat mengubah, kata sandi boleh
// dikosongkan — perubahan sandi punya jalurnya sendiri (reset sandi) agar
// jejaknya jelas.
func (a *App) simpanPengguna(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		pesanError(w, http.StatusBadRequest, "Data form tidak valid", err.Error())
		return
	}

	username := bersihkanUsername(r.FormValue("username"))
	nama := strings.TrimSpace(r.FormValue("nama"))
	role := strings.ToUpper(strings.TrimSpace(r.FormValue("role")))
	sandi := r.FormValue("sandi")

	if role != models.RoleAdmin && role != models.RoleKasir {
		redirect(w, r, "/pengguna/tambah", "Peran harus ADMIN atau KASIR.")
		return
	}

	var id int64
	mode := "tambah"
	tujuan := "/pengguna/tambah"
	if raw := r.PathValue("id"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			pesanError(w, http.StatusBadRequest, "ID pengguna tidak valid", raw)
			return
		}
		id = parsed
		mode = "edit"
		tujuan = "/pengguna/" + strconv.FormatInt(id, 10)
	}

	if username == "" {
		redirect(w, r, tujuan, "Username wajib diisi.")
		return
	}
	if nama == "" {
		redirect(w, r, tujuan, "Nama wajib diisi.")
		return
	}
	dipakai, err := a.Store.UsernameDipakai(r.Context(), username, id)
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal memeriksa username", err.Error())
		return
	}
	if dipakai {
		redirect(w, r, tujuan, "Username sudah dipakai akun lain.")
		return
	}

	if mode == "tambah" {
		if !auth.PasswordValid(sandi) {
			redirect(w, r, tujuan, "Kata sandi minimal "+itoa(auth.PanjangMinimumSandi)+" karakter.")
			return
		}
		hash, err := auth.HashPassword(sandi)
		if err != nil {
			pesanError(w, http.StatusInternalServerError, "Gagal memproses kata sandi", err.Error())
			return
		}
		baru := models.Pengguna{
			Username:     username,
			Nama:         nama,
			PasswordHash: hash,
			Role:         role,
			Aktif:        true,
			CreatedAt:    format.WIB(),
		}
		// Akun baru wajib mengganti sandi awal pada login pertama.
		baru.HarusGantiSandi = true
		idBaru, err := a.Store.SimpanPengguna(r.Context(), baru)
		if err != nil {
			pesanError(w, http.StatusInternalServerError, "Gagal menyimpan akun", err.Error())
			return
		}
		redirectSukses(w, r, "/pengguna/"+strconv.FormatInt(idBaru, 10), "Akun "+username+" berhasil dibuat.")
		return
	}

	// Mode ubah: nama, peran, dan status aktif.
	sekarang, err := a.Store.AmbilPenggunaByID(r.Context(), id)
	if err != nil {
		pesanError(w, http.StatusNotFound, "Akun tidak ditemukan", err.Error())
		return
	}
	sekarang.Nama = nama
	sekarang.Role = role

	// Admin tidak boleh menurunkan peran akunnya sendiri menjadi KASIR,
	// karena ada kalanya ia harus tetap bisa mengelola akun staf.
	if id == identitasDari(r).PenggunaID && role != models.RoleAdmin {
		redirect(w, r, tujuan, "Peran akun Anda sendiri tidak bisa diturunkan.")
		return
	}
	if err := a.Store.UbahPengguna(r.Context(), sekarang); err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal mengubah akun", err.Error())
		return
	}
	redirectSukses(w, r, tujuan, "Akun berhasil diperbarui.")
}

// resetSandiPengguna mengganti kata sandi sebuah akun staf.
//
// Sesi lama milik akun tersebut ikut dicabut supaya sandi baru langsung
// berlaku dan tidak ada sesi yang menetap dengan sandi lama.
func (a *App) resetSandiPengguna(w http.ResponseWriter, r *http.Request) {
	id, err := idDariPath(r)
	if err != nil {
		pesanError(w, http.StatusBadRequest, "ID pengguna tidak valid", err.Error())
		return
	}
	if err := r.ParseForm(); err != nil {
		pesanError(w, http.StatusBadRequest, "Data form tidak valid", err.Error())
		return
	}
	sandi := r.FormValue("sandi")
	if !auth.PasswordValid(sandi) {
		redirect(w, r, "/pengguna/"+strconv.FormatInt(id, 10),
			"Kata sandi minimal "+itoa(auth.PanjangMinimumSandi)+" karakter.")
		return
	}
	hash, err := auth.HashPassword(sandi)
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal memproses kata sandi", err.Error())
		return
	}
	// Reset sandi berarti pengguna wajib menggantinya sendiri pada login berikutnya.
	if err := a.Store.SetPasswordPengguna(r.Context(), id, hash, true); err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal menyimpan kata sandi", err.Error())
		return
	}
	_ = a.Store.HapusSemuaSesiPengguna(r.Context(), id)
	redirectSukses(w, r, "/pengguna/"+strconv.FormatInt(id, 10), "Kata sandi berhasil diganti.")
}

// ubahStatusPengguna mengaktifkan atau menonaktifkan akun staf.
//
// Akun yang dinonaktifkan otomatis kehilangan sesi aktifnya (diperiksa
// ulang setiap request) sehingga tidak bisa dipakai meski cookie masih ada.
func (a *App) ubahStatusPengguna(w http.ResponseWriter, r *http.Request) {
	id, err := idDariPath(r)
	if err != nil {
		pesanError(w, http.StatusBadRequest, "ID pengguna tidak valid", err.Error())
		return
	}
	if err := r.ParseForm(); err != nil {
		pesanError(w, http.StatusBadRequest, "Data form tidak valid", err.Error())
		return
	}
	aktif := r.FormValue("aktif") == "1"
	if id == identitasDari(r).PenggunaID && !aktif {
		redirect(w, r, "/pengguna/"+strconv.FormatInt(id, 10), "Akun Anda sendiri tidak bisa dinonaktifkan.")
		return
	}
	if err := a.Store.SetAktifPengguna(r.Context(), id, aktif); err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal mengubah status akun", err.Error())
		return
	}
	if !aktif {
		_ = a.Store.HapusSemuaSesiPengguna(r.Context(), id)
	}
	pesan := "Akun diaktifkan."
	if !aktif {
		pesan = "Akun dinonaktifkan dan semua sesinya dicabut."
	}
	redirectSukses(w, r, "/pengguna/"+strconv.FormatInt(id, 10), pesan)
}

// detailPengguna menampilkan rincian satu akun staf.
func (a *App) detailPengguna(w http.ResponseWriter, r *http.Request) {
	id, err := idDariPath(r)
	if err != nil {
		pesanError(w, http.StatusBadRequest, "ID pengguna tidak valid", err.Error())
		return
	}
	p, err := a.Store.AmbilPenggunaByID(r.Context(), id)
	if err != nil {
		pesanError(w, http.StatusNotFound, "Akun tidak ditemukan", err.Error())
		return
	}
	a.render(w, r, "pengguna/detail.html", View{
		Title: "Detail Akun Pengguna",
		Menu:  "pengguna",
		Data: map[string]any{
			"Pengguna": p,
		},
	})
}
