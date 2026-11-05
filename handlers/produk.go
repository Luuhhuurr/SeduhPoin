package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"seduhpoin/internal/models"
)

// registerProduk mendaftarkan rute CRUD produk.
//
// Admin mengelola data produk; kasir hanya membaca katalog (dibutuhkan saat
// memilih item di form transaksi).
func (a *App) registerProduk(mux *http.ServeMux) {
	mux.HandleFunc("GET /produk", a.akses(peranStaff, a.daftarProduk))
	mux.HandleFunc("GET /produk/{id}", a.akses(peranStaff, a.detailProduk))
	mux.HandleFunc("GET /produk/tambah", a.akses(peranAdmin, a.formProduk))
	mux.HandleFunc("POST /produk/tambah", a.akses(peranAdmin, a.simpanProduk))
	mux.HandleFunc("GET /produk/{id}/edit", a.akses(peranAdmin, a.formProduk))
	mux.HandleFunc("POST /produk/{id}/edit", a.akses(peranAdmin, a.simpanProduk))
	mux.HandleFunc("POST /produk/{id}/hapus", a.akses(peranAdmin, a.hapusProduk))
}

// daftarProduk menampilkan katalog produk.
//
// Filter ?q= (kata kunci kode/nama) dan ?kategori= disusun di sini sebagai
// parameter query; seluruh SQL berada di internal/store.
func (a *App) daftarProduk(w http.ResponseWriter, r *http.Request) {
	keyword := strings.TrimSpace(r.URL.Query().Get("q"))
	kategori := strings.TrimSpace(r.URL.Query().Get("kategori"))

	daftar, err := a.Store.DaftarProduk(r.Context(), keyword, kategori)
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal memuat daftar produk", err.Error())
		return
	}
	// Daftar kategori diambil dari data produk yang ada, bukan dari tabel terpisah.
	kategoriList, err := a.Store.DaftarKategori(r.Context())
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal memuat daftar kategori", err.Error())
		return
	}
	a.render(w, r, "produk/list.html", View{
		Title: "Data Produk",
		Menu:  "produk",
		Error: ambilError(r),
		Data: map[string]any{
			"Daftar":        daftar,
			"Kategori":      kategoriList,
			"Keyword":       keyword,
			"KategoriAktif": kategori,
		},
	})
}

// formProduk menampilkan form tambah / ubah produk.
//
// Kategori yang sudah dipakai produk lain juga ditampilkan sebagai pilihan,
// sehingga kategori tetap konsisten antar transaksi (kategori dipakai promo B2G1).
func (a *App) formProduk(w http.ResponseWriter, r *http.Request) {
	var p models.Produk
	mode := "tambah"

	if raw := r.PathValue("id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			pesanError(w, http.StatusBadRequest, "ID produk tidak valid", raw)
			return
		}
		p, err = a.Store.AmbilProduk(r.Context(), id)
		if err != nil {
			pesanError(w, http.StatusNotFound, "Produk tidak ditemukan", err.Error())
			return
		}
		mode = "edit"
	}
	kategoriList, err := a.Store.DaftarKategori(r.Context())
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal memuat daftar kategori", err.Error())
		return
	}
	a.render(w, r, "produk/form.html", View{
		Title: "Form Produk",
		Menu:  "produk",
		Error: ambilError(r),
		Data: map[string]any{
			"Produk":   p,
			"Mode":     mode,
			"Kategori": kategoriList,
		},
	})
}

// simpanProduk memproses penyimpanan produk (tambah maupun ubah).
//
// Harga memakai int64 (rupiah bulat). Nominal tidak pernah memakai float
// sehingga tidak ada galat pembulatan pada perhitungan promo dan poin.
func (a *App) simpanProduk(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		pesanError(w, http.StatusBadRequest, "Data form tidak valid", err.Error())
		return
	}

	p := models.Produk{
		Kode:     strings.TrimSpace(r.FormValue("kode")),
		Nama:     strings.TrimSpace(r.FormValue("nama")),
		Kategori: strings.TrimSpace(r.FormValue("kategori")),
		Harga:    parseInt64(r.FormValue("harga"), 0),
	}
	if raw := r.PathValue("id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			pesanError(w, http.StatusBadRequest, "ID produk tidak valid", raw)
			return
		}
		p.ID = id
	}

	// Validasi server-side: semua aturan ditegakkan di sini, bukan hanya di HTML.
	if p.Kode == "" {
		redirect(w, r, "/produk/tambah", "Kode produk wajib diisi.")
		return
	}
	if p.Nama == "" {
		redirect(w, r, "/produk/tambah", "Nama produk wajib diisi.")
		return
	}
	if p.Kategori == "" {
		redirect(w, r, "/produk/tambah", "Kategori produk wajib diisi.")
		return
	}
	// Harga harus bulat positif; 0 atau negatif ditolak karena akan merusak
	// perhitungan subtotal dan promo.
	if p.Harga <= 0 {
		redirect(w, r, "/produk/tambah", "Harga harus berupa bilangan bulat lebih besar dari 0.")
		return
	}
	dipakai, err := a.Store.KodeProdukDipakai(r.Context(), p.Kode, p.ID)
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal memeriksa kode produk", err.Error())
		return
	}
	if dipakai {
		redirect(w, r, "/produk/tambah", "Kode produk sudah dipakai. Gunakan kode lain.")
		return
	}

	// ID == 0 berarti data baru (tambah); selain itu berarti ubah.
	if p.ID == 0 {
		id, err := a.Store.SimpanProduk(r.Context(), p)
		if err != nil {
			pesanError(w, http.StatusInternalServerError, "Gagal menyimpan produk", err.Error())
			return
		}
		redirect(w, r, "/produk/"+strconv.FormatInt(id, 10), "")
		return
	}
	if err := a.Store.UbahProduk(r.Context(), p); err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal mengubah produk", err.Error())
		return
	}
	redirect(w, r, "/produk/"+strconv.FormatInt(p.ID, 10), "")
}

// detailProduk menampilkan rincian satu produk.
func (a *App) detailProduk(w http.ResponseWriter, r *http.Request) {
	id, err := idDariPath(r)
	if err != nil {
		pesanError(w, http.StatusBadRequest, "ID produk tidak valid", err.Error())
		return
	}
	p, err := a.Store.AmbilProduk(r.Context(), id)
	if err != nil {
		pesanError(w, http.StatusNotFound, "Produk tidak ditemukan", err.Error())
		return
	}
	a.render(w, r, "produk/detail.html", View{
		Title: "Detail Produk",
		Menu:  "produk",
		Data: map[string]any{
			"Produk": p,
		},
	})
}

// hapusProduk melakukan soft delete produk.
//
// Baris tabel tidak dihapus melainkan ditandai deleted_at, supaya transaksi
// lama tetap menampilkan nama & harga produk yang waktu itu digunakan.
func (a *App) hapusProduk(w http.ResponseWriter, r *http.Request) {
	id, err := idDariPath(r)
	if err != nil {
		pesanError(w, http.StatusBadRequest, "ID produk tidak valid", err.Error())
		return
	}
	if err := a.Store.HapusProduk(r.Context(), id); err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal menghapus produk", err.Error())
		return
	}
	redirect(w, r, "/produk", "")
}
