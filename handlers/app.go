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
//   - Aplikasi ini memakai autentikasi berbasis sesi: login dulu, lalu hak
//     akses tiap rute diperiksa di sisi server melalui a.akses(peran, ...).
//     Menyembunyikan menu di sidebar bukan pengaman; pengaman yang
//     sesungguhnya ada di middleware wajibPeran.
//   - Pembagian peran mengikuti tanggung jawab: admin mengelola master data
//     (produk, cabang, promo, member), laporan, dan akun staf; kasir
//     memproses transaksi, menerapkan promo, memakai poin, dan membatalkan
//     transaksi; member hanya melihat data keanggotaannya sendiri.
//   - Untuk menambah halaman baru cukup daftarkan satu baris mux.HandleFunc di
//     sini; router tidak memakai framework web eksternal (dilarang Gin/Echo).
func (a *App) Register(mux *http.ServeMux) {
	// Autentikasi. Halaman login bersifat publik; logout merusak sesi.
	a.registerAuth(mux)

	// Halaman dashboard. "{$}" berarti path persis "/" (bukan semua path).
	// Member diarahkan ke portalnya, kasir ke form transaksi (itu pekerjaannya),
	// sedangkan admin melihat ringkasan operasional.
	// Gate peran dipakai (bukan hanya withDB) supaya akun yang masih harus
	// mengganti kata sandi awal ikut tertahan di halaman ganti sandi.
	mux.HandleFunc("GET /{$}", a.akses("", a.halamanDashboard))

	// Data master: dibaca staf, diubah admin.
	a.registerCabang(mux)
	a.registerProduk(mux)
	a.registerMember(mux)
	a.registerPromo(mux)

	// Transaksi: operasional kasir (admin juga boleh-own).
	a.registerTransaksi(mux)
	a.registerRiwayat(mux)

	// Loyalty
	a.registerPoin(mux)

	// Laporan: khusus admin (pemilik).
	a.registerLaporan(mux)

	// Akun staf: khusus admin.
	a.registerPengguna(mux)

	// Portal member: khusus member.
	a.registerPortal(mux)

	// Akun saya: ganti kata sandi, terbuka untuk semua peran.
	a.registerAkun(mux)
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
