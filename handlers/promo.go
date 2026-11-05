package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"seduhpoin/internal/format"
	"seduhpoin/internal/models"
	"seduhpoin/internal/store"
)

// registerPromo mendaftarkan rute CRUD promo dan voucher.
//
// Admin mengelola promo (tambah, ubah, status, hapus); kasir hanya membaca
// daftar promo dan voucher karena penerapannya terjadi di form transaksi.
func (a *App) registerPromo(mux *http.ServeMux) {
	mux.HandleFunc("GET /promo", a.akses(peranStaff, a.daftarPromo))
	mux.HandleFunc("GET /promo/{id}", a.akses(peranStaff, a.detailPromo))
	mux.HandleFunc("GET /promo/tambah", a.akses(peranAdmin, a.formPromo))
	mux.HandleFunc("POST /promo/tambah", a.akses(peranAdmin, a.simpanPromo))
	mux.HandleFunc("GET /promo/{id}/edit", a.akses(peranAdmin, a.formPromo))
	mux.HandleFunc("POST /promo/{id}/edit", a.akses(peranAdmin, a.simpanPromo))
	mux.HandleFunc("POST /promo/{id}/status", a.akses(peranAdmin, a.ubahStatusPromo))
	mux.HandleFunc("POST /promo/{id}/hapus", a.akses(peranAdmin, a.hapusPromo))
}

// daftarPromo menampilkan daftar promo dan daftar voucher.
//
// Kuota voucher ditampilkan sebagai sisa (kuota - terpakai) sehingga kasir
// dapat melihat berapa voucher yang masih bisa dipakai.
func (a *App) daftarPromo(w http.ResponseWriter, r *http.Request) {
	keyword := strings.TrimSpace(r.URL.Query().Get("q"))
	jenis := strings.TrimSpace(r.URL.Query().Get("jenis"))

	daftar, err := a.Store.DaftarPromo(r.Context(), keyword, jenis)
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal memuat daftar promo", err.Error())
		return
	}
	voucher, err := a.Store.DaftarVoucher(r.Context())
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal memuat daftar voucher", err.Error())
		return
	}
	cabang, err := a.Store.DaftarCabang(r.Context(), "")
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal memuat daftar cabang", err.Error())
		return
	}
	a.render(w, r, "promo/list.html", View{
		Title: "Promo & Voucher",
		Menu:  "promo",
		Error: ambilError(r),
		Data: map[string]any{
			"Daftar":  daftar,
			"Voucher": voucher,
			"Cabang":  cabang,
			"Keyword": keyword,
			"Jenis":   jenis,
		},
	})
}

// formPromo menampilkan form tambah / ubah promo.
func (a *App) formPromo(w http.ResponseWriter, r *http.Request) {
	var p models.Promo
	mode := "tambah"
	kodeVoucher := ""
	kuota := 0

	if raw := r.PathValue("id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			pesanError(w, http.StatusBadRequest, "ID promo tidak valid", raw)
			return
		}
		p, err = a.Store.AmbilPromo(r.Context(), id)
		if err != nil {
			pesanError(w, http.StatusNotFound, "Promo tidak ditemukan", err.Error())
			return
		}
		mode = "edit"
		if p.Jenis == models.JenisVoucher {
			if v, err := a.Store.AmbilVoucherPromo(r.Context(), id); err == nil {
				kodeVoucher = v.Kode
				kuota = v.Kuota
			}
		}
	} else {
		p.TglMulai = format.HariIni()
		p.TglSelesai = format.HariIni().AddDate(0, 3, 0)
		p.Aktif = true
	}

	produk, err := a.Store.DaftarProduk(r.Context(), "", "")
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal memuat daftar produk", err.Error())
		return
	}
	cabang, err := a.Store.DaftarCabang(r.Context(), "")
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal memuat daftar cabang", err.Error())
		return
	}
	kategori, err := a.Store.DaftarKategori(r.Context())
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal memuat daftar kategori", err.Error())
		return
	}

	a.render(w, r, "promo/form.html", View{
		Title: "Form Promo",
		Menu:  "promo",
		Error: ambilError(r),
		Data: map[string]any{
			"Promo":       p,
			"Mode":        mode,
			"Produk":      produk,
			"Cabang":      cabang,
			"Kategori":    kategori,
			"KodeVoucher": kodeVoucher,
			"Kuota":       kuota,
		},
	})
}

// simpanPromo memproses penyimpanan promo (tambah maupun ubah).
func (a *App) simpanPromo(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		pesanError(w, http.StatusBadRequest, "Data form tidak valid", err.Error())
		return
	}

	p := models.Promo{
		Kode:         strings.TrimSpace(r.FormValue("kode")),
		Nama:         strings.TrimSpace(r.FormValue("nama")),
		Jenis:        strings.TrimSpace(r.FormValue("jenis")),
		NilaiPersen:  int(parseInt64(r.FormValue("nilai_persen"), 0)),
		NilaiNominal: parseInt64(r.FormValue("nilai_nominal"), 0),
		MinBelanja:   parseInt64(r.FormValue("min_belanja"), 0),
		Aktif:        r.FormValue("aktif") == "1",
	}
	if raw := r.PathValue("id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			pesanError(w, http.StatusBadRequest, "ID promo tidak valid", raw)
			return
		}
		p.ID = id
	}

	tglMulai, err := parseTanggal(r.FormValue("tgl_mulai"))
	if err != nil {
		redirect(w, r, targetPromo(p.ID), "Tanggal mulai promo tidak valid.")
		return
	}
	tglSelesai, err := parseTanggal(r.FormValue("tgl_selesai"))
	if err != nil {
		redirect(w, r, targetPromo(p.ID), "Tanggal selesai promo tidak valid.")
		return
	}
	p.TglMulai, p.TglSelesai = tglMulai, tglSelesai
	if p.TglSelesai.Before(p.TglMulai) {
		redirect(w, r, targetPromo(p.ID), "Tanggal selesai tidak boleh lebih awal dari tanggal mulai.")
		return
	}

	if jam := strings.TrimSpace(r.FormValue("jam_mulai")); jam != "" {
		jam, err := normalisasiJam(jam)
		if err != nil {
			redirect(w, r, targetPromo(p.ID), "Jam mulai tidak valid (format HH:MM).")
			return
		}
		p.JamMulai = jam
	}
	if jam := strings.TrimSpace(r.FormValue("jam_selesai")); jam != "" {
		jam, err := normalisasiJam(jam)
		if err != nil {
			redirect(w, r, targetPromo(p.ID), "Jam selesai tidak valid (format HH:MM).")
			return
		}
		p.JamSelesai = jam
	}
	if p.JamMulai != "" && p.JamSelesai != "" && p.JamSelesai <= p.JamMulai {
		redirect(w, r, targetPromo(p.ID), "Jam selesai harus lebih besar dari jam mulai.")
		return
	}

	// Cabang tujuan promo (kosong = semua cabang).
	for _, raw := range r.Form["cabang_ids"] {
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil {
			p.CabangIDs = append(p.CabangIDs, id)
		}
	}

	// Aturan detail sesuai jenis promo.
	//
	// Empat jenis promo (BR-09): diskon produk (%), potongan minimal belanja,
	// beli 2 gratis 1 per kategori, dan voucher berkode.
	switch p.Jenis {
	case models.JenisDiskonProduk:
		produkIDs := r.Form["produk_ids"]
		for _, raw := range produkIDs {
			id, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				continue
			}
			p.DiskonProduk = append(p.DiskonProduk, models.PromoDiskonProduk{
				ProdukID: id,
				Persen:   p.NilaiPersen,
			})
		}
	case models.JenisB2G1:
		for _, k := range r.Form["kategori"] {
			if k = strings.TrimSpace(k); k != "" {
				p.KategoriB2G1 = append(p.KategoriB2G1, k)
			}
		}
	}

	// Validasi wajib per jenis promo.
	if err := validasiPromo(p, r); err != "" {
		redirect(w, r, targetPromo(p.ID), err)
		return
	}
	dipakai, err := a.Store.KodePromoDipakai(r.Context(), p.Kode, p.ID)
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal memeriksa kode promo", err.Error())
		return
	}
	if dipakai {
		redirect(w, r, targetPromo(p.ID), "Kode promo sudah dipakai. Gunakan kode lain.")
		return
	}

	kodeVoucher := strings.TrimSpace(r.FormValue("kode_voucher"))
	kuota := int(parseInt64(r.FormValue("kuota"), 0))
	if p.Jenis == models.JenisVoucher {
		voucherDipakai, err := a.Store.KodeVoucherDipakai(r.Context(), kodeVoucher, p.ID)
		if err != nil {
			pesanError(w, http.StatusInternalServerError, "Gagal memeriksa kode voucher", err.Error())
			return
		}
		if voucherDipakai {
			redirect(w, r, targetPromo(p.ID), "Kode voucher sudah dipakai promo lain.")
			return
		}
	}

	if p.ID == 0 {
		id, err := a.Store.SimpanPromo(r.Context(), p, kodeVoucher, kuota)
		if err != nil {
			pesanError(w, http.StatusInternalServerError, "Gagal menyimpan promo", err.Error())
			return
		}
		redirect(w, r, "/promo/"+strconv.FormatInt(id, 10), "")
		return
	}
	if err := a.Store.UbahPromo(r.Context(), p, kodeVoucher, kuota); err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal mengubah promo", err.Error())
		return
	}
	redirect(w, r, "/promo/"+strconv.FormatInt(p.ID, 10), "")
}

// validasiPromo memeriksa kelengkapan data promo sesuai jenisnya.
//
// Validasi dilakukan di server (bukan hanya di sisi browser) karena setiap
// jenis promo membutuhkan data yang berbeda (BR-09). Pesan dikembalikan dalam
// bentuk kalimat Bahasa Indonesia agar bisa langsung ditampilkan ke pengguna.
func validasiPromo(p models.Promo, r *http.Request) string {
	if p.Kode == "" {
		return "Kode promo wajib diisi."
	}
	if p.Nama == "" {
		return "Nama promo wajib diisi."
	}
	switch p.Jenis {
	case models.JenisDiskonProduk:
		if p.NilaiPersen <= 0 || p.NilaiPersen > 100 {
			return "Persen diskon harus antara 1 sampai 100."
		}
		if len(p.DiskonProduk) == 0 {
			return "Pilih minimal satu produk yang mendapat diskon."
		}
	case models.JenisPotonganMinBelanja:
		if p.NilaiNominal <= 0 {
			return "Nilai potongan harus lebih besar dari 0."
		}
		if p.MinBelanja <= 0 {
			return "Minimal belanja harus lebih besar dari 0."
		}
	case models.JenisB2G1:
		if len(p.KategoriB2G1) == 0 {
			return "Pilih minimal satu kategori untuk promo beli 2 gratis 1."
		}
	case models.JenisVoucher:
		if strings.TrimSpace(r.FormValue("kode_voucher")) == "" {
			return "Kode voucher wajib diisi."
		}
		if parseInt64(r.FormValue("kuota"), 0) <= 0 {
			return "Kuota voucher harus lebih besar dari 0."
		}
	default:
		return "Jenis promo tidak dikenal."
	}
	return ""
}

// targetPromo mengembalikan URL form promo sesuai mode tambah atau ubah.
func targetPromo(id int64) string {
	if id == 0 {
		return "/promo/tambah"
	}
	return "/promo/" + strconv.FormatInt(id, 10) + "/edit"
}

// detailPromo menampilkan rincian satu promo.
func (a *App) detailPromo(w http.ResponseWriter, r *http.Request) {
	id, err := idDariPath(r)
	if err != nil {
		pesanError(w, http.StatusBadRequest, "ID promo tidak valid", err.Error())
		return
	}
	p, err := a.Store.AmbilPromo(r.Context(), id)
	if err != nil {
		pesanError(w, http.StatusNotFound, "Promo tidak ditemukan", err.Error())
		return
	}
	dipakai, err := a.Store.PromoSudahDipakai(r.Context(), id)
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal memeriksa pemakaian promo", err.Error())
		return
	}
	a.render(w, r, "promo/detail.html", View{
		Title: "Detail Promo",
		Menu:  "promo",
		Data: map[string]any{
			"Promo":   p,
			"Dipakai": dipakai,
		},
	})
}

// ubahStatusPromo mengaktifkan atau menonaktifkan promo.
func (a *App) ubahStatusPromo(w http.ResponseWriter, r *http.Request) {
	id, err := idDariPath(r)
	if err != nil {
		pesanError(w, http.StatusBadRequest, "ID promo tidak valid", err.Error())
		return
	}
	aktif := r.FormValue("aktif") == "1"
	if err := a.Store.SetAktifPromo(r.Context(), id, aktif); err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal mengubah status promo", err.Error())
		return
	}
	redirect(w, r, "/promo/"+strconv.FormatInt(id, 10), "")
}

// hapusPromo melakukan soft delete promo. Promo yang sudah dipakai hanya
// dapat dinonaktifkan, tidak dapat dihapus.
//
// Aturan ini dijaga di store.HapusPromo (yang mengecek PromoSudahDipakai),
// sehingga aturan tetap berlaku meskipun dipanggil dari tempat lain.
func (a *App) hapusPromo(w http.ResponseWriter, r *http.Request) {
	id, err := idDariPath(r)
	if err != nil {
		pesanError(w, http.StatusBadRequest, "ID promo tidak valid", err.Error())
		return
	}
	err = a.Store.HapusPromo(r.Context(), id)
	if errors.Is(err, store.ErrPromoSudahDipakai) {
		redirect(w, r, "/promo/"+strconv.FormatInt(id, 10),
			"Promo sudah pernah dipakai sehingga tidak dapat dihapus. Nonaktifkan promo saja.")
		return
	}
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal menghapus promo", err.Error())
		return
	}
	redirect(w, r, "/promo", "")
}
