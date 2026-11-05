package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"seduhpoin/internal/models"
)

// registerCabang mendaftarkan rute CRUD cabang.
//
// Pola rute memakai pola net/http Go 1.22+ sehingga metode dan parameter path
// ditulis langsung di string ("GET /cabang/{id}"), tanpa router pihak ketiga.
//
// Pembagian hak akses: admin mengelola data cabang (tambah, ubah, hapus),
// sedangkan kasir hanya boleh membaca daftar dan detail cabang.
func (a *App) registerCabang(mux *http.ServeMux) {
	mux.HandleFunc("GET /cabang", a.akses(peranStaff, a.daftarCabang))
	mux.HandleFunc("GET /cabang/{id}", a.akses(peranStaff, a.detailCabang))
	mux.HandleFunc("GET /cabang/tambah", a.akses(peranAdmin, a.formCabang))
	mux.HandleFunc("POST /cabang/tambah", a.akses(peranAdmin, a.simpanCabang))
	mux.HandleFunc("GET /cabang/{id}/edit", a.akses(peranAdmin, a.formCabang))
	mux.HandleFunc("POST /cabang/{id}/edit", a.akses(peranAdmin, a.simpanCabang))
	mux.HandleFunc("POST /cabang/{id}/hapus", a.akses(peranAdmin, a.hapusCabang))
}

// daftarCabang menampilkan daftar seluruh cabang.
//
// Pencarian memakai parameter ?q= dan dicocokkan pada kode maupun nama.
func (a *App) daftarCabang(w http.ResponseWriter, r *http.Request) {
	keyword := strings.TrimSpace(r.URL.Query().Get("q"))
	daftar, err := a.Store.DaftarCabang(r.Context(), keyword)
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal memuat daftar cabang", err.Error())
		return
	}
	a.render(w, r, "cabang/list.html", View{
		Title: "Data Cabang",
		Menu:  "cabang",
		Error: ambilError(r),
		Data: map[string]any{
			"Daftar":  daftar,
			"Keyword": keyword,
		},
	})
}

// formCabang menampilkan form tambah / ubah cabang.
//
// Satu handler dipakai untuk dua keperluan: bila path memuat {id} maka mode
// "edit" (data lama dimuat), selain itu mode "tambah".
func (a *App) formCabang(w http.ResponseWriter, r *http.Request) {
	var c models.Cabang
	var mode string

	if raw := r.PathValue("id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			pesanError(w, http.StatusBadRequest, "ID cabang tidak valid", raw)
			return
		}
		c, err = a.Store.AmbilCabang(r.Context(), id)
		if err != nil {
			pesanError(w, http.StatusNotFound, "Cabang tidak ditemukan", err.Error())
			return
		}
		mode = "edit"
	} else {
		mode = "tambah"
	}

	a.render(w, r, "cabang/form.html", View{
		Title: "Form Cabang",
		Menu:  "cabang",
		Error: ambilError(r),
		Data: map[string]any{
			"Cabang": c,
			"Mode":   mode,
		},
	})
}

// simpanCabang memproses penyimpanan cabang (tambah maupun ubah).
//
// Pola yang dipakai di semua master data: validasi di sisi server (bukan hanya
// atribut "required" di HTML), lalu bila gagal pengguna diarahkan kembali ke
// form dengan pesan ramah. Server tidak pernah dimatikan karena input salah.
func (a *App) simpanCabang(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		pesanError(w, http.StatusBadRequest, "Data form tidak valid", err.Error())
		return
	}

	c := models.Cabang{
		Kode:    strings.TrimSpace(r.FormValue("kode")),
		Nama:    strings.TrimSpace(r.FormValue("nama")),
		Alamat:  strings.TrimSpace(r.FormValue("alamat")),
		Telepon: strings.TrimSpace(r.FormValue("telepon")),
	}
	if raw := r.PathValue("id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			pesanError(w, http.StatusBadRequest, "ID cabang tidak valid", raw)
			return
		}
		c.ID = id
	}

	if c.Kode == "" {
		redirect(w, r, "/cabang/tambah", "Kode cabang wajib diisi.")
		return
	}
	if c.Nama == "" {
		redirect(w, r, "/cabang/tambah", "Nama cabang wajib diisi.")
		return
	}
	// Kode cabang bersifat unik; dicek lebih dulu agar pesan yang muncul
	// ramah, bukan error unik dari database.
	dipakai, err := a.Store.KodeCabangDipakai(r.Context(), c.Kode, c.ID)
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal memeriksa kode cabang", err.Error())
		return
	}
	if dipakai {
		redirect(w, r, "/cabang/tambah", "Kode cabang sudah dipakai. Gunakan kode lain.")
		return
	}

	// ID == 0 berarti data baru (tambah); selain itu berarti ubah.
	if c.ID == 0 {
		id, err := a.Store.SimpanCabang(r.Context(), c)
		if err != nil {
			pesanError(w, http.StatusInternalServerError, "Gagal menyimpan cabang", err.Error())
			return
		}
		redirect(w, r, "/cabang/"+strconv.FormatInt(id, 10), "")
		return
	}
	if err := a.Store.UbahCabang(r.Context(), c); err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal mengubah cabang", err.Error())
		return
	}
	redirect(w, r, "/cabang/"+strconv.FormatInt(c.ID, 10), "")
}

// detailCabang menampilkan rincian satu cabang.
func (a *App) detailCabang(w http.ResponseWriter, r *http.Request) {
	id, err := idDariPath(r)
	if err != nil {
		pesanError(w, http.StatusBadRequest, "ID cabang tidak valid", err.Error())
		return
	}
	c, err := a.Store.AmbilCabang(r.Context(), id)
	if err != nil {
		pesanError(w, http.StatusNotFound, "Cabang tidak ditemukan", err.Error())
		return
	}
	a.render(w, r, "cabang/detail.html", View{
		Title: "Detail Cabang",
		Menu:  "cabang",
		Data: map[string]any{
			"Cabang": c,
		},
	})
}

// hapusCabang melakukan soft delete cabang.
//
// Baris tabel tidak dihapus, melainkan ditandai deleted_at. Ini diperlukan
// karena transaksi lama harus tetap menampilkan nama cabang/produk/member
// sewaktu-waktu.
func (a *App) hapusCabang(w http.ResponseWriter, r *http.Request) {
	id, err := idDariPath(r)
	if err != nil {
		pesanError(w, http.StatusBadRequest, "ID cabang tidak valid", err.Error())
		return
	}
	if err := a.Store.HapusCabang(r.Context(), id); err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal menghapus cabang", err.Error())
		return
	}
	redirect(w, r, "/cabang", "")
}
