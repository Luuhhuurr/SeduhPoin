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
//
// Pola rute memakai sintaks http.ServeMux Go 1.22+: metode dan path digabung
// dalam satu string, dan bagian dinamis ditulis dengan {nama} yang dibaca
// lewat r.PathValue("nama") — misalnya "GET /riwayat/{id}".
//
// Catatan penting untuk pembicaraan dosen:
//   - Aplikasi ini SENGAJA tidak memakai login, sesi, atau autentikasi apa pun.
//     Sesuai spesifikasi tugas akhir hanya ada satu peran (kasir/admin), sehingga
//     seluruh menu dapat diakses langsung tanpa login.
//   - Untuk menambah halaman baru cukup daftarkan satu baris mux.HandleFunc di
//     sini; router tidak memakai framework web eksternal (dilarang Gin/Echo).
func (a *App) Register(mux *http.ServeMux) {
	// Halaman dashboard. "{$}" berarti path persis "/" (bukan semua path).
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
//
// Pola penanganan error di seluruh aplikasi: kesalahan ALWAYS ditulis ke
// respons HTTP (http.Error atau halaman pesan) dan server TETAP BERJALAN.
// Karena itu log.Fatal DILARANG dipakai di proyek ini — log.Fatal akan
// menghentikan proses server beserta seluruh koneksi database.
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
