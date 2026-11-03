package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"seduhpoin/internal/store"
)

// registerRiwayat mendaftarkan rute riwayat transaksi.
func (a *App) registerRiwayat(mux *http.ServeMux) {
	mux.HandleFunc("GET /riwayat", a.withDB(a.daftarRiwayat))
	mux.HandleFunc("GET /riwayat/{id}", a.withDB(a.detailRiwayat))
	mux.HandleFunc("POST /riwayat/{id}/batal", a.withDB(a.batalkanTransaksi))
}

// daftarRiwayat menampilkan daftar transaksi dengan filter tanggal dan cabang.
func (a *App) daftarRiwayat(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()

	dari, err := parseTanggalOpsional(query.Get("dari"))
	if err != nil {
		redirect(w, r, "/riwayat", "Tanggal awal tidak valid. Gunakan format YYYY-MM-DD.")
		return
	}
	sampai, err := parseTanggalOpsional(query.Get("sampai"))
	if err != nil {
		redirect(w, r, "/riwayat", "Tanggal akhir tidak valid. Gunakan format YYYY-MM-DD.")
		return
	}
	// Batas atas bersifat inklusif, sedangkan query memakai "< ?".
	var sampaiT *time.Time
	if !sampai.IsZero() {
		besok := sampai.AddDate(0, 0, 1)
		sampaiT = &besok
	}
	var dariT *time.Time
	if !dari.IsZero() {
		dariT = &dari
	}
	cabangID := parseInt64(query.Get("cabang_id"), 0)

	transaksi, err := a.Store.DaftarTransaksi(ctx, dariT, sampaiT, cabangID)
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal memuat riwayat transaksi", err.Error())
		return
	}
	cabang, err := a.Store.DaftarCabang(ctx, "")
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal memuat daftar cabang", err.Error())
		return
	}

	var total int64
	for _, t := range transaksi {
		total += t.TotalBayar
	}

	a.render(w, "riwayat/list.html", View{
		Title: "Riwayat Transaksi",
		Menu:  "riwayat",
		Error: ambilError(r),
		Data: map[string]any{
			"Transaksi": transaksi,
			"Cabang":    cabang,
			"Total":     total,
			"Dari":      query.Get("dari"),
			"Sampai":    query.Get("sampai"),
			"CabangID":  cabangID,
			"Sukses":    ambilSukses(r),
		},
	})
}

// detailRiwayat menampilkan detail satu transaksi beserta rinciannya.
func (a *App) detailRiwayat(w http.ResponseWriter, r *http.Request) {
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

	a.render(w, "riwayat/detail.html", View{
		Title: "Transaksi " + transaksi.Kode,
		Menu:  "riwayat",
		Error: ambilError(r),
		Data: map[string]any{
			"Transaksi": transaksi,
			"Sukses":    ambilSukses(r),
		},
	})
}

// batalkanTransaksi memproses pembatalan transaksi (BR-14).
//
// Pembatalan berjalan dalam satu sql.Tx di store: transaksi menjadi BATAL,
// poin perolehan ditarik, poin tukar dikembalikan, dan kuota voucher kembali.
func (a *App) batalkanTransaksi(w http.ResponseWriter, r *http.Request) {
	id, err := idDariPath(r)
	if err != nil {
		pesanError(w, http.StatusBadRequest, "ID transaksi tidak valid", err.Error())
		return
	}

	if err := a.Store.BatalkanTransaksi(r.Context(), id); err != nil {
		switch {
		case errors.Is(err, store.ErrTransaksiTidakDitemukan):
			redirect(w, r, "/riwayat", "Transaksi tidak ditemukan.")
		case errors.Is(err, store.ErrTransaksiSudahBatal):
			redirect(w, r, "/riwayat/"+strconv.FormatInt(id, 10), "Transaksi ini sudah dibatalkan sebelumnya.")
		case errors.Is(err, store.ErrPoinSudahTerpakai):
			redirect(w, r, "/riwayat/"+strconv.FormatInt(id, 10),
				"Pembatalan ditolak: poin yang diperoleh dari transaksi ini sudah dipakai pada transaksi lain.")
		default:
			redirect(w, r, "/riwayat/"+strconv.FormatInt(id, 10), "Gagal membatalkan transaksi: "+err.Error())
		}
		return
	}

	redirectSukses(w, r, "/riwayat/"+strconv.FormatInt(id, 10),
		"Transaksi berhasil dibatalkan. Poin dan kuota voucher telah dikembalikan.")
}
