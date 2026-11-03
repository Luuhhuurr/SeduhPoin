// Package handlers berisi seluruh HTTP handler aplikasi SeduhPoin.
package handlers

import (
	"net/http"

	"seduhpoin/internal/store"
)

// App adalah aplikasi web SeduhPoin.
type App struct {
	Store *store.Store
}

// New membuat instance aplikasi baru.
func New(s *store.Store) *App {
	return &App{Store: s}
}

// Register mendaftarkan seluruh rute aplikasi pada mux.
func (a *App) Register(mux *http.ServeMux) {
	// Halaman dashboard
	mux.HandleFunc("GET /{$}", a.withDB(a.dashboard))

	// Data master
	a.registerCabang(mux)
	a.registerProduk(mux)
	a.registerMember(mux)
	a.registerPromo(mux)

	// Transaksi
	a.registerTransaksi(mux)
	a.registerRiwayat(mux)

	// Loyalty
	a.registerPoin(mux)

	// Laporan
	a.registerLaporan(mux)
}

// withDB memastikan database dapat dijangkau sebelum handler dijalankan,
// sehingga pengguna melihat pesan yang jelas, bukan error teknis.
func (a *App) withDB(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if a.Store == nil || a.Store.DB == nil {
			pesanError(w, http.StatusServiceUnavailable, "Database belum siap",
				"Koneksi database belum dibuat. Periksa konfigurasi DB_DSN pada file .env.")
			return
		}
		if err := a.Store.Ping(r.Context()); err != nil {
			pesanError(w, http.StatusServiceUnavailable, "Database tidak terhubung",
				"MySQL XAMPP belum berjalan atau kredensial .env salah. Detail: "+err.Error())
			return
		}
		next(w, r)
	}
}
